package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Revachol/SpamBreaker_VK_back/internal/main/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewConnect(ctx context.Context, cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	if cfg.Port == "" {
		return nil, fmt.Errorf("PostgreSQL port is empty")
	}

	host := cfg.Host
	sport := cfg.Port
	user := cfg.User
	password := cfg.Password
	base := cfg.Base

	// Преобразуем порт в число
	port, err := strconv.Atoi(sport)
	if err != nil {
		return nil, fmt.Errorf("failed to convert port to int: %w", err)
	}

	poolConfig, err := pgxpool.ParseConfig(
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s", user, password, host, port, base),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = cfg.MaxConns
	poolConfig.MinConns = cfg.MinConns
	poolConfig.MaxConnLifetime = cfg.MaxLife
	poolConfig.MaxConnIdleTime = cfg.MaxIdle
	poolConfig.HealthCheckPeriod = cfg.HealthCheckInterval

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}
