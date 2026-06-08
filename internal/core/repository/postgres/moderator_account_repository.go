package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/repository/interfaces"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain"
	"github.com/Revachol/SpamBreaker_VK_back/internal/domain/expectation"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ interfaces.ModeratorAccountRepository = (*ModeratorAccountRepo)(nil)

type ModeratorAccountRepo struct {
	pool   *pgxpool.Pool
	logger *logger.Log
}

func NewModeratorAccountRepo(pool *pgxpool.Pool, l *logger.Log) *ModeratorAccountRepo {
	return &ModeratorAccountRepo{pool: pool, logger: l}
}

func (r *ModeratorAccountRepo) Create(ctx context.Context, acc *domain.ModeratorAccount) error {
	query := `INSERT INTO moderator_account (moderator_id, platform, account_id, verification_token, token_expires_at)
              VALUES ($1, $2, $3, $4, $5) RETURNING id`
	return r.pool.QueryRow(ctx, query,
		acc.ModeratorID, acc.Platform, acc.AccountID, acc.VerificationToken, acc.TokenExpiresAt,
	).Scan(&acc.ID)
}

func (r *ModeratorAccountRepo) FindByVerificationToken(ctx context.Context, token string) (*domain.ModeratorAccount, error) {
	query := `SELECT id, moderator_id, platform, account_id,
                     verification_token, token_expires_at, verified_at
              FROM moderator_account WHERE verification_token = $1`
	row := r.pool.QueryRow(ctx, query, token)
	return scanAccount(row)
}

func (r *ModeratorAccountRepo) FindByID(ctx context.Context, id string) (*domain.ModeratorAccount, error) {
	query := `SELECT id, moderator_id, platform, account_id,
                     verification_token, token_expires_at, verified_at
              FROM moderator_account WHERE id = $1`
	row := r.pool.QueryRow(ctx, query, id)
	return scanAccount(row)
}

func (r *ModeratorAccountRepo) FindByPlatformAndModeratorID(ctx context.Context, platform, moderatorID string) (*domain.ModeratorAccount, error) {
	query := `SELECT id, moderator_id, platform, account_id,
                     verification_token, token_expires_at, verified_at
              FROM moderator_account WHERE platform = $1 AND moderator_id = $2`
	row := r.pool.QueryRow(ctx, query, platform, moderatorID)
	return scanAccount(row)
}

func (r *ModeratorAccountRepo) FindByPlatformAndAccountID(ctx context.Context, platform, accountID string) (*domain.ModeratorAccount, error) {
	query := `SELECT id, moderator_id, platform, account_id,
                     verification_token, token_expires_at, verified_at
              FROM moderator_account
              WHERE platform = $1 AND account_id = $2`
	row := r.pool.QueryRow(ctx, query, platform, accountID)
	return scanAccount(row)
}

func (r *ModeratorAccountRepo) ListByModeratorID(ctx context.Context, moderatorID, platform string, active *bool) ([]domain.ModeratorAccount, error) {
	query := `SELECT id, moderator_id, platform, account_id,
                     verification_token, token_expires_at, verified_at
              FROM moderator_account`
	args := []interface{}{moderatorID}
	conditions := []string{"moderator_id = $1"}

	if platform != "" {
		args = append(args, platform)
		conditions = append(conditions, fmt.Sprintf("platform = $%d", len(args)))
	}
	if active != nil {
		args = append(args, *active)
		conditions = append(conditions, fmt.Sprintf("(verified_at IS NOT NULL) = $%d", len(args)))
	}

	query += " WHERE " + strings.Join(conditions, " AND ") + " ORDER BY platform, account_id"
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []domain.ModeratorAccount
	for rows.Next() {
		acc, err := scanAccountFromRows(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, *acc)
	}
	return accounts, rows.Err()
}

func (r *ModeratorAccountRepo) VerifyAccount(ctx context.Context, id, accID string) error {
	query := `UPDATE moderator_account
              SET verified_at = NOW(),
                  verification_token = NULL,
                  token_expires_at = NULL,
                  accoutnt_id = $2
              WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, accID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return expectation.ErrNotFound
	}
	return nil
}

func (r *ModeratorAccountRepo) UpdateToken(ctx context.Context, id string, token string, expiresAt time.Time) error {
	query := `UPDATE moderator_account
              SET verification_token = $2, token_expires_at = $3
              WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id, token, expiresAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return expectation.ErrNotFound
	}
	return nil
}

func (r *ModeratorAccountRepo) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM moderator_account WHERE id = $1`
	tag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return expectation.ErrNotFound
	}
	return nil
}

// вспомогательные функции сканирования
func scanAccount(row pgx.Row) (*domain.ModeratorAccount, error) {
	acc := &domain.ModeratorAccount{}
	err := row.Scan(
		&acc.ID, &acc.ModeratorID, &acc.Platform, &acc.AccountID,
		&acc.VerificationToken, &acc.TokenExpiresAt, &acc.VerifiedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, expectation.ErrNotFound
	}
	return acc, err
}

func scanAccountFromRows(rows pgx.Rows) (*domain.ModeratorAccount, error) {
	acc := &domain.ModeratorAccount{}
	err := rows.Scan(
		&acc.ID, &acc.ModeratorID, &acc.Platform, &acc.AccountID,
		&acc.VerificationToken, &acc.TokenExpiresAt, &acc.VerifiedAt,
	)
	return acc, err
}
