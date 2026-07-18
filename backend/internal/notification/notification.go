// Package notification finds due event reminders and records what has
// been sent, so each (event, user, kind) fires exactly once.
package notification

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Kind string

const (
	KindReminder24h Kind = "reminder_24h"
	KindReminder1h  Kind = "reminder_1h"
)

// windows defines how far ahead each reminder kind looks, and the minimum
// lead time (so the 24h reminder doesn't fire for events starting in
// minutes — the 1h reminder covers those).
var windows = map[Kind]struct {
	Ahead   time.Duration
	MinLead time.Duration
}{
	KindReminder24h: {Ahead: 24 * time.Hour, MinLead: 2 * time.Hour},
	KindReminder1h:  {Ahead: time.Hour, MinLead: 0},
}

type Reminder struct {
	Kind           Kind
	EventID        int64
	EventTitle     string
	StartsAt       time.Time
	LocationName   *string
	CitySlug       *string
	IsOnline       bool
	UserID         int64
	UserTelegramID int64
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Due returns reminders of one kind that should be sent now.
func (r *Repository) Due(ctx context.Context, kind Kind) ([]*Reminder, error) {
	w, ok := windows[kind]
	if !ok {
		return nil, fmt.Errorf("unknown reminder kind %q", kind)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT e.id, e.title, e.starts_at, e.location_name, ci.slug, e.is_online,
		       u.id, u.telegram_id
		FROM events e
		JOIN rsvps rv ON rv.event_id = e.id AND rv.status = 'going'
		JOIN users u ON u.id = rv.user_id
		LEFT JOIN cities ci ON ci.id = e.city_id
		WHERE e.status = 'published'
		  AND e.starts_at > now() + make_interval(secs => $2)
		  AND e.starts_at <= now() + make_interval(secs => $1)
		  AND NOT EXISTS (
		      SELECT 1 FROM notification_log nl
		      WHERE nl.event_id = e.id AND nl.user_id = u.id AND nl.kind = $3
		  )
		ORDER BY e.starts_at`,
		w.Ahead.Seconds(), w.MinLead.Seconds(), string(kind))
	if err != nil {
		return nil, fmt.Errorf("query due reminders: %w", err)
	}
	defer rows.Close()

	reminders := make([]*Reminder, 0, 32)
	for rows.Next() {
		rem := &Reminder{Kind: kind}
		if err := rows.Scan(&rem.EventID, &rem.EventTitle, &rem.StartsAt,
			&rem.LocationName, &rem.CitySlug, &rem.IsOnline,
			&rem.UserID, &rem.UserTelegramID); err != nil {
			return nil, err
		}
		reminders = append(reminders, rem)
	}
	return reminders, rows.Err()
}

// MarkSent records the send attempt. It is recorded even when Telegram
// rejects the message (e.g. the user never started the bot) so the worker
// does not retry forever.
func (r *Repository) MarkSent(ctx context.Context, rem *Reminder) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO notification_log (event_id, user_id, kind)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id, user_id, kind) DO NOTHING`,
		rem.EventID, rem.UserID, string(rem.Kind))
	if err != nil {
		return fmt.Errorf("mark sent: %w", err)
	}
	return nil
}
