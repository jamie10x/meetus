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

	"github.com/redis/go-redis/v9"

	"meetus.uz/backend/internal/channel"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/groupfeed"
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

	// digestCheckInterval polls frequently so the actual send happens
	// close to the top of the target hour; digestLockKeyPrefix is keyed
	// per ISO week (not a fixed key like the other locks) so a new send
	// window opens automatically every week without any reset logic.
	digestCheckInterval = 15 * time.Minute
	digestLockKeyPrefix = "meetus:worker:weekly-digest:"
	digestLockTTL       = 8 * 24 * time.Hour
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
	users := user.NewRepository(pool)

	go reminderLoop(ctx, notifications, bot, rdb)
	go housekeepingLoop(ctx, housekeeping.NewRunner(pool), rdb)
	go digestLoop(ctx, users, bot, rdb)

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

// digestLoop wakes up periodically and, once per ISO week during the
// Monday-morning send hour, fans the weekly "what's on" digest out to
// every opted-in subscriber. The Redis lock (keyed per week, not a fixed
// key) is what makes this safe to run on every worker instance without
// double-sending.
func digestLoop(ctx context.Context, users *user.Repository, bot *tgbot.Bot, rdb *redis.Client) {
	loc, err := time.LoadLocation("Asia/Tashkent")
	if err != nil {
		slog.Error("digest loop: load location failed", "err", err)
		return
	}
	ticker := time.NewTicker(digestCheckInterval)
	defer ticker.Stop()

	check := func() {
		now := time.Now().In(loc)
		if now.Weekday() != time.Monday || now.Hour() != digestSendHour {
			return
		}
		year, week := now.ISOWeek()
		lockKey := fmt.Sprintf("%s%d-%02d", digestLockKeyPrefix, year, week)
		ok, err := rdb.SetNX(ctx, lockKey, "1", digestLockTTL).Result()
		if err != nil {
			slog.Error("digest lock failed", "err", err)
			return
		}
		if !ok {
			return
		}
		sendWeeklyDigest(ctx, users, bot)
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

func sendWeeklyDigest(ctx context.Context, users *user.Repository, bot *tgbot.Bot) {
	subs, err := users.ListWeeklyDigestSubscribers(ctx)
	if err != nil {
		slog.Error("load weekly digest subscribers failed", "err", err)
		return
	}
	failed := 0
	for _, sub := range subs {
		if err := bot.SendWeeklyDigest(ctx, sub); err != nil {
			slog.Warn("weekly digest send failed", "user", sub.UserID, "err", err)
			failed++
		}
	}
	slog.Info("weekly digest processed", "subscribers", len(subs), "failed", failed)
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
