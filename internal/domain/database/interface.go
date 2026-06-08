package database

type DatabaseType string

const (
	DatabaseTypePostgres   DatabaseType = "postgres"
	DatabaseTypeSqlite DatabaseType = "sqlite"
)