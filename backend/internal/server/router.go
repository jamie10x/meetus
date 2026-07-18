// Package server assembles the Gin engine and mounts all module routes.
package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"meetus.uz/backend/internal/auth"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/meta"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/user"
)

type Deps struct {
	Config *config.Config
	Pool   *pgxpool.Pool
	Redis  *redis.Client
}

func New(deps Deps) *gin.Engine {
	cfg := deps.Config
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger(), corsMiddleware(cfg))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Shared infrastructure.
	tokens := authn.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)
	requireAuth := authn.RequireAuth(tokens)

	// Modules.
	userRepo := user.NewRepository(deps.Pool)
	authRepo := auth.NewRepository(deps.Pool)
	authService := auth.NewService(userRepo, authRepo, tokens, cfg.TelegramBotToken, cfg.RefreshTokenTTL)

	api := r.Group("/api")
	auth.NewHandler(authService).Register(api)
	user.NewHandler(userRepo).Register(api, requireAuth)
	meta.NewHandler(deps.Pool).Register(api)

	return r
}

func corsMiddleware(cfg *config.Config) gin.HandlerFunc {
	corsCfg := cors.DefaultConfig()
	if cfg.IsProduction() {
		corsCfg.AllowOrigins = []string{"https://meetus.uz", "https://www.meetus.uz"}
	} else {
		corsCfg.AllowOrigins = []string{"http://localhost:3000"}
	}
	corsCfg.AllowHeaders = append(corsCfg.AllowHeaders, "Authorization")
	return cors.New(corsCfg)
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
		)
	}
}
