// Package httpx defines the JSON response envelope and error mapping
// shared by every handler.
package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
)

// OK writes {"data": ...} with the given status.
func OK(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// Error maps an application error to a JSON error response:
// {"error": {"code": "...", "message": "..."}}
func Error(c *gin.Context, err error) {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		slog.Error("unhandled error", "path", c.FullPath(), "err", err)
		ae = apperr.Internal()
	}
	c.AbortWithStatusJSON(statusFor(ae.Code), gin.H{
		"error": gin.H{"code": ae.Code, "message": ae.Message},
	})
}

func statusFor(code apperr.Code) int {
	switch code {
	case apperr.CodeValidation:
		return http.StatusBadRequest
	case apperr.CodeUnauthorized:
		return http.StatusUnauthorized
	case apperr.CodeForbidden:
		return http.StatusForbidden
	case apperr.CodeNotFound:
		return http.StatusNotFound
	case apperr.CodeConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
