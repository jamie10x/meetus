// Package organizer manages organizer profiles and the organizer guard
// used by event-management endpoints.
package organizer

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
)

type Organizer struct {
	ID          int64
	UserID      int64
	DisplayName string
	Bio         *string
	AvatarURL   *string
	IsVerified  bool
	CreatedAt   time.Time
}

type DTO struct {
	ID          int64     `json:"id"`
	DisplayName string    `json:"displayName"`
	Bio         *string   `json:"bio"`
	AvatarURL   *string   `json:"avatarUrl"`
	IsVerified  bool      `json:"isVerified"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (o *Organizer) ToDTO() DTO {
	return DTO{ID: o.ID, DisplayName: o.DisplayName, Bio: o.Bio, AvatarURL: o.AvatarURL, IsVerified: o.IsVerified, CreatedAt: o.CreatedAt}
}

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, userID int64, displayName string, bio *string) (*Organizer, error) {
	var o Organizer
	err := r.pool.QueryRow(ctx, `
		INSERT INTO organizers (user_id, display_name, bio)
		VALUES ($1, $2, $3)
		RETURNING id, user_id, display_name, bio, avatar_url, is_verified, created_at`,
		userID, displayName, bio).
		Scan(&o.ID, &o.UserID, &o.DisplayName, &o.Bio, &o.AvatarURL, &o.IsVerified, &o.CreatedAt)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return nil, apperr.Conflict("you are already an organizer")
	}
	if err != nil {
		return nil, fmt.Errorf("create organizer: %w", err)
	}
	return &o, nil
}

func (r *Repository) GetByUserID(ctx context.Context, userID int64) (*Organizer, error) {
	var o Organizer
	err := r.pool.QueryRow(ctx, `
		SELECT id, user_id, display_name, bio, avatar_url, is_verified, created_at
		FROM organizers WHERE user_id = $1`, userID).
		Scan(&o.ID, &o.UserID, &o.DisplayName, &o.Bio, &o.AvatarURL, &o.IsVerified, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("organizer profile not found")
	}
	if err != nil {
		return nil, fmt.Errorf("get organizer: %w", err)
	}
	return &o, nil
}

// GetLanguage returns the language preference of the user who owns the
// given organizer profile — used as the default announcement language
// for channels without their own per-channel override.
func (r *Repository) GetLanguage(ctx context.Context, organizerID int64) (string, error) {
	var lang string
	err := r.pool.QueryRow(ctx, `
		SELECT u.language FROM organizers o
		JOIN users u ON u.id = o.user_id
		WHERE o.id = $1`, organizerID).Scan(&lang)
	if err != nil {
		return "", fmt.Errorf("get organizer language: %w", err)
	}
	return lang, nil
}

const ctxOrganizerIDKey = "organizerID"

// RequireOrganizer loads the caller's organizer profile and stores its ID
// in the context. Must run after authn.RequireAuth.
func RequireOrganizer(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		o, err := repo.GetByUserID(c.Request.Context(), authn.UserID(c))
		if err != nil {
			httpx.Error(c, apperr.Forbidden("organizer profile required"))
			return
		}
		c.Set(ctxOrganizerIDKey, o.ID)
		c.Next()
	}
}

// OrganizerID returns the organizer ID set by RequireOrganizer.
func OrganizerID(c *gin.Context) int64 {
	return c.GetInt64(ctxOrganizerIDKey)
}

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

func (h *Handler) Register(r gin.IRouter, requireAuth gin.HandlerFunc) {
	g := r.Group("/organizers", requireAuth)
	g.POST("", h.become)
	g.GET("/me", h.me)
	g.GET("/me/stats", h.stats)
}

type becomeRequest struct {
	DisplayName string  `json:"displayName" binding:"required,max=100"`
	Bio         *string `json:"bio" binding:"omitempty,max=1000"`
}

func (h *Handler) become(c *gin.Context) {
	var req becomeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("displayName is required (max 100 chars)"))
		return
	}
	o, err := h.repo.Create(c.Request.Context(), authn.UserID(c), req.DisplayName, req.Bio)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, o.ToDTO())
}

func (h *Handler) me(c *gin.Context) {
	o, err := h.repo.GetByUserID(c.Request.Context(), authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, o.ToDTO())
}

type Stats struct {
	TotalEvents       int64 `json:"totalEvents"`
	UpcomingPublished int64 `json:"upcomingPublished"`
	TotalRsvps        int64 `json:"totalRsvps"`
	TotalCheckins     int64 `json:"totalCheckins"`
}

func (r *Repository) StatsFor(ctx context.Context, organizerID int64) (*Stats, error) {
	var s Stats
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM events e WHERE e.organizer_id = $1),
			(SELECT count(*) FROM events e
			  WHERE e.organizer_id = $1 AND e.status = 'published' AND e.starts_at > now()),
			(SELECT count(*) FROM rsvps rv
			  JOIN events e ON e.id = rv.event_id
			  WHERE e.organizer_id = $1 AND rv.status = 'going'),
			(SELECT count(*) FROM tickets t
			  JOIN rsvps rv ON rv.id = t.rsvp_id
			  JOIN events e ON e.id = rv.event_id
			  WHERE e.organizer_id = $1 AND t.checked_in_at IS NOT NULL)`,
		organizerID).
		Scan(&s.TotalEvents, &s.UpcomingPublished, &s.TotalRsvps, &s.TotalCheckins)
	if err != nil {
		return nil, fmt.Errorf("organizer stats: %w", err)
	}
	return &s, nil
}

func (h *Handler) stats(c *gin.Context) {
	o, err := h.repo.GetByUserID(c.Request.Context(), authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	s, err := h.repo.StatsFor(c.Request.Context(), o.ID)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, s)
}
