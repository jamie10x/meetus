package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv   string
	HTTPAddr string

	DatabaseURL string
	RedisAddr   string

	JWTSecret    string
	TicketSecret string

	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration

	TelegramBotToken    string
	TelegramBotUsername string

	// OfficialChannelID, if set, is the chat ID of the platform's own
	// Telegram channel — every published event auto-posts there in
	// addition to the publishing organizer's own connected channels.
	// Zero means "not configured" (valid channel IDs are large negatives,
	// never zero). Obtained the same way an organizer's own channel is:
	// add the bot as admin, then read the chat_id the worker logs.
	OfficialChannelID       int64
	OfficialChannelLanguage string

	UploadDir  string
	APIBaseURL string
	WebBaseURL string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:   getenv("APP_ENV", "development"),
		HTTPAddr: getenv("HTTP_ADDR", ":8080"),

		DatabaseURL: getenv("DATABASE_URL", "postgres://meetus:meetus@localhost:5432/meetus?sslmode=disable"),
		RedisAddr:   getenv("REDIS_ADDR", "localhost:6379"),

		JWTSecret:    os.Getenv("JWT_SECRET"),
		TicketSecret: os.Getenv("TICKET_SECRET"),

		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,

		TelegramBotToken:    os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramBotUsername: os.Getenv("TELEGRAM_BOT_USERNAME"),

		OfficialChannelLanguage: getenv("TELEGRAM_OFFICIAL_CHANNEL_LANGUAGE", "uz"),

		UploadDir:  getenv("UPLOAD_DIR", "./uploads"),
		APIBaseURL: getenv("API_BASE_URL", "http://localhost:8080"),
		WebBaseURL: getenv("WEB_BASE_URL", "http://localhost:3000"),
	}

	if raw := os.Getenv("TELEGRAM_OFFICIAL_CHANNEL_ID"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("TELEGRAM_OFFICIAL_CHANNEL_ID must be a valid integer chat ID: %w", err)
		}
		cfg.OfficialChannelID = id
	}

	if cfg.AppEnv == "production" {
		if cfg.JWTSecret == "" || cfg.TicketSecret == "" {
			return nil, fmt.Errorf("JWT_SECRET and TICKET_SECRET are required in production")
		}
		if cfg.TelegramBotToken == "" {
			return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required in production")
		}
	}
	// Development fallbacks so the server runs without a .env.
	if cfg.JWTSecret == "" {
		cfg.JWTSecret = "dev-jwt-secret"
	}
	if cfg.TicketSecret == "" {
		cfg.TicketSecret = "dev-ticket-secret"
	}
	return cfg, nil
}

func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
