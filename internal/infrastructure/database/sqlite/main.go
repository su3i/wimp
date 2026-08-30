package sqlite

import (
	"errors"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/application"
	"github.com/su3i/wimp/internal/domain/apppool"
	"github.com/su3i/wimp/internal/domain/incident"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/metadata"
	"github.com/su3i/wimp/internal/domain/monitor"
	"github.com/su3i/wimp/internal/domain/notification"
	"github.com/su3i/wimp/internal/domain/organization"
	"github.com/su3i/wimp/internal/domain/project"
	"github.com/su3i/wimp/internal/domain/setting"
	"github.com/su3i/wimp/internal/domain/site"
)

var DB *gorm.DB

func ValidateConfig(c *config.DatabaseConfig) error {
	if c.DatabasePath == "" {
		return errors.New("DATABASE_PATH is required")
	}
	return nil
}

func Connect(cfg *config.DatabaseConfig) {
	if err := ValidateConfig(cfg); err != nil {
		log.Fatalf("Invalid sqlite config: %v", err)
	}

	var err error

	DB, err = gorm.Open(sqlite.Open(cfg.DatabasePath), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to sqlite: %v", err)
	}

	log.Println("Successfully connected to sqlite")
}

func Migrate() {
	err := DB.AutoMigrate(&organization.Organization{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (organization): %v", err)
	}

	err = DB.AutoMigrate(&account.Account{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (account): %v", err)
	}

	err = DB.AutoMigrate(&metadata.Metadata{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (metadata): %v", err)
	}

	err = DB.AutoMigrate(&project.Project{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (project): %v", err)
	}

	err = DB.AutoMigrate(&machine.Machine{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (machine): %v", err)
	}

	err = DB.AutoMigrate(&apppool.AppPool{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (app_pool): %v", err)
	}

	err = DB.AutoMigrate(&site.Site{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (site): %v", err)
	}

	err = DB.AutoMigrate(&application.Application{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (application): %v", err)
	}

	err = DB.AutoMigrate(&application.ApplicationAppPool{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (application_app_pool): %v", err)
	}

	err = DB.AutoMigrate(&notification.Notification{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (notification): %v", err)
	}

	err = DB.AutoMigrate(&setting.Setting{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (setting): %v", err)
	}

	err = DB.AutoMigrate(&incident.Incident{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (incident): %v", err)
	}

	err = DB.AutoMigrate(&monitor.Monitor{})
	if err != nil {
		log.Fatalf("failed to migrate sqlite database (monitor): %v", err)
	}

}
