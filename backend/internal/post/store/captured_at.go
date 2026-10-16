package store

import (
	"database/sql"
	"time"

	"github.com/postpilot/backend/internal/platform/capturetime"
)

func capturedAtColumn(at *time.Time) sql.NullString {
	value := capturetime.Format(at)
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}
func readCapturedAt(value sql.NullString) (*time.Time, error) {
	if !value.Valid {
		return nil, nil
	}
	return capturetime.Parse(&value.String)
}
