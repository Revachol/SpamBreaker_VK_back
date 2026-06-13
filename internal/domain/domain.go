package domain

import (
	"context"
	"time"
)

// ---------- Value objects ----------

// Verdict — итог проверки одного сообщения.
type Verdict struct {
	Label      string             `json:"label"`      // "neutral" | "positive" | "negative"
	Confidence float64            `json:"confidence"` // вероятность победившего класса
	AllScores  map[string]float64 `json:"all_scores"` // все три вероятности
}

// CheckRecord — запись в истории проверок.
type CheckRecord struct {
	ID            string    `json:"id"`
	Text          string    `json:"text"`
	Verdict       Verdict   `json:"verdict"`
	ApplicationID string    `json:"application_id,omitempty"` // пусто для ручных проверок
	CreatedAt     time.Time `json:"created_at"`
}

// Application — подключённое приложение (бот Telegram/VK, API клиент).
type Application struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Platform   string    `json:"platform"`    // "telegram", "vk", "api"
	ExternalID string    `json:"external_id"` // ID чата/группы во внешней платформе
	Token      string    `json:"token"`       // секретный токен для Core API
	OwnerID    string    `json:"owner_id"`    // ID модератора-владельца
	OwnAccID   string    `json:"own_acc_id"`  // ID аккаунта модератора, которым подключили бота
	Status     string    `json:"status"`      // "active", "suspended", "inactive"
	VerifiedAt time.Time `json:"verified_at"` // время последней успешной верификации
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type ApplicationAdminInfo struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// ApplicationSettings — настройки приложения.
type ApplicationSettings struct {
	ID                string    `json:"id"`
	ApplicationID     string    `json:"application_id"`
	ToxicityThreshold int       `json:"toxicity_threshold"` // 0-100
	ActionOnSpam      string    `json:"action_on_spam"`     // "notify", "delete", "ban", "shadow_ban"
	AutoModerate      bool      `json:"auto_moderate"`
	NotifyModerator   bool      `json:"notify_moderator"`
	AllowedLanguages  []string  `json:"allowed_languages"`
	BannedWords       []string  `json:"banned_words"` // список запрещенных слов
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ---------- Port: ML-клиент ----------

// Classifier — абстракция над ML-микросервисом.
// Реализация живёт в internal/client/ml.
type Classifier interface {
	Classify(ctx context.Context, text string) (*Verdict, error)
}
