package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplicationRepository реализует интерфейс ApplicationRepository для PostgreSQL.
type ApplicationRepository struct {
	db     *pgxpool.Pool
	logger logger.Log
}

func NewApplicationRepository(db *pgxpool.Pool, l logger.Log) *ApplicationRepository {
	return &ApplicationRepository{db: db, logger: l}
}

// Create сохраняет новое приложение.
func (r *ApplicationRepository) Create(ctx context.Context, app *domain.Application) error {
	// For active applications, set VerifiedAt to current time
	if app.Status == "active" {
		app.VerifiedAt = time.Now().UTC()
	}

	query := `
		INSERT INTO application (id, name, platform, external_id, token, owner_id, status, verified_at, created_at, updated_at)
		VALUES (COALESCE($1, gen_random_uuid()), $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	var idParam interface{} = nil
	if app.ID != "" {
		parsed, err := uuid.Parse(app.ID)
		if err != nil {
			r.logger.Errorf("Error parsing application id %s: %s", app.ID, err)
			return err
		}
		idParam = parsed
	}

	now := time.Now().UTC()
	var generatedID uuid.UUID

	err := r.db.QueryRow(ctx, query,
		idParam,
		app.Name,
		app.Platform,
		app.ExternalID,
		app.Token,
		app.OwnerID,
		app.Status,
		app.VerifiedAt,
		now,
		now,
	).Scan(&generatedID, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		r.logger.Errorf("Error creating application %s: %s", app.ID, err)
		return err
	}

	app.ID = generatedID.String()
	return nil
}

// GetByID ищет приложение по UUID. Возвращает (nil, nil) если не найден.
func (r *ApplicationRepository) GetByID(ctx context.Context, id string) (*domain.Application, error) {
	query := `
		SELECT id, name, platform, external_id, token, owner_id, status, verified_at, created_at, updated_at
		FROM application
		WHERE id = $1
	`

	var app domain.Application
	var appID uuid.UUID
	var ownerID uuid.NullUUID

	err := r.db.QueryRow(ctx, query, id).Scan(
		&appID,
		&app.Name,
		&app.Platform,
		&app.ExternalID,
		&app.Token,
		&ownerID,
		&app.Status,
		&app.VerifiedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Errorf("Error getting application with id - %s: %s", id, err)
		return nil, err
	}

	app.ID = appID.String()
	if ownerID.Valid {
		app.OwnerID = ownerID.UUID.String()
	}

	return &app, nil
}

// GetByToken ищет приложение по токену. Возвращает (nil, nil) если не найден.
func (r *ApplicationRepository) GetByToken(ctx context.Context, token string) (*domain.Application, error) {
	query := `
		SELECT id, name, platform, external_id, token, owner_id, status, verified_at, created_at, updated_at
		FROM application
		WHERE token = $1
	`

	var app domain.Application
	var appID uuid.UUID
	var ownerID uuid.NullUUID

	err := r.db.QueryRow(ctx, query, token).Scan(
		&appID,
		&app.Name,
		&app.Platform,
		&app.ExternalID,
		&app.Token,
		&ownerID,
		&app.Status,
		&app.VerifiedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Errorf("Error getting application with token - %s: %s", token, err)
		return nil, err
	}

	app.ID = appID.String()
	if ownerID.Valid {
		app.OwnerID = ownerID.UUID.String()
	}

	return &app, nil
}

// GetByExternalIDAndPlatform ищет приложение по внешнему ID и платформе. Возвращает (nil, nil) если не найден.
func (r *ApplicationRepository) GetByExternalIDAndPlatform(ctx context.Context, externalID string, platform string) (*domain.Application, error) {
	query := `
		SELECT id, name, platform, external_id, token, owner_id, status, verified_at, created_at, updated_at
		FROM application
		WHERE external_id = $1 AND platform = $2
	`

	var app domain.Application
	var appID uuid.UUID
	var ownerID uuid.NullUUID

	err := r.db.QueryRow(ctx, query, externalID, platform).Scan(
		&appID,
		&app.Name,
		&app.Platform,
		&app.ExternalID,
		&app.Token,
		&ownerID,
		&app.Status,
		&app.VerifiedAt,
		&app.CreatedAt,
		&app.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		r.logger.Errorf("Error getting application with external_id - %s and platform - %s: %s", externalID, platform, err)
		return nil, err
	}

	app.ID = appID.String()
	if ownerID.Valid {
		app.OwnerID = ownerID.UUID.String()
	}

	return &app, nil
}

// Update обновляет приложение.
func (r *ApplicationRepository) Update(ctx context.Context, app *domain.Application) error {
	query := `
		UPDATE application
		SET name = $2, platform = $3, external_id = $4, token = $5, owner_id = $6, status = $7, verified_at = $8, updated_at = $9
		WHERE id = $1
		RETURNING updated_at
	`

	now := time.Now().UTC()
	var ownerID interface{} = nil
	if app.OwnerID != "" {
		parsed, err := uuid.Parse(app.OwnerID)
		if err != nil {
			r.logger.Errorf("Error parsing owner id %s: %s", app.OwnerID, err)
			return err
		}
		ownerID = parsed
	}

	err := r.db.QueryRow(ctx, query,
		app.ID,
		app.Name,
		app.Platform,
		app.ExternalID,
		app.Token,
		ownerID,
		app.Status,
		app.VerifiedAt,
		now,
	).Scan(&app.UpdatedAt)
	if err != nil {
		r.logger.Errorf("Error updating application %s: %s", app.ID, err)
		return err
	}

	return nil
}

// Delete удаляет приложение.
func (r *ApplicationRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM application WHERE id = $1`

	_, err := r.db.Exec(ctx, query, id)
	if err != nil {
		r.logger.Errorf("Error deleting application %s: %s", id, err)
		return err
	}

	return nil
}

// ListByOwner возвращает список приложений владельца.
func (r *ApplicationRepository) ListByOwner(ctx context.Context, ownerID string) ([]*domain.Application, error) {
	query := `
		SELECT id, name, platform, external_id, token, owner_id, status, verified_at, created_at, updated_at
		FROM application
		WHERE owner_id = $1
		ORDER BY created_at DESC
	`

	parsedOwnerID, err := uuid.Parse(ownerID)
	if err != nil {
		r.logger.Errorf("Error parsing owner id %s: %s", ownerID, err)
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, parsedOwnerID)
	if err != nil {
		r.logger.Errorf("Error listing applications for owner %s: %s", ownerID, err)
		return nil, err
	}
	defer rows.Close()

	var apps []*domain.Application
	for rows.Next() {
		var app domain.Application
		var appID uuid.UUID
		var appOwnerID uuid.NullUUID

		err := rows.Scan(
			&appID,
			&app.Name,
			&app.Platform,
			&app.ExternalID,
			&app.Token,
			&appOwnerID,
			&app.Status,
			&app.VerifiedAt,
			&app.CreatedAt,
			&app.UpdatedAt,
		)
		if err != nil {
			r.logger.Errorf("Error scanning application row: %s", err)
			return nil, err
		}

		app.ID = appID.String()
		if appOwnerID.Valid {
			app.OwnerID = appOwnerID.UUID.String()
		}

		apps = append(apps, &app)
	}

	if err := rows.Err(); err != nil {
		r.logger.Errorf("Error iterating application rows: %s", err)
		return nil, err
	}

	return apps, nil
}
