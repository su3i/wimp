package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	applicationService "github.com/su3i/wimp/internal/application/application"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func ClearLogs(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "write")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	appId, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	machineIDStr := c.Query("machine_id")
	if machineIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "machine_id is required"})
		return
	}

	machineID, err := strconv.ParseUint(machineIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine_id"})
		return
	}

	appPoolIDStr := c.Query("pool_id")
	filename := c.Query("filename")

	detail, err := applicationService.GetDetail(uint(appId), projectKey, config.Database())
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// ── 1. Delete from Loki ───────────────────────────────────────────────────

	labels := []string{
		`job="wimp"`,
		fmt.Sprintf(`application_id="%d"`, appId),
		fmt.Sprintf(`machine_id="%s"`, machineIDStr),
	}
	if appPoolIDStr != "" {
		labels = append(labels, fmt.Sprintf(`pool_id="%s"`, appPoolIDStr))
	}
	if filename != "" {
		escaped := strings.ReplaceAll(filename, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		labels = append(labels, fmt.Sprintf(`filename="%s"`, escaped))
	}
	query := "{" + strings.Join(labels, ", ") + "}"

	cfg := config.Common()
	scheme := "http"
	if cfg.LokiTlsEnabled {
		scheme = "https"
	}
	lokiURL := fmt.Sprintf("%s://%s:%s/loki/api/v1/delete", scheme, cfg.LokiHost, cfg.LokiPort)

	lokiReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, lokiURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build loki request"})
		return
	}
	q := lokiReq.URL.Query()
	q.Set("query", query)
	q.Set("start", "1970-01-01T00:00:00Z")
	lokiReq.URL.RawQuery = q.Encode()

	lokiClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.LokiTlsSkipVerify}, //nolint:gosec
		},
	}
	lokiResp, err := lokiClient.Do(lokiReq)
	if err != nil {
		log.Printf("loki delete failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach loki"})
		return
	}
	defer lokiResp.Body.Close()

	if lokiResp.StatusCode >= 400 {
		body, _ := io.ReadAll(lokiResp.Body)
		log.Printf("loki delete error %d: %s", lokiResp.StatusCode, body)
		c.JSON(http.StatusBadGateway, gin.H{"error": "loki rejected the delete request, ensure compactor.deletion-mode is enabled"})
		return
	}

	// ── 2. Delete files from disk via agent ───────────────────────────────────

	// Collect unique log directories for this machine (optionally scoped to one pool).
	seen := map[string]bool{}
	var logPaths []string
	for _, pool := range detail.AppPools {
		if pool.Machine.ID != uint(machineID) {
			continue
		}
		if appPoolIDStr != "" {
			poolID, _ := strconv.ParseUint(appPoolIDStr, 10, 64)
			if pool.ID != uint(poolID) {
				continue
			}
		}
		if pool.LogPath == nil || *pool.LogPath == "" {
			continue
		}
		if !seen[*pool.LogPath] {
			seen[*pool.LogPath] = true
			logPaths = append(logPaths, *pool.LogPath)
		}
	}

	if len(logPaths) > 0 && hub.Get().IsOnline(uint(machineID)) {
		for _, logPath := range logPaths {
			reqID := uuid.New().String()
			ch := hub.RegisterCommand(reqID)

			payload, _ := json.Marshal(protocol.ClearLogsPayload{RequestID: reqID, LogPath: logPath})
			msg, _ := json.Marshal(protocol.Message{Type: protocol.TypeClearLogs, Payload: json.RawMessage(payload)})

			if err := hub.Get().Send(uint(machineID), msg); err != nil {
				hub.DeregisterCommand(reqID)
				continue
			}

			ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
			select {
			case <-ch:
			case <-ctx.Done():
				log.Printf("clear logs timeout for machine %d path %s", machineID, logPath)
			}
			cancel()
			hub.DeregisterCommand(reqID)
		}
	}

	c.Status(http.StatusNoContent)
}

func QueryLogs(c *gin.Context) {
	projectKey := c.Param("key")

	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	appId, err := strconv.ParseUint(c.Param("appId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid application id"})
		return
	}

	if _, err := applicationService.GetDetail(uint(appId), projectKey, config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	startStr := c.DefaultQuery("start", now.Add(-1*time.Hour).Format(time.RFC3339))
	endStr := c.DefaultQuery("end", now.Format(time.RFC3339))
	limitStr := c.DefaultQuery("limit", "100")
	direction := c.DefaultQuery("direction", "backward")
	machineID := c.Query("machine_id")
	appPoolID := c.Query("pool_id")
	filename := c.Query("filename")

	labels := []string{
		`job="wimp"`,
		fmt.Sprintf(`application_id="%d"`, appId),
	}
	if machineID != "" {
		labels = append(labels, fmt.Sprintf(`machine_id="%s"`, machineID))
	}
	if appPoolID != "" {
		labels = append(labels, fmt.Sprintf(`pool_id="%s"`, appPoolID))
	}
	if filename != "" {
		escaped := strings.ReplaceAll(filename, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		labels = append(labels, fmt.Sprintf(`filename="%s"`, escaped))
	}
	query := "{" + strings.Join(labels, ", ") + "}"

	cfg := config.Common()
	scheme := "http"
	if cfg.LokiTlsEnabled {
		scheme = "https"
	}
	lokiURL := fmt.Sprintf("%s://%s:%s/loki/api/v1/query_range", scheme, cfg.LokiHost, cfg.LokiPort)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, lokiURL, nil)
	if err != nil {
		log.Printf("loki request build failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build loki request"})
		return
	}

	q := req.URL.Query()
	q.Set("query", query)
	q.Set("start", startStr)
	q.Set("end", endStr)
	q.Set("limit", limitStr)
	q.Set("direction", direction)
	req.URL.RawQuery = q.Encode()

	lokiClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.LokiTlsSkipVerify}, //nolint:gosec
		},
	}
	resp, err := lokiClient.Do(req)
	if err != nil {
		log.Printf("loki query failed: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to query loki"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read loki response"})
		return
	}

	c.Data(resp.StatusCode, "application/json", body)
}
