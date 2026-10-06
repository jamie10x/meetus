package upload

import (
	"bytes"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"testing"
)

func TestUploadValidatesFullImageAndLeavesNoPartialFiles(t *testing.T) {
	dir := t.TempDir()
	handler, err := NewHandler(dir, "http://localhost")
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.POST("/uploads", handler.upload)
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		data   []byte
		status int
	}{{"truncated", encoded.Bytes()[:40], 400}, {"valid", encoded.Bytes(), 201}} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			file, _ := form.CreateFormFile("file", "test.png")
			file.Write(tc.data)
			form.Close()
			req := httptest.NewRequest("POST", "/uploads", &body)
			req.Header.Set("Content-Type", form.FormDataContentType())
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
		})
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("files=%d err=%v", len(files), err)
	}
}
