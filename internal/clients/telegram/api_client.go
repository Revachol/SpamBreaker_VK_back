package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// CheckRequest — запрос к Core API.
type CheckRequest struct {
	Text string `json:"text"`
}

// CheckResponse — ответ от Core API.
type CheckResponse struct {
	ID         string             `json:"id"`
	Text       string             `json:"text"`
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	AllScores  map[string]float64 `json:"all_scores"`
	CreatedAt  string             `json:"created_at"`
}

// APIClient — HTTP-клиент к Core API.
// Один экземпляр создаётся при старте бота и переиспользуется.
type APIClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
	return &APIClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Check отправляет текст на проверку и возвращает вердикт.
func (c *APIClient) Check(ctx context.Context, text string) (*CheckResponse, error) {
	body, err := json.Marshal(CheckRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("api client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v1/check",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("api client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("api client: unexpected status %d", resp.StatusCode)
	}

	var result CheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("api client: decode: %w", err)
	}

	return &result, nil
}
