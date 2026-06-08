package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/su3i/wimp/internal/application/authorization"
	"github.com/su3i/wimp/internal/application/metadata"
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/infrastructure/database"
	"github.com/su3i/wimp/internal/infrastructure/server"

	accountService "github.com/su3i/wimp/internal/application/account"
	organizationService "github.com/su3i/wimp/internal/application/organization"
	projectService "github.com/su3i/wimp/internal/application/project"
	"github.com/su3i/wimp/internal/domain/account"
	organizationDomain "github.com/su3i/wimp/internal/domain/organization"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Failed to load env: %v", err)
	}

	// Load config
	config.Initialize()

	// Initialize database
	database.Initialize(config.Database())

	// Run database migrations
	database.Migrate(config.Database())

	// Load bootstrap token
	metadata.LoadBootstrapToken(config.Common().BootstrapToken, config.Database())

	// Seed default account
	if _, err := accountService.NewAccount("Administrator", config.Common().DefaultAdminEmail, config.Common().DefaultAdminPassword, account.SuperAdmin, config.Database()); err != nil {
		log.Printf("Seed account: %v", err)
	}

	// Seed default organization
	if _, err := organizationService.NewOrganization("Default", "default", string(organizationDomain.Public), config.Database()); err != nil {
		log.Printf("Seed organization: %v", err)
	}

	// Seed default project
	if _, err := projectService.NewProject("Default", "default", "", "default", config.Common().DefaultAdminEmail, config.Database()); err != nil {
		log.Printf("Seed project: %v", err)
	}

	// Initialize authorization module
	authorization.Initialize(config.Casbin())

	// Initialize router
	router := server.InitializeRouter()

	// Setup http server
	httpServer := &http.Server{
		Addr:    ":" + config.Common().AppPort,
		Handler: router,
	}

	go func() {
		log.Printf("Application is running on port: %s", config.Common().AppPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Application startup failed: %s", err)
		}
	}()

	// Handle graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down application..")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Graceful shutdown failed: %s", err)
	}

	log.Println("Application shutdown successfully..")
}
