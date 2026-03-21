package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MessageRepository реализует интерфейс MessageRepository для PostgreSQL.
type MessageRepository struct {
	db *pgxpool.Pool
}

// NewMessageRepository создаёт новый экземпляр репозитория.
func NewMessageRepository(db *pgxpool.Pool) *MessageRepository {
	return &MessageRepository{db: db}
}

// Save сохраняет запись в базу данных.
// Если ID не указан, генерируется новый UUID в БД.
// Verdict.Label сохраняется в поле status, Verdict.Confidence — в toxicity_score (умноженная на 100).
func (r *MessageRepository) Save(ctx context.Context, record *domain.CheckRecord) error {
	query := `
        INSERT INTO message (id, text, status, toxicity_score, created_at)
        VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5)
        RETURNING id
    `

	var idParam interface{} = nil
	if record.ID != "" {
		parsedID, err := uuid.Parse(record.ID)
		if err != nil {
			return err
		}
		idParam = parsedID
	}

	toxicityScore := int(record.Verdict.Confidence * 100)

	var generatedID uuid.UUID
	err := r.db.QueryRow(ctx, query,
		idParam, record.Text, record.Verdict.Label, toxicityScore, record.CreatedAt,
	).Scan(&generatedID)
	if err != nil {
		return err
	}

	record.ID = generatedID.String()
	return nil
}

// List возвращает список записей с учётом лимита и смещения.
// Сортировка по created_at DESC (новые сверху).
func (r *MessageRepository) List(ctx context.Context, limit, offset int) ([]*domain.CheckRecord, error) {
	query := `
        SELECT id, text, status, toxicity_score, created_at
        FROM message
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*domain.CheckRecord
	for rows.Next() {
		var id uuid.UUID
		var text string
		var status string
		var toxicityScore int
		var createdAt time.Time

		if err := rows.Scan(&id, &text, &status, &toxicityScore, &createdAt); err != nil {
			return nil, err
		}

		confidence := float64(toxicityScore) / 100.0

		records = append(records, &domain.CheckRecord{
			ID:   id.String(),
			Text: text,
			Verdict: domain.Verdict{
				Label:      status,
				Confidence: confidence,
				AllScores:  nil,
			},
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// GetByID возвращает запись по её идентификатору.
// Если запись не найдена, возвращает (nil, nil).
func (r *MessageRepository) GetByID(ctx context.Context, id string) (*domain.CheckRecord, error) {
	query := `
        SELECT id, text, status, toxicity_score, created_at
        FROM message
        WHERE id = $1
    `

	var uid uuid.UUID
	var text string
	var status string
	var toxicityScore int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(&uid, &text, &status, &toxicityScore, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	confidence := float64(toxicityScore) / 100.0

	return &domain.CheckRecord{
		ID:   uid.String(),
		Text: text,
		Verdict: domain.Verdict{
			Label:      status,
			Confidence: confidence,
			AllScores:  nil,
		},
		CreatedAt: createdAt,
	}, nil
}
