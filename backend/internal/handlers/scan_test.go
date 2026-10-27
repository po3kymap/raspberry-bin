package handlers

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"
)

// pngHeader is the signature every PNG file starts with.
var pngHeader = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0, 0, 0, 0x0d, 'I', 'H', 'D', 'R'}

// makeFileHeader builds a multipart file the same way Gin receives it.
func makeFileHeader(t *testing.T, filename string, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("image", filename)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(content)
	w.Close()

	req := httptest.NewRequest("POST", "/api/scan", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(MaxImageSize * 2); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["image"][0]
}

func TestValidateBinID(t *testing.T) {
	tests := []struct {
		binID string
		valid bool
	}{
		{"bin-1", true},
		{"kitchen_bin_02", true},
		{"", false},
		{"../../etc/passwd", false},
		{"bin 1", false},
		{"<script>alert(1)</script>", false},
		{strings.Repeat("a", 33), false},
	}

	for _, tt := range tests {
		err := ValidateBinID(tt.binID)
		if (err == nil) != tt.valid {
			t.Errorf("ValidateBinID(%q) error = %v, want valid = %v", tt.binID, err, tt.valid)
		}
	}
}

func TestDetectImageTypeAcceptsPNG(t *testing.T) {
	file := makeFileHeader(t, "photo.png", append(pngHeader, make([]byte, 100)...))

	contentType, err := DetectImageType(file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if contentType != "image/png" {
		t.Errorf("contentType = %q, want image/png", contentType)
	}
}

func TestDetectImageTypeRejectsFakeImage(t *testing.T) {
	// A script renamed to .jpg must be rejected based on its content
	file := makeFileHeader(t, "photo.jpg", []byte("#!/bin/sh\nrm -rf /\n"))

	if _, err := DetectImageType(file); !errors.Is(err, errUnsupportedImage) {
		t.Errorf("error = %v, want %v", err, errUnsupportedImage)
	}
}

func TestDetectImageTypeRejectsLargeFile(t *testing.T) {
	file := makeFileHeader(t, "big.png", append(pngHeader, make([]byte, MaxImageSize)...))

	if _, err := DetectImageType(file); !errors.Is(err, errImageTooLarge) {
		t.Errorf("error = %v, want %v", err, errImageTooLarge)
	}
}

func TestDetectImageTypeRejectsEmptyFile(t *testing.T) {
	file := makeFileHeader(t, "empty.png", nil)

	if _, err := DetectImageType(file); !errors.Is(err, errImageTooLarge) {
		t.Errorf("error = %v, want %v", err, errImageTooLarge)
	}
}
