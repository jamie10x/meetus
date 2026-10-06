package user

import (
	"encoding/json"
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/authn"
	"meetus.uz/backend/internal/platform/httpx"
)

var allowedLanguages = []string{"uz", "ru", "en"}

type Handler struct {
	repo *Repository
}

func NewHandler(repo *Repository) *Handler {
	return &Handler{repo: repo}
}

// Register mounts /me routes behind the auth middleware.
func (h *Handler) Register(r gin.IRouter, requireAuth gin.HandlerFunc) {
	g := r.Group("/me", requireAuth)
	g.GET("", h.getMe)
	g.PATCH("", h.updateMe)
}

func (h *Handler) getMe(c *gin.Context) {
	u, err := h.repo.GetByID(c.Request.Context(), authn.UserID(c))
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, u.ToDTO())
}

type nullable[T any] struct {
	Set   bool
	Value *T
}

func (n *nullable[T]) UnmarshalJSON(data []byte) error {
	n.Set = true
	return json.Unmarshal(data, &n.Value)
}

type updateMeRequest struct {
	Name      *string          `json:"name"`
	CityID    nullable[int32]  `json:"cityId"`
	District  nullable[string] `json:"district"`
	Language  *string          `json:"language"`
	AvatarURL *string          `json:"avatarUrl"`
}

func (h *Handler) updateMe(c *gin.Context) {
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("invalid request body"))
		return
	}
	if req.Name != nil && *req.Name == "" {
		httpx.Error(c, apperr.Validation("name cannot be empty"))
		return
	}
	if req.Language != nil && !slices.Contains(allowedLanguages, *req.Language) {
		httpx.Error(c, apperr.Validation("language must be one of: uz, ru, en"))
		return
	}
	if req.AvatarURL != nil && *req.AvatarURL == "" {
		httpx.Error(c, apperr.Validation("avatarUrl cannot be empty"))
		return
	}

	u, err := h.repo.UpdateProfile(c.Request.Context(), authn.UserID(c), ProfileUpdate{
		Name:        req.Name,
		CityID:      req.CityID.Value,
		CityIDSet:   req.CityID.Set,
		District:    req.District.Value,
		DistrictSet: req.District.Set,
		Language:    req.Language,
		AvatarURL:   req.AvatarURL,
	})
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, u.ToDTO())
}
