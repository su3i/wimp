package server

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/infrastructure/server/handlers"
	middleware "github.com/su3i/wimp/internal/infrastructure/server/middlewares"
)

func InitializeRouter() *gin.Engine {
	router := gin.Default()

	// Cors Settings
	router.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: false,
	}))

	// Agent WebSocket (public — token is the credential)
	router.GET("/ws/agent", handlers.AgentWebSocket)

	// Bootstrap (public — token is the credential)
	router.GET("/bootstrap", handlers.Bootstrap)
	router.GET("/bootstrap/uninstall", handlers.Uninstall)

	// Health
	router.GET("/health", handlers.Health)

	// Config
	router.GET("/config", handlers.RetrieveConfig)

	// Language Settings
	router.GET("/supported-languages", handlers.SupportedLanguages)
	router.PUT("/set-language", handlers.SetLanguagePreference)
	router.GET("/get-language", handlers.RetrieveLanguagePreference)

	// Organization
	router.POST("/organization", handlers.NewOrganization)
	router.PUT("/organization", handlers.UpdateOrganization)
	router.GET("/organization", handlers.RetrieveOrganization)

	// Account
	router.POST("/account", handlers.NewAccount)
	router.GET("/account", handlers.RetrieveAccountByEmail)
	router.PUT("/account", handlers.UpdateAccount)
	router.GET("/accounts", middleware.AuthMiddleware(), handlers.RetrieveAccounts)

	// MFA
	router.POST("/mfa/totp-uri", handlers.RetrieveTotpURI)
	router.POST("/mfa/confirm", handlers.ConfirmMFA)

	// Auth
	router.POST("/auth/login", handlers.Login)
	router.POST("/auth/mfa", handlers.MFA)
	router.POST("/auth/refresh-token", handlers.RefreshToken)
	router.POST("/auth/revoke-token", handlers.RevokeToken)

	// Project
	router.POST("/project", middleware.AuthMiddleware(), handlers.NewProject)
	router.GET("/project/:key", middleware.AuthMiddleware(), handlers.RetrieveProject)
	router.PUT("/project/:key", middleware.AuthMiddleware(), handlers.UpdateProject)
	router.GET("/projects", middleware.AuthMiddleware(), handlers.RetrieveProjects)

	// Machine (project-scoped)
	router.POST("/projects/:key/machines", middleware.AuthMiddleware(), handlers.NewMachine)
	router.GET("/projects/:key/machines", middleware.AuthMiddleware(), handlers.RetrieveMachines)
	router.GET("/projects/:key/machines/:machineId/bootstrap", middleware.AuthMiddleware(), handlers.GetBootstrapToken)
	router.GET("/projects/:key/machines/:machineId/uninstall", middleware.AuthMiddleware(), handlers.GetUninstallCommand)

	// Applications (project-scoped)
	router.POST("/projects/:key/applications", middleware.AuthMiddleware(), handlers.NewApplication)
	router.GET("/projects/:key/applications", middleware.AuthMiddleware(), handlers.RetrieveApplications)
	router.GET("/projects/:key/applications/:appId", middleware.AuthMiddleware(), handlers.RetrieveApplication)
	router.POST("/projects/:key/applications/:appId/app-pools", middleware.AuthMiddleware(), handlers.AddAppPoolToApplication)

	// App Pools (machine-scoped)
	router.GET("/projects/:key/machines/:machineId/app-pools", middleware.AuthMiddleware(), handlers.RetrieveAppPools)
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/start", middleware.AuthMiddleware(), handlers.AppPoolCommand("start"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/stop", middleware.AuthMiddleware(), handlers.AppPoolCommand("stop"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/restart", middleware.AuthMiddleware(), handlers.AppPoolCommand("restart"))
	router.POST("/projects/:key/machines/:machineId/app-pools/:poolId/recycle", middleware.AuthMiddleware(), handlers.AppPoolCommand("recycle"))

	// Metrics
	handlers.MetricsHandler(router)

	return router
}
