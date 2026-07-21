package event

import (
	"context"
	"testing"
	"time"

	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/platform/db"
)

// TestListNearby_DistanceFiltering exercises the haversine query against a
// real Postgres: an event a few hundred meters from the search point must
// be found, one ~275km away (Tashkent to Samarkand) must not.
func TestListNearby_DistanceFiltering(t *testing.T) {
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

	const telegramID = 900000501
	var userID, orgID, nearEventID, farEventID int64
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Nearby Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, telegramID).Scan(&userID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Nearby Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID))

	// Tashkent center-ish point; "near" is a few hundred meters off,
	// "far" is Samarkand (~275km away).
	const searchLat, searchLng = 41.3111, 69.2797
	must(pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status, is_online, lat, lng)
		VALUES ($1, 'Nearby Event', 1, 1, now() + interval '2 days', 'published', false, 41.3150, 69.2830)
		RETURNING id`, orgID).Scan(&nearEventID))
	must(pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status, is_online, lat, lng)
		VALUES ($1, 'Far Event', 1, 1, now() + interval '2 days', 'published', false, 39.6542, 66.9597)
		RETURNING id`, orgID).Scan(&farEventID))

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM events WHERE id IN ($1, $2)`, nearEventID, farEventID)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id = $1`, orgID)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	repo := NewRepository(pool)
	results, err := repo.ListNearby(ctx, searchLat, searchLng, 15, 10)
	if err != nil {
		t.Fatal(err)
	}

	var foundNear, foundFar bool
	for _, ne := range results {
		if ne.ID == nearEventID {
			foundNear = true
			if ne.DistanceKm > 1 {
				t.Errorf("near event distance = %.2fkm, want < 1km", ne.DistanceKm)
			}
		}
		if ne.ID == farEventID {
			foundFar = true
		}
	}
	if !foundNear {
		t.Error("expected the near event in results, not found")
	}
	if foundFar {
		t.Error("far event (Samarkand, ~275km away) should be excluded by the 15km radius")
	}
}

// TestListRelated_RankingAndFiltering exercises the relevance tiering
// against a real Postgres: category+city match ranks above a
// category-only or city-only match, and an event sharing neither is
// excluded entirely.
func TestListRelated_RankingAndFiltering(t *testing.T) {
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

	const telegramID = 900000701
	var userID, orgID int64
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Related Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, telegramID).Scan(&userID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Related Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID))

	// category 1 = tech, category 2 = education; city 1 = Tashkent, city 2 = Samarkand.
	insert := func(title string, categoryID, cityID int32, leadDays int) int64 {
		var id int64
		must(pool.QueryRow(ctx, `
			INSERT INTO events (organizer_id, title, category_id, city_id, starts_at, status, is_online)
			VALUES ($1, $2, $3, $4, now() + make_interval(days => $5), 'published', false)
			RETURNING id`, orgID, title, categoryID, cityID, leadDays).Scan(&id))
		return id
	}

	targetID := insert("Target Event", 1, 1, 1)
	bothMatchID := insert("Category+City Match", 1, 1, 2)
	categoryOnlyID := insert("Category Only Match", 1, 2, 3)
	cityOnlyID := insert("City Only Match", 2, 1, 4)
	unrelatedID := insert("Unrelated Event", 2, 2, 5)

	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM events WHERE id IN ($1, $2, $3, $4, $5)`,
			targetID, bothMatchID, categoryOnlyID, cityOnlyID, unrelatedID)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id = $1`, orgID)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	repo := NewRepository(pool)
	cityID := int32(1)
	// limit=10 rather than the default 4: the dev DB may carry other
	// published events sharing category 1 or city 1 (seed/demo data,
	// earlier smoke tests) — this test doesn't assume a clean slate, only
	// that the events it created rank and filter correctly relative to
	// each other and to whatever else shows up.
	results, err := repo.ListRelated(ctx, targetID, 1, &cityID, 10)
	if err != nil {
		t.Fatal(err)
	}

	indexOf := func(id int64) int {
		for i, re := range results {
			if re.ID == id {
				return i
			}
		}
		return -1
	}
	bothIdx, categoryIdx, cityIdx := indexOf(bothMatchID), indexOf(categoryOnlyID), indexOf(cityOnlyID)
	if bothIdx == -1 || categoryIdx == -1 || cityIdx == -1 {
		t.Fatalf("expected all three related events present, got indices both=%d category=%d city=%d in %d results",
			bothIdx, categoryIdx, cityIdx, len(results))
	}
	if bothIdx > categoryIdx || bothIdx > cityIdx {
		t.Errorf("category+city match (index %d) should rank before category-only (index %d) and city-only (index %d)",
			bothIdx, categoryIdx, cityIdx)
	}
	if indexOf(targetID) != -1 {
		t.Error("target event should not appear in its own related list")
	}
	if indexOf(unrelatedID) != -1 {
		t.Error("event sharing neither category nor city should be excluded")
	}
}

// TestCreateSeries_WeeklyOccurrencesAndListing exercises the recurring
// weekly series against a real Postgres: CreateSeries produces the
// right number of events one week apart, all sharing a series_id, and
// ListSeries returns the published siblings of one instance (excluding
// itself and any still-draft occurrence).
func TestCreateSeries_WeeklyOccurrencesAndListing(t *testing.T) {
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

	const telegramID = 900000801
	var userID, orgID int64
	must(pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name) VALUES ($1, 'Series Test')
		ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, telegramID).Scan(&userID))
	must(pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name) VALUES ($1, 'Series Org')
		ON CONFLICT (user_id) DO UPDATE SET display_name = EXCLUDED.display_name
		RETURNING id`, userID).Scan(&orgID))

	repo := NewRepository(pool)
	start := time.Now().Add(48 * time.Hour).Format(time.RFC3339)
	events, err := repo.CreateSeries(ctx, orgID, WriteFields{
		Title:      "Weekly Series Test",
		CategoryID: 1,
		CityID:     ptrInt32(1),
		IsOnline:   false,
		StartsAt:   start,
		Visibility: VisibilityPublic,
	}, 4)
	must(err)

	t.Cleanup(func() {
		ids := make([]int64, len(events))
		for i, e := range events {
			ids[i] = e.ID
		}
		pool.Exec(ctx, `DELETE FROM events WHERE id = ANY($1)`, ids)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id = $1`, orgID)
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}
	seriesID := events[0].ID
	for i, e := range events {
		if e.SeriesID == nil || *e.SeriesID != seriesID {
			t.Errorf("events[%d].SeriesID = %v, want %d", i, e.SeriesID, seriesID)
		}
		if i > 0 {
			gotGap := e.StartsAt.Sub(events[i-1].StartsAt)
			wantGap := 7 * 24 * time.Hour
			if gotGap != wantGap {
				t.Errorf("events[%d]-events[%d] gap = %v, want %v", i, i-1, gotGap, wantGap)
			}
		}
	}

	// All still drafts (CreateSeries doesn't publish), so ListSeries
	// (published-only) should find none yet.
	none, err := repo.ListSeries(ctx, seriesID, events[0].ID)
	must(err)
	if len(none) != 0 {
		t.Fatalf("expected no series siblings before publishing, got %d", len(none))
	}

	// Publish all but the first; ListSeries from the first's perspective
	// should then find exactly the other three, soonest first.
	for _, e := range events[1:] {
		must(repo.SetStatus(ctx, e.ID, StatusPublished))
	}
	siblings, err := repo.ListSeries(ctx, seriesID, events[0].ID)
	must(err)
	if len(siblings) != 3 {
		t.Fatalf("got %d siblings, want 3", len(siblings))
	}
	for i, s := range siblings {
		if s.ID != events[i+1].ID {
			t.Errorf("siblings[%d].ID = %d, want %d (soonest-first order)", i, s.ID, events[i+1].ID)
		}
	}
}

func ptrInt32(v int32) *int32 { return &v }
