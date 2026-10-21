package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"
)

type AIClient struct {
	BaseURL string
	HTTP    *http.Client
}

func NewAIClient(baseURL string) *AIClient {
	return &AIClient{
		BaseURL: baseURL,
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type predictResponse struct {
	Category string `json:"category"`
}

func (c *AIClient) PredictCategory(image *multipart.FileHeader) (string, error) {
	file, err := image.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("image", filepath.Base(image.Filename))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	writer.Close()

	resp, err := c.HTTP.Post(c.BaseURL+"/predict", writer.FormDataContentType(), &body)
	if err != nil {
		return "", fmt.Errorf("failed to contact AI service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("AI service returned status: %d", resp.StatusCode)
	}

	var aiResp predictResponse
	if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
		return "", fmt.Errorf("failed to decode AI response: %w", err)
	}

	return aiResp.Category, nil
}