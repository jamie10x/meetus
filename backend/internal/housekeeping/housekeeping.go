// Package housekeeping runs periodic maintenance: closing out past events
// and purging expired refresh tokens.
package housekeeping

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Runner struct {
	pool *pgxpool.Pool
}

func NewRunner(pool *pgxpool.Pool) *Runner {
	return &Runner{pool: pool}
}

// Run executes all maintenance steps; failures are logged, not fatal —
// the next tick retries.
func (r *Runner) Run(ctx context.Context) {
	if n, err := r.finishPastEvents(ctx); err != nil {
		slog.Error("housekeeping: finish events failed", "err", err)
	} else if n > 0 {
		slog.Info("housekeeping: events finished", "count", n)
	}

	if n, err := r.purgeExpiredRefreshTokens(ctx); err != nil {
		slog.Error("housekeeping: purge tokens failed", "err", err)
	} else if n > 0 {
		slog.Info("housekeeping: refresh tokens purged", "count", n)
	}
}

// finishPastEvents flips published events to finished once they are over:
// past ends_at, or 4 hours past starts_at when no end time was given.
func (r *Runner) finishPastEvents(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE events SET status = 'finished', updated_at = now()
		WHERE status = 'published'
		  AND (
		      (ends_at IS NOT NULL AND ends_at < now())
		      OR (ends_at IS NULL AND starts_at < now() - interval '4 hours')
		  )`)
	if err != nil {
		return 0, fmt.Errorf("finish past events: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Runner) purgeExpiredRefreshTokens(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at < now()`)
	if err != nil {
		return 0, fmt.Errorf("purge refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}
