package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

var (
	cache	 *CacheConfig
    common   *CommonConfig
    database *DatabaseConfig
    casbin *CasbinConfig
    alerts   *AlertConfig
)

func Initialize() {
	cache = &CacheConfig{}
    if err := envconfig.Process("", cache); err != nil {
		log.Fatalf("cache config: %v", err)
    }
	common = &CommonConfig{}
    if err := envconfig.Process("", common); err != nil {
		log.Fatalf("common config: %v", err)
    }
	database = &DatabaseConfig{}
	if err := envconfig.Process("", database); err != nil {
		log.Fatalf("database config: %v", err)
	}
	casbin = &CasbinConfig{}
	if err := envconfig.Process("", casbin); err != nil {
		log.Fatalf("database config: %v", err)
	}
	alerts = &AlertConfig{}
	if err := envconfig.Process("", alerts); err != nil {
		log.Fatalf("alert config: %v", err)
	}
}

func Cache() *CacheConfig     { return cache }
func Common() *CommonConfig     { return common }
func Database() *DatabaseConfig { return database }
func Casbin() *CasbinConfig { return casbin }
func Alerts() *AlertConfig { return alerts }