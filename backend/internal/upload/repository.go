package upload

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func NewRepository(pool *pgxpool.Pool) *Repository { return &Repository{pool} }

// ReferencedURLs includes drafts, canceled events and inactive users: none of
// those states gives us permission to discard their associated images.
func (r *Repository) ReferencedURLs(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT cover_url FROM events WHERE cover_url IS NOT NULL UNION SELECT avatar_url FROM users WHERE avatar_url IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var urls []string
	for rows.Next() {
		var value string
		if err = rows.Scan(&value); err != nil {
			return nil, err
		}
		urls = append(urls, value)
	}
	return urls, rows.Err()
}
