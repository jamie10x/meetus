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
	CityID       *int32
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
	if f.CityID != nil {
		conds = append(conds, "e.city_id = "+arg(*f.CityID))
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
	       e.status, e.visibility, e.series_id, e.created_at, e.updated_at,
	       o.display_name, o.is_verified, c.slug, ci.slug,
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
		&e.Status, &e.Visibility, &e.SeriesID, &e.CreatedAt, &e.UpdatedAt,
		&e.OrganizerName, &e.OrganizerVerified, &e.CategorySlug, &e.CitySlug, &e.GoingCount, &te.RecentGoing)
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

const relatedMaxLimit = 20
const relatedDefaultLimit = 4

// ListRelated returns other published upcoming public events that might
// interest someone looking at eventID — same category or same city,
// ranked category-and-city matches first, then category-only, then
// city-only, soonest start breaking ties. A plain relevance ORDER BY
// rather than a scored/weighted query: at this scale a simple tiered
// ranking is plenty, and it's easy to reason about.
func (r *Repository) ListRelated(ctx context.Context, eventID int64, categoryID int32, cityID *int32, limit int) ([]*Event, error) {
	if limit <= 0 || limit > relatedMaxLimit {
		limit = relatedDefaultLimit
	}

	query := eventSelect + `
		WHERE e.status = 'published' AND e.visibility = 'public' AND e.starts_at > now()
		      AND e.id != $1 AND (e.category_id = $2 OR e.city_id = $3)
		ORDER BY (CASE
		              WHEN e.category_id = $2 AND e.city_id = $3 THEN 0
		              WHEN e.category_id = $2 THEN 1
		              ELSE 2
		          END), e.starts_at ASC
		LIMIT $4`

	rows, err := r.pool.Query(ctx, query, eventID, categoryID, cityID, limit)
	if err != nil {
		return nil, fmt.Errorf("list related events: %w", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

// ListSeries returns the other published upcoming occurrences of the
// same weekly series, soonest first — for showing "other dates" on one
// instance's event page. Excludes eventID itself and non-published
// siblings (a series can have some instances published and others still
// draft, since each occurrence is edited/published independently).
func (r *Repository) ListSeries(ctx context.Context, seriesID, eventID int64) ([]*Event, error) {
	rows, err := r.pool.Query(ctx, eventSelect+`
		WHERE e.series_id = $1 AND e.id != $2
		      AND e.status = 'published' AND e.starts_at > now()
		ORDER BY e.starts_at ASC`, seriesID, eventID)
	if err != nil {
		return nil, fmt.Errorf("list series events: %w", err)
	}
	defer rows.Close()
	return collectEvents(rows)
}

const nearbyMaxLimit = 20

// nearbySelect mirrors eventSelect but adds a haversine distance-from-point
// column — a plain-SQL great-circle distance (no PostGIS) since this is a
// single ORDER BY, not a spatial index lookup; fine at this scale.
const nearbySelect = `
	SELECT e.id, e.organizer_id, e.title, e.description, e.category_id,
	       e.city_id, e.district, e.location_name, e.address, e.lat, e.lng,
	       e.is_online, e.starts_at, e.ends_at, e.capacity, e.cover_url,
	       e.status, e.visibility, e.series_id, e.created_at, e.updated_at,
	       o.display_name, o.is_verified, c.slug, ci.slug,
	       (SELECT count(*) FROM rsvps r WHERE r.event_id = e.id AND r.status = 'going')::int,
	       6371 * acos(least(1.0, greatest(-1.0,
	           cos(radians($1)) * cos(radians(e.lat)) * cos(radians(e.lng) - radians($2))
	           + sin(radians($1)) * sin(radians(e.lat))
	       ))) AS distance_km
	FROM events e
	JOIN organizers o ON o.id = e.organizer_id
	JOIN categories c ON c.id = e.category_id
	LEFT JOIN cities ci ON ci.id = e.city_id
`

// NearbyEvent is a published upcoming in-person event with its
// great-circle distance (km) from the point the caller searched from.
type NearbyEvent struct {
	Event
	DistanceKm float64
}

func scanNearbyEvent(row pgx.Row) (*NearbyEvent, error) {
	var ne NearbyEvent
	e := &ne.Event
	err := row.Scan(&e.ID, &e.OrganizerID, &e.Title, &e.Description, &e.CategoryID,
		&e.CityID, &e.District, &e.LocationName, &e.Address, &e.Lat, &e.Lng,
		&e.IsOnline, &e.StartsAt, &e.EndsAt, &e.Capacity, &e.CoverURL,
		&e.Status, &e.Visibility, &e.SeriesID, &e.CreatedAt, &e.UpdatedAt,
		&e.OrganizerName, &e.OrganizerVerified, &e.CategorySlug, &e.CitySlug, &e.GoingCount, &ne.DistanceKm)
	if err != nil {
		return nil, err
	}
	return &ne, nil
}

// ListNearby returns published upcoming in-person events within radiusKm
// of (lat, lng), closest first. Online events and events without
// coordinates are excluded — there's nothing to measure distance to.
func (r *Repository) ListNearby(ctx context.Context, lat, lng, radiusKm float64, limit int) ([]*NearbyEvent, error) {
	if limit <= 0 || limit > nearbyMaxLimit {
		limit = 6
	}

	query := nearbySelect + `
		WHERE e.status = 'published' AND e.visibility = 'public' AND e.starts_at > now()
		      AND e.is_online = FALSE AND e.lat IS NOT NULL AND e.lng IS NOT NULL
		      AND 6371 * acos(least(1.0, greatest(-1.0,
		              cos(radians($1)) * cos(radians(e.lat)) * cos(radians(e.lng) - radians($2))
		              + sin(radians($1)) * sin(radians(e.lat))
		          ))) <= $3
		ORDER BY distance_km ASC
		LIMIT $4`

	rows, err := r.pool.Query(ctx, query, lat, lng, radiusKm, limit)
	if err != nil {
		return nil, fmt.Errorf("list nearby events: %w", err)
	}
	defer rows.Close()

	events := make([]*NearbyEvent, 0, limit)
	for rows.Next() {
		ne, err := scanNearbyEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, ne)
	}
	return events, rows.Err()
}
