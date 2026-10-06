// Package ratelimit provides a Redis fixed-window rate limiter used on
// abuse-prone endpoints (login, RSVP, check-in).
package ratelimit

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"meetus.uz/backend/internal/platform/authn"
)

// PerIP limits requests per client IP for the wrapped routes.
// On Redis failure the request is allowed: availability over strictness.
func PerIP(rdb *redis.Client, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return limiter(rdb, scope, limit, window, func(c *gin.Context) string { return c.ClientIP() })
}

func PerUser(rdb *redis.Client, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return limiter(rdb, scope, limit, window, func(c *gin.Context) string { return fmt.Sprint(authn.UserID(c)) })
}

func limiter(rdb *redis.Client, scope string, limit int, window time.Duration, identity func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s:%d", scope, identity(c),
			time.Now().Unix()/int64(window.Seconds()))

		count, err := rdb.Eval(c.Request.Context(), `local n=redis.call("INCR",KEYS[1]);if n==1 then redis.call("EXPIRE",KEYS[1],ARGV[1]) end;return n`, []string{key}, int(window.Seconds())).Int64()
		if err != nil {
			slog.Warn("rate limiter unavailable", "err", err)
			c.Next()
			return
		}
		if count > int64(limit) {
			c.Header("Retry-After", fmt.Sprint(int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": gin.H{
					"code":    "rate_limited",
					"message": "too many requests, slow down",
				},
			})
			return
		}
		c.Next()
	}
}
