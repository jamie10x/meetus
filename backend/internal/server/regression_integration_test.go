package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"meetus.uz/backend/internal/auth"
	"meetus.uz/backend/internal/config"
	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/feedback"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/outbox"
	"meetus.uz/backend/internal/rsvp"
	"meetus.uz/backend/internal/user"
	"net/http/httptest"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func regressionDB(t *testing.T) (*pgxpool.Pool, context.Context, int64, int64) {
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://meetus:meetus@localhost:5432/meetus?sslmode=disable"
	}
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		if os.Getenv("CI") != "" {
			t.Fatalf("postgres required in CI: %v", err)
		}
		t.Skipf("postgres unavailable: %v", err)
	}
	var uid, oid int64
	if err = pool.QueryRow(ctx, `INSERT INTO users(telegram_id,name) VALUES($1,'Regression') RETURNING id`, time.Now().UnixNano()).Scan(&uid); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `INSERT INTO organizers(user_id,display_name) VALUES($1,'Regression') RETURNING id`, uid).Scan(&oid); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM event_feedback WHERE event_id IN(SELECT id FROM events WHERE organizer_id=$1)`, oid)
		pool.Exec(ctx, `DELETE FROM tickets WHERE rsvp_id IN(SELECT id FROM rsvps WHERE event_id IN(SELECT id FROM events WHERE organizer_id=$1))`, oid)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE event_id IN(SELECT id FROM events WHERE organizer_id=$1)`, oid)
		pool.Exec(ctx, `DELETE FROM delivery_jobs WHERE payload->>'eventId' IN (SELECT id::text FROM events WHERE organizer_id=$1) OR payload->>'EventID' IN (SELECT id::text FROM events WHERE organizer_id=$1)`, oid)
		pool.Exec(ctx, `DELETE FROM events WHERE organizer_id=$1`, oid)
		pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE user_id=$1`, uid)
		pool.Exec(ctx, `DELETE FROM organizers WHERE id=$1`, oid)
		pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, uid)
		pool.Close()
	})
	return pool, ctx, uid, oid
}
func TestRefreshAtomicConsumptionAndRollback(t *testing.T) {
	pool, ctx, uid, _ := regressionDB(t)
	repo := auth.NewRepository(pool)
	expiry := time.Now().Add(time.Hour)
	if err := repo.StoreRefreshToken(ctx, uid, "original", expiry); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if repo.Rotate(ctx, "original", time.Now(), func(int64) (string, time.Time, error) { return "successor", expiry, nil }) == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("rotation successes=%d, want 1", successes.Load())
	}
	if err := repo.StoreRefreshToken(ctx, uid, "rollback", expiry); err != nil {
		t.Fatal(err)
	}
	if err := repo.Rotate(ctx, "rollback", time.Now(), func(int64) (string, time.Time, error) { return "successor", expiry, nil }); err == nil {
		t.Fatal("expected duplicate successor insertion failure")
	}
	if err := repo.Rotate(ctx, "rollback", time.Now(), func(int64) (string, time.Time, error) { return "recovered", expiry, nil }); err != nil {
		t.Fatalf("old token should survive rollback: %v", err)
	}
}
func TestLifecycleVisibilityAndCapacity(t *testing.T) {
	pool, ctx, uid, oid := regressionDB(t)
	repo := event.NewRepository(pool)
	service := event.NewService(repo)
	visibility := "unlisted"
	capacity := int32(2)
	input := event.Input{Title: "Regression", CategoryID: 1, IsOnline: true, StartsAt: time.Now().Add(time.Hour).Format(time.RFC3339), Visibility: &visibility, Capacity: &capacity}
	e, err := service.Create(ctx, oid, input)
	if err != nil {
		t.Fatal(err)
	}
	input.Visibility = nil
	input.Title = "Edited"
	e, err = service.Update(ctx, oid, e.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if e.Visibility != event.VisibilityUnlisted {
		t.Fatal("visibility was changed")
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if repo.Transition(ctx, e, event.StatusPublished) == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("publish winners=%d", wins.Load())
	}
	var jobs int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM delivery_jobs WHERE kind='publish' AND payload->>'eventId'=$1`, fmt.Sprint(e.ID)).Scan(&jobs); err != nil || jobs != 1 {
		t.Fatalf("publish jobs=%d err=%v", jobs, err)
	}
	rr := rsvp.NewRepository(pool)
	joined, err := rr.Join(ctx, e.ID, uid)
	if err != nil {
		t.Fatal(err)
	}
	rs := rsvp.NewService(rr, rsvp.NewTicketSigner("test-secret"))
	qr := rsvp.NewTicketSigner("test-secret").QRValue(joined.Ticket.Code)
	other, err := service.Create(ctx, oid, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Publish(ctx, oid, other.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = rs.CheckIn(ctx, oid, other.ID, qr); err == nil {
		t.Fatal("wrong event accepted")
	}
	if _, err = rs.CheckIn(ctx, oid, e.ID, qr); err != nil {
		t.Fatal(err)
	}
	if _, err = rs.CheckIn(ctx, oid, e.ID, qr); err == nil {
		t.Fatal("duplicate scan accepted")
	}
	if err = feedback.NewRepository(pool).Submit(ctx, e.ID, uid, 5); err == nil {
		t.Fatal("future feedback accepted")
	}
	if _, err = service.Cancel(ctx, oid, e.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = rs.CheckIn(ctx, oid, e.ID, qr); err == nil {
		t.Fatal("canceled event accepted")
	}
}
func TestProfileNullAndBan(t *testing.T) {
	pool, ctx, uid, _ := regressionDB(t)
	repo := user.NewRepository(pool)
	city := int32(1)
	district := "District"
	if _, err := repo.UpdateProfile(ctx, uid, user.ProfileUpdate{CityID: &city, District: &district}); err != nil {
		t.Fatal(err)
	}
	u, err := repo.UpdateProfile(ctx, uid, user.ProfileUpdate{CityIDSet: true, DistrictSet: true})
	if err != nil {
		t.Fatal(err)
	}
	if u.CityID != nil || u.District != nil {
		t.Fatal("explicit null did not clear fields")
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET is_banned=true WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	if err = repo.RequireActive(ctx, uid); err == nil {
		t.Fatal("banned account accepted")
	}
}

func TestDeliveryQueueDeduplicatesAndRecovers(t *testing.T) {
	pool, ctx, uid, _ := regressionDB(t)
	key := fmt.Sprintf("regression:%d", uid)
	t.Cleanup(func() { pool.Exec(ctx, `DELETE FROM delivery_jobs WHERE dedupe_key=$1`, key) })
	for i := 0; i < 2; i++ {
		if err := outbox.Enqueue(ctx, pool, key, "test", map[string]int64{"user": uid}); err != nil {
			t.Fatal(err)
		}
	}
	var id int64
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*),min(id) FROM delivery_jobs WHERE dedupe_key=$1`, key).Scan(&count, &id); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("dedupe failed")
	}
	// Prioritize our fixture without consuming unrelated queued jobs.
	pool.Exec(ctx, `UPDATE delivery_jobs SET available_at=now()-interval '1 year' WHERE id=$1`, id)
	ran := 0
	processed, err := outbox.ProcessOne(ctx, pool, func(_ context.Context, j outbox.Job) error {
		if j.ID != id {
			t.Fatalf("wrong job %d", j.ID)
		}
		ran++
		return fmt.Errorf("temporary failure")
	})
	if err != nil || !processed {
		t.Fatalf("process: %v", err)
	}
	pool.Exec(ctx, `UPDATE delivery_jobs SET available_at=now()-interval '1 year' WHERE id=$1`, id)
	_, err = outbox.ProcessOne(ctx, pool, func(_ context.Context, j outbox.Job) error {
		if j.ID != id {
			t.Fatalf("wrong job %d", j.ID)
		}
		ran++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var done bool
	pool.QueryRow(ctx, `SELECT done_at IS NOT NULL FROM delivery_jobs WHERE id=$1`, id).Scan(&done)
	if !done || ran != 2 {
		t.Fatal("retry did not complete")
	}
}

func TestWaitlistRejoinCapacityAndClosedEvent(t *testing.T) {
	pool, ctx, uid, oid := regressionDB(t)
	var second, third int64
	for _, dst := range []*int64{&second, &third} {
		if err := pool.QueryRow(ctx, `INSERT INTO users(telegram_id,name) VALUES($1,'Waiting') RETURNING id`, time.Now().UnixNano()).Scan(dst); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		pool.Exec(ctx, `DELETE FROM tickets WHERE rsvp_id IN(SELECT id FROM rsvps WHERE user_id IN($1,$2))`, second, third)
		pool.Exec(ctx, `DELETE FROM rsvps WHERE user_id IN($1,$2)`, second, third)
		pool.Exec(ctx, `DELETE FROM users WHERE id IN($1,$2)`, second, third)
	}()
	events := event.NewService(event.NewRepository(pool))
	capacity := int32(1)
	input := event.Input{Title: "Queue regression", CategoryID: 1, IsOnline: true, StartsAt: time.Now().Add(time.Hour).Format(time.RFC3339), Capacity: &capacity}
	e, err := events.Create(ctx, oid, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = events.Publish(ctx, oid, e.ID); err != nil {
		t.Fatal(err)
	}
	repo := rsvp.NewRepository(pool)
	for _, id := range []int64{uid, second, third} {
		if _, err = repo.Join(ctx, e.ID, id); err != nil {
			t.Fatal(err)
		}
	}
	larger := int32(2)
	input.Capacity = &larger
	if _, err = events.Update(ctx, oid, e.ID, input); err == nil {
		t.Fatal("capacity changed despite existing waitlist")
	}
	if _, err = repo.Cancel(ctx, e.ID, second); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Join(ctx, e.ID, second); err != nil {
		t.Fatal(err)
	}
	promoted, err := repo.Cancel(ctx, e.ID, uid)
	if err != nil {
		t.Fatal(err)
	}
	if promoted == nil || promoted.UserID != third {
		t.Fatalf("rejoin jumped queue: %+v", promoted)
	}
	if _, err = events.Cancel(ctx, oid, e.ID); err != nil {
		t.Fatal(err)
	}
	promoted, err = repo.Cancel(ctx, e.ID, third)
	if err != nil {
		t.Fatal(err)
	}
	if promoted != nil {
		t.Fatal("closed event promoted attendee")
	}
}

func TestManagementPaginationDoesNotDropRows(t *testing.T) {
	pool, ctx, _, oid := regressionDB(t)
	_, err := pool.Exec(ctx, `INSERT INTO events(organizer_id,title,category_id,is_online,starts_at) SELECT $1,'Page '||n,1,true,now()+interval '1 day' FROM generate_series(1,53) n`, oid)
	if err != nil {
		t.Fatal(err)
	}
	repo := event.NewRepository(pool)
	first, err := repo.ListByOrganizer(ctx, oid, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 50 {
		t.Fatalf("first page=%d", len(first))
	}
	secondPage, err := repo.ListByOrganizer(ctx, oid, first[49].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 3 || secondPage[0].ID >= first[49].ID {
		t.Fatal("missing or overlapping continuation")
	}
	if _, err = repo.ListForAdmin(ctx, "", 50, 0); err != nil {
		t.Fatalf("empty status filter: %v", err)
	}
}

func TestOperationsRequiresActiveAdmin(t *testing.T) {
	pool, ctx, uid, _ := regressionDB(t)
	cfg := &config.Config{JWTSecret: "operations-test", AccessTokenTTL: time.Hour, UploadDir: t.TempDir(), WebBaseURL: "http://localhost:3000", GeocodeBaseURL: "http://localhost"}
	router, err := New(Deps{Config: cfg, Pool: pool})
	if err != nil {
		t.Fatal(err)
	}
	token, err := authn.NewTokenManager(cfg.JWTSecret, time.Hour).IssueAccess(uid, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	request := func(authorized bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/api/admin/operations", nil)
		if authorized {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	if response := request(false); response.Code != 401 {
		t.Fatalf("anonymous status=%d", response.Code)
	}
	if response := request(true); response.Code != 403 {
		t.Fatalf("non-admin status=%d", response.Code)
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET is_admin=true WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	response := request(true)
	if response.Code != 200 {
		t.Fatalf("admin status=%d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data struct {
			Worker struct {
				Status string `json:"status"`
			} `json:"worker"`
			Delivery struct {
				Pending int64 `json:"pending"`
			} `json:"delivery"`
		} `json:"data"`
	}
	if err = json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Data.Worker.Status != "disabled" {
		t.Fatal("unconfigured worker status missing")
	}
	if _, err = pool.Exec(ctx, `UPDATE users SET is_banned=true WHERE id=$1`, uid); err != nil {
		t.Fatal(err)
	}
	if response := request(true); response.Code != 403 {
		t.Fatalf("banned admin status=%d", response.Code)
	}
}
