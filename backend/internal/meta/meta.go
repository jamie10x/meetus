// Package meta serves reference data: cities and categories. Reads are
// public; writes are admin-only (see RegisterAdmin).
package meta

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"meetus.uz/backend/internal/platform/apperr"
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

// RegisterAdmin mounts write endpoints for the same two reference
// tables, gated by the caller's requireAdmin middleware.
func (h *Handler) RegisterAdmin(r gin.IRouter, requireAuth, requireAdmin gin.HandlerFunc) {
	for _, table := range []string{"cities", "categories"} {
		g := r.Group("/admin/"+table, requireAuth, requireAdmin)
		g.POST("", h.createIn(table))
		g.PATCH("/:id", h.updateIn(table))
		g.DELETE("/:id", h.deleteIn(table))
	}
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
// constants (never user input) throughout this file.
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

type itemRequest struct {
	Slug   string `json:"slug" binding:"required,max=50"`
	NameUz string `json:"nameUz" binding:"required,max=100"`
	NameRu string `json:"nameRu" binding:"required,max=100"`
	NameEn string `json:"nameEn" binding:"required,max=100"`
}

func (h *Handler) createIn(table string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req itemRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.Error(c, apperr.Validation("slug, nameUz, nameRu, and nameEn are required"))
			return
		}
		var it Item
		err := h.pool.QueryRow(c.Request.Context(), fmt.Sprintf(
			`INSERT INTO %s (slug, name_uz, name_ru, name_en) VALUES ($1, $2, $3, $4)
			 RETURNING id, slug, name_uz, name_ru, name_en`, table),
			req.Slug, req.NameUz, req.NameRu, req.NameEn).
			Scan(&it.ID, &it.Slug, &it.NameUz, &it.NameRu, &it.NameEn)
		if err != nil {
			httpx.Error(c, mapMetaErr(err, table))
			return
		}
		httpx.OK(c, http.StatusCreated, it)
	}
}

type itemUpdateRequest struct {
	Slug   *string `json:"slug" binding:"omitempty,max=50"`
	NameUz *string `json:"nameUz" binding:"omitempty,max=100"`
	NameRu *string `json:"nameRu" binding:"omitempty,max=100"`
	NameEn *string `json:"nameEn" binding:"omitempty,max=100"`
}

func (h *Handler) updateIn(table string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			httpx.Error(c, apperr.Validation("invalid id"))
			return
		}
		var req itemUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			httpx.Error(c, apperr.Validation("invalid request body"))
			return
		}

		var it Item
		err = h.pool.QueryRow(c.Request.Context(), fmt.Sprintf(
			`UPDATE %s SET
				slug     = COALESCE($2, slug),
				name_uz  = COALESCE($3, name_uz),
				name_ru  = COALESCE($4, name_ru),
				name_en  = COALESCE($5, name_en)
			 WHERE id = $1
			 RETURNING id, slug, name_uz, name_ru, name_en`, table),
			id, req.Slug, req.NameUz, req.NameRu, req.NameEn).
			Scan(&it.ID, &it.Slug, &it.NameUz, &it.NameRu, &it.NameEn)
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.Error(c, apperr.NotFound("not found"))
			return
		}
		if err != nil {
			httpx.Error(c, mapMetaErr(err, table))
			return
		}
		httpx.OK(c, http.StatusOK, it)
	}
}

func (h *Handler) deleteIn(table string) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil || id <= 0 {
			httpx.Error(c, apperr.Validation("invalid id"))
			return
		}
		tag, err := h.pool.Exec(c.Request.Context(),
			fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, table), id)
		if err != nil {
			httpx.Error(c, mapMetaErr(err, table))
			return
		}
		if tag.RowsAffected() == 0 {
			httpx.Error(c, apperr.NotFound("not found"))
			return
		}
		httpx.OK(c, http.StatusOK, gin.H{"deleted": true})
	}
}

// mapMetaErr translates unique/FK violations into user-facing validation
// errors instead of leaking raw Postgres text.
func mapMetaErr(err error, table string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return apperr.Validation("that slug is already in use")
		case "23503":
			noun := "events"
			if table == "cities" {
				noun = "events or users"
			}
			return apperr.Conflict(fmt.Sprintf("cannot delete — still referenced by existing %s", noun))
		}
	}
	return fmt.Errorf("%s: %w", table, err)
}
