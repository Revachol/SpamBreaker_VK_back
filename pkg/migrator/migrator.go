package migrator

import (
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type Migrator struct {
	m *migrate.Migrate
}

func NewMigrator(databaseURL, migrationsPath string) (*Migrator, error) {
	m, err := migrate.New(
		fmt.Sprintf("file://%s", migrationsPath),
		databaseURL,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Migrator{m: m}, nil
}

func (mg *Migrator) Up() error {
	if err := mg.m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}
	return nil
}

func (mg *Migrator) Down() error {
	if err := mg.m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}
	return nil
}

func (mg *Migrator) Steps(steps int) error {
	if err := mg.m.Steps(steps); err != nil {
		return fmt.Errorf("failed to apply steps: %w", err)
	}
	return nil
}

func (mg *Migrator) Version() (uint, bool, error) {
	return mg.m.Version()
}

func (mg *Migrator) Close() {
	if mg.m != nil {
		mg.m.Close()
	}
}
