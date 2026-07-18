package authn

import (
	"strings"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
)

const ctxUserIDKey = "authUserID"

// RequireAuth validates the Bearer access token and stores the user ID
// in the request context.
func RequireAuth(tokens *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			httpx.Error(c, apperr.Unauthorized("missing bearer token"))
			return
		}
		userID, err := tokens.ParseAccess(token)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		c.Set(ctxUserIDKey, userID)
		c.Next()
	}
}

// UserID returns the authenticated user's ID set by RequireAuth.
func UserID(c *gin.Context) int64 {
	return c.GetInt64(ctxUserIDKey)
}
