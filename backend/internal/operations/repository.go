package operations

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"
)

type Delivery struct {
	Pending         int64      `json:"pending"`
	Failed          int64      `json:"failed"`
	OldestPendingAt *time.Time `json:"oldestPendingAt"`
}
type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool} }
func (r *Repository) Delivery(ctx context.Context) (Delivery, error) {
	var result Delivery
	err := r.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE done_at IS NULL AND failed_at IS NULL), count(*) FILTER (WHERE failed_at IS NOT NULL), min(created_at) FILTER (WHERE done_at IS NULL AND failed_at IS NULL) FROM delivery_jobs`).Scan(&result.Pending, &result.Failed, &result.OldestPendingAt)
	return result, err
}
