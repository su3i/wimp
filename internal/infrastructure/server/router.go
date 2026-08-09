package server

import (
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/server/handlers"
	middleware "github.com/su3i/wimp/internal/infrastructure/server/middlewares"
)

func InitializeRouter() *gin.Engine {
	router := gin.Default()

	// Cors Settings - restricted to WEBURL (the frontend's own origin) plus the local Vite
	// dev server, rather than AllowAllOrigins. If WEBURL isn't set, only the dev server
	// origin is allowed - same-origin deployments don't need CORS headers at all.
	corsConfig := cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: false,
	}
	if webUrl := config.Common().WebUrl; webUrl != "" {
		corsConfig.AllowOrigins = append(corsConfig.AllowOrigins, webUrl)
	} else {
		log.Println("WARNING: WEBURL is not set, only http://localhost:5173 will be allowed cross-origin. Set WEBURL if the frontend is served from a different origin than the control plane.")
	}
	router.Use(cors.New(corsConfig))

	// Agent WebSocket (public - token is the credential)
	router.GET("/ws/agent", handlers.AgentWebSocket)

	router.GET("/ws", handlers.ClientWebSocket)

	// Bootstrap (public - token is the credential)
	router.GET("/bootstrap", handlers.Bootstrap)
	router.GET("/bootstrap/uninstall", handlers.Uninstall)

	router.GET("/health", handlers.Health)

	router.GET("/prometheus/targets", handlers.PrometheusTargets)
	router.GET("/prometheus/monitors", handlers.PrometheusMonitorTargets)

	router.GET("/config", handlers.RetrieveConfig)

	router.GET("/supported-languages", handlers.SupportedLanguages)
	router.PUT("/set-language", handlers.SetLanguagePreference)
	router.GET("/get-language", handlers.RetrieveLanguagePreference)

	// NewOrganization is public only for first-run setup (gated in the handler itself
	// once an organization exists). Everything else needs a real, authenticated caller.
	router.POST("/organization", handlers.NewOrganization)
	router.PUT("/organization", middleware.AuthMiddleware(), handlers.UpdateOrganization)
	router.GET("/organization", handlers.RetrieveOrganization)

	// NewAccount is public only for first-run setup (gated in the handler itself once
	// any account exists) - OptionalAuthMiddleware lets it recognize an authenticated
	// admin for every case after that, without aborting the bootstrap case.
	router.POST("/account", middleware.OptionalAuthMiddleware(), handlers.NewAccount)
	router.GET("/account", middleware.AuthMiddleware(), handlers.RetrieveAccountByUsername)
	router.PUT("/account", middleware.AuthMiddleware(), handlers.UpdateAccount)
	router.GET("/accounts", middleware.AuthMiddleware(), handlers.RetrieveAccounts)

	router.POST("/mfa/totp-uri", handlers.RetrieveTotpURI)
	router.POST("/mfa/confirm", handlers.ConfirmMFA)

	// Login and MFA are rate limited per IP - both are brute-forceable (password guessing,
	// TOTP code guessing) and were previously unbounded.
	router.POST("/auth/login", middleware.RateLimit("login", 10, 5*time.Minute), handlers.Login)
	router.POST("/auth/mfa", middleware.RateLimit("mfa", 10, 5*time.Minute), handlers.MFA)
	router.POST("/auth/refresh-token", handlers.RefreshToken)
	router.POST("/auth/revoke-token", handlers.RevokeToken)

	router.POST("/project", middleware.AuthMiddleware(), handlers.NewProject)
	router.GET("/project/:key", middleware.AuthMiddleware(), handlers.RetrieveProject)
	router.PUT("/project/:key", middleware.AuthMiddleware(), handlers.UpdateProject)
	router.DELETE("/project/:key", middleware.AuthMiddleware(), handlers.DeleteProject)
	router.GET("/projects", middleware.AuthMiddleware(), handlers.RetrieveProjects)

	router.POST("/projects/:key/machines", middleware.AuthMiddleware(), handlers.NewMachine)
	router.GET("/projects/:key/machines", middleware.AuthMiddleware(), handlers.RetrieveMachines)
	router.GET("/projects/:key/machines/:machineId/bootstrap", middleware.AuthMiddleware(), handlers.GetBootstrapToken)
	router.GET("/projects/:key/machines/:machineId/uninstall", middleware.AuthMiddleware(), handlers.GetUninstallCommand)
	router.DELETE("/projects/:key/machines/:machineId", middleware.AuthMiddleware(), handlers.DeleteMachine)
	router.POST("/projects/:key/machines/:machineId/shutdown", middleware.AuthMiddleware(), handlers.MachineCommand("shutdown"))
	router.POST("/projects/:key/machines/:machineId/restart", middleware.AuthMiddleware(), handlers.MachineCommand("restart"))
	router.POST("/projects/:key/machines/:machineId/agent/update", middleware.AuthMiddleware(), handlers.UpdateAgentCommand)
	router.GET("/projects/:key/machines/:machineId/logs/download", middleware.AuthMiddleware(), handlers.DownloadLogs)
	router.GET("/downloads/:token", middleware.AuthMiddleware(), handlers.FetchStagedDownload)

	router.POST("/projects/:key/applications", middleware.AuthMiddleware(), handlers.NewApplication)
	router.GET("/projects/:key/applications", middleware.AuthMiddleware(), handlers.RetrieveApplications)
	router.GET("/projects/:key/applications/:appId", middleware.AuthMiddleware(), handlers.RetrieveApplication)
	router.PUT("/projects/:key/applications/:appId", middleware.AuthMiddleware(), handlers.UpdateApplication)
	router.GET("/projects/:key/applications/:appId/app-pools", middleware.AuthMiddleware(), handlers.ListApplicationAppPools)
	router.POST("/projects/:key/applications/:appId/app-pools", middleware.AuthMiddleware(), handlers.AddAppPoolToApplication)
	router.DELETE("/projects/:key/applications/:appId/app-pools/:poolId", middleware.AuthMiddleware(), handlers.RemoveAppPoolFromApplication)
	router.PUT("/projects/:key/applications/:appId/app-pools/:poolId", middleware.AuthMiddleware(), handlers.UpdateAppPoolInApplication)
	router.GET("/projects/:key/applications/:appId/files", middleware.AuthMiddleware(), handlers.ListApplicationFiles)
	router.GET("/projects/:key/applications/:appId/logs", middleware.AuthMiddleware(), handlers.QueryLogs)
	router.DELETE("/projects/:key/applications/:appId/logs", middleware.AuthMiddleware(), handlers.ClearLogs)
	router.DELETE("/projects/:key/applications/:appId", middleware.AuthMiddleware(), handlers.DeleteApplication)

	router.GET("/projects/:key/machines/:machineId/app-pools", middleware.AuthMiddleware(), handlers.RetrieveAppPools)
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/start", middleware.AuthMiddleware(), handlers.AppPoolCommand("start"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/stop", middleware.AuthMiddleware(), handlers.AppPoolCommand("stop"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/restart", middleware.AuthMiddleware(), handlers.AppPoolCommand("restart"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/recycle", middleware.AuthMiddleware(), handlers.AppPoolCommand("recycle"))

	router.GET("/projects/:key/machines/:machineId/sites", middleware.AuthMiddleware(), handlers.RetrieveSites)
	router.POST("/projects/:key/machines/:machineId/sites/:siteId/start", middleware.AuthMiddleware(), handlers.SiteCommand("start"))
	router.POST("/projects/:key/machines/:machineId/sites/:siteId/stop", middleware.AuthMiddleware(), handlers.SiteCommand("stop"))
	router.POST("/projects/:key/machines/:machineId/sites/:siteId/restart", middleware.AuthMiddleware(), handlers.SiteCommand("restart"))

	router.GET("/projects/:key/dashboard/stats", middleware.AuthMiddleware(), handlers.DashboardStats)

	router.GET("/notifications", middleware.AuthMiddleware(), handlers.ListNotifications)
	router.GET("/notifications/unread-count", middleware.AuthMiddleware(), handlers.GetUnreadCount)
	router.PUT("/notifications/:id/read", middleware.AuthMiddleware(), handlers.MarkNotificationRead)
	router.PUT("/notifications/read-all", middleware.AuthMiddleware(), handlers.MarkAllNotificationsRead)
	router.GET("/projects/:key/notifications", middleware.AuthMiddleware(), handlers.ListProjectNotifications)

	handlers.MetricsHandler(router)

	return router
}
