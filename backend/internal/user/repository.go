package user

import (
	"context"
	"errors"
	"fmt"

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

const userColumns = `id, telegram_id, name, username, avatar_url, city_id, district, language, is_banned, is_admin, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.TelegramID, &u.Name, &u.Username, &u.AvatarURL,
		&u.CityID, &u.District, &u.Language, &u.IsBanned, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertTelegramUser creates the user on first login and refreshes
// name/username/avatar on subsequent logins.
func (r *Repository) UpsertTelegramUser(ctx context.Context, p TelegramProfile) (*User, error) {
	// language is set from $5 on INSERT only — deliberately absent from
	// the DO UPDATE SET list, so an existing user's language choice is
	// never overwritten by a later login.
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (telegram_id, name, username, avatar_url, language)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
		ON CONFLICT (telegram_id) DO UPDATE SET
			name       = EXCLUDED.name,
			username   = COALESCE(EXCLUDED.username, users.username),
			avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
			updated_at = now()
		RETURNING `+userColumns,
		p.TelegramID, p.Name, p.Username, p.AvatarURL, p.Language)

	u, err := scanUser(row)
	if err != nil {
		return nil, fmt.Errorf("upsert telegram user: %w", err)
	}
	return u, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*User, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	return u, nil
}

type ProfileUpdate struct {
	Name     *string
	CityID   *int32
	District *string
	Language *string
}

func (r *Repository) UpdateProfile(ctx context.Context, id int64, p ProfileUpdate) (*User, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE users SET
			name       = COALESCE($2, name),
			city_id    = COALESCE($3, city_id),
			district   = COALESCE($4, district),
			language   = COALESCE($5, language),
			updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns,
		id, p.Name, p.CityID, p.District, p.Language)

	u, err := scanUser(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}
	return u, nil
}
