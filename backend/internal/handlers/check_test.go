package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// checkRequestWith builds a POST /api/check with the given multipart fields.
func checkRequestWith(t *testing.T, field, filename string, content []byte) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	if field != "" {
		part, err := w.CreateFormFile(field, filename)
		if err != nil {
			t.Fatal(err)
		}
		part.Write(content)
	}
	w.Close()

	req := httptest.NewRequest("POST", "/api/check", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestCheckRejectsBadInputBeforeTheModel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	// No AI client or storage: invalid requests must be refused before either is used
	handler := &CheckHandler{}
	router := gin.New()
	router.POST("/api/check", handler.Check)

	tests := map[string]*http.Request{
		"no image":       checkRequestWith(t, "", "", nil),
		"not an image":   checkRequestWith(t, "image", "fake.jpg", []byte("this is not an image")),
		"wrong field":    checkRequestWith(t, "photo", "item.png", pngHeader),
		"empty image":    checkRequestWith(t, "image", "empty.png", nil),
		"too large file": checkRequestWith(t, "image", "big.png", append(pngHeader, make([]byte, MaxImageSize)...)),
	}
	for name, req := range tests {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want %d", name, rec.Code, http.StatusBadRequest)
		}
	}
}
