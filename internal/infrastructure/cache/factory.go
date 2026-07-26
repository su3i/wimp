package cache

import (
	"sync"

	"github.com/su3i/wimp/internal/config"
	domain "github.com/su3i/wimp/internal/domain/cache"
	"github.com/su3i/wimp/internal/infrastructure/cache/memory"
	"github.com/su3i/wimp/internal/infrastructure/cache/redis"
)

var (
	instance domain.Cache
	once     sync.Once
)

func GetCache() domain.Cache {

	once.Do(func() {
		switch config.Cache().CacheType {
		case domain.CacheTypeRedis:
			instance = redis.NewCache(config.Cache())
		case domain.CacheTypeMemory:
			instance = memory.NewCache()
		default:
			instance = memory.NewCache()
		}
	})
	return instance
}
