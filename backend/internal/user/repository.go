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

const userColumns = `id, telegram_id, name, username, avatar_url, city_id, district, language, is_banned, is_admin, notifications_muted, weekly_digest_enabled, created_at, updated_at`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.TelegramID, &u.Name, &u.Username, &u.AvatarURL,
		&u.CityID, &u.District, &u.Language, &u.IsBanned, &u.IsAdmin,
		&u.NotificationsMuted, &u.WeeklyDigestEnabled, &u.CreatedAt, &u.UpdatedAt)
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

// SetNotificationsMuted toggles whether reminders and feedback prompts are
// sent to this user (bot-only setting, via /mute and /unmute).
func (r *Repository) SetNotificationsMuted(ctx context.Context, id int64, muted bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET notifications_muted = $2 WHERE id = $1`, id, muted)
	if err != nil {
		return fmt.Errorf("set notifications muted: %w", err)
	}
	return nil
}

// SetWeeklyDigest toggles the opt-in weekly "what's on" summary (bot-only
// setting, via /digest).
func (r *Repository) SetWeeklyDigest(ctx context.Context, id int64, enabled bool) error {
	_, err := r.pool.Exec(ctx, `UPDATE users SET weekly_digest_enabled = $2 WHERE id = $1`, id, enabled)
	if err != nil {
		return fmt.Errorf("set weekly digest: %w", err)
	}
	return nil
}

// DigestSubscriber is the slice of a user's profile the weekly digest
// send needs: who to message, in what language, and which city (if any)
// to scope the "what's on this week" listing to.
type DigestSubscriber struct {
	UserID     int64
	TelegramID int64
	Language   string
	CityID     *int32
}

// ListWeeklyDigestSubscribers returns every user opted into the weekly
// digest. A muted user is excluded even if they opted in — /mute is the
// user's explicit "stop messaging me" signal and should win over a
// standing preference set earlier.
func (r *Repository) ListWeeklyDigestSubscribers(ctx context.Context) ([]*DigestSubscriber, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, telegram_id, language, city_id
		FROM users
		WHERE weekly_digest_enabled = TRUE AND notifications_muted = FALSE`)
	if err != nil {
		return nil, fmt.Errorf("list weekly digest subscribers: %w", err)
	}
	defer rows.Close()

	subs := make([]*DigestSubscriber, 0, 32)
	for rows.Next() {
		s := &DigestSubscriber{}
		if err := rows.Scan(&s.UserID, &s.TelegramID, &s.Language, &s.CityID); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}
	return subs, rows.Err()
}
