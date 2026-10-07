package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

// migrationsFS carries the schema inside the binary ([I7]). There is no migration
// container and no manual step: whatever binary is running defines the schema it
// expects, so the two can never disagree.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies every pending migration through the single writer connection.
//
// Callers must treat a failure as fatal and exit before serving traffic — that is what
// makes the deploy's /health gate a rollback mechanism (DEPLOY.md §2): a bad migration
// means the container never answers, so the rollout restores the previous image.
func Migrate(ctx context.Context, writer *sql.DB) error {
	// The migrations live in a subdirectory of the embedded FS; goose expects the
	// migration files at the root of the FS it is given.
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}
	return migrate(ctx, writer, sub)
}

// migrate is the testable core: it takes the FS so a test can hand it a broken
// migration and assert the error propagates.
func migrate(ctx context.Context, writer *sql.DB, fsys fs.FS) error {
	if err := reconcileCreationLineage(ctx, writer, fsys); err != nil {
		return fmt.Errorf("reconcile creation migration lineage: %w", err)
	}
	options := []goose.ProviderOption{goose.WithLogger(goose.NopLogger())}
	if _, err := fs.Stat(fsys, waitExpiryMigrationFile); err == nil {
		// SQL remains available to schema generation. Runtime uses the narrow
		// transactional guard so an already-applied local144 keeps its values.
		options = append(options, goose.WithExcludeNames([]string{waitExpiryMigrationFile}),
			goose.WithGoMigrations(waitExpiryMigration()))
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read wait expiry migration: %w", err)
	}
	for _, guarded := range []struct {
		name    string
		version int64
	}{{creationMigrationFile, 150}, {writingMigrationFile, 151}} {
		if _, err := fs.Stat(fsys, guarded.name); err == nil {
			name, version := guarded.name, guarded.version
			options = append(options, goose.WithExcludeNames([]string{name}), goose.WithGoMigrations(goose.NewGoMigration(version,
				&goose.GoFunc{RunTx: func(ctx context.Context, tx *sql.Tx) error { return applyCreationSchema(ctx, tx, fsys, name) }},
				&goose.GoFunc{RunTx: func(context.Context, *sql.Tx) error { return nil }})))
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, writer, fsys, options...)
	if err != nil {
		return fmt.Errorf("init migrations: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	for _, r := range results {
		slog.Info("migration applied", "version", r.Source.Version, "name", r.Source.Path)
	}
	return nil
}
