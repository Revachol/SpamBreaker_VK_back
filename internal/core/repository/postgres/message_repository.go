package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ interfaces.MessageRepository = (*MessageRepository)(nil)

// MessageRepository реализует интерфейс MessageRepository для PostgreSQL.
type MessageRepository struct {
	db     *pgxpool.Pool
	logger logger.Log
}

// NewMessageRepository создаёт новый экземпляр репозитория.
func NewMessageRepository(db *pgxpool.Pool, l logger.Log) *MessageRepository {
	return &MessageRepository{db: db, logger: l}
}

// Save сохраняет запись в базу данных.
// Если ID не указан, генерируется новый UUID в БД.
// Verdict.Label сохраняется в поле status, Verdict.Confidence — в toxicity_score (умноженная на 100).
func (r *MessageRepository) Save(ctx context.Context, record *domain.CheckRecord) error {
	query := `
        INSERT INTO message (id, message_id, text, status, toxicity_score, application_id, created_at)
        VALUES (COALESCE($1, gen_random_uuid()), NULLIF($2, ''), $3, $4, $5, $6, $7)
        RETURNING id
    `

	var idParam interface{} = nil
	if record.ID != "" {
		parsedID, err := uuid.Parse(record.ID)
		if err != nil {
			r.logger.Errorf("Message repo: parse id failed: %s", err)
			return err
		}
		idParam = parsedID
	}

	var appID interface{} = nil
	if record.ApplicationID != "" {
		parsed, err := uuid.Parse(record.ApplicationID)
		if err == nil {
			appID = parsed
		}
	}

	toxicityScore := int(record.Verdict.Confidence * 100)

	var generatedID uuid.UUID
	err := r.db.QueryRow(ctx, query,
		idParam, record.MessageID, record.Text, record.Verdict.Label, toxicityScore, appID, record.CreatedAt,
	).Scan(&generatedID)
	if err != nil {
		r.logger.Errorf("Message repo: save failed: %s", err)
		return err
	}

	record.ID = generatedID.String()
	return nil
}

// ListByApplication возвращает историю проверок для конкретного приложения.
func (r *MessageRepository) ListByApplication(ctx context.Context, applicationID string, limit, offset int) ([]*domain.CheckRecord, error) {
	query := `
        SELECT id, COALESCE(message_id, ''), text, status, toxicity_score, created_at
        FROM message
        WHERE application_id = $1
        ORDER BY created_at DESC
        LIMIT $2 OFFSET $3
    `

	appID, err := uuid.Parse(applicationID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, appID, limit, offset)
	if err != nil {
		r.logger.Errorf("Message repo: list by application failed: %s", err)
		return nil, err
	}
	defer rows.Close()

	var records []*domain.CheckRecord
	for rows.Next() {
		var id uuid.UUID
		var messageID string
		var text, status string
		var toxicityScore int
		var createdAt time.Time

		if err := rows.Scan(&id, &messageID, &text, &status, &toxicityScore, &createdAt); err != nil {
			r.logger.Errorf("Message repo: scan failed: %s", err)
			return nil, err
		}

		records = append(records, &domain.CheckRecord{
			ID:            id.String(),
			MessageID:     messageID,
			Text:          text,
			ApplicationID: applicationID,
			Verdict: domain.Verdict{
				Label:      status,
				Confidence: float64(toxicityScore) / 100.0,
			},
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return records, nil
}

// List возвращает список записей с учётом лимита и смещения.
// Сортировка по created_at DESC (новые сверху).
func (r *MessageRepository) List(ctx context.Context, limit, offset int) ([]*domain.CheckRecord, error) {
	query := `
        SELECT id, COALESCE(message_id, ''), text, status, toxicity_score, created_at
        FROM message
        ORDER BY created_at DESC
        LIMIT $1 OFFSET $2
    `

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		r.logger.Errorf("Message repo: list failed: %s", err)
		return nil, err
	}
	defer rows.Close()

	var records []*domain.CheckRecord
	for rows.Next() {
		var id uuid.UUID
		var messageID string
		var text string
		var status string
		var toxicityScore int
		var createdAt time.Time

		if err := rows.Scan(&id, &messageID, &text, &status, &toxicityScore, &createdAt); err != nil {
			r.logger.Errorf("Message repo: scan list failed: %s", err)
			return nil, err
		}

		confidence := float64(toxicityScore) / 100.0

		records = append(records, &domain.CheckRecord{
			ID:        id.String(),
			MessageID: messageID,
			Text:      text,
			Verdict: domain.Verdict{
				Label:      status,
				Confidence: confidence,
				AllScores:  nil,
			},
			CreatedAt: createdAt,
		})
	}

	if err := rows.Err(); err != nil {
		r.logger.Errorf("Message repo: process list rows failed: %s", err)
		return nil, err
	}

	return records, nil
}

// GetByID возвращает запись по её идентификатору.
// Если запись не найдена, возвращает (nil, nil).
func (r *MessageRepository) GetByID(ctx context.Context, id string) (*domain.CheckRecord, error) {
	query := `
        SELECT id, COALESCE(message_id, ''), text, status, toxicity_score, created_at
        FROM message
        WHERE id = $1
    `

	var uid uuid.UUID
	var messageID string
	var text string
	var status string
	var toxicityScore int
	var createdAt time.Time

	err := r.db.QueryRow(ctx, query, id).Scan(&uid, &messageID, &text, &status, &toxicityScore, &createdAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		r.logger.Errorf("Message repo: get by id failed: %s", err)
		return nil, err
	}

	confidence := float64(toxicityScore) / 100.0

	return &domain.CheckRecord{
		ID:        uid.String(),
		MessageID: messageID,
		Text:      text,
		Verdict: domain.Verdict{
			Label:      status,
			Confidence: confidence,
			AllScores:  nil,
		},
		CreatedAt: createdAt,
	}, nil
}
