package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/su3i/wimp/internal/infrastructure/cache"
)

// RateLimit throttles requests per client IP using the shared cache (Redis-backed in
// multi-replica deployments, in-memory otherwise - same store already used for refresh
// tokens and MFA challenges). Every request within the window bumps the counter and
// refreshes its TTL, so this is a sliding window: at most `limit` requests with no more
// than `window` of idle time between them, which is a tighter fit for brute-force
// protection than a strict fixed-window reset.
func RateLimit(name string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("ratelimit:%s:%s", name, c.ClientIP())
		store := cache.GetCache()

		count := 1
		if raw, err := store.Get(key); err == nil {
			if n, err := strconv.Atoi(raw); err == nil {
				count = n + 1
			}
		}

		if count > limit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts, please try again later"})
			return
		}

		// A cache write failure shouldn't block login entirely - fail open.
		if err := store.Set(key, strconv.Itoa(count), window); err != nil {
			c.Next()
			return
		}

		c.Next()
	}
}
