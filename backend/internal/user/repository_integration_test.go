package user

import (
	"context"
	"testing"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestUpsertTelegramUser_LanguageSetOnInsertOnly guards the bot i18n
// contract: a brand-new user's language comes from the caller's hint
// (e.g. Telegram's language_code), but once set, later logins/upserts
// must never overwrite it — the user may have since changed it via
// /language or the web profile page.
func TestUpsertTelegramUser_LanguageSetOnInsertOnly(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	t.Cleanup(pool.Close)

	const telegramID = 900000101
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM users WHERE telegram_id = $1`, telegramID)
	})

	repo := NewRepository(pool)

	u1, err := repo.UpsertTelegramUser(ctx, TelegramProfile{
		TelegramID: telegramID, Name: "Lang Test", Language: "ru",
	})
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if u1.Language != "ru" {
		t.Fatalf("language after insert = %q, want %q", u1.Language, "ru")
	}

	u2, err := repo.UpsertTelegramUser(ctx, TelegramProfile{
		TelegramID: telegramID, Name: "Lang Test", Language: "en",
	})
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if u2.Language != "ru" {
		t.Errorf("language after second upsert = %q, want unchanged %q", u2.Language, "ru")
	}
}
