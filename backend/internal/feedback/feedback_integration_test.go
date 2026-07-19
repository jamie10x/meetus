package feedback

import (
	"context"
	"testing"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestSubmitSetCommentAndListComments exercises the rating + free-text
// comment flow end to end: a rating must exist (via an RSVP) before a
// comment can attach to it, and ListComments only ever returns rows that
// actually have a comment.
func TestSubmitSetCommentAndListComments(t *testing.T) {
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

	var userID, orgID, eventID int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES (900000301, 'Comment Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`).Scan(&userID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Comment Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID))
	must(pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status)
		VALUES ($1, 'Feedback Comment Event', 1, 1, now() - interval '1 day', 'finished')
		RETURNING id`, orgID).Scan(&eventID))
	_, err = pool.Exec(ctx, `INSERT INTO rsvps (event_id, user_id) VALUES ($1, $2)`, eventID, userID)
	must(err)

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM event_feedback WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE event_id = $1`, eventID)
		pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, eventID)
	})

	repo := NewRepository(pool)

	if err := repo.Submit(ctx, eventID, userID, 5); err != nil {
		t.Fatalf("submit: %v", err)
	}

	comments, err := repo.ListComments(ctx, eventID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 0 {
		t.Fatalf("ListComments before SetComment = %+v, want empty", comments)
	}

	if err := repo.SetComment(ctx, eventID, userID, "Great meetup!"); err != nil {
		t.Fatalf("set comment: %v", err)
	}

	comments, err = repo.ListComments(ctx, eventID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 {
		t.Fatalf("ListComments after SetComment = %+v, want 1 entry", comments)
	}
	c := comments[0]
	if c.Comment != "Great meetup!" || c.Rating != 5 || c.UserName != "Comment Test" {
		t.Fatalf("comment = %+v, want {Great meetup! 5 Comment Test ...}", c)
	}
}
