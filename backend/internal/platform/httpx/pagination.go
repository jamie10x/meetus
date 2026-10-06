package httpx

import (
	"github.com/gin-gonic/gin"
	"meetus.uz/backend/internal/platform/apperr"
	"strconv"
)

func BeforeID(c *gin.Context) (int64, error) {
	raw := c.Query("beforeId")
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Validation("beforeId must be a positive integer")
	}
	return id, nil
}
