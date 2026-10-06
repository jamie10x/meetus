// Package geocode provides submitted address searches with an app-wide rate
// limit and cache, rather than per-keystroke calls to the public provider.
package geocode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"io"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Handler struct {
	redis  *redis.Client
	base   string
	client *http.Client
}

func New(rdb *redis.Client, base string) *Handler {
	return &Handler{rdb, strings.TrimRight(base, "/"), &http.Client{Timeout: 8 * time.Second}}
}
func (h *Handler) Register(r gin.IRouter, auth gin.HandlerFunc) {
	r.GET("/geocode/search", auth, h.lookup(false))
	r.GET("/geocode/reverse", auth, h.lookup(true))
}
func (h *Handler) lookup(reverse bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		values := url.Values{"format": {"json"}}
		path := "/search"
		if reverse {
			path = "/reverse"
			for _, param := range []string{"lat", "lon"} {
				n, err := strconv.ParseFloat(c.Query(param), 64)
				bound := 180.0
				if param == "lat" {
					bound = 90
				}
				if err != nil || !(n >= -bound && n <= bound) {
					httpx.Error(c, apperr.Validation("invalid coordinates"))
					return
				}
				values.Set(param, strconv.FormatFloat(n, 'f', 6, 64))
			}
		} else {
			q := strings.TrimSpace(c.Query("q"))
			if len(q) < 3 || len(q) > 300 {
				httpx.Error(c, apperr.Validation("query must be 3-300 bytes"))
				return
			}
			values.Set("q", q)
			values.Set("limit", "5")
		}
		endpoint := h.base + path + "?" + values.Encode()
		digest := sha256.Sum256([]byte(endpoint))
		key := "meetus:geocode:" + hex.EncodeToString(digest[:])
		ctx := c.Request.Context()
		if cached, err := h.redis.Get(ctx, key).Bytes(); err == nil {
			httpx.OK(c, 200, json.RawMessage(cached))
			return
		}
		allowed, err := h.redis.SetNX(ctx, "meetus:geocode:provider-limit", "1", time.Second).Result()
		if err != nil {
			httpx.Error(c, fmt.Errorf("geocode limiter: %w", err))
			return
		}
		if !allowed {
			c.Header("Retry-After", "1")
			c.AbortWithStatusJSON(429, gin.H{"error": gin.H{"code": "rate_limited", "message": "please wait before searching again"}})
			return
		}
		req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		req.Header.Set("User-Agent", "Meetus.uz/1.0 (https://meetus.uz)")
		response, err := h.client.Do(req)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			httpx.Error(c, fmt.Errorf("geocoder status %d", response.StatusCode))
			return
		}
		data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
		if err != nil || len(data) > 65536 || !json.Valid(data) {
			httpx.Error(c, apperr.Internal())
			return
		}
		h.redis.Set(ctx, key, data, 7*24*time.Hour)
		httpx.OK(c, 200, json.RawMessage(data))
	}
}
