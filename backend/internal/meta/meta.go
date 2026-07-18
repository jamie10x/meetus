// Package meta serves reference data: cities and categories.
package meta

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/platform/httpx"
)

type Item struct {
	ID     int32  `json:"id"`
	Slug   string `json:"slug"`
	NameUz string `json:"nameUz"`
	NameRu string `json:"nameRu"`
	NameEn string `json:"nameEn"`
}

type Handler struct {
	pool *pgxpool.Pool
}

func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

func (h *Handler) Register(r gin.IRouter) {
	g := r.Group("/meta")
	g.GET("/cities", h.listTable("cities"))
	g.GET("/categories", h.listTable("categories"))
}

func (h *Handler) listTable(table string) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := h.list(c.Request.Context(), table)
		if err != nil {
			httpx.Error(c, err)
			return
		}
		httpx.OK(c, http.StatusOK, items)
	}
}

// list reads a reference table. The table name is one of two hardcoded
// constants above, never user input.
func (h *Handler) list(ctx context.Context, table string) ([]Item, error) {
	rows, err := h.pool.Query(ctx,
		fmt.Sprintf(`SELECT id, slug, name_uz, name_ru, name_en FROM %s ORDER BY id`, table))
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", table, err)
	}
	defer rows.Close()

	items := make([]Item, 0, 16)
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Slug, &it.NameUz, &it.NameRu, &it.NameEn); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}
