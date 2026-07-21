package event

import (
	"context"
	"errors"
	"fmt"
	"time"

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
	       e.status, e.visibility, e.series_id, e.created_at, e.updated_at,
	       o.display_name, o.is_verified, c.slug, ci.slug,
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
		&e.Status, &e.Visibility, &e.SeriesID, &e.CreatedAt, &e.UpdatedAt,
		&e.OrganizerName, &e.OrganizerVerified, &e.CategorySlug, &e.CitySlug, &e.GoingCount)
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

// CreateSeries creates a weekly-recurring series: `occurrences` events,
// one week apart starting at f.StartsAt, all sharing a series_id equal
// to the first occurrence's own ID — no separate sequence/table needed
// for the grouping key. Returns every created event, first occurrence
// first.
func (r *Repository) CreateSeries(ctx context.Context, organizerID int64, f WriteFields, occurrences int) ([]*Event, error) {
	startsAt, err := time.Parse(time.RFC3339, f.StartsAt)
	if err != nil {
		return nil, fmt.Errorf("parse startsAt: %w", err)
	}
	var endsAt *time.Time
	if f.EndsAt != nil {
		t, err := time.Parse(time.RFC3339, *f.EndsAt)
		if err != nil {
			return nil, fmt.Errorf("parse endsAt: %w", err)
		}
		endsAt = &t
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx)

	ids := make([]int64, 0, occurrences)
	for i := 0; i < occurrences; i++ {
		offset := time.Duration(i) * 7 * 24 * time.Hour
		occStarts := startsAt.Add(offset).Format(time.RFC3339)
		var occEnds *string
		if endsAt != nil {
			s := endsAt.Add(offset).Format(time.RFC3339)
			occEnds = &s
		}

		var id int64
		err := tx.QueryRow(ctx, `
			INSERT INTO events (organizer_id, title, description, category_id, city_id,
				district, location_name, address, lat, lng, is_online,
				starts_at, ends_at, capacity, cover_url, visibility)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::timestamptz,$13::timestamptz,$14,$15,$16)
			RETURNING id`,
			organizerID, f.Title, f.Description, f.CategoryID, f.CityID,
			f.District, f.LocationName, f.Address, f.Lat, f.Lng, f.IsOnline,
			occStarts, occEnds, f.Capacity, f.CoverURL, f.Visibility).Scan(&id)
		if err != nil {
			return nil, fmt.Errorf("create series event: %w", mapWriteErr(err))
		}
		ids = append(ids, id)
	}

	seriesID := ids[0]
	if _, err := tx.Exec(ctx,
		`UPDATE events SET series_id = $1 WHERE id = ANY($2)`,
		seriesID, ids); err != nil {
		return nil, fmt.Errorf("set series id: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	events := make([]*Event, 0, occurrences)
	for _, id := range ids {
		e, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, nil
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

// ListForAdmin returns events in any status (optionally filtered),
// newest first. Admin-only — never expose through public routes.
func (r *Repository) ListForAdmin(ctx context.Context, status string, limit int) ([]*Event, error) {
	rows, err := r.pool.Query(ctx, eventSelect+`
		WHERE $1 = '' OR e.status = $1::event_status
		ORDER BY e.created_at DESC
		LIMIT $2`, status, limit)
	if err != nil {
		return nil, fmt.Errorf("list events for admin: %w", err)
	}
	defer rows.Close()
	return collectEvents(rows)
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
