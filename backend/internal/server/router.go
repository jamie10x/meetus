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
	"meetus.uz/backend/internal/groupfeed"
	"meetus.uz/backend/internal/meta"
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

	organizerRepo := organizer.NewRepository(deps.Pool)
	requireOrganizer := organizer.RequireOrganizer(organizerRepo)
	eventRepo := event.NewRepository(deps.Pool)
	eventService := event.NewService(eventRepo)

	uploadHandler, err := upload.NewHandler(cfg.UploadDir, cfg.APIBaseURL)
	if err != nil {
		return nil, err
	}

	api := r.Group("/api")

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
	rsvpService := rsvp.NewService(rsvp.NewRepository(deps.Pool), ticketSigner, eventRepo, userRepo)
	rsvpGroup := api.Group("", ratelimit.PerIP(deps.Redis, "rsvp", 60, time.Minute))
	rsvp.NewHandler(rsvpService, eventRepo).Register(rsvpGroup, requireAuth, requireOrganizer)

	admin.NewHandler(deps.Pool, eventRepo).Register(api, requireAuth, requireAdmin)

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
		rsvpService.SetPromotionNotifier(a)
	}
	channelRepo := channel.NewRepository(deps.Pool)
	channel.NewHandler(channelRepo, eventRepo, userRepo, announcer).Register(api, requireAuth, requireOrganizer)

	groupRepo := groupfeed.NewRepository(deps.Pool)

	eventHandler.SetOnPublished(func(ctx context.Context, e *event.Event) {
		if announcer == nil {
			return
		}

		// The platform's own channel gets every published event,
		// independent of whatever the publishing organizer has (or
		// hasn't) connected — never gated on the per-organizer lookup
		// below.
		if cfg.OfficialChannelID != 0 {
			if err := announcer.SendAnnouncement(ctx, cfg.OfficialChannelID, cfg.OfficialChannelLanguage, e); err != nil {
				slog.Error("official channel auto-announce failed", "event_id", e.ID, "err", err)
			} else {
				slog.Info("official channel auto-announce sent", "event_id", e.ID)
			}
		}

		// Groups that opted into the same platform-wide feed (see
		// groupfeed package) get every published event too, same as the
		// official channel — always in the configured official-channel
		// language, since a group subscription has no per-chat language
		// override of its own (unlike organizer channels).
		if groupChatIDs, err := groupRepo.ListChatIDs(ctx); err != nil {
			slog.Error("group feed auto-announce: could not list subscribed groups", "event_id", e.ID, "err", err)
		} else {
			lang := cfg.OfficialChannelLanguage
			for _, chatID := range groupChatIDs {
				if err := announcer.SendAnnouncement(ctx, chatID, lang, e); err != nil {
					slog.Error("group feed auto-announce failed", "event_id", e.ID, "chat_id", chatID, "err", err)
				} else {
					slog.Info("group feed auto-announce sent", "event_id", e.ID, "chat_id", chatID)
				}
			}
		}

		channels, err := channelRepo.ListForOrganizer(ctx, e.OrganizerID)
		if err != nil || len(channels) == 0 {
			return
		}
		orgLang, err := organizerRepo.GetLanguage(ctx, e.OrganizerID)
		if err != nil {
			slog.Error("auto-announce: could not load organizer language", "event_id", e.ID, "err", err)
			return
		}
		for _, ch := range channels {
			// A channel can be both someone's own organizer channel and
			// the official channel (e.g. the platform's own account
			// connected its own channel as an organizer first) — it
			// already got the official-channel send above, so skip it
			// here rather than posting the same event twice.
			if cfg.OfficialChannelID != 0 && ch.ChatID == cfg.OfficialChannelID {
				continue
			}
			lang := orgLang
			if ch.Language != nil {
				lang = *ch.Language
			}
			if err := announcer.SendAnnouncement(ctx, ch.ChatID, lang, e); err != nil {
				slog.Error("auto-announce failed", "event_id", e.ID, "channel_id", ch.ID, "err", err)
			} else {
				slog.Info("auto-announce sent", "event_id", e.ID, "channel_id", ch.ID)
			}
		}
	})

	uploadHandler.Register(api, r, requireAuth)

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
