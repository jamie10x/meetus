package rsvp

import (
	"context"
	"testing"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestJoin_WaitlistAndPromotion exercises the full waitlist lifecycle
// against a real Postgres: joining a full event waitlists instead of
// erroring, and canceling a confirmed RSVP promotes the longest-waiting
// waitlisted attendee — issuing their ticket in the same transaction.
func TestJoin_WaitlistAndPromotion(t *testing.T) {
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

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	var organizerUserID, orgID, userAID, userBID, userCID, eventID int64
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000601, 'Waitlist Organizer')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&organizerUserID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Waitlist Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, organizerUserID).Scan(&orgID))
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000602, 'Attendee A')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userAID))
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000603, 'Attendee B')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userBID))
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000604, 'Attendee C')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userCID))
	must(pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status, is_online, capacity)
		VALUES ($1, 'Capacity One Event', 1, 1, now() + interval '2 days', 'published', false, 1)
		RETURNING id`, orgID).Scan(&eventID))

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM tickets WHERE rsvp_id IN (SELECT id FROM rsvps WHERE event_id = $1)`, eventID)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id = $1`, orgID)
		pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3, $4)`, organizerUserID, userAID, userBID, userCID)
	})

	repo := NewRepository(pool)

	// A joins the only spot.
	resA, err := repo.Join(ctx, eventID, userAID)
	must(err)
	if resA.Status != "going" {
		t.Fatalf("A: status = %q, want going", resA.Status)
	}
	if resA.Ticket == nil || resA.Ticket.Code == "" {
		t.Fatal("A: expected a ticket, got none")
	}

	// B joins a full event: waitlisted, no ticket.
	resB, err := repo.Join(ctx, eventID, userBID)
	must(err)
	if resB.Status != "waitlisted" {
		t.Fatalf("B: status = %q, want waitlisted", resB.Status)
	}
	if resB.Ticket != nil {
		t.Fatal("B: expected no ticket while waitlisted, got one")
	}

	mineB, err := repo.GetMine(ctx, eventID, userBID)
	must(err)
	if mineB.Status != "waitlisted" || mineB.Ticket != nil {
		t.Fatalf("B: GetMine = %+v, want waitlisted with no ticket", mineB)
	}

	// A cancels, freeing the spot: B should be promoted with a ticket.
	promotion, err := repo.Cancel(ctx, eventID, userAID)
	must(err)
	if promotion == nil {
		t.Fatal("expected a promotion after A canceled, got nil")
	}
	if promotion.UserID != userBID {
		t.Fatalf("promoted user = %d, want %d (B)", promotion.UserID, userBID)
	}
	if promotion.Ticket == nil || promotion.Ticket.Code == "" {
		t.Fatal("promotion: expected a ticket, got none")
	}

	mineB2, err := repo.GetMine(ctx, eventID, userBID)
	must(err)
	if mineB2.Status != "going" {
		t.Fatalf("B after promotion: status = %q, want going", mineB2.Status)
	}
	if mineB2.Ticket == nil || mineB2.Ticket.Code != promotion.Ticket.Code {
		t.Fatalf("B after promotion: ticket = %+v, want code %q", mineB2.Ticket, promotion.Ticket.Code)
	}

	// A has canceled and is no longer in the event.
	if _, err := repo.GetMine(ctx, eventID, userAID); err == nil {
		t.Fatal("A after canceling: expected NotFound, got a result")
	}

	// C joins the now-full (B occupies the one spot) event: waitlisted.
	resC, err := repo.Join(ctx, eventID, userCID)
	must(err)
	if resC.Status != "waitlisted" {
		t.Fatalf("C: status = %q, want waitlisted", resC.Status)
	}

	// C leaves the waitlist directly — nobody's spot opens up, so no
	// promotion should fire.
	noPromotion, err := repo.Cancel(ctx, eventID, userCID)
	must(err)
	if noPromotion != nil {
		t.Fatalf("expected no promotion when a waitlisted user leaves, got %+v", noPromotion)
	}
	if _, err := repo.GetMine(ctx, eventID, userCID); err == nil {
		t.Fatal("C after leaving waitlist: expected NotFound, got a result")
	}
}
