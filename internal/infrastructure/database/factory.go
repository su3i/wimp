package database

import (
	"github.com/su3i/wimp/internal/config"
	"github.com/su3i/wimp/internal/domain/account"
	"github.com/su3i/wimp/internal/domain/apppool"
	databaseDomain "github.com/su3i/wimp/internal/domain/database"
	"github.com/su3i/wimp/internal/domain/machine"
	"github.com/su3i/wimp/internal/domain/metadata"
	"github.com/su3i/wimp/internal/domain/organization"
	"github.com/su3i/wimp/internal/domain/project"
	"github.com/su3i/wimp/internal/domain/site"
	"github.com/su3i/wimp/internal/infrastructure/database/postgres"
	postgresRepository "github.com/su3i/wimp/internal/infrastructure/database/postgres/repositories"
	"github.com/su3i/wimp/internal/infrastructure/database/sqlite"
	sqliteRepository "github.com/su3i/wimp/internal/infrastructure/database/sqlite/repositories"
	"gorm.io/gorm"
)

func Initialize(config *config.DatabaseConfig) {
	switch config.DatabaseType {
		case databaseDomain.DatabaseTypePostgres:
			postgres.Connect(config)
		case databaseDomain.DatabaseTypeSqlite:
			sqlite.Connect(config)
		default:
			sqlite.Connect(config) // Treat SQLite as Default
	}
}

func Migrate(config *config.DatabaseConfig) {
	switch config.DatabaseType {
		case databaseDomain.DatabaseTypePostgres:
			postgres.Migrate()
		case databaseDomain.DatabaseTypeSqlite:
			sqlite.Migrate()
		default:
			sqlite.Migrate() // Treat SQLite as Default
	}
}

func GetDB(config *config.DatabaseConfig) *gorm.DB {
	switch config.DatabaseType {
		case databaseDomain.DatabaseTypePostgres:
			return postgres.DB
		case databaseDomain.DatabaseTypeSqlite:
			return sqlite.DB
		default:
			return sqlite.DB // Treat SQLite as Default
	}
}

func newRepository[T any](
    config *config.DatabaseConfig,
    pgFactory func(*gorm.DB) T,
    sqliteFactory func(*gorm.DB) T,
) T {
    db := GetDB(config)
    switch config.DatabaseType {
		case databaseDomain.DatabaseTypePostgres:
			return pgFactory(db)
		default:
			return sqliteFactory(db)
    }
}

func NewMetadataRepository(config *config.DatabaseConfig) metadata.MetadataRepository {
    return newRepository(config, postgresRepository.NewMetadataRepository, sqliteRepository.NewMetadataRepository)
}

func NewOrganizationRepository(config *config.DatabaseConfig) organization.OrganizationRepository {
	return newRepository(config, postgresRepository.NewOrganizationRepository, sqliteRepository.NewOrganizationRepository)
}

func NewAccountRepository(config *config.DatabaseConfig) account.AccountRepository {
	return newRepository(config, postgresRepository.NewAccountRepository, sqliteRepository.NewAccountRepository)
}

func NewProjectRepository(config *config.DatabaseConfig) project.ProjectRepository {
	return newRepository(config, postgresRepository.NewProjectRepository, sqliteRepository.NewProjectRepository)
}

func NewMachineRepository(config *config.DatabaseConfig) machine.MachineRepository {
	return newRepository(config, postgresRepository.NewMachineRepository, sqliteRepository.NewMachineRepository)
}

func NewAppPoolRepository(config *config.DatabaseConfig) apppool.AppPoolRepository {
	return newRepository(config, postgresRepository.NewAppPoolRepository, sqliteRepository.NewAppPoolRepository)
}

func NewSiteRepository(config *config.DatabaseConfig) site.SiteRepository {
	return newRepository(config, postgresRepository.NewSiteRepository, sqliteRepository.NewSiteRepository)
}
