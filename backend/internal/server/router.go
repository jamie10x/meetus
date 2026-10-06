// Package server assembles the Gin engine and mounts all module routes.
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"meetus.uz/backend/internal/admin"
	"meetus.uz/backend/internal/auth"
	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/geocode"
	"meetus.uz/backend/internal/meta"
	"meetus.uz/backend/internal/operations"
	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/ratelimit"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/tgbot"
	"meetus.uz/backend/internal/upload"
	"meetus.uz/backend/internal/user"
)

type Deps struct {
	Config *config.Config
	Pool   *pgxpool.Pool
	Redis  *redis.Client
}

func New(deps Deps) (*gin.Engine, error) {
	cfg := deps.Config
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	// Caddy is the only production ingress. Trust private Docker-network hops;
	// never trust arbitrary direct internet clients' forwarding headers.
	if cfg.IsProduction() {
		r.SetTrustedProxies([]string{"127.0.0.1", "::1", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"})
	} else {
		r.SetTrustedProxies(nil)
	}
	r.Use(gin.Recovery(), requestLogger(), corsMiddleware(cfg), func(c *gin.Context) {
		limit := int64(1 << 20)
		if c.Request.URL.Path == "/api/uploads" {
			limit = 6 << 20
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		if c.GetHeader("Authorization") != "" {
			c.Header("Cache-Control", "private, no-store")
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if deps.Pool.Ping(ctx) != nil || deps.Redis.Ping(ctx).Err() != nil {
			c.JSON(503, gin.H{"status": "unavailable"})
			return
		}
		c.JSON(200, gin.H{"status": "ready", "revision": cfg.Revision})
	})

	// Shared infrastructure.
	tokens := authn.NewTokenManager(cfg.JWTSecret, cfg.AccessTokenTTL)

	// Modules.
	userRepo := user.NewRepository(deps.Pool)
	requireAuth := authn.RequireAuth(tokens, userRepo.RequireActive)

	authRepo := auth.NewRepository(deps.Pool)
	authService := auth.NewService(userRepo, authRepo, tokens, cfg.TelegramBotToken, cfg.RefreshTokenTTL)

	organizerRepo := organizer.NewRepository(deps.Pool)
	requireOrganizer := organizer.RequireOrganizer(organizerRepo)
	eventRepo := event.NewRepository(deps.Pool)
	eventService := event.NewService(eventRepo)

	uploadHandler, err := upload.NewHandler(cfg.UploadDir, cfg.APIBaseURL)
	if err != nil {
		return nil, err
	}

	api := r.Group("/api")
	geocode.New(deps.Redis, cfg.GeocodeBaseURL).Register(api, requireAuth)

	// Abuse-prone endpoints get per-IP rate limits.
	authGroup := api.Group("", ratelimit.PerIP(deps.Redis, "auth", 20, time.Minute))
	auth.NewHandler(authService).Register(authGroup)
	user.NewHandler(userRepo).Register(api, requireAuth)

	requireAdmin := admin.RequireAdmin(userRepo)
	metaHandler := meta.NewHandler(deps.Pool)
	metaHandler.Register(api)
	metaHandler.RegisterAdmin(api, requireAuth, requireAdmin)

	organizer.NewHandler(organizerRepo).Register(api, requireAuth)
	eventHandler := event.NewHandler(eventService)
	eventHandler.Register(api, requireAuth, requireOrganizer)
	event.NewPublicHandler(eventRepo).Register(api)

	ticketSigner := rsvp.NewTicketSigner(cfg.TicketSecret)
	rsvpService := rsvp.NewService(rsvp.NewRepository(deps.Pool), ticketSigner)
	rsvp.NewHandler(rsvpService, eventRepo).Register(api, requireAuth, requireOrganizer, ratelimit.PerUser(deps.Redis, "rsvp", 120, time.Minute))

	admin.NewHandler(deps.Pool, eventRepo).Register(api, requireAuth, requireAdmin)
	operations.NewHandler(operations.NewRepository(deps.Pool), deps.Redis, cfg.TelegramBotToken != "").Register(api, requireAuth, requireAdmin)

	feedback.NewHandler(feedback.NewRepository(deps.Pool), eventRepo).Register(api, requireAuth, requireOrganizer)

	// The announcer needs a real bot token; dev environments without one
	// configured simply don't get channel announcements or waitlist
	// promotion pings (the endpoints return a clear error / silently skip
	// notifying) rather than failing the whole server to boot.
	var announcer channel.Announcer
	if cfg.TelegramBotToken != "" {
		a, err := tgbot.NewAnnouncer(cfg.TelegramBotToken, cfg.WebBaseURL, ticketSigner)
		if err != nil {
			return nil, err
		}
		announcer = a
	}
	channelRepo := channel.NewRepository(deps.Pool)
	channel.NewHandler(channelRepo, eventRepo, userRepo, announcer).Register(api, requireAuth, requireOrganizer)

	uploadHandler.Register(api, r, requireAuth, ratelimit.PerUser(deps.Redis, "upload", 20, 24*time.Hour))

	return r, nil
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
