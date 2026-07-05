package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	machineService "github.com/su3i/wimp/internal/application/machine"
	"github.com/su3i/wimp/internal/cache"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/protocol"
	"github.com/su3i/wimp/internal/hub"
	"github.com/su3i/wimp/internal/infrastructure/database"
)

const downloadLogsTimeout = 120 * time.Second

func DownloadLogs(c *gin.Context) {
	projectKey := c.Param("key")

	machineID, err := strconv.ParseUint(c.Param("machineId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid machine id"})
		return
	}

	logPath := c.Query("path")
	if logPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path query param is required"})
		return
	}

	// Validate machine belongs to this project (reuse bootstrap lookup).
	if _, _, err := machineService.GetBootstrapToken(uint(machineID), projectKey, "", "", config.Database()); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	cfg := config.Database()
	machine, err := database.NewMachineRepository(cfg).FindOneByID(uint(machineID))
	if err != nil || machine == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "machine not found"})
		return
	}

	if !hub.Get().IsOnline(uint(machineID)) {
		c.JSON(http.StatusConflict, gin.H{"error": "machine is offline"})
		return
	}

	reqID := uuid.New().String()
	ch := hub.RegisterCommand(reqID)
	defer hub.DeregisterCommand(reqID)

	payload, _ := json.Marshal(protocol.DownloadLogsPayload{
		RequestID: reqID,
		LogPath:   logPath,
	})
	msg, _ := json.Marshal(protocol.Message{
		Type:    protocol.TypeDownloadLogs,
		Payload: json.RawMessage(payload),
	})

	if err := hub.Get().Send(uint(machineID), msg); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "failed to reach agent"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), downloadLogsTimeout)
	defer cancel()

	select {
	case res := <-ch:
		if !res.Success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": res.Error})
			return
		}
		data, err := base64.StdEncoding.DecodeString(res.Output)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode archive"})
			return
		}
		hostname := strings.ToLower(machine.Hostname)
		if hostname == "" {
			hostname = strconv.FormatUint(machineID, 10)
		}
		fileName := "logs-" + hostname + ".zip"

		tmpFile, err := os.CreateTemp("", "wimp-logs-*.zip")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stage archive"})
			return
		}
		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stage archive"})
			return
		}
		tmpFile.Close()

		token := uuid.New().String()
		cache.StageDownload(token, tmpFile.Name(), fileName, int64(len(data)))

		c.JSON(http.StatusOK, gin.H{
			"message":   "success",
			"token":     token,
			"file_name": fileName,
			"file_size": len(data),
		})
	case <-ctx.Done():
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "agent did not respond in time"})
	}
}

// FetchStagedDownload serves a previously staged log archive by its one-time token,
// then deletes it - the temp file is not meant to be fetched twice.
func FetchStagedDownload(c *gin.Context) {
	token := c.Param("token")

	staged, ok := cache.GetStagedDownload(token)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "download expired or not found"})
		return
	}

	data, err := os.ReadFile(staged.Path)
	cache.ConsumeStagedDownload(token)
	os.Remove(staged.Path) //nolint:errcheck
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read staged archive"})
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+staged.FileName+`"`)
	c.Data(http.StatusOK, "application/zip", data)
}
