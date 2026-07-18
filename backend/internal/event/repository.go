package event

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/platform/apperr"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// eventSelect joins reference data and the live RSVP count.
const eventSelect = `
	SELECT e.id, e.organizer_id, e.title, e.description, e.category_id,
	       e.city_id, e.district, e.location_name, e.address, e.lat, e.lng,
	       e.is_online, e.starts_at, e.ends_at, e.capacity, e.cover_url,
	       e.status, e.visibility, e.created_at, e.updated_at,
	       o.display_name, c.slug, ci.slug,
	       (SELECT count(*) FROM rsvps r WHERE r.event_id = e.id AND r.status = 'going')::int
	FROM events e
	JOIN organizers o ON o.id = e.organizer_id
	JOIN categories c ON c.id = e.category_id
	LEFT JOIN cities ci ON ci.id = e.city_id
`

func scanEvent(row pgx.Row) (*Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.OrganizerID, &e.Title, &e.Description, &e.CategoryID,
		&e.CityID, &e.District, &e.LocationName, &e.Address, &e.Lat, &e.Lng,
		&e.IsOnline, &e.StartsAt, &e.EndsAt, &e.Capacity, &e.CoverURL,
		&e.Status, &e.Visibility, &e.CreatedAt, &e.UpdatedAt,
		&e.OrganizerName, &e.CategorySlug, &e.CitySlug, &e.GoingCount)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// mapWriteErr converts FK violations into user-facing validation errors.
func mapWriteErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return apperr.Validation("invalid categoryId or cityId")
		case "23514":
			return apperr.Validation("invalid field value")
		}
	}
	return err
}

type WriteFields struct {
	Title        string
	Description  string
	CategoryID   int32
	CityID       *int32
	District     *string
	LocationName *string
	Address      *string
	Lat          *float64
	Lng          *float64
	IsOnline     bool
	StartsAt     string // RFC3339, validated by service
	EndsAt       *string
	Capacity     *int32
	CoverURL     *string
	Visibility   Visibility
}

func (r *Repository) Create(ctx context.Context, organizerID int64, f WriteFields) (*Event, error) {
	var id int64
	err := r.pool.QueryRow(ctx, `
		INSERT INTO events (organizer_id, title, description, category_id, city_id,
			district, location_name, address, lat, lng, is_online,
			starts_at, ends_at, capacity, cover_url, visibility)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::timestamptz,$13::timestamptz,$14,$15,$16)
		RETURNING id`,
		organizerID, f.Title, f.Description, f.CategoryID, f.CityID,
		f.District, f.LocationName, f.Address, f.Lat, f.Lng, f.IsOnline,
		f.StartsAt, f.EndsAt, f.Capacity, f.CoverURL, f.Visibility).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("create event: %w", mapWriteErr(err))
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) Update(ctx context.Context, id int64, f WriteFields) (*Event, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE events SET
			title = $2, description = $3, category_id = $4, city_id = $5,
			district = $6, location_name = $7, address = $8, lat = $9, lng = $10,
			is_online = $11, starts_at = $12::timestamptz, ends_at = $13::timestamptz,
			capacity = $14, cover_url = $15, visibility = $16, updated_at = now()
		WHERE id = $1`,
		id, f.Title, f.Description, f.CategoryID, f.CityID,
		f.District, f.LocationName, f.Address, f.Lat, f.Lng, f.IsOnline,
		f.StartsAt, f.EndsAt, f.Capacity, f.CoverURL, f.Visibility)
	if err != nil {
		return nil, fmt.Errorf("update event: %w", mapWriteErr(err))
	}
	if tag.RowsAffected() == 0 {
		return nil, apperr.NotFound("event not found")
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*Event, error) {
	e, err := scanEvent(r.pool.QueryRow(ctx, eventSelect+` WHERE e.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get event: %w", err)
	}
	return e, nil
}

func (r *Repository) ListByOrganizer(ctx context.Context, organizerID int64) ([]*Event, error) {
	rows, err := r.pool.Query(ctx, eventSelect+`
		WHERE e.organizer_id = $1
		ORDER BY e.starts_at DESC`, organizerID)
	if err != nil {
		return nil, fmt.Errorf("list organizer events: %w", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

func collectEvents(rows pgx.Rows) ([]*Event, error) {
	events := make([]*Event, 0, 16)
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *Repository) SetStatus(ctx context.Context, id int64, status Status) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE events SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err != nil {
		return fmt.Errorf("set event status: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM events WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	return nil
}
