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
)

// PerIP limits requests per client IP for the wrapped routes.
// On Redis failure the request is allowed: availability over strictness.
func PerIP(rdb *redis.Client, scope string, limit int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := fmt.Sprintf("rl:%s:%s:%d", scope, c.ClientIP(),
			time.Now().Unix()/int64(window.Seconds()))

		count, err := rdb.Incr(c.Request.Context(), key).Result()
		if err != nil {
			slog.Warn("rate limiter unavailable", "err", err)
			c.Next()
			return
		}
		if count == 1 {
			rdb.Expire(c.Request.Context(), key, window)
		}
		if count > int64(limit) {
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
