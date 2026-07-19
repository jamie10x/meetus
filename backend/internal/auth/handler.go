package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/auth")
	g.POST("/telegram", h.loginWithTelegram)
	g.POST("/telegram-miniapp", h.loginWithMiniApp)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)
}

// loginWithTelegram accepts the raw field map produced by the Telegram
// Login Widget (id, first_name, username, photo_url, auth_date, hash, ...).
func (h *Handler) loginWithTelegram(c *gin.Context) {
	var fields map[string]string
	if err := c.ShouldBindJSON(&fields); err != nil {
		httpx.Error(c, apperr.Validation("invalid request body"))
		return
	}
	result, err := h.service.LoginWithTelegram(c.Request.Context(), fields)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, result)
}

type miniAppLoginRequest struct {
	InitData string `json:"initData" binding:"required"`
}

// loginWithMiniApp accepts the raw initData string from
// window.Telegram.WebApp.initData, unparsed — VerifyMiniAppInitData does
// its own URL-query decoding.
func (h *Handler) loginWithMiniApp(c *gin.Context) {
	var req miniAppLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("initData is required"))
		return
	}
	result, err := h.service.LoginWithMiniApp(c.Request.Context(), req.InitData)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, result)
}

type refreshRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("refreshToken is required"))
		return
	}
	pair, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, pair)
}

func (h *Handler) logout(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.Validation("refreshToken is required"))
		return
	}
	if err := h.service.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.Error(c, err)
		return
	}
	httpx.OK(c, http.StatusOK, gin.H{"loggedOut": true})
}
