package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/su3i/wimp/internal/config"
	domain "github.com/su3i/wimp/internal/domain/cache"
)

type CacheType struct {
	client *redis.Client
	ctx    context.Context
}

func NewCache(c *config.CacheConfig) domain.Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.RedisAddr,
		Password: c.RedisPassword,
		DB:       c.RedisDB,
	})

	return &CacheType{
		client: rdb,
		ctx:    context.Background(),
	}
}

func (c *CacheType) Set(key string, value string, ttl time.Duration) error {
	return c.client.Set(c.ctx, key, value, ttl).Err()
}

func (c *CacheType) Get(key string) (string, error) {
	return c.client.Get(c.ctx, key).Result()
}

func (c *CacheType) Delete(key string) error {
	return c.client.Del(c.ctx, key).Err()
}