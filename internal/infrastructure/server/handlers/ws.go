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

	// Mark online. The token was already issued with a never-expire TTL in NewMachine,
	// so there's nothing to extend here.
	now := time.Now()
	m.Status = machineDomain.Online
	m.LastSeenAt = &now
	if err := repo.Update(m); err != nil {
		log.Printf("failed to mark machine online: %v", err)
		return
	}

	hub.Get().Register(m.ID, conn)
	log.Printf("machine (%d) agent connected", m.ID)

	cfg := config.Database()
	go notificationService.EmitAlert(notification.AlertMachineConnected, m.ProjectID, m.ID, m.Hostname, "",
		notificationService.AlertTitle(m.Hostname, "Machine Online"), m.Hostname+" came online", cfg)

	defer func() {
		hub.Get().Deregister(m.ID)

		// Re-read rather than trusting this loop's copy: the row's status can have been
		// changed since the connection opened (deletion requested from the API, or the
		// row retired because the same box was re-bootstrapped elsewhere), and those are
		// terminal states that a blanket flip to offline would silently undo.
		if current, err := repo.FindOneByID(m.ID); err == nil && current != nil {
			m.Status = current.Status
		}

		if m.Status == machineDomain.Deleting {
			if err := machineService.HardDelete(m.ID, cfg); err != nil {
				log.Printf("machine (%d) hard delete failed: %v", m.ID, err)
			}
			return
		}
		if m.Status == machineDomain.Reassigned {
			// The host moved on deliberately; it is not an outage and the reassignment
			// alert has already been emitted by whichever connection superseded this one.
			log.Printf("machine (%d) agent disconnected after being reassigned", m.ID)
			return
		}
		m.Status = machineDomain.Offline
		repo.Update(m)
		log.Printf("machine (%d) agent disconnected", m.ID)
		if action, ok := cache.GetMachineActionPending(m.ID); ok {
			cache.ClearMachineActionPending(m.ID)
			if action == "shutdown" {
				go notificationService.EmitAlert(notification.AlertMachineShutdown, m.ProjectID, m.ID, m.Hostname, "",
					notificationService.AlertTitle(m.Hostname, "Machine Shut Down"), m.Hostname+" was shut down", cfg)
			} else {
				go notificationService.EmitAlert(notification.AlertMachineRestarting, m.ProjectID, m.ID, m.Hostname, "",
					notificationService.AlertTitle(m.Hostname, "Machine Restarting"), m.Hostname+" is restarting", cfg)
			}
		} else {
			go notificationService.EmitAlert(notification.AlertMachineDisconnected, m.ProjectID, m.ID, m.Hostname, "",
				notificationService.AlertTitle(m.Hostname, "Machine Offline"), m.Hostname+" went offline unexpectedly", cfg)
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
				if reg.MachineUID != "" {
					m.MachineUID = reg.MachineUID
				}
				// Scoped column update rather than a full-row save: the row may be
				// concurrently marked reassigned or deleting by another connection's
				// reconcile, and a full save of this loop's copy would revert it.
				if err := repo.SetIdentity(m.ID, m.Hostname, m.IPs, m.AgentVersion, m.WindowsVersion, m.MachineUID); err != nil {
					log.Printf("machine (%d) identity update failed: %v", m.ID, err)
				}
				if reg.Version != "" && reg.Version != "dev" && reg.Version != prevVersion {
					go notificationService.EmitAlert(notification.AlertAgentUpdated, m.ProjectID, m.ID, m.Hostname, "",
						notificationService.AlertTitle(m.Hostname, "Agent Updated"), m.Hostname+" agent updated to "+reg.Version, cfg)
				}

				// Retire any row describing this same physical box. Runs after the
				// identity write above so the hostname/UID being matched on is the one
				// this agent just reported, not whatever the row held beforehand.
				for _, old := range machineService.ReconcileReassignment(m, hub.Get().IsOnline, cfg) {
					log.Printf("machine (%d) supersedes machine (%d) - same host re-bootstrapped", m.ID, old.ID)
					name := old.Hostname
					if name == "" {
						name = m.Hostname
					}
					go notificationService.EmitAlert(notification.AlertMachineReassigned, old.ProjectID, old.ID, name, "",
						notificationService.AlertTitle(name, "Host Reassigned"),
						name+" was re-bootstrapped and now reports to another host entry. This entry is stale and will not come back online.", cfg)
				}
			}

			ack, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterAck})
			// Route through the hub's mutex-guarded write instead of writing to conn
			// directly - this read loop already races against hub.Get().Send() calls
			// made from HTTP command handlers on other goroutines.
			if err := hub.Get().Send(m.ID, ack); err != nil {
				log.Printf("machine (%d) register ack send failed: %v", m.ID, err)
			}

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
				go notificationService.EmitAlert(notification.AlertAppPoolStopped, m.ProjectID, m.ID, m.Hostname, name,
					notificationService.AlertTitle(m.Hostname, "App Pool Stopped: "+name), name+" stopped on "+m.Hostname, cfg)
			}
			for _, name := range startedPools {
				go notificationService.EmitAlert(notification.AlertAppPoolStarted, m.ProjectID, m.ID, m.Hostname, name,
					notificationService.AlertTitle(m.Hostname, "App Pool Started: "+name), name+" started on "+m.Hostname, cfg)
			}
			for _, name := range stoppedSites {
				go notificationService.EmitAlert(notification.AlertSiteStopped, m.ProjectID, m.ID, m.Hostname, name,
					notificationService.AlertTitle(m.Hostname, "Site Stopped: "+name), name+" stopped on "+m.Hostname, cfg)
			}
			for _, name := range startedSites {
				go notificationService.EmitAlert(notification.AlertSiteStarted, m.ProjectID, m.ID, m.Hostname, name,
					notificationService.AlertTitle(m.Hostname, "Site Started: "+name), name+" started on "+m.Hostname, cfg)
			}

			if prevWE, prevFB, known := cache.GetSidecarHealth(m.ID); known {
				if prevWE && !hb.WindowsExporterHealthy {
					go notificationService.EmitAlert(notification.AlertWindowsExporterDown, m.ProjectID, m.ID, m.Hostname, "",
						notificationService.AlertTitle(m.Hostname, "windows_exporter Down"), "windows_exporter service is not running on "+m.Hostname, cfg)
				} else if !prevWE && hb.WindowsExporterHealthy {
					go notificationService.EmitAlert(notification.AlertWindowsExporterUp, m.ProjectID, m.ID, m.Hostname, "",
						notificationService.AlertTitle(m.Hostname, "windows_exporter recovered"), "windows_exporter service is running again on "+m.Hostname, cfg)
				}
				if prevFB && !hb.FluentBitHealthy {
					go notificationService.EmitAlert(notification.AlertFluentBitDown, m.ProjectID, m.ID, m.Hostname, "",
						notificationService.AlertTitle(m.Hostname, "fluent-bit Down"), "fluent-bit service is not running on "+m.Hostname, cfg)
				} else if !prevFB && hb.FluentBitHealthy {
					go notificationService.EmitAlert(notification.AlertFluentBitUp, m.ProjectID, m.ID, m.Hostname, "",
						notificationService.AlertTitle(m.Hostname, "fluent-bit recovered"), "fluent-bit service is running again on "+m.Hostname, cfg)
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
