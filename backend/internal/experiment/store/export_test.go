package store

import (
	"database/sql"

	"github.com/postpilot/backend/internal/experiment/store/sqlc"
)

// NewWithReader is New with the read side handed over, so a test can see every read it makes.
func NewWithReader(writer *sql.DB, reader sqlc.DBTX) *Store {
	return &Store{writer: writer, write: sqlc.New(writer), read: sqlc.New(reader)}
}
