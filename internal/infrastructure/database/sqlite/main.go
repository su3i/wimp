package sqlite

import (
	"errors"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/metadata"
	"github.com/su3i/wimp/internal/domain/organization"
	"github.com/su3i/wimp/internal/domain/project"
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
}
