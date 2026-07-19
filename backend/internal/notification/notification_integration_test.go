package notification

import (
	"context"
	"testing"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestDueAndMarkSent exercises the reminder window query against a real
// PostgreSQL (the dev docker-compose instance). Skipped when unreachable.
func TestDueAndMarkSent(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Skipf("postgres unavailable: %v", err)
	}
	// Registered before the data cleanup below: t.Cleanup runs LIFO, so the
	// pool closes after the fixture rows are deleted. (A defer would close
	// it before any t.Cleanup runs.)
	t.Cleanup(pool.Close)

	// Self-contained fixtures with a unique telegram_id, removed on cleanup.
	var userID, orgID, eventID, rsvpID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000001, 'Reminder Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Reminder Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status)
		VALUES ($1, 'Reminder Integration Event', 1, 1, now() + interval '30 minutes', 'published')
		RETURNING id`, orgID).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO rsvps (event_id, user_id) VALUES ($1, $2) RETURNING id`,
		eventID, userID).Scan(&rsvpID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM notification_log WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE id = $1`, rsvpID)
		pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	})

	repo := NewRepository(pool)

	find := func(kind Kind) *Reminder {
		due, err := repo.Due(ctx, kind)
		if err != nil {
			t.Fatalf("due %s: %v", kind, err)
		}
		for _, d := range due {
			if d.EventID == eventID {
				return d
			}
		}
		return nil
	}

	// Event starts in 30 min: the 1h reminder is due, the 24h one is not
	// (30 min lead is under its 2h minimum).
	if find(KindReminder24h) != nil {
		t.Error("24h reminder should not fire for an event 30 minutes away")
	}
	rem := find(KindReminder1h)
	if rem == nil {
		t.Fatal("1h reminder not found for event 30 minutes away")
	}
	if rem.UserTelegramID != 900000001 {
		t.Errorf("telegram id = %d, want 900000001", rem.UserTelegramID)
	}

	if err := repo.MarkSent(ctx, rem); err != nil {
		t.Fatal(err)
	}
	if find(KindReminder1h) != nil {
		t.Error("1h reminder fired again after MarkSent")
	}

	// Move the event ~20h out: now the 24h reminder becomes due.
	if _, err := pool.Exec(ctx,
		`UPDATE events SET starts_at = now() + interval '20 hours' WHERE id = $1`,
		eventID); err != nil {
		t.Fatal(err)
	}
	if find(KindReminder24h) == nil {
		t.Error("24h reminder not found for event 20 hours away")
	}

	// UserLanguage should ride along with the reminder for bot rendering.
	if rem.UserLanguage != "uz" {
		t.Errorf("UserLanguage = %q, want the column default %q", rem.UserLanguage, "uz")
	}
}

// TestDueFeedbackAndMarkSent exercises the post-event feedback prompt
// query: only finished events with a "going" RSVP, deduped by
// notification_log, are returned.
func TestDueFeedbackAndMarkSent(t *testing.T) {
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

	var userID, orgID, eventID, rsvpID int64
	err = pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000002, 'Feedback Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Feedback Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status)
		VALUES ($1, 'Feedback Integration Event', 1, 1, now() - interval '5 hours', 'published')
		RETURNING id`, orgID).Scan(&eventID)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `
		INSERT INTO rsvps (event_id, user_id) VALUES ($1, $2) RETURNING id`,
		eventID, userID).Scan(&rsvpID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM notification_log WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM event_feedback WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE id = $1`, rsvpID)
		pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	})

	repo := NewRepository(pool)

	find := func() *FeedbackDue {
		due, err := repo.DueFeedback(ctx)
		if err != nil {
			t.Fatalf("due feedback: %v", err)
		}
		for _, d := range due {
			if d.EventID == eventID {
				return d
			}
		}
		return nil
	}

	// Still published (though past its start time): not due yet — only
	// housekeeping flipping it to "finished" makes it eligible.
	if find() != nil {
		t.Error("feedback should not be due before the event is marked finished")
	}

	if _, err := pool.Exec(ctx,
		`UPDATE events SET status = 'finished' WHERE id = $1`, eventID); err != nil {
		t.Fatal(err)
	}

	f := find()
	if f == nil {
		t.Fatal("feedback prompt not found for finished event with a going RSVP")
	}
	if f.UserTelegramID != 900000002 {
		t.Errorf("telegram id = %d, want 900000002", f.UserTelegramID)
	}

	if err := repo.MarkFeedbackSent(ctx, f); err != nil {
		t.Fatal(err)
	}
	if find() != nil {
		t.Error("feedback prompt fired again after MarkFeedbackSent")
	}
}
