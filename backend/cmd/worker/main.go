// Command worker runs the Telegram bot (long polling) and the reminder
// loop that notifies attendees before their events start.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/housekeeping"
	"meetus.uz/backend/internal/notification"
	"meetus.uz/backend/internal/platform/db"
	"meetus.uz/backend/internal/platform/redisx"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/tgbot"
	"meetus.uz/backend/internal/user"
)

const (
	scanInterval = time.Minute
	scanLockKey  = "meetus:worker:reminder-scan"
	scanLockTTL  = 50 * time.Second

	housekeepingInterval = time.Hour
	housekeepingLockKey  = "meetus:worker:housekeeping"
	housekeepingLockTTL  = 55 * time.Minute
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.TelegramBotToken == "" {
		slog.Error("TELEGRAM_BOT_TOKEN is not set; the worker needs it for the bot and reminders")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := redisx.NewClient(ctx, cfg.RedisAddr)
	if err != nil {
		return err
	}
	defer rdb.Close()

	bot, err := tgbot.New(
		cfg.TelegramBotToken,
		user.NewRepository(pool),
		event.NewRepository(pool),
		rsvp.NewRepository(pool),
		feedback.NewRepository(pool),
		channel.NewRepository(pool),
		rdb,
		cfg.WebBaseURL,
	)
	if err != nil {
		return err
	}

	notifications := notification.NewRepository(pool)

	go reminderLoop(ctx, notifications, bot, rdb)
	go housekeepingLoop(ctx, housekeeping.NewRunner(pool), rdb)

	// Blocks until ctx is canceled.
	bot.Start(ctx)
	slog.Info("worker stopped")
	return nil
}

func reminderLoop(ctx context.Context, repo *notification.Repository, bot *tgbot.Bot, rdb *redis.Client) {
	ticker := time.NewTicker(scanInterval)
	defer ticker.Stop()

	scan := func() {
		// One scan at a time across all worker instances.
		ok, err := rdb.SetNX(ctx, scanLockKey, "1", scanLockTTL).Result()
		if err != nil {
			slog.Error("reminder lock failed", "err", err)
			return
		}
		if !ok {
			return
		}
		for _, kind := range []notification.Kind{
			notification.KindReminder24h,
			notification.KindReminder1h,
		} {
			sendDue(ctx, repo, bot, kind)
		}
		sendDueFeedback(ctx, repo, bot)
	}

	scan()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			scan()
		}
	}
}

func housekeepingLoop(ctx context.Context, runner *housekeeping.Runner, rdb *redis.Client) {
	ticker := time.NewTicker(housekeepingInterval)
	defer ticker.Stop()

	run := func() {
		ok, err := rdb.SetNX(ctx, housekeepingLockKey, "1", housekeepingLockTTL).Result()
		if err != nil {
			slog.Error("housekeeping lock failed", "err", err)
			return
		}
		if ok {
			runner.Run(ctx)
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func sendDue(ctx context.Context, repo *notification.Repository, bot *tgbot.Bot, kind notification.Kind) {
	due, err := repo.Due(ctx, kind)
	if err != nil {
		slog.Error("load due reminders failed", "kind", kind, "err", err)
		return
	}
	for _, rem := range due {
		if err := bot.SendReminder(ctx, rem); err != nil {
			// Typical case: the user never opened the bot chat (403).
			// Recorded anyway so we don't retry forever.
			slog.Warn("reminder send failed", "event", rem.EventID,
				"user", rem.UserID, "err", err)
		}
		if err := repo.MarkSent(ctx, rem); err != nil {
			slog.Error("mark sent failed", "event", rem.EventID,
				"user", rem.UserID, "err", err)
		}
	}
	if len(due) > 0 {
		slog.Info("reminders processed", "kind", kind, "count", len(due))
	}
}

func sendDueFeedback(ctx context.Context, repo *notification.Repository, bot *tgbot.Bot) {
	due, err := repo.DueFeedback(ctx)
	if err != nil {
		slog.Error("load due feedback prompts failed", "err", err)
		return
	}
	for _, f := range due {
		if err := bot.SendFeedbackRequest(ctx, f); err != nil {
			slog.Warn("feedback prompt send failed", "event", f.EventID, "user", f.UserID, "err", err)
		}
		if err := repo.MarkFeedbackSent(ctx, f); err != nil {
			slog.Error("mark feedback sent failed", "event", f.EventID, "user", f.UserID, "err", err)
		}
	}
	if len(due) > 0 {
		slog.Info("feedback prompts processed", "count", len(due))
	}
}
