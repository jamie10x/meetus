package rsvp

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/event"
	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
)

type Handler struct {
	service   *Service
	eventRepo *event.Repository
}

func NewHandler(service *Service, eventRepo *event.Repository) *Handler {
	return &Handler{service: service, eventRepo: eventRepo}
}

func (h *Handler) Register(r gin.IRouter, requireAuth, requireOrganizer gin.HandlerFunc) {
	r.POST("/events/:id/rsvp", requireAuth, h.join)
	r.DELETE("/events/:id/rsvp", requireAuth, h.cancel)
	r.GET("/events/:id/rsvp", requireAuth, h.mine)
	r.GET("/me/tickets", requireAuth, h.myTickets)

	r.POST("/checkin", requireAuth, requireOrganizer, h.checkIn)
	r.GET("/events/:id/attendees", requireAuth, requireOrganizer, h.attendees)
	r.GET("/events/:id/attendees.csv", requireAuth, requireOrganizer, h.attendeesCSV)
}

func eventID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Validation("invalid event id")
	}
	return id, nil
}

func (h *Handler) join(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	ticket, err := h.service.Join(c.Request.Context(), id, authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, ticket)
}

func (h *Handler) cancel(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if err := h.service.Cancel(c.Request.Context(), id, authn.UserID(c)); err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"canceled": true})
}

func (h *Handler) mine(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	ticket, err := h.service.GetMine(c.Request.Context(), id, authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, ticket)
}

func (h *Handler) myTickets(c *gin.Context) {
	tickets, err := h.service.ListMyTickets(c.Request.Context(), authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, tickets)
}

type checkInRequest struct {
	QR string `json:"qr" binding:"required"`
}

func (h *Handler) checkIn(c *gin.Context) {
	var req checkInRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("qr is required"))
		return
	}
	result, err := h.service.CheckIn(c.Request.Context(), organizer.OrganizerID(c), req.QR)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, result)
}

type attendeeDTO struct {
	UserID      int64      `json:"userId"`
	Name        string     `json:"name"`
	Username    *string    `json:"username"`
	AvatarURL   *string    `json:"avatarUrl"`
	RSVPAt      time.Time  `json:"rsvpAt"`
	CheckedInAt *time.Time `json:"checkedInAt"`
}

// ownedAttendees loads the attendee list after verifying the caller owns
// the event.
func (h *Handler) ownedAttendees(c *gin.Context) (*event.Event, []*Attendee, error) {
	id, err := eventID(c)
	if err != nil {
		return nil, nil, err
	}
	e, err := h.eventRepo.GetByID(c.Request.Context(), id)
	if err != nil {
		return nil, nil, err
	}
	if e.OrganizerID != organizer.OrganizerID(c) {
		return nil, nil, apperr.Forbidden("you do not own this event")
	}
	attendees, err := h.service.repo.ListAttendees(c.Request.Context(), id)
	if err != nil {
		return nil, nil, err
	}
	return e, attendees, nil
}

func (h *Handler) attendees(c *gin.Context) {
	_, attendees, err := h.ownedAttendees(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]attendeeDTO, len(attendees))
	for i, a := range attendees {
		dtos[i] = attendeeDTO{
			UserID:      a.UserID,
			Name:        a.Name,
			Username:    a.Username,
			AvatarURL:   a.AvatarURL,
			RSVPAt:      a.RSVPAt,
			CheckedInAt: a.CheckedInAt,
		}
	}
	httpx.OK(c, http.StatusOK, dtos)
}

func (h *Handler) attendeesCSV(c *gin.Context) {
	e, attendees, err := h.ownedAttendees(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	filename := fmt.Sprintf("attendees-event-%d.csv", e.ID)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Status(http.StatusOK)

	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"name", "username", "rsvp_at", "checked_in_at"})
	for _, a := range attendees {
		username, checkedIn := "", ""
		if a.Username != nil {
			username = *a.Username
		}
		if a.CheckedInAt != nil {
			checkedIn = a.CheckedInAt.Format(time.RFC3339)
		}
		_ = w.Write([]string{a.Name, username, a.RSVPAt.Format(time.RFC3339), checkedIn})
	}
	w.Flush()
}
