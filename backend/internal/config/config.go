package config

import (
	"fmt"
	"os"
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

	UploadDir  string
	APIBaseURL string
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

		UploadDir:  getenv("UPLOAD_DIR", "./uploads"),
		APIBaseURL: getenv("API_BASE_URL", "http://localhost:8080"),
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
