package postgres

import (
	"errors"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/application"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/metadata"
	"github.com/su3i/wimp/internal/domain/monitor"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/organization"
	"github.com/su3i/wimp/internal/domain/project"
	"github.com/su3i/wimp/internal/domain/site"
)

var DB *gorm.DB

func ValidateConfig(c *config.DatabaseConfig) error {
	if c.DatabaseHost == "" {
		return errors.New("DATABASE_HOST is required")
	}
	if c.DatabasePort == "" {
		return errors.New("DATABASE_PORT is required")
	}
	if c.DatabaseUsername == "" {
		return errors.New("DATABASE_USERNAME is required")
	}
	if c.DatabasePassword == "" {
		return errors.New("DATABASE_PASSWORD is required")
	}
	if c.DatabaseName == "" {
		return errors.New("DATABASE_NAME is required")
	}
	return nil
}

func Connect(config *config.DatabaseConfig) {
	if err := ValidateConfig(config); err != nil {
		log.Fatalf("Invalid postgres config: %v", err)
	}

	sslMode := "disable"
	if config.DatabaseUseSSL {
		sslMode = "require"
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.DatabaseHost,
		config.DatabasePort,
		config.DatabaseUsername,
		config.DatabasePassword,
		config.DatabaseName,
		sslMode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to postgres: %v", err)
	}

	log.Printf("Successfully connected to postgres")
}

func Migrate() {
	err := DB.AutoMigrate(&organization.Organization{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (organization): %v", err)
	}

	err = DB.AutoMigrate(&account.Account{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (account): %v", err)
	}

	// Drop legacy email column if it still exists (migrated to username)
	if DB.Migrator().HasColumn(&account.Account{}, "email") {
		if err := DB.Migrator().DropColumn(&account.Account{}, "email"); err != nil {
			log.Printf("failed to drop accounts.email column: %v", err)
		}
	}

	err = DB.AutoMigrate(&metadata.Metadata{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (metadata): %v", err)
	}
	
	err = DB.AutoMigrate(&project.Project{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (project): %v", err)
	}

	err = DB.AutoMigrate(&machine.Machine{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (machine): %v", err)
	}

	err = DB.AutoMigrate(&apppool.AppPool{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (app_pool): %v", err)
	}

	err = DB.AutoMigrate(&site.Site{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (site): %v", err)
	}

	err = DB.AutoMigrate(&application.Application{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (application): %v", err)
	}

	err = DB.AutoMigrate(&application.ApplicationAppPool{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (application_app_pool): %v", err)
	}

	err = DB.AutoMigrate(&notification.Notification{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (notification): %v", err)
	}

	err = DB.AutoMigrate(&monitor.Monitor{})
	if err != nil {
		log.Fatalf("failed to migrate postgres database (monitor): %v", err)
	}

}
