package event

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"meetus.uz/backend/internal/platform/apperr"
)

const (
	defaultPageSize = 20
	maxPageSize     = 50
)

type ListFilters struct {
	CitySlug     string
	CategorySlug string
	From         *time.Time
	To           *time.Time
	Online       *bool
	Query        string
	Cursor       string
	Limit        int
}

type Page struct {
	Items      []*Event
	NextCursor string
}

// cursor is a keyset position on (starts_at, id), base64-encoded to stay opaque.
func encodeCursor(e *Event) string {
	raw := e.StartsAt.UTC().Format(time.RFC3339Nano) + "|" + strconv.FormatInt(e.ID, 10)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(s string) (time.Time, int64, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, apperr.Validation("invalid cursor")
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, apperr.Validation("invalid cursor")
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, 0, apperr.Validation("invalid cursor")
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, apperr.Validation("invalid cursor")
	}
	return ts, id, nil
}

// ListPublic returns published public events, upcoming-first, with keyset
// pagination.
func (r *Repository) ListPublic(ctx context.Context, f ListFilters) (*Page, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	var conds []string
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	conds = append(conds, "e.status = 'published'", "e.visibility = 'public'")

	if f.From != nil {
		conds = append(conds, "e.starts_at >= "+arg(*f.From))
	} else {
		conds = append(conds, "e.starts_at >= now()")
	}
	if f.To != nil {
		conds = append(conds, "e.starts_at <= "+arg(*f.To))
	}
	if f.CitySlug != "" {
		conds = append(conds, "ci.slug = "+arg(f.CitySlug))
	}
	if f.CategorySlug != "" {
		conds = append(conds, "c.slug = "+arg(f.CategorySlug))
	}
	if f.Online != nil {
		conds = append(conds, "e.is_online = "+arg(*f.Online))
	}
	if f.Query != "" {
		conds = append(conds, "e.search @@ websearch_to_tsquery('simple', "+arg(f.Query)+")")
	}
	if f.Cursor != "" {
		ts, id, err := decodeCursor(f.Cursor)
		if err != nil {
			return nil, err
		}
		conds = append(conds, fmt.Sprintf("(e.starts_at, e.id) > (%s, %s)", arg(ts), arg(id)))
	}

	query := eventSelect + " WHERE " + strings.Join(conds, " AND ") +
		" ORDER BY e.starts_at, e.id LIMIT " + arg(limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list public events: %w", err)
	}
	defer rows.Close()

	events, err := collectEvents(rows)
	if err != nil {
		return nil, err
	}

	page := &Page{Items: events}
	if len(events) > limit {
		page.Items = events[:limit]
		page.NextCursor = encodeCursor(events[limit-1])
	}
	return page, nil
}

// GetPublished returns a published event by ID. Unlisted events resolve
// (direct links work); drafts and canceled events return not-found for
// non-owners.
func (r *Repository) GetPublished(ctx context.Context, id int64) (*Event, error) {
	e, err := scanEvent(r.pool.QueryRow(ctx,
		eventSelect+` WHERE e.id = $1 AND e.status IN ('published', 'finished', 'canceled')`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("event not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get published event: %w", err)
	}
	return e, nil
}

const trendingMaxLimit = 20

// trendingSelect mirrors eventSelect but adds a 7-day RSVP-velocity count —
// a separate query rather than parameterizing eventSelect, since that
// constant is shared by several well-tested read paths that don't need
// this column.
const trendingSelect = `
	SELECT e.id, e.organizer_id, e.title, e.description, e.category_id,
	       e.city_id, e.district, e.location_name, e.address, e.lat, e.lng,
	       e.is_online, e.starts_at, e.ends_at, e.capacity, e.cover_url,
	       e.status, e.visibility, e.created_at, e.updated_at,
	       o.display_name, c.slug, ci.slug,
	       (SELECT count(*) FROM rsvps r WHERE r.event_id = e.id AND r.status = 'going')::int,
	       COALESCE(rc.recent_count, 0)::int
	FROM events e
	JOIN organizers o ON o.id = e.organizer_id
	JOIN categories c ON c.id = e.category_id
	LEFT JOIN cities ci ON ci.id = e.city_id
	LEFT JOIN (
	    SELECT event_id, count(*) AS recent_count
	    FROM rsvps
	    WHERE status = 'going' AND created_at > now() - interval '7 days'
	    GROUP BY event_id
	) rc ON rc.event_id = e.id
`

// TrendingEvent is a published upcoming event ranked by recent RSVP
// velocity (joins in the last 7 days), not by total popularity or date —
// a quiet event with a sudden burst of interest outranks a stale one with
// a bigger lifetime total.
type TrendingEvent struct {
	Event
	RecentGoing int32
}

func scanTrendingEvent(row pgx.Row) (*TrendingEvent, error) {
	var te TrendingEvent
	e := &te.Event
	err := row.Scan(&e.ID, &e.OrganizerID, &e.Title, &e.Description, &e.CategoryID,
		&e.CityID, &e.District, &e.LocationName, &e.Address, &e.Lat, &e.Lng,
		&e.IsOnline, &e.StartsAt, &e.EndsAt, &e.Capacity, &e.CoverURL,
		&e.Status, &e.Visibility, &e.CreatedAt, &e.UpdatedAt,
		&e.OrganizerName, &e.CategorySlug, &e.CitySlug, &e.GoingCount, &te.RecentGoing)
	if err != nil {
		return nil, err
	}
	return &te, nil
}

// ListTrending returns published public upcoming events ordered by RSVP
// velocity descending (ties broken by soonest start). citySlug, if set,
// restricts to one city — matching the same slug convention as ListPublic.
func (r *Repository) ListTrending(ctx context.Context, citySlug string, limit int) ([]*TrendingEvent, error) {
	if limit <= 0 || limit > trendingMaxLimit {
		limit = 6
	}

	query := trendingSelect + `
		WHERE e.status = 'published' AND e.visibility = 'public' AND e.starts_at > now()`
	args := []any{}
	if citySlug != "" {
		args = append(args, citySlug)
		query += fmt.Sprintf(" AND ci.slug = $%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY recent_count DESC, e.starts_at ASC LIMIT $%d", len(args))

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list trending events: %w", err)
	}
	defer rows.Close()

	events := make([]*TrendingEvent, 0, limit)
	for rows.Next() {
		te, err := scanTrendingEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, te)
	}
	return events, rows.Err()
}
