package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	appPoolService "github.com/su3i/wimp/internal/application/apppool"
	siteService "github.com/su3i/wimp/internal/application/site"
	"github.com/su3i/wimp/internal/config"
	machineDomain "github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

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

	defer func() {
		hub.Get().Deregister(m.ID)
		m.Status = machineDomain.Offline
		repo.Update(m)
		log.Printf("machine (%d) agent disconnected", m.ID)
	}()

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case protocol.TypeRegister:
			var reg protocol.RegisterPayload
			if err := json.Unmarshal(msg.Payload, &reg); err == nil {
				m.Hostname = reg.Hostname
				m.IPs = reg.IPs
			}
			repo.Update(m)

			ack, _ := json.Marshal(protocol.Message{Type: protocol.TypeRegisterAck})
			conn.WriteMessage(websocket.TextMessage, ack)

		case protocol.TypeDiscovery:
			var disc protocol.DiscoveryPayload
			if err := json.Unmarshal(msg.Payload, &disc); err != nil {
				log.Printf("machine (%d) bad discovery payload: %v", m.ID, err)
				continue
			}
			cfg := config.Database()
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
			cfg := config.Database()
			appPoolService.SyncHeartbeat(m.ID, hb.AppPools, cfg)
			siteService.SyncHeartbeat(m.ID, hb.Sites, cfg)

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
		}
	}
}
