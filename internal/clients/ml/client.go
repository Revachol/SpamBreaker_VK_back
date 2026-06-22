package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
)

var _ domain.Classifier = (*Client)(nil)

// Client реализует domain.Classifier.
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     logger.Log
}

func NewClient(baseURL string, l logger.Log) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		logger: l,
	}
}

// mlRequest — тело запроса к ML-сервису.
type mlRequest struct {
	Text   string    `json:"text"`
	SendAt time.Time `json:"send_at"`
}

type mlBatchRequest struct {
	Messages []mlRequest
}

// mlResponse — ответ от ML-сервиса.
type mlResponse struct {
	Label      string             `json:"label"`
	Confidence float64            `json:"confidence"`
	AllScores  map[string]float64 `json:"all_scores"`
}

// Classify отправляет текст в ML-сервис и возвращает вердикт.
func (c *Client) Classify(ctx context.Context, batch []domain.BMessage) (*domain.Verdict, error) {
	request := mlBatchRequest{
		Messages: make([]mlRequest, 0, len(batch)),
	}

	for _, msg := range batch {
		request.Messages = append(request.Messages, mlRequest{
			Text:   msg.Text,
			SendAt: msg.SendAt,
		})
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("ml client: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/classify/batch",
		bytes.NewReader(body),
	)
	if err != nil {
		c.logger.Errorf("ml client: create request: %w", err)
		return nil, fmt.Errorf("ml client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logger.Errorf("ml client: do request: %w", err)
		return nil, fmt.Errorf("ml client: do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.logger.Errorf("ml client: bad status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("ml client: unexpected status %d", resp.StatusCode)
	}

	var mlResp mlResponse
	if err := json.NewDecoder(resp.Body).Decode(&mlResp); err != nil {
		c.logger.Errorf("ml client: decode response: %w", err)
		return nil, fmt.Errorf("ml client: decode response: %w", err)
	}

	return &domain.Verdict{
		Label:      mlResp.Label,
		Confidence: mlResp.Confidence,
		AllScores:  mlResp.AllScores,
	}, nil
}
