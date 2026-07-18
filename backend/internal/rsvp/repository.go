package rsvp

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

type Ticket struct {
	ID          int64
	Code        string
	CheckedInAt *time.Time
}

type MyTicket struct {
	Ticket
	EventID      int64
	EventTitle   string
	EventStatus  string
	StartsAt     time.Time
	IsOnline     bool
	LocationName *string
	CitySlug     *string
	CoverURL     *string
}

type Attendee struct {
	UserID      int64
	Name        string
	Username    *string
	AvatarURL   *string
	RSVPAt      time.Time
	CheckedInAt *time.Time
}

// ScannedTicket carries everything the check-in flow needs to authorize
// and record a scan.
type ScannedTicket struct {
	TicketID         int64
	CheckedInAt      *time.Time
	AttendeeName     string
	RSVPStatus       string
	EventID          int64
	EventTitle       string
	EventOrganizerID int64
	EventStatus      string
}

// Join creates (or re-activates) an RSVP and its ticket inside one
// transaction. The event row is locked to serialize capacity checks.
func (r *Repository) Join(ctx context.Context, eventID, userID int64) (*Ticket, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	var status string
	var capacity *int32
	var startsAt time.Time
	err = tx.QueryRow(ctx,
		`SELECT status, capacity, starts_at FROM events WHERE id = $1 FOR UPDATE`,
		eventID).Scan(&status, &capacity, &startsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("lock event: %w", err)
	}
	if status != "published" {
		return nil, apperr.Conflict("this event is not open for RSVPs")
	}
	if !startsAt.After(time.Now()) {
		return nil, apperr.Conflict("this event has already started")
	}

	// Existing RSVP: re-activate if canceled, reject if already going.
	var rsvpID int64
	var rsvpStatus string
	err = tx.QueryRow(ctx,
		`SELECT id, status FROM rsvps WHERE event_id = $1 AND user_id = $2`,
		eventID, userID).Scan(&rsvpID, &rsvpStatus)
	switch {
	case err == nil && rsvpStatus == "going":
		return nil, apperr.Conflict("you have already joined this event")
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return nil, fmt.Errorf("check existing rsvp: %w", err)
	}

	if capacity != nil {
		var going int32
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM rsvps WHERE event_id = $1 AND status = 'going'`,
			eventID).Scan(&going); err != nil {
			return nil, fmt.Errorf("count rsvps: %w", err)
		}
		if going >= *capacity {
			return nil, apperr.Conflict("this event is full")
		}
	}

	if rsvpID != 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE rsvps SET status = 'going', updated_at = now() WHERE id = $1`,
			rsvpID); err != nil {
			return nil, fmt.Errorf("reactivate rsvp: %w", err)
		}
	} else {
		if err := tx.QueryRow(ctx,
			`INSERT INTO rsvps (event_id, user_id) VALUES ($1, $2) RETURNING id`,
			eventID, userID).Scan(&rsvpID); err != nil {
			return nil, fmt.Errorf("insert rsvp: %w", err)
		}
	}

	// One ticket per RSVP, kept across cancel/re-join.
	var t Ticket
	err = tx.QueryRow(ctx,
		`SELECT id, code, checked_in_at FROM tickets WHERE rsvp_id = $1`,
		rsvpID).Scan(&t.ID, &t.Code, &t.CheckedInAt)
	if errors.Is(err, pgx.ErrNoRows) {
		code, cerr := NewTicketCode()
		if cerr != nil {
			return nil, cerr
		}
		if err := tx.QueryRow(ctx,
			`INSERT INTO tickets (rsvp_id, code) VALUES ($1, $2) RETURNING id`,
			rsvpID, code).Scan(&t.ID); err != nil {
			return nil, fmt.Errorf("insert ticket: %w", err)
		}
		t.Code = code
	} else if err != nil {
		return nil, fmt.Errorf("load ticket: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return &t, nil
}

func (r *Repository) Cancel(ctx context.Context, eventID, userID int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE rsvps SET status = 'canceled', updated_at = now()
		 WHERE event_id = $1 AND user_id = $2 AND status = 'going'`,
		eventID, userID)
	if err != nil {
		return fmt.Errorf("cancel rsvp: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("you have not joined this event")
	}
	return nil
}

// GetMine returns the caller's active ticket for one event.
func (r *Repository) GetMine(ctx context.Context, eventID, userID int64) (*Ticket, error) {
	var t Ticket
	err := r.pool.QueryRow(ctx, `
		SELECT t.id, t.code, t.checked_in_at
		FROM rsvps rv JOIN tickets t ON t.rsvp_id = rv.id
		WHERE rv.event_id = $1 AND rv.user_id = $2 AND rv.status = 'going'`,
		eventID, userID).Scan(&t.ID, &t.Code, &t.CheckedInAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("no rsvp for this event")
	}
	if err != nil {
		return nil, fmt.Errorf("get my rsvp: %w", err)
	}
	return &t, nil
}

func (r *Repository) ListMyTickets(ctx context.Context, userID int64) ([]*MyTicket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.code, t.checked_in_at,
		       e.id, e.title, e.status, e.starts_at, e.is_online,
		       e.location_name, ci.slug, e.cover_url
		FROM rsvps rv
		JOIN tickets t ON t.rsvp_id = rv.id
		JOIN events e ON e.id = rv.event_id
		LEFT JOIN cities ci ON ci.id = e.city_id
		WHERE rv.user_id = $1 AND rv.status = 'going'
		ORDER BY e.starts_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list my tickets: %w", err)
	}
	defer rows.Close()

	tickets := make([]*MyTicket, 0, 8)
	for rows.Next() {
		var t MyTicket
		if err := rows.Scan(&t.ID, &t.Code, &t.CheckedInAt,
			&t.EventID, &t.EventTitle, &t.EventStatus, &t.StartsAt, &t.IsOnline,
			&t.LocationName, &t.CitySlug, &t.CoverURL); err != nil {
			return nil, err
		}
		tickets = append(tickets, &t)
	}
	return tickets, rows.Err()
}

func (r *Repository) GetByCode(ctx context.Context, code string) (*ScannedTicket, error) {
	var s ScannedTicket
	err := r.pool.QueryRow(ctx, `
		SELECT t.id, t.checked_in_at, u.name, rv.status,
		       e.id, e.title, e.organizer_id, e.status
		FROM tickets t
		JOIN rsvps rv ON rv.id = t.rsvp_id
		JOIN users u ON u.id = rv.user_id
		JOIN events e ON e.id = rv.event_id
		WHERE t.code = $1`, code).
		Scan(&s.TicketID, &s.CheckedInAt, &s.AttendeeName, &s.RSVPStatus,
			&s.EventID, &s.EventTitle, &s.EventOrganizerID, &s.EventStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("ticket not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get ticket by code: %w", err)
	}
	return &s, nil
}

func (r *Repository) MarkCheckedIn(ctx context.Context, ticketID int64) (time.Time, error) {
	var at time.Time
	err := r.pool.QueryRow(ctx, `
		UPDATE tickets SET checked_in_at = now()
		WHERE id = $1 AND checked_in_at IS NULL
		RETURNING checked_in_at`, ticketID).Scan(&at)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, apperr.Conflict("ticket already checked in")
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("mark checked in: %w", err)
	}
	return at, nil
}

func (r *Repository) ListAttendees(ctx context.Context, eventID int64) ([]*Attendee, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT u.id, u.name, u.username, u.avatar_url, rv.created_at, t.checked_in_at
		FROM rsvps rv
		JOIN users u ON u.id = rv.user_id
		LEFT JOIN tickets t ON t.rsvp_id = rv.id
		WHERE rv.event_id = $1 AND rv.status = 'going'
		ORDER BY rv.created_at`, eventID)
	if err != nil {
		return nil, fmt.Errorf("list attendees: %w", err)
	}
	defer rows.Close()

	attendees := make([]*Attendee, 0, 16)
	for rows.Next() {
		var a Attendee
		if err := rows.Scan(&a.UserID, &a.Name, &a.Username, &a.AvatarURL,
			&a.RSVPAt, &a.CheckedInAt); err != nil {
			return nil, err
		}
		attendees = append(attendees, &a)
	}
	return attendees, rows.Err()
}
