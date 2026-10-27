package clients

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// makeFileHeader builds an uploaded file the same way Gin receives it.
func makeFileHeader(t *testing.T, content []byte) *multipart.FileHeader {
	t.Helper()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, _ := w.CreateFormFile("image", "../../evil name.png")
	part.Write(content)
	w.Close()

	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["image"][0]
}

// fakeML answers /predict with a fixed status and body and records what it received.
func fakeML(t *testing.T, status int, body string, received *[]byte, filename *string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/predict" {
			t.Errorf("path = %q, want /predict", r.URL.Path)
		}
		file, header, err := r.FormFile("image")
		if err != nil {
			t.Errorf("no multipart field \"image\": %v", err)
		} else {
			data, _ := io.ReadAll(file)
			if received != nil {
				*received = data
			}
			if filename != nil {
				*filename = header.Filename
			}
		}
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestPredictSendsImageAndParsesSortableResult(t *testing.T) {
	var received []byte
	var filename string
	server := fakeML(t, http.StatusOK,
		`{"status":"ok","category":"PMD","confidence":0.97,"can_sort":true,"reason":null}`, &received, &filename)

	image := []byte("fake png bytes")
	prediction, err := NewAIClient(server.URL).Predict(makeFileHeader(t, image), "image/png")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(received, image) {
		t.Error("ML service did not receive the uploaded image")
	}
	if filename != "scan.png" {
		t.Errorf("forwarded file name = %q, want scan.png (client name must not be forwarded)", filename)
	}
	if prediction.Category != "PMD" || !prediction.CanSort || prediction.Confidence != 0.97 {
		t.Errorf("prediction = %+v", prediction)
	}
}

func TestPredictUnsupportedItemCannotBeSorted(t *testing.T) {
	server := fakeML(t, http.StatusOK,
		`{"status":"ok","category":"UNSUPPORTED","confidence":0.99,"can_sort":false,"reason":"unsupported_item"}`, nil, nil)

	prediction, err := NewAIClient(server.URL).Predict(makeFileHeader(t, []byte("x")), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if prediction.CanSort || prediction.Reason != "unsupported_item" {
		t.Errorf("prediction = %+v, want can_sort=false, reason=unsupported_item", prediction)
	}
}

func TestPredictImageRejectedByML(t *testing.T) {
	server := fakeML(t, http.StatusBadRequest,
		`{"status":"error","reason":"unreadable_image","message":"Image could not be read.","can_sort":false}`, nil, nil)

	prediction, err := NewAIClient(server.URL).Predict(makeFileHeader(t, []byte("x")), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if prediction.CanSort || prediction.Category != "rejected" || prediction.Reason != "unreadable_image" {
		t.Errorf("prediction = %+v, want rejected / unreadable_image", prediction)
	}
}

func TestPredictFailsOnServiceError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"server error", http.StatusInternalServerError, "boom"},
		{"invalid JSON", http.StatusOK, "not json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := fakeML(t, tt.status, tt.body, nil, nil)
			if _, err := NewAIClient(server.URL).Predict(makeFileHeader(t, []byte("x")), "image/jpeg"); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestPredictFailsWhenServiceIsDown(t *testing.T) {
	if _, err := NewAIClient("http://127.0.0.1:1").Predict(makeFileHeader(t, []byte("x")), "image/jpeg"); err == nil {
		t.Error("expected an error when the ML service is unreachable")
	}
}
