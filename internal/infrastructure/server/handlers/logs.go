package handlers

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	applicationService "github.com/su3i/wimp/internal/application/application"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

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
	filename := c.Query("filename")

	labels := []string{
		`job="wimp"`,
		fmt.Sprintf(`application_id="%d"`, appId),
	}
	if machineID != "" {
		labels = append(labels, fmt.Sprintf(`machine_id="%s"`, machineID))
	}
	if filename != "" {
		escaped := strings.ReplaceAll(filename, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		labels = append(labels, fmt.Sprintf(`filename=~".*%s.*"`, escaped))
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
