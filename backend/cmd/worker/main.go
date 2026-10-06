// Command worker runs the Telegram bot (long polling) and the reminder
// loop that notifies attendees before their events start.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/groupfeed"
	"meetus.uz/backend/internal/housekeeping"
	"meetus.uz/backend/internal/notification"
	"meetus.uz/backend/internal/platform/db"
	"meetus.uz/backend/internal/platform/outbox"
	"meetus.uz/backend/internal/platform/redisx"
	"meetus.uz/backend/internal/platform/workerhealth"
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

	digestCheckInterval = 15 * time.Minute
	digestSendHour      = 9 // 09:00 Asia/Tashkent, Mondays
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

	bot, err := tgbot.New(tgbot.Deps{
		Token:        cfg.TelegramBotToken,
		Users:        user.NewRepository(pool),
		Events:       event.NewRepository(pool),
		RSVPs:        rsvp.NewRepository(pool),
		Feedback:     feedback.NewRepository(pool),
		Channels:     channel.NewRepository(pool),
		Groups:       groupfeed.NewRepository(pool),
		TicketSigner: rsvp.NewTicketSigner(cfg.TicketSecret),
		Redis:        rdb,
		WebBaseURL:   cfg.WebBaseURL,
	})
	if err != nil {
		return err
	}

	notifications := notification.NewRepository(pool)

	announcer, err := tgbot.NewAnnouncer(cfg.TelegramBotToken, cfg.WebBaseURL, rsvp.NewTicketSigner(cfg.TicketSecret))
	if err != nil {
		return err
	}
	monitor := workerhealth.New(rdb, cfg.Revision)
	go outbox.Run(ctx, pool, deliveryHandler(pool, cfg, bot, announcer), func(ctx context.Context) {
		if err := monitor.Pulse(ctx); err != nil {
			slog.Warn("worker progress unavailable")
		}
	})
	go reminderLoop(ctx, notifications, bot, rdb)
	go housekeepingLoop(ctx, housekeeping.NewRunner(pool), rdb)
	go digestLoop(ctx, pool)

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
		if err := repo.QueueReminder(ctx, rem); err != nil {
			slog.Error("queue reminder failed", "event_id", rem.EventID)
		}
	}
	if len(due) > 0 {
		slog.Info("reminders processed", "kind", kind, "count", len(due))
	}
}

// digestLoop queues one durable batch per ISO week, with catch-up after Monday 09:00.
func digestLoop(ctx context.Context, pool *pgxpool.Pool) {
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		return
	}
	ticker := time.NewTicker(digestCheckInterval)
	defer ticker.Stop()
	check := func() {
		now := time.Now().In(loc)
		if now.Weekday() == time.Monday && now.Hour() < digestSendHour {
			return
		}
		year, week := now.ISOWeek()
		if err := outbox.Enqueue(ctx, pool, fmt.Sprintf("digest-week:%d:%d", year, week), "digest-batch", map[string]int{"year": year, "week": week}); err != nil {
			slog.Error("queue digest failed")
		}
	}
	check()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
		}
	}
}

func sendDueFeedback(ctx context.Context, repo *notification.Repository, bot *tgbot.Bot) {
	due, err := repo.DueFeedback(ctx)
	if err != nil {
		slog.Error("load due feedback prompts failed", "err", err)
		return
	}
	for _, f := range due {
		if err := repo.QueueFeedback(ctx, f); err != nil {
			slog.Error("queue feedback failed", "event_id", f.EventID)
		}
	}
	if len(due) > 0 {
		slog.Info("feedback prompts processed", "count", len(due))
	}
}
