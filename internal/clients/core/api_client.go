package check_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	activateBot     = ""
	removeBotHandle = ""
	verifyUser      = ""
	checkMessage    = ""
)

// ActivateRequest — запрос к Core API.
type ActivateRequest struct {
	UserID int64 `json:"user_id,omitempty"`
	ChatID int64 `json:"chat_id,omitempty"`
}

// VerifyRequest — запрос к Core API.
type VerifyRequest struct {
	Token  string `json:"token"`
	UserID int64  `json:"user_id,omitempty"`
}

// CheckRequest — запрос к Core API.
type CheckRequest struct {
	Text   string `json:"text"`
	ChatID string `json:"chat_id,omitempty"`
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

// ActivateChat активирует чат по токену (вызывается ботом при получении /connect TOKEN).
func (c *APIClient) ActivateChat(ctx context.Context, userID, chatID int64) error {
	body, err := json.Marshal(ActivateRequest{UserID: userID, ChatID: chatID})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s", c.baseURL, activateBot)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("api client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api client: activate failed with status %d", resp.StatusCode)
	}

	return nil
}

// VerifyUser some
func (c *APIClient) VerifyUser(ctx context.Context, token string, UserId int64) error {
	body, err := json.Marshal(VerifyRequest{Token: token, UserID: UserId})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s", c.baseURL, verifyUser)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("api client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api client: verify failed with status %d", resp.StatusCode)
	}

	return nil
}

// IsChatActive проверяет, зарегистрирован ли чат в системе.
func (c *APIClient) IsChatActive(ctx context.Context, chatID string) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/bots/telegram/internal/chat-active?chat_id=%s", c.baseURL, chatID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, fmt.Errorf("api client: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("api client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("api client: unexpected status %d", resp.StatusCode)
	}

	var result struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, fmt.Errorf("api client: decode: %w", err)
	}

	return result.Active, nil
}

// Check отправляет текст на проверку и возвращает вердикт.
// chatID передаётся, чтобы бэкенд мог привязать запись к конкретному приложению.
func (c *APIClient) CheckMessage(ctx context.Context, text, chatID string) (*CheckResponse, error) {
	body, err := json.Marshal(CheckRequest{Text: text, ChatID: chatID})
	if err != nil {
		return nil, fmt.Errorf("api client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+checkMessage,
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
