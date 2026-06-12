package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	authorizationService "github.com/su3i/wimp/internal/application/authorization"
	notificationService "github.com/su3i/wimp/internal/application/notification"
	uptimeService "github.com/su3i/wimp/internal/application/uptime"
	"github.com/su3i/wimp/internal/config"
	authorizationDomain "github.com/su3i/wimp/internal/domain/authorization"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/server/utils"
)

func DashboardStats(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	cfg := config.Database()

	activeAlerts, err := notificationService.ActiveAlerts(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	unread, err := notificationService.UnreadCount(cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"active_alerts":         len(activeAlerts),
		"unread_notifications":  unread,
	})
}

func DashboardActiveAlerts(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	alerts, err := notificationService.ActiveAlerts(config.Database())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": alerts})
}

func DashboardAlertHistory(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	history, err := notificationService.AlertHistory(hours, config.Database())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"history": history})
}

func DashboardUptime(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	allMachines, err := database.NewMachineRepository(config.Database()).FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	machineInputs := make([]struct {
		ID       uint
		Hostname string
	}, len(*allMachines))
	for i, m := range *allMachines {
		machineInputs[i].ID = m.ID
		machineInputs[i].Hostname = m.Hostname
	}

	stats, err := uptimeService.GetAllMachineStats(days, machineInputs, config.Database())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"uptime": stats})
}

type prometheusResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type prometheusData struct {
	ResultType string             `json:"resultType"`
	Result     []prometheusResult `json:"result"`
}

type prometheusResponse struct {
	Status string         `json:"status"`
	Data   prometheusData `json:"data"`
}

func queryPrometheus(promURL, query string) ([]prometheusResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/query?query=%s", promURL, query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pr prometheusResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return nil, err
	}
	return pr.Data.Result, nil
}

func DashboardMachines(c *gin.Context) {
	allow, err := authorizationService.EnforceRoles(utils.GetUserRolesFromContext(c), authorizationDomain.AuthorizationDomainProject, authorizationDomain.Project, "read")
	if err != nil || !allow {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	allMachines, err := database.NewMachineRepository(config.Database()).FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	machines := *allMachines

	promURL := config.Common().PrometheusUrl
	type machineMetrics struct {
		ID       uint    `json:"id"`
		Hostname string  `json:"hostname"`
		Status   string  `json:"status"`
		CPUPct   float64 `json:"cpu_pct"`
		RAMPct   float64 `json:"ram_pct"`
		DiskPct  float64 `json:"disk_pct"`
	}

	result := make([]machineMetrics, 0, len(machines))
	for _, m := range machines {
		mm := machineMetrics{
			ID:       m.ID,
			Hostname: m.Hostname,
			Status:   string(m.Status),
		}

		if promURL != "" {
			machineIDStr := fmt.Sprintf("%d", m.ID)

			parseMetric := func(results []prometheusResult) float64 {
				if len(results) == 0 || len(results[0].Value) < 2 {
					return 0
				}
				v, _ := strconv.ParseFloat(fmt.Sprintf("%v", results[0].Value[1]), 64)
				return v
			}

			cpuResults, _ := queryPrometheus(promURL,
				fmt.Sprintf(`100 - (avg by (machine_id) (rate(windows_cpu_time_total{mode="idle",machine_id="%s"}[5m])) * 100)`, machineIDStr))
			mm.CPUPct = parseMetric(cpuResults)

			ramResults, _ := queryPrometheus(promURL,
				fmt.Sprintf(`100 * (1 - windows_memory_available_bytes{machine_id="%s"} / windows_cs_physical_memory_bytes{machine_id="%s"})`, machineIDStr, machineIDStr))
			mm.RAMPct = parseMetric(ramResults)

			diskResults, _ := queryPrometheus(promURL,
				fmt.Sprintf(`100 * (1 - windows_logical_disk_free_bytes{volume="C:",machine_id="%s"} / windows_logical_disk_size_bytes{volume="C:",machine_id="%s"})`, machineIDStr, machineIDStr))
			mm.DiskPct = parseMetric(diskResults)
		}

		result = append(result, mm)
	}

	c.JSON(http.StatusOK, gin.H{"machines": result})
}
