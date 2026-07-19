// Package feedback lets attendees rate an event 1-5 after it's over, and
// lets the owning organizer see the aggregate.
package feedback

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Submit records or updates a user's rating for an event. Only someone
// who RSVP'd (in any status — canceling afterward doesn't retract the
// right to rate) may leave feedback.
func (r *Repository) Submit(ctx context.Context, eventID, userID int64, rating int) error {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM rsvps WHERE event_id = $1 AND user_id = $2)`,
		eventID, userID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check rsvp for feedback: %w", err)
	}
	if !exists {
		return apperr.Forbidden("you must have joined this event to leave feedback")
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO event_feedback (event_id, user_id, rating)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id, user_id) DO UPDATE SET
			rating = EXCLUDED.rating, created_at = now()`,
		eventID, userID, rating)
	if err != nil {
		return fmt.Errorf("submit feedback: %w", err)
	}
	return nil
}

type Summary struct {
	Count   int64   `json:"count"`
	Average float64 `json:"average"`
}

func (r *Repository) SummaryFor(ctx context.Context, eventID int64) (*Summary, error) {
	var s Summary
	err := r.pool.QueryRow(ctx,
		`SELECT count(*), coalesce(avg(rating), 0) FROM event_feedback WHERE event_id = $1`,
		eventID).Scan(&s.Count, &s.Average)
	if errors.Is(err, pgx.ErrNoRows) {
		return &Summary{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("feedback summary: %w", err)
	}
	return &s, nil
}

type Handler struct {
	repo      *Repository
	eventRepo *event.Repository
}

func NewHandler(repo *Repository, eventRepo *event.Repository) *Handler {
	return &Handler{repo: repo, eventRepo: eventRepo}
}

func (h *Handler) Register(r gin.IRouter, requireAuth, requireOrganizer gin.HandlerFunc) {
	r.POST("/events/:id/feedback", requireAuth, h.submit)
	r.GET("/events/:id/feedback", requireAuth, requireOrganizer, h.summary)
}

func eventID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Validation("invalid event id")
	}
	return id, nil
}

type submitRequest struct {
	Rating int `json:"rating" binding:"required,min=1,max=5"`
}

func (h *Handler) submit(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var req submitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("rating must be an integer from 1 to 5"))
		return
	}
	if err := h.repo.Submit(c.Request.Context(), id, authn.UserID(c), req.Rating); err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"submitted": true})
}

// summary is owner-only — same ownership check pattern as attendees.
func (h *Handler) summary(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	e, err := h.eventRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if e.OrganizerID != organizer.OrganizerID(c) {
		httpx.Error(c, apperr.Forbidden("you do not own this event"))
		return
	}
	s, err := h.repo.SummaryFor(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, s)
}
