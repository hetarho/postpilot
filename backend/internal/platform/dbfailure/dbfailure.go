// Package dbfailure owns how a failure (reason, params, technical detail) is stored in
// three text columns. It has no business meaning: each store converts to and from its
// own domain Failure type.
package dbfailure

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Failure is the decoded column triple.
type Failure struct {
	Reason          string
	Params          map[string]string
	TechnicalDetail string
}

// ValidReason reports whether reason is an upper-snake code: first char A-Z, then A-Z,
// 0-9 or single underscores, not ending in an underscore.
func ValidReason(reason string) bool {
	if reason == "" || reason[0] < 'A' || reason[0] > 'Z' || reason[len(reason)-1] == '_' {
		return false
	}
	previousUnderscore := false
	for i, char := range reason {
		if char >= 'A' && char <= 'Z' {
			previousUnderscore = false
			continue
		}
		if char == '_' && !previousUnderscore {
			previousUnderscore = true
			continue
		}
		if i > 0 && char >= '0' && char <= '9' {
			previousUnderscore = false
			continue
		}
		return false
	}
	return true
}

func nullString(value string) sql.NullString {
	return sql.NullString{String: value, Valid: value != ""}
}

// Columns encodes a failure as reason, params and detail columns. A nil failure stores
// NULLs; nil params are stored as an empty JSON object.
func Columns(failure *Failure) (sql.NullString, sql.NullString, sql.NullString, error) {
	if failure == nil {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, nil
	}
	if !ValidReason(failure.Reason) {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("invalid reason %q", failure.Reason)
	}
	params := failure.Params
	if params == nil {
		params = map[string]string{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("encode params: %w", err)
	}
	return nullString(failure.Reason), sql.NullString{String: string(raw), Valid: true}, nullString(failure.TechnicalDetail), nil
}

// FromRow decodes the columns. A row with no reason falls back to the legacy single
// column and reports unknownReason; empty decoded params come back as nil.
func FromRow(reason, params, detail, legacy sql.NullString, unknownReason string) (*Failure, error) {
	if !reason.Valid || strings.TrimSpace(reason.String) == "" {
		if params.Valid || detail.Valid {
			return nil, errors.New("failure params/detail present without reason")
		}
		technical := strings.TrimSpace(legacy.String)
		if technical == "" {
			return nil, nil
		}
		return &Failure{Reason: unknownReason, TechnicalDetail: technical}, nil
	}
	if !ValidReason(reason.String) {
		return nil, fmt.Errorf("invalid reason %q", reason.String)
	}
	if !params.Valid || !strings.HasPrefix(strings.TrimSpace(params.String), "{") {
		return nil, errors.New("failure params must be a JSON object")
	}
	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(params.String), &decoded); err != nil {
		return nil, fmt.Errorf("decode failure params: %w", err)
	}
	if len(decoded) == 0 {
		decoded = nil
	}
	return &Failure{Reason: reason.String, Params: decoded, TechnicalDetail: detail.String}, nil
}
