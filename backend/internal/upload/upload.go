// Package upload stores event cover images on local disk and serves them
// statically. Good enough for a single-VPS deployment; swap for object
// storage if the app ever runs on more than one server.
package upload

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"meetus.uz/backend/internal/platform/apperr"
	"meetus.uz/backend/internal/platform/httpx"
)

const maxUploadBytes = 5 << 20 // 5 MB

var extByContentType = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
}

type Handler struct {
	dir        string
	apiBaseURL string
}

func NewHandler(dir, apiBaseURL string) (*Handler, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload dir: %w", err)
	}
	return &Handler{dir: dir, apiBaseURL: apiBaseURL}, nil
}

// Register mounts the authenticated upload endpoint and the public
// static file route.
func (h *Handler) Register(r gin.IRouter, engine *gin.Engine, requireAuth gin.HandlerFunc) {
	r.POST("/uploads", requireAuth, h.upload)
	engine.Static("/uploads", h.dir)
}

func (h *Handler) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadBytes)

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		httpx.Error(c, apperr.Validation("expected multipart field 'file' up to 5 MB"))
		return
	}
	defer file.Close()

	// Sniff the real content type; never trust the client's filename.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && n == 0 {
		httpx.Error(c, apperr.Validation("empty file"))
		return
	}
	contentType := http.DetectContentType(head[:n])
	ext, ok := extByContentType[contentType]
	if !ok {
		httpx.Error(c, apperr.Validation("only JPEG, PNG, or WebP images are allowed"))
		return
	}

	nameBytes := make([]byte, 16)
	if _, err := rand.Read(nameBytes); err != nil {
		httpx.Error(c, err)
		return
	}
	name := hex.EncodeToString(nameBytes) + ext

	dst, err := os.Create(filepath.Join(h.dir, name))
	if err != nil {
		httpx.Error(c, fmt.Errorf("create file: %w", err))
		return
	}
	defer dst.Close()

	if _, err := dst.Write(head[:n]); err != nil {
		httpx.Error(c, fmt.Errorf("write file: %w", err))
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		httpx.Error(c, fmt.Errorf("write file: %w", err))
		return
	}

	httpx.OK(c, http.StatusCreated, gin.H{"url": h.apiBaseURL + "/uploads/" + name})
}
