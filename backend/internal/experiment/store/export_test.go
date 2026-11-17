package store

import (
	"context"
	"database/sql"

	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

// NewWithReader is New with the read side handed over, so a test can see every read it makes.
func NewWithReader(writer *sql.DB, reader sqlc.DBTX) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}

// AttachTestPhotosTx runs AttachTestPhotos in a transaction of its own that commits only on
// success, the way an admission holds it.
func AttachTestPhotosTx(ctx context.Context, writer *sql.DB, user, testID string, ids []string) error {
	tx, err := writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = AttachTestPhotos(ctx, sqlc.New(tx), user, testID, ids); err != nil {
		return err
	}
	return tx.Commit()
}
