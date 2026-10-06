package event

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
	"strings"
	"testing"
	"time"
)

type queryCapture struct {
	sql  string
	args []any
}

func (q *queryCapture) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(strings.TrimSpace(data.SQL), "SELECT e.id") {
		q.sql = data.SQL
		q.args = append([]any(nil), data.Args...)
	}
	return ctx
}
func (q *queryCapture) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// Explicitly opt in using a disposable DB. Normal CI never seeds this volume.
// Captures the repository's actual SQL so measurements cannot silently drift
// away from the implementation. All synthetic rows are removed afterward.
func TestDiscoveryQueryPlans(t *testing.T) {
	url := os.Getenv("BENCHMARK_DATABASE_URL")
	if url == "" {
		t.Skip("set BENCHMARK_DATABASE_URL to a migrated disposable database")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	capture := &queryCapture{}
	cfg.ConnConfig.Tracer = capture
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var owner, org int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(telegram_id,name) VALUES($1,'Query benchmark') RETURNING id`, time.Now().UnixNano()).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO organizers(user_id,display_name) VALUES($1,'Query benchmark') RETURNING id`, owner).Scan(&org); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM rsvps WHERE event_id IN(SELECT id FROM events WHERE organizer_id=$1)`, org)
		pool.Exec(ctx, `DELETE FROM events WHERE organizer_id=$1`, org)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id=$1`, org)
		pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, owner)
	})
	_, err = pool.Exec(ctx, `INSERT INTO events(organizer_id,title,category_id,city_id,starts_at,is_online,status,visibility)
 SELECT $1,'Benchmark meetup '||n,1+(n%10),1+(n%14),now()+make_interval(days=>1+(n%30),hours=>n%24),false,
 CASE WHEN n%5=0 THEN 'draft' ELSE 'published' END::event_status,
 CASE WHEN n%10=1 THEN 'unlisted' ELSE 'public' END::event_visibility
 FROM generate_series(1,20000) n`, org)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO rsvps(event_id,user_id,status,created_at)
 SELECT id,$2,'going',now()-make_interval(days=>(id%10)::int) FROM events WHERE organizer_id=$1`, org, owner)
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"events", "rsvps"} {
		if _, err = pool.Exec(ctx, "ANALYZE "+table); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewRepository(pool)
	first, err := repo.ListPublic(ctx, ListFilters{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 20 || first.NextCursor == "" {
		t.Fatal("fixture missing public results")
	}
	cases := []struct {
		name string
		run  func() error
	}{
		{"discovery", func() error { _, e := repo.ListPublic(ctx, ListFilters{}); return e }},
		{"city", func() error { _, e := repo.ListPublic(ctx, ListFilters{CitySlug: "tashkent"}); return e }},
		{"search", func() error { _, e := repo.ListPublic(ctx, ListFilters{Query: "benchmark"}); return e }},
		{"continuation", func() error { _, e := repo.ListPublic(ctx, ListFilters{Cursor: first.NextCursor}); return e }},
		{"trending", func() error { _, e := repo.ListTrending(ctx, "", 6); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err != nil {
				t.Fatal(err)
			}
			rows, err := pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+capture.sql, capture.args...)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var plan []string
			for rows.Next() {
				var line string
				if err = rows.Scan(&line); err != nil {
					t.Fatal(err)
				}
				plan = append(plan, line)
			}
			if err = rows.Err(); err != nil {
				t.Fatal(err)
			}
			t.Log(fmt.Sprintf("%s (20,000 synthetic events, 20,000 RSVPs):\n%s", tc.name, strings.Join(plan, "\n")))
		})
	}
}
