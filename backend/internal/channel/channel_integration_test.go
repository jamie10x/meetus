package channel

import (
	"context"
	"testing"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestConnectByTelegramID_RequiresOrganizer verifies the core safety
// property of channel linking: only a Telegram user with an existing
// organizer profile can have a channel connected to them, and connecting
// again for a different organizer reassigns ownership rather than erroring.
func TestConnectByTelegramID_RequiresOrganizer(t *testing.T) {
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

	const (
		plainUserTelegramID = 900000201
		org1TelegramID      = 900000202
		org2TelegramID      = 900000203
		chatID              = -1000000000123
	)

	var plainUserID, org1UserID, org2UserID, org1ID, org2ID int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Plain User')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, plainUserTelegramID).Scan(&plainUserID))
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Org One')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, org1TelegramID).Scan(&org1UserID))
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Org Two')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, org2TelegramID).Scan(&org2UserID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Channel Org One')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, org1UserID).Scan(&org1ID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Channel Org Two')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, org2UserID).Scan(&org2ID))

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM channel_connections WHERE chat_id = $1`, chatID)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id IN ($1, $2)`, org1ID, org2ID)
		pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3)`, plainUserID, org1UserID, org2UserID)
	})

	repo := NewRepository(pool)

	t.Run("plain user without organizer profile cannot connect", func(t *testing.T) {
		name, ok, err := repo.ConnectByTelegramID(ctx, plainUserTelegramID, chatID, "Test Channel")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatalf("expected ok=false for non-organizer, got organizer %q", name)
		}
	})

	t.Run("organizer connects successfully", func(t *testing.T) {
		name, ok, err := repo.ConnectByTelegramID(ctx, org1TelegramID, chatID, "Test Channel")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || name != "Channel Org One" {
			t.Fatalf("got (%q, %v), want (\"Channel Org One\", true)", name, ok)
		}

		channels, err := repo.ListForOrganizer(ctx, org1ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(channels) != 1 || channels[0].ChatID != chatID {
			t.Fatalf("ListForOrganizer = %+v, want one channel with chat_id %d", channels, chatID)
		}
	})

	t.Run("reconnecting under a different organizer reassigns ownership", func(t *testing.T) {
		name, ok, err := repo.ConnectByTelegramID(ctx, org2TelegramID, chatID, "Test Channel Renamed")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || name != "Channel Org Two" {
			t.Fatalf("got (%q, %v), want (\"Channel Org Two\", true)", name, ok)
		}

		org1Channels, err := repo.ListForOrganizer(ctx, org1ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(org1Channels) != 0 {
			t.Errorf("org1 should no longer own the channel, got %+v", org1Channels)
		}
	})

	t.Run("disconnect removes the row", func(t *testing.T) {
		if err := repo.Disconnect(ctx, chatID); err != nil {
			t.Fatal(err)
		}
		channels, err := repo.ListForOrganizer(ctx, org2ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(channels) != 0 {
			t.Errorf("expected no channels after disconnect, got %+v", channels)
		}
	})
}
