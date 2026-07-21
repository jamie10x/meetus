package event

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
)

// PublicHandler serves unauthenticated event discovery.
type PublicHandler struct {
	repo *Repository
}

func NewPublicHandler(repo *Repository) *PublicHandler {
	return &PublicHandler{repo: repo}
}

func (h *PublicHandler) Register(r gin.IRouter) {
	g := r.Group("/explore")
	g.GET("/events", h.list)
	g.GET("/events/:id", h.get)
	g.GET("/events/:id/related", h.related)
	g.GET("/events/:id/series", h.series)
	g.GET("/trending", h.trending)
}

type pageResponse struct {
	Items      []DTO   `json:"items"`
	NextCursor *string `json:"nextCursor"`
}

func (h *PublicHandler) list(c *gin.Context) {
	f := ListFilters{
		CitySlug:     c.Query("city"),
		CategorySlug: c.Query("category"),
		Query:        c.Query("q"),
		Cursor:       c.Query("cursor"),
	}

	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			httpx.Error(c, apperr.Validation("invalid limit"))
			return
		}
		f.Limit = n
	}
	if v := c.Query("online"); v != "" {
		online, err := strconv.ParseBool(v)
		if err != nil {
			httpx.Error(c, apperr.Validation("online must be true or false"))
			return
		}
		f.Online = &online
	}
	for name, dst := range map[string]**time.Time{"from": &f.From, "to": &f.To} {
		if v := c.Query(name); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				httpx.Error(c, apperr.Validation(name+" must be RFC3339"))
				return
			}
			*dst = &t
		}
	}

	page, err := h.repo.ListPublic(c.Request.Context(), f)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	resp := pageResponse{Items: make([]DTO, len(page.Items))}
	for i, e := range page.Items {
		resp.Items[i] = e.ToDTO()
	}
	if page.NextCursor != "" {
		resp.NextCursor = &page.NextCursor
	}
	httpx.OK(c, http.StatusOK, resp)
}

func (h *PublicHandler) trending(c *gin.Context) {
	limit := 0
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			httpx.Error(c, apperr.Validation("invalid limit"))
			return
		}
		limit = n
	}

	events, err := h.repo.ListTrending(c.Request.Context(), c.Query("city"), limit)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]TrendingDTO, len(events))
	for i, te := range events {
		dtos[i] = te.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}

func (h *PublicHandler) get(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	e, err := h.repo.GetPublished(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, e.ToDTO())
}

func (h *PublicHandler) related(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	e, err := h.repo.GetPublished(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	limit := 0
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			httpx.Error(c, apperr.Validation("invalid limit"))
			return
		}
		limit = n
	}

	related, err := h.repo.ListRelated(c.Request.Context(), e.ID, e.CategoryID, e.CityID, limit)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]DTO, len(related))
	for i, re := range related {
		dtos[i] = re.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}

func (h *PublicHandler) series(c *gin.Context) {
	id, err := eventID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	e, err := h.repo.GetPublished(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	if e.SeriesID == nil {
		httpx.OK(c, http.StatusOK, []DTO{})
		return
	}

	siblings, err := h.repo.ListSeries(c.Request.Context(), *e.SeriesID, e.ID)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	dtos := make([]DTO, len(siblings))
	for i, se := range siblings {
		dtos[i] = se.ToDTO()
	}
	httpx.OK(c, http.StatusOK, dtos)
}
