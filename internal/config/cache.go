package config

import (
	domain "github.com/su3i/wimp/internal/domain/cache"
)

type CacheConfig struct {
	CacheType        domain.CacheType `default:"memory"`
	RedisAddr  string             `required:"false"`
	RedisPassword    string             `required:"false"`
	RedisDB int             `required:"false"`
}
