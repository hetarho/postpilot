package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/postpilot/backend/internal/voice"
)

func encodeFailure(failure *voice.Failure) (sql.NullString, sql.NullString, sql.NullString, error) {
	if failure == nil || failure.Empty() {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, nil
	}
	if !validFailureReason(failure.Reason) {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("invalid failure reason %q", failure.Reason)
	}
	params := failure.Params
	if params == nil {
		params = map[string]string{}
	}
	raw, err := json.Marshal(params)
	if err != nil {
		return sql.NullString{}, sql.NullString{}, sql.NullString{}, fmt.Errorf("encode failure params: %w", err)
	}
	return nullableString(failure.Reason), nullableString(string(raw)), nullableString(failure.TechnicalDetail), nil
}

func decodeFailure(reason, params, detail, legacyRaw sql.NullString) (*voice.Failure, error) {
	if !reason.Valid || strings.TrimSpace(reason.String) == "" {
		if params.Valid || detail.Valid {
			return nil, errors.New("failure params/detail present without reason")
		}
		legacy := strings.TrimSpace(legacyRaw.String)
		if legacy == "" {
			return nil, nil
		}
		return &voice.Failure{Reason: voice.FailureReasonUnknown, Params: map[string]string{}, TechnicalDetail: legacy}, nil
	}
	if !validFailureReason(reason.String) {
		return nil, fmt.Errorf("invalid failure reason %q", reason.String)
	}
	if !params.Valid || !strings.HasPrefix(strings.TrimSpace(params.String), "{") {
		return nil, errors.New("failure params must be a JSON object")
	}
	decoded := map[string]string{}
	if err := json.Unmarshal([]byte(params.String), &decoded); err != nil {
		return nil, fmt.Errorf("decode failure params: %w", err)
	}
	return &voice.Failure{Reason: reason.String, Params: decoded, TechnicalDetail: detail.String}, nil
}

func validFailureReason(reason string) bool {
	if reason == "" || reason[0] < 'A' || reason[0] > 'Z' || reason[len(reason)-1] == '_' {
		return false
	}
	previousUnderscore := false
	for _, char := range reason {
		switch {
		case char >= 'A' && char <= 'Z':
			previousUnderscore = false
		case char >= '0' && char <= '9':
			previousUnderscore = false
		case char == '_' && !previousUnderscore:
			previousUnderscore = true
		default:
			return false
		}
	}
	return true
}
