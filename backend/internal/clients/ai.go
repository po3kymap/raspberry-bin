package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Largest ML response we read; a normal one is under 200 bytes
const maxResponseSize = 64 << 10

var fileExtensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

type AIClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewAIClient(baseURL string) *AIClient {
	return &AIClient{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Prediction is the ML service's answer. CanSort is false when the item is
// unsupported, the model is not confident enough, or the image was rejected;
// in that case Reason says why and the bin must keep all lids closed.
type Prediction struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	CanSort    bool    `json:"can_sort"`
	Reason     string  `json:"reason"`
}

// Predict sends the image itself to POST /predict as the multipart field "image".
func (c *AIClient) Predict(image *multipart.FileHeader, contentType string) (*Prediction, error) {
	file, err := image.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	// The client's file name is not forwarded; only the checked type decides the extension
	part, err := writer.CreateFormFile("image", "scan"+fileExtensions[contentType])
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	resp, err := c.HTTP.Post(c.BaseURL+"/predict", writer.FormDataContentType(), &body)
	if err != nil {
		return nil, fmt.Errorf("failed to contact AI service: %w", err)
	}
	defer resp.Body.Close()

	// 200 = classified, 400 = the ML service refused the image (e.g. unreadable).
	// Both have a JSON body; anything else is a failure of the service itself.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return nil, fmt.Errorf("AI service returned status: %d", resp.StatusCode)
	}

	var prediction Prediction
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseSize)).Decode(&prediction); err != nil {
		return nil, fmt.Errorf("failed to decode AI response: %w", err)
	}

	if resp.StatusCode == http.StatusBadRequest {
		prediction.CanSort = false
		if prediction.Category == "" {
			prediction.Category = "rejected"
		}
	}
	return &prediction, nil
}
