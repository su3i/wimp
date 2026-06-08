package cache

import "time"

type CacheType string

const (
	CacheTypeRedis   CacheType = "redis"
	CacheTypeMemory CacheType = "memory"
)

// Cache defines the minimal cache operations
type Cache interface {
	Set(key string, value string, ttl time.Duration) error
	Get(key string) (string, error)
	Delete(key string) error
}