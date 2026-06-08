package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ interfaces.ApplicationSettingsRepository = (*ApplicationSettingsRepository)(nil)

// ApplicationSettingsRepository реализует интерфейс ApplicationSettingsRepository для PostgreSQL.
type ApplicationSettingsRepository struct {
	db     *pgxpool.Pool
	logger logger.Log
}

func NewApplicationSettingsRepository(db *pgxpool.Pool, l logger.Log) *ApplicationSettingsRepository {
	return &ApplicationSettingsRepository{db: db, logger: l}
}

// Create сохраняет новые настройки приложения.
func (r *ApplicationSettingsRepository) Create(ctx context.Context, settings *domain.ApplicationSettings) error {
	query := `
		INSERT INTO application_settings
		(id, application_id, toxicity_threshold, action_on_spam, auto_moderate, notify_moderator, allowed_languages, banned_words, created_at, updated_at)
		VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	var idParam interface{} = nil
	if settings.ID != "" {
		parsed, err := uuid.Parse(settings.ID)
		if err != nil {
			r.logger.Errorf("Error parsing settings id %s: %s", settings.ID, err)
			return err
		}
		idParam = parsed
	}

	now := time.Now().UTC()
	var generatedID uuid.UUID

	err := r.db.QueryRow(ctx, query,
		idParam,
		settings.ApplicationID,
		settings.ToxicityThreshold,
		settings.ActionOnSpam,
		settings.AutoModerate,
		settings.NotifyModerator,
		settings.AllowedLanguages,
		settings.BannedWords,
		now,
		now,
	).Scan(&generatedID, &settings.CreatedAt, &settings.UpdatedAt)
	if err != nil {
		r.logger.Errorf("Error creating application settings %s: %s", settings.ID, err)
		return err
	}

	settings.ID = generatedID.String()
	return nil
}

// GetByApplicationID ищет настройки по ID приложения. Возвращает (nil, nil) если не найдены.
func (r *ApplicationSettingsRepository) GetByApplicationID(ctx context.Context, applicationID string) (*domain.ApplicationSettings, error) {
	query := `
		SELECT id, application_id, toxicity_threshold, action_on_spam, auto_moderate, notify_moderator, allowed_languages, banned_words, created_at, updated_at
		FROM application_settings
		WHERE application_id = $1
	`

	var settings domain.ApplicationSettings
	var settingsID uuid.UUID
	var appID uuid.UUID

	err := r.db.QueryRow(ctx, query, applicationID).Scan(
		&settingsID,
		&appID,
		&settings.ToxicityThreshold,
		&settings.ActionOnSpam,
		&settings.AutoModerate,
		&settings.NotifyModerator,
		&settings.AllowedLanguages,
		&settings.BannedWords,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Errorf("Error getting application settings for application %s: %s", applicationID, err)
		return nil, err
	}

	settings.ID = settingsID.String()
	settings.ApplicationID = appID.String()

	return &settings, nil
}

// Update обновляет настройки приложения.
func (r *ApplicationSettingsRepository) Update(ctx context.Context, settings *domain.ApplicationSettings) error {
	query := `
		UPDATE application_settings
		SET toxicity_threshold = $2, action_on_spam = $3, auto_moderate = $4, notify_moderator = $5, allowed_languages = $6, banned_words = $7, updated_at = $8
		WHERE application_id = $1
		RETURNING updated_at
	`

	now := time.Now().UTC()

	err := r.db.QueryRow(ctx, query,
		settings.ApplicationID,
		settings.ToxicityThreshold,
		settings.ActionOnSpam,
		settings.AutoModerate,
		settings.NotifyModerator,
		settings.AllowedLanguages,
		settings.BannedWords,
		now,
	).Scan(&settings.UpdatedAt)
	if err != nil {
		r.logger.Errorf("Error updating application settings for application %s: %s", settings.ApplicationID, err)
		return err
	}

	return nil
}

// Delete удаляет настройки приложения.
func (r *ApplicationSettingsRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM application_settings WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.logger.Errorf("Error deleting application settings %s: %s", id, err)
		return err
	}

	return nil
}
