package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
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

type predictRequest struct {
	ImageURL string `json:"image_url"`
}

type predictResponse struct {
	Category string `json:"category"`
}

func (c *AIClient) PredictCategory(imageURL string) (string, error) {
	reqBody, _ := json.Marshal(predictRequest{ImageURL: imageURL})

	resp, err := c.HTTP.Post(c.BaseURL+"/predict", "application/json", bytes.NewBuffer(reqBody))
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