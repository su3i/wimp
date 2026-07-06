package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	applicationService "github.com/su3i/wimp/internal/application/application"
	appPoolService "github.com/su3i/wimp/internal/application/apppool"
	machineService "github.com/su3i/wimp/internal/application/machine"
	notificationService "github.com/su3i/wimp/internal/application/notification"
	siteService "github.com/su3i/wimp/internal/application/site"
	"github.com/su3i/wimp/internal/cache"
	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// wsReadTimeout bounds how long a machine can go silent before it's treated as
// disconnected - 3x the agent's 30s heartbeat interval, refreshed on every received
// message. Without this, a hard crash/network black-hole is only ever detected
// whenever the raw OS/network layer itself notices the TCP connection is dead, which
// is unbounded (can take minutes to hours).
const wsReadTimeout = 90 * time.Second

func AgentWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	repo := database.NewMachineRepository(config.Database())

	m, err := repo.FindOneByToken(token)
	if err != nil || m == nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	if time.Now().After(m.TokenExpiresAt) {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer conn.Close()
	conn.SetReadDeadline(time.Now().Add(wsReadTimeout)) //nolint:errcheck

	// Mark online and extend token validity so reconnects always work.
	now := time.Now()
	m.Status = machineDomain.Online
	m.LastSeenAt = &now
	m.TokenExpiresAt = now.Add(100 * 365 * 24 * time.Hour)
	if err := repo.Update(m); err != nil {
		log.Printf("failed to mark machine online: %v", err)
		return
	}

	hub.Get().Register(m.ID, conn)
	log.Printf("machine (%d) agent connected", m.ID)

	cfg := config.Database()
	go notificationService.EmitAlert(notification.AlertMachineConnected, m.ProjectID, m.ID, m.Hostname,
		m.Hostname+" — Machine Online", m.Hostname+" came online", cfg)

	defer func() {
		hub.Get().Deregister(m.ID)
		if m.Status == machineDomain.Deleting {
			if err := machineService.HardDelete(m.ID, cfg); err != nil {
				log.Printf("machine (%d) hard delete failed: %v", m.ID, err)
			}
			return
		}
		m.Status = machineDomain.Offline
		repo.Update(m)
		log.Printf("machine (%d) agent disconnected", m.ID)
		if action, ok := cache.GetMachineActionPending(m.ID); ok {
			cache.ClearMachineActionPending(m.ID)
			if action == "shutdown" {
				go notificationService.EmitAlert(notification.AlertMachineShutdown, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — Machine Shut Down", m.Hostname+" was shut down", cfg)
			} else {
				go notificationService.EmitAlert(notification.AlertMachineRestarting, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — Machine Restarting", m.Hostname+" is restarting", cfg)
			}
		} else {
			go notificationService.EmitAlert(notification.AlertMachineDisconnected, m.ProjectID, m.ID, m.Hostname,
				m.Hostname+" — Machine Offline", m.Hostname+" went offline unexpectedly", cfg)
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		conn.SetReadDeadline(time.Now().Add(wsReadTimeout)) //nolint:errcheck

		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case protocol.TypeRegister:
			var reg protocol.RegisterPayload
			if err := json.Unmarshal(msg.Payload, &reg); err == nil {
				prevVersion := m.AgentVersion
				m.Hostname = reg.Hostname
				m.IPs = reg.IPs
				m.AgentVersion = reg.Version
				if reg.WindowsVersion != "" {
					m.WindowsVersion = reg.WindowsVersion
				}
				if reg.Version != "" && reg.Version != "dev" && reg.Version != prevVersion {
					go notificationService.EmitAlert(notification.AlertAgentUpdated, m.ProjectID, m.ID, m.Hostname,
						m.Hostname+" — Agent Updated", m.Hostname+" agent updated to "+reg.Version, cfg)
				}
			}
			repo.Update(m)

			ack, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterAck})
			conn.WriteMessage(websocket.TextMessage, ack)

			go func(machineID uint) {
				if err := applicationService.PushFluentConfig(machineID, config.Database()); err != nil {
					log.Printf("machine (%d) fluent config push: %v", machineID, err)
				}
			}(m.ID)

			if reg.Version != "dev" && reg.Version != config.Common().AgentVersion && shouldAutoUpdate(m.ProjectID) {
				go func(machineID uint, reportedVersion string) {
					log.Printf("machine (%d) agent version %q is outdated (latest %q) - pushing auto-update", machineID, reportedVersion, config.Common().AgentVersion)
					if err := pushAgentUpdate(machineID); err != nil {
						log.Printf("machine (%d) agent auto-update push failed: %v", machineID, err)
					}
				}(m.ID, reg.Version)
			}

		case protocol.TypeDiscovery:
			var disc protocol.DiscoveryPayload
			if err := json.Unmarshal(msg.Payload, &disc); err != nil {
				log.Printf("machine (%d) bad discovery payload: %v", m.ID, err)
				continue
			}
			if err := appPoolService.UpsertFromDiscovery(m.ID, disc.AppPools, cfg); err != nil {
				log.Printf("machine (%d) app pool upsert failed: %v", m.ID, err)
			}
			if err := siteService.UpsertFromDiscovery(m.ID, disc.Sites, cfg); err != nil {
				log.Printf("machine (%d) site upsert failed: %v", m.ID, err)
			}

		case protocol.TypeHeartbeat:
			var hb protocol.HeartbeatPayload
			if err := json.Unmarshal(msg.Payload, &hb); err != nil {
				continue
			}
			t := time.Now()
			m.LastSeenAt = &t
			repo.Update(m)
			stoppedPools, startedPools, _ := appPoolService.SyncHeartbeat(m.ID, hb.AppPools, cfg)
			cache.InvalidatePoolsByMachine(m.ID)
			stoppedSites, startedSites, _ := siteService.SyncHeartbeat(m.ID, hb.Sites, cfg)
			cache.InvalidateSitesByMachine(m.ID)
			for _, name := range stoppedPools {
				go notificationService.EmitAlert(notification.AlertAppPoolStopped, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — App Pool Stopped: "+name, name+" stopped on "+m.Hostname, cfg)
			}
			for _, name := range startedPools {
				go notificationService.EmitAlert(notification.AlertAppPoolStarted, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — App Pool Started: "+name, name+" started on "+m.Hostname, cfg)
			}
			for _, name := range stoppedSites {
				go notificationService.EmitAlert(notification.AlertSiteStopped, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — Site Stopped: "+name, name+" stopped on "+m.Hostname, cfg)
			}
			for _, name := range startedSites {
				go notificationService.EmitAlert(notification.AlertSiteStarted, m.ProjectID, m.ID, m.Hostname,
					m.Hostname+" — Site Started: "+name, name+" started on "+m.Hostname, cfg)
			}

			if prevWE, prevFB, known := cache.GetSidecarHealth(m.ID); known {
				if prevWE && !hb.WindowsExporterHealthy {
					go notificationService.EmitAlert(notification.AlertWindowsExporterDown, m.ProjectID, m.ID, m.Hostname,
						m.Hostname+" — windows_exporter Down", "windows_exporter service is not running on "+m.Hostname, cfg)
				} else if !prevWE && hb.WindowsExporterHealthy {
					go notificationService.EmitAlert(notification.AlertWindowsExporterUp, m.ProjectID, m.ID, m.Hostname,
						m.Hostname+" — windows_exporter Recovered", "windows_exporter service is running again on "+m.Hostname, cfg)
				}
				if prevFB && !hb.FluentBitHealthy {
					go notificationService.EmitAlert(notification.AlertFluentBitDown, m.ProjectID, m.ID, m.Hostname,
						m.Hostname+" — fluent-bit Down", "fluent-bit service is not running on "+m.Hostname, cfg)
				} else if !prevFB && hb.FluentBitHealthy {
					go notificationService.EmitAlert(notification.AlertFluentBitUp, m.ProjectID, m.ID, m.Hostname,
						m.Hostname+" — fluent-bit Recovered", "fluent-bit service is running again on "+m.Hostname, cfg)
				}
			}
			cache.SetSidecarHealth(m.ID, hb.WindowsExporterHealthy, hb.FluentBitHealthy)

		case protocol.TypeCommandResult:
			var result protocol.CommandResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err != nil {
				continue
			}
			hub.ResolveCommand(result.CommandID, hub.CommandResult{
				Success: result.Success,
				Output:  result.Output,
				Error:   result.Error,
			})

		case protocol.TypeListFilesResult:
			var result protocol.ListFilesResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err != nil {
				continue
			}
			filesJSON, _ := json.Marshal(result.Files)
			hub.ResolveCommand(result.RequestID, hub.CommandResult{
				Success: result.Error == "",
				Output:  string(filesJSON),
				Error:   result.Error,
			})

		case protocol.TypeDownloadLogsResult:
			var result protocol.DownloadLogsResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err != nil {
				continue
			}
			hub.ResolveCommand(result.RequestID, hub.CommandResult{
				Success: result.Error == "",
				Output:  result.Data,
				Error:   result.Error,
			})

		case protocol.TypeClearLogsResult:
			var result protocol.ClearLogsResultPayload
			if err := json.Unmarshal(msg.Payload, &result); err != nil {
				continue
			}
			hub.ResolveCommand(result.RequestID, hub.CommandResult{
				Success: result.Error == "",
				Error:   result.Error,
			})
		}
	}
}
