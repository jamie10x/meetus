package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/platform/apperr"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

type refreshToken struct {
	ID        int64
	UserID    int64
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (r *Repository) StoreRefreshToken(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("store refresh token: %w", err)
	}
	return nil
}

func (r *Repository) GetRefreshToken(ctx context.Context, tokenHash string) (*refreshToken, error) {
	var t refreshToken
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at
		FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash).Scan(&t.ID, &t.UserID, &t.ExpiresAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.Unauthorized("invalid refresh token")
	}
	if err != nil {
		return nil, fmt.Errorf("get refresh token: %w", err)
	}
	return &t, nil
}

func (r *Repository) RevokeRefreshToken(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke refresh token: %w", err)
	}
	return nil
}

// Rotate consumes and replaces a refresh token in one transaction. The row
// lock serializes concurrent consumers; rollback preserves the old token.
func (r *Repository) Rotate(ctx context.Context, oldHash string, now time.Time, issue func(int64) (string, time.Time, error)) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id, userID int64
	var banned bool
	err = tx.QueryRow(ctx, `SELECT rt.id, rt.user_id, u.is_banned
 FROM refresh_tokens rt JOIN users u ON u.id = rt.user_id
 WHERE rt.token_hash=$1 AND rt.revoked_at IS NULL AND rt.expires_at > $2
 FOR UPDATE OF rt`, oldHash, now).Scan(&id, &userID, &banned)
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.Unauthorized("invalid refresh token")
	}
	if err != nil {
		return err
	}
	if banned {
		return apperr.Forbidden("account is banned")
	}
	nextHash, expires, err := issue(userID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE refresh_tokens SET revoked_at=$2 WHERE id=$1`, id, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO refresh_tokens(user_id,token_hash,expires_at) VALUES($1,$2,$3)`, userID, nextHash, expires); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
