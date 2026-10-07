package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/pressly/goose/v3"
)

const waitExpiryMigrationFile = "0149_job_wait_expiry.sql"

func waitExpiryMigration() *goose.Migration {
	return goose.NewGoMigration(149, &goose.GoFunc{RunTx: addWaitExpiry},
		&goose.GoFunc{RunTx: func(context.Context, *sql.Tx) error { return nil }})
}

// Deployed main has migrations145–147 without the uncommitted local144 draft.
// Add the column at a new forward version. A database that already applied that
// local draft keeps both its nullable TEXT column values and its144 history.
// No missing or out-of-order migration is silently admitted by this guard.
func addWaitExpiry(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_info(generation_jobs)")
	if err != nil {
		return err
	}
	foundTable, foundColumn := false, false
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, declaredType string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &declaredType, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		foundTable = true
		if strings.EqualFold(name, "wait_expires_at") {
			if !strings.EqualFold(strings.TrimSpace(declaredType), "TEXT") || notNull != 0 || defaultValue.Valid || primaryKey != 0 {
				rows.Close()
				return fmt.Errorf("generation_jobs.wait_expires_at must be nullable TEXT without a default or primary key")
			}
			foundColumn = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !foundTable {
		return fmt.Errorf("generation_jobs is required before wait expiry migration")
	}
	if foundColumn {
		return nil
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE generation_jobs ADD COLUMN wait_expires_at TEXT")
	return err
}
