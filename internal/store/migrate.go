package store

import (
	"errors"
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations applies all pending up-migrations from migrationsFS against
// databaseURL. It is idempotent: already-applied migrations are skipped.
// migrationsFS must contain a "migrations" sub-directory with *.sql files.
func RunMigrations(databaseURL string, migrationsFS fs.FS) error {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: open migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("store: create migrator: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}
