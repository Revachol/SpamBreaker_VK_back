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
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	Verdict   Verdict   `json:"verdict"`
	CreatedAt time.Time `json:"created_at"`
}

// ---------- Port: ML-клиент ----------

// Classifier — абстракция над ML-микросервисом.
// Реализация живёт в internal/client/ml.
type Classifier interface {
	Classify(ctx context.Context, text string) (*Verdict, error)
}

// ---------- Port: хранилище ----------

// MessageRepository — абстракция над БД.
// Реализация появится позже (PostgreSQL).
type MessageRepository interface {
	Save(ctx context.Context, record *CheckRecord) error
	List(ctx context.Context, limit, offset int) ([]*CheckRecord, error)
	GetByID(ctx context.Context, id string) (*CheckRecord, error)
}
