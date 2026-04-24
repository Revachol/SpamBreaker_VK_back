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

// ListByOwnerOrAdmin возвращает приложения, где пользователь — владелец или соадмин.
func (r *ApplicationRepository) ListByOwnerOrAdmin(ctx context.Context, userID string) ([]*domain.Application, error) {
	query := `
		SELECT DISTINCT a.id, a.name, a.platform, a.external_id, a.token, a.owner_id, a.status, a.verified_at, a.created_at, a.updated_at
		FROM application a
		LEFT JOIN application_admins aa ON aa.application_id = a.id
		WHERE a.owner_id = $1 OR aa.moderator_id = $1
		ORDER BY a.created_at DESC
	`

	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		r.logger.Errorf("Error parsing user id %s: %s", userID, err)
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, parsedUserID)
	if err != nil {
		r.logger.Errorf("Error listing accessible applications for user %s: %s", userID, err)
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

// AddAdmin добавляет соадмина к приложению.
func (r *ApplicationRepository) AddAdmin(ctx context.Context, appID, moderatorID string) error {
	query := `
		INSERT INTO application_admins (application_id, moderator_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`

	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}
	parsedModID, err := uuid.Parse(moderatorID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, parsedAppID, parsedModID)
	if err != nil {
		r.logger.Errorf("Error adding admin %s to app %s: %s", moderatorID, appID, err)
	}
	return err
}

// RemoveAdmin удаляет соадмина из приложения.
func (r *ApplicationRepository) RemoveAdmin(ctx context.Context, appID, moderatorID string) error {
	query := `DELETE FROM application_admins WHERE application_id = $1 AND moderator_id = $2`

	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		return err
	}
	parsedModID, err := uuid.Parse(moderatorID)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, query, parsedAppID, parsedModID)
	if err != nil {
		r.logger.Errorf("Error removing admin %s from app %s: %s", moderatorID, appID, err)
	}
	return err
}

// ListAdminIDs возвращает список ID модераторов-соадминов приложения.
func (r *ApplicationRepository) ListAdminIDs(ctx context.Context, appID string) ([]string, error) {
	query := `SELECT moderator_id FROM application_admins WHERE application_id = $1`

	parsedAppID, err := uuid.Parse(appID)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, query, parsedAppID)
	if err != nil {
		r.logger.Errorf("Error listing admins for app %s: %s", appID, err)
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var modID uuid.UUID
		if err := rows.Scan(&modID); err != nil {
			return nil, err
		}
		ids = append(ids, modID.String())
	}

	return ids, rows.Err()
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
