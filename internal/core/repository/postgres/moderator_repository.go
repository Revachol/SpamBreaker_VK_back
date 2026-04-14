package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ModeratorRepository реализует интерфейс ModeratorRepository для PostgreSQL.
type ModeratorRepository struct {
	db *pgxpool.Pool
}

func NewModeratorRepository(db *pgxpool.Pool) *ModeratorRepository {
	return &ModeratorRepository{db: db}
}

// Create сохраняет нового модератора.
func (r *ModeratorRepository) Create(ctx context.Context, mod *domain.Moderator) error {
	query := `
		INSERT INTO moderator (id, username, email, password_hash, role, is_active, created_at, updated_at)
		VALUES (COALESCE($1, gen_random_uuid()), $2, NULLIF($3, ''), $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at
	`

	var idParam interface{} = nil
	if mod.ID != "" {
		parsed, err := uuid.Parse(mod.ID)
		if err != nil {
			return err
		}
		idParam = parsed
	}

	now := time.Now().UTC()
	var generatedID uuid.UUID

	err := r.db.QueryRow(ctx, query,
		idParam,
		mod.Username,
		mod.Email,
		mod.PasswordHash,
		mod.Role,
		mod.IsActive,
		now,
		now,
	).Scan(&generatedID, &mod.CreatedAt, &mod.UpdatedAt)
	if err != nil {
		return err
	}

	mod.ID = generatedID.String()
	return nil
}

// GetByUsername ищет модератора по имени. Возвращает (nil, nil) если не найден.
func (r *ModeratorRepository) GetByUsername(ctx context.Context, username string) (*domain.Moderator, error) {
	query := `
		SELECT id, username, COALESCE(email, ''), password_hash, role, is_active, created_at, updated_at
		FROM moderator
		WHERE username = $1
	`

	var mod domain.Moderator
	var id uuid.UUID

	err := r.db.QueryRow(ctx, query, username).Scan(
		&id,
		&mod.Username,
		&mod.Email,
		&mod.PasswordHash,
		&mod.Role,
		&mod.IsActive,
		&mod.CreatedAt,
		&mod.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	mod.ID = id.String()
	return &mod, nil
}

// GetByID ищет модератора по UUID. Возвращает (nil, nil) если не найден.
func (r *ModeratorRepository) GetByID(ctx context.Context, id string) (*domain.Moderator, error) {
	query := `
		SELECT id, username, COALESCE(email, ''), password_hash, role, is_active, created_at, updated_at
		FROM moderator
		WHERE id = $1
	`

	var mod domain.Moderator
	var uid uuid.UUID

	err := r.db.QueryRow(ctx, query, id).Scan(
		&uid,
		&mod.Username,
		&mod.Email,
		&mod.PasswordHash,
		&mod.Role,
		&mod.IsActive,
		&mod.CreatedAt,
		&mod.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	mod.ID = uid.String()
	return &mod, nil
}
