package event

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/organizer"
	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Register mounts organizer event-management routes. Public discovery
// routes live in the explore handler.
func (h *Handler) Register(r gin.IRouter, requireAuth, requireOrganizer gin.HandlerFunc) {
	g := r.Group("/events", requireAuth, requireOrganizer)
	g.POST("", h.create)
	g.GET("/mine", h.listMine)
	g.PATCH("/:id", h.update)
	g.POST("/:id/publish", h.transition(h.service.Publish))
	g.POST("/:id/unpublish", h.transition(h.service.Unpublish))
	g.POST("/:id/cancel", h.transition(h.service.Cancel))
	g.DELETE("/:id", h.delete)
}

func eventID(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Validation("invalid event id")
	}
	return id, nil
}

func (h *Handler) create(c *gin.Context) {
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.Error(c, apperr.Validation("title, categoryId and startsAt are required"))
		return
	}
	e, err := h.service.Create(c.Request.Context(), organizer.OrganizerID(c), in)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusCreated, e.ToDTO())
}

func (h *Handler) listMine(c *gin.Context) {
	events, err := h.service.ListMine(c.Request.Context(), organizer.OrganizerID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]DTO, len(events))
	for i, e := range events {
		dtos[i] = e.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}

func (h *Handler) update(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	var in Input
	if err := c.ShouldBindJSON(&in); err != nil {
		httpx.Error(c, apperr.Validation("title, categoryId and startsAt are required"))
		return
	}
	e, err := h.service.Update(c.Request.Context(), organizer.OrganizerID(c), id, in)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, e.ToDTO())
}

// transition wraps the publish/unpublish/cancel service calls that share
// a signature.
func (h *Handler) transition(fn func(ctx context.Context, organizerID, eventID int64) (*Event, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := eventID(c)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		e, err := fn(c.Request.Context(), organizer.OrganizerID(c), id)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		httpx.OK(c, http.StatusOK, e.ToDTO())
	}
}

func (h *Handler) delete(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if err := h.service.Delete(c.Request.Context(), organizer.OrganizerID(c), id); err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"deleted": true})
}
