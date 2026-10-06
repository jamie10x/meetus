package operations

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"meetus.uz/backend/internal/platform/httpx"
	"meetus.uz/backend/internal/platform/workerhealth"
	"net/http"
	"time"
)

type Handler struct {
	repo    *Repository
	redis   *redis.Client
	enabled bool
}

func NewHandler(repo *Repository, rdb *redis.Client, enabled bool) *Handler {
	return &Handler{repo, rdb, enabled}
}
func (h *Handler) Register(r gin.IRouter, auth, admin gin.HandlerFunc) {
	r.GET("/admin/operations", auth, admin, h.get)
}
func (h *Handler) get(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	worker, err := workerhealth.Read(ctx, h.redis, h.enabled)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	delivery, err := h.repo.Delivery(ctx)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"worker": worker, "delivery": delivery})
}
