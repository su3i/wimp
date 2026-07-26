package memory

import (
	"errors"
	"sync"
	"time"

	domain "github.com/su3i/wimp/internal/domain/cache"
)

type item struct {
	value      string
	expiration time.Time
}

type MemoryCache struct {
	data map[string]item
	mu   sync.RWMutex
}

func NewCache() domain.Cache {
	return &MemoryCache{
		data: make(map[string]item),
	}
}

func (m *MemoryCache) Set(key string, value string, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = item{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
	return nil
}

func (m *MemoryCache) Get(key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	it, ok := m.data[key]
	if !ok {
		return "", errors.New("key not found")
	}

	if time.Now().After(it.expiration) {
		delete(m.data, key)
		return "", errors.New("key not found")
	}

	return it.value, nil
}

func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
	return nil
}
