package check_client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	activateBot     = "/api/bot/v1/telegram/chat/active"
	removeBotHandle = "/api/bot/v1/telegram/chat/active"
	verifyUser      = "/api/bot/v1/telegram/active"
	checkMessage    = "/api/bot/v1/telegram/check"
)

// ActivateRequest — запрос к Core API.
type ActivateRequest struct {
	UserID string `json:"user_id,omitempty"`
	ChatID string `json:"chat_id,omitempty"`
}

// DeactivateRequest — запрос на деактивацию чата в Core API.
type DeactivateRequest struct {
	ChatID string `json:"chat_id"`
}

// VerifyRequest — запрос к Core API.
type VerifyRequest struct {
	Token  string `json:"token"`
	UserID string `json:"user_id,omitempty"`
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

// ActivateChat регистрирует чат после добавления бота.
func (c *APIClient) ActivateChat(ctx context.Context, userID, chatID int64) error {
	body, err := json.Marshal(ActivateRequest{
		UserID: strconv.FormatInt(userID, 10),
		ChatID: strconv.FormatInt(chatID, 10),
	})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s", c.baseURL, activateBot)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("api client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

// DeactivateChat деактивирует чат в Core API.
func (c *APIClient) DeactivateChat(ctx context.Context, chatID int64) error {
	body, err := json.Marshal(DeactivateRequest{ChatID: strconv.FormatInt(chatID, 10)})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s%s", c.baseURL, removeBotHandle)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("api client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("api client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("api client: deactivate failed with status %d", resp.StatusCode)
	}

	return nil
}

// VerifyUser подтверждает аккаунт пользователя по токену.
func (c *APIClient) VerifyUser(ctx context.Context, token string, userID int64) error {
	body, err := json.Marshal(VerifyRequest{Token: token, UserID: strconv.FormatInt(userID, 10)})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s%s", c.baseURL, verifyUser)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("api client: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

// Check отправляет текст на проверку и возвращает вердикт.
// chatID передаётся, чтобы бэкенд мог привязать запись к конкретному приложению.
func (c *APIClient) CheckMessage(ctx context.Context, text string, chatID int64) (*CheckResponse, error) {
	body, err := json.Marshal(CheckRequest{
		Text:   text,
		ChatID: strconv.FormatInt(chatID, 10),
	})
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
