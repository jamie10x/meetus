// Package admin provides platform moderation: stats, event moderation,
// and user bans. Admins are flagged via users.is_admin (granted by SQL —
// there is deliberately no endpoint to grant admin).
package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
	"meetus.uz/backend/internal/user"
)

const listLimit = 50

// RequireAdmin loads the caller and rejects non-admins. Runs after
// authn.RequireAuth.
func RequireAdmin(users *user.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, err := users.GetByID(c.Request.Context(), authn.UserID(c))
		if err != nil || !u.IsAdmin {
			httpx.Error(c, apperr.Forbidden("admin access required"))
			return
		}
		c.Next()
	}
}

type Handler struct {
	pool      *pgxpool.Pool
	eventRepo *event.Repository
}

func NewHandler(pool *pgxpool.Pool, eventRepo *event.Repository) *Handler {
	return &Handler{pool: pool, eventRepo: eventRepo}
}

func (h *Handler) Register(r gin.IRouter, requireAuth, requireAdmin gin.HandlerFunc) {
	g := r.Group("/admin", requireAuth, requireAdmin)
	g.GET("/stats", h.stats)
	g.GET("/events", h.listEvents)
	g.POST("/events/:id/unpublish", h.eventAction("draft"))
	g.POST("/events/:id/cancel", h.eventAction("canceled"))
	g.GET("/users", h.listUsers)
	g.POST("/users/:id/ban", h.setBan(true))
	g.POST("/users/:id/unban", h.setBan(false))
	g.GET("/organizers", h.listOrganizers)
	g.POST("/organizers/:id/verify", h.setVerified(true))
	g.POST("/organizers/:id/unverify", h.setVerified(false))
}

type stats struct {
	Users          int64            `json:"users"`
	Organizers     int64            `json:"organizers"`
	EventsByStatus map[string]int64 `json:"eventsByStatus"`
	UpcomingEvents int64            `json:"upcomingEvents"`
	Rsvps7d        int64            `json:"rsvps7d"`
	Rsvps30d       int64            `json:"rsvps30d"`
	Checkins30d    int64            `json:"checkins30d"`
}

func (h *Handler) stats(c *gin.Context) {
	ctx := c.Request.Context()
	s := stats{EventsByStatus: map[string]int64{}}

	singles := []struct {
		dst   *int64
		query string
	}{
		{&s.Users, `SELECT count(*) FROM users`},
		{&s.Organizers, `SELECT count(*) FROM organizers`},
		{&s.UpcomingEvents, `SELECT count(*) FROM events WHERE status = 'published' AND starts_at > now()`},
		{&s.Rsvps7d, `SELECT count(*) FROM rsvps WHERE created_at > now() - interval '7 days'`},
		{&s.Rsvps30d, `SELECT count(*) FROM rsvps WHERE created_at > now() - interval '30 days'`},
		{&s.Checkins30d, `SELECT count(*) FROM tickets WHERE checked_in_at > now() - interval '30 days'`},
	}
	for _, q := range singles {
		if err := h.pool.QueryRow(ctx, q.query).Scan(q.dst); err != nil {
			httpx.Error(c, fmt.Errorf("admin stats: %w", err))
			return
		}
	}

	rows, err := h.pool.Query(ctx, `SELECT status, count(*) FROM events GROUP BY status`)
	if err != nil {
		httpx.Error(c, fmt.Errorf("admin stats: %w", err))
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			httpx.Error(c, err)
			return
		}
		s.EventsByStatus[status] = n
	}

	httpx.OK(c, http.StatusOK, s)
}

// listEvents returns events in any status, newest first, optionally
// filtered by status.
func (h *Handler) listEvents(c *gin.Context) {
	status := c.Query("status")
	switch status {
	case "", "draft", "published", "canceled", "finished":
	default:
		httpx.Error(c, apperr.Validation("invalid status filter"))
		return
	}

	events, err := h.eventRepo.ListForAdmin(c.Request.Context(), status, listLimit)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]event.DTO, len(events))
	for i, e := range events {
		dtos[i] = e.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}

// eventAction force-sets an event status (moderation overrides organizer
// ownership; lifecycle preconditions deliberately do not apply here).
func (h *Handler) eventAction(target event.Status) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			httpx.Error(c, apperr.Validation("invalid event id"))
			return
		}
		if _, err := h.eventRepo.GetByID(c.Request.Context(), id); err != nil {
			httpx.Error(c, err)
			return
		}
		if err := h.eventRepo.SetStatus(c.Request.Context(), id, target); err != nil {
			httpx.Error(c, err)
			return
		}
		e, err := h.eventRepo.GetByID(c.Request.Context(), id)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		httpx.OK(c, http.StatusOK, e.ToDTO())
	}
}

type adminUser struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Username  *string   `json:"username"`
	IsBanned  bool      `json:"isBanned"`
	IsAdmin   bool      `json:"isAdmin"`
	CreatedAt time.Time `json:"createdAt"`
}

func (h *Handler) listUsers(c *gin.Context) {
	q := c.Query("q")
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT id, name, username, is_banned, is_admin, created_at
		FROM users
		WHERE $1 = '' OR name ILIKE '%' || $1 || '%' OR username ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT $2`, q, listLimit)
	if err != nil {
		httpx.Error(c, fmt.Errorf("admin list users: %w", err))
		return
	}
	defer rows.Close()

	users := make([]adminUser, 0, listLimit)
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.Name, &u.Username, &u.IsBanned, &u.IsAdmin, &u.CreatedAt); err != nil {
			httpx.Error(c, err)
			return
		}
		users = append(users, u)
	}
	httpx.OK(c, http.StatusOK, users)
}

func (h *Handler) setBan(banned bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			httpx.Error(c, apperr.Validation("invalid user id"))
			return
		}
		if id == authn.UserID(c) {
			httpx.Error(c, apperr.Validation("you cannot ban yourself"))
			return
		}
		tag, err := h.pool.Exec(c.Request.Context(),
			// Admins cannot ban other admins.
			`UPDATE users SET is_banned = $2, updated_at = now()
			 WHERE id = $1 AND NOT is_admin`, id, banned)
		if err != nil {
			httpx.Error(c, fmt.Errorf("set ban: %w", err))
			return
		}
		if tag.RowsAffected() == 0 {
			httpx.Error(c, apperr.NotFound("user not found or is an admin"))
			return
		}
		httpx.OK(c, http.StatusOK, gin.H{"id": id, "isBanned": banned})
	}
}

type adminOrganizer struct {
	ID          int64     `json:"id"`
	DisplayName string    `json:"displayName"`
	UserName    string    `json:"userName"`
	IsVerified  bool      `json:"isVerified"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h *Handler) listOrganizers(c *gin.Context) {
	q := c.Query("q")
	rows, err := h.pool.Query(c.Request.Context(), `
		SELECT o.id, o.display_name, u.name, o.is_verified, o.created_at
		FROM organizers o
		JOIN users u ON u.id = o.user_id
		WHERE $1 = '' OR o.display_name ILIKE '%' || $1 || '%'
		ORDER BY o.created_at DESC
		LIMIT $2`, q, listLimit)
	if err != nil {
		httpx.Error(c, fmt.Errorf("admin list organizers: %w", err))
		return
	}
	defer rows.Close()

	organizers := make([]adminOrganizer, 0, listLimit)
	for rows.Next() {
		var o adminOrganizer
		if err := rows.Scan(&o.ID, &o.DisplayName, &o.UserName, &o.IsVerified, &o.CreatedAt); err != nil {
			httpx.Error(c, err)
			return
		}
		organizers = append(organizers, o)
	}
	httpx.OK(c, http.StatusOK, organizers)
}

func (h *Handler) setVerified(verified bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil || id <= 0 {
			httpx.Error(c, apperr.Validation("invalid organizer id"))
			return
		}
		tag, err := h.pool.Exec(c.Request.Context(),
			`UPDATE organizers SET is_verified = $2 WHERE id = $1`, id, verified)
		if err != nil {
			httpx.Error(c, fmt.Errorf("set verified: %w", err))
			return
		}
		if tag.RowsAffected() == 0 {
			httpx.Error(c, apperr.NotFound("organizer not found"))
			return
		}
		httpx.OK(c, http.StatusOK, gin.H{"id": id, "isVerified": verified})
	}
}
