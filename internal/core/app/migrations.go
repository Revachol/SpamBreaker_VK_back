package app

import (
	"fmt"
	"strconv"

	"github.com/Revachol/SpamBreaker_VK_back/internal/core/config"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/logger"
	"github.com/Revachol/SpamBreaker_VK_back/pkg/migrator"
	"github.com/golang-migrate/migrate/v4"
)

func runMigrations(cfg *config.PostgresConfig, l logger.Log) {
	if !cfg.Migrated {
		l.Info("Skip migrations")
		return
	}
	l.Info("🔄 Running database migrations...")

	host := cfg.Host
	sport := cfg.Port
	user := cfg.User
	password := cfg.Password
	base := cfg.Base

	// Преобразуем порт в число
	port, err := strconv.Atoi(sport)
	if err != nil {
		l.Fatal("failed to convert port to int: %w", err)
	}

	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s", user, password, host, port, base, "disable")

	mgr, err := migrator.New(dbURL, cfg.Migrations)
	if err != nil {
		l.Fatalf("failed to create migrator: %w", err)
	}
	defer mgr.Close()

	if err := mgr.Up(); err != nil {
		l.Fatalf("failed to run migrations: %w", err)
	}

	version, dirty, err := mgr.Version()
	if err != nil && err != migrate.ErrNilVersion {
		l.Fatalf("failed to get migration version: %w", err)
	}

	if dirty {
		l.Warn("Database is in dirty state, consider fixing manually")
	}

	l.Infof("✅ Database migrations applied successfully. Version: %d", version)
}
