// Package outbox persists work alongside domain transactions. Delivery is
// at-least-once: a remote success followed by a crash can be retried.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"log/slog"
	"time"
)

type Executor interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func Enqueue(ctx context.Context, db Executor, key, kind string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO delivery_jobs(dedupe_key,kind,payload) VALUES($1,$2,$3) ON CONFLICT(dedupe_key) DO NOTHING`, key, kind, data)
	return err
}

type Job struct {
	ID      int64
	Kind    string
	Payload json.RawMessage
	Attempt int
}
type PermanentError struct{ Err error }

func (p *PermanentError) Error() string { return p.Err.Error() }
func Permanent(err error) error         { return &PermanentError{err} }
func Run(ctx context.Context, pool *pgxpool.Pool, deliver func(context.Context, Job) error, progress func(context.Context)) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		cycleCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		processed, err := ProcessOne(cycleCtx, pool, deliver)
		cancel()
		if err == nil && progress != nil {
			progress(ctx)
		}
		if err != nil {
			slog.Error("outbox processing failed", "error_type", fmt.Sprintf("%T", err))
		}
		if processed && err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func ProcessOne(ctx context.Context, pool *pgxpool.Pool, deliver func(context.Context, Job) error) (bool, error) {
	if _, err := pool.Exec(ctx, `UPDATE delivery_jobs SET failed_at=now(),lease_until=NULL WHERE attempts>=6 AND done_at IS NULL AND failed_at IS NULL AND lease_until<now()`); err != nil {
		return false, err
	}
	var j Job
	err := pool.QueryRow(ctx, `UPDATE delivery_jobs SET attempts=attempts+1,lease_until=now()+interval '90 seconds'
 WHERE id=(SELECT id FROM delivery_jobs WHERE done_at IS NULL AND failed_at IS NULL
 AND attempts<6 AND available_at<=now() AND (lease_until IS NULL OR lease_until<now())
 ORDER BY available_at,id FOR UPDATE SKIP LOCKED LIMIT 1)
 RETURNING id,kind,payload,attempts`).Scan(&j.ID, &j.Kind, &j.Payload, &j.Attempt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	jobCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	err = deliver(jobCtx, j)
	cancel()
	// Use a bounded independent context so shutdown still records completion.
	finishCtx, finishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer finishCancel()
	if err == nil {
		_, err = pool.Exec(finishCtx, `UPDATE delivery_jobs SET done_at=now(),lease_until=NULL WHERE id=$1 AND attempts=$2`, j.ID, j.Attempt)
		return true, err
	}
	var permanent *PermanentError
	terminal := errors.As(err, &permanent) || j.Attempt >= 6
	slog.Warn("delivery attempt failed", "job_id", j.ID, "kind", j.Kind, "attempt", j.Attempt, "terminal", terminal)
	_, err = pool.Exec(finishCtx, `UPDATE delivery_jobs SET lease_until=NULL,available_at=now()+make_interval(secs=>$3),failed_at=CASE WHEN $4 THEN now() END WHERE id=$1 AND attempts=$2`, j.ID, j.Attempt, float64(int64(1)<<uint(j.Attempt))*10, terminal)
	return true, err
}
