package store

import (
	"database/sql"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/voice"
)

func TestFailureCodecRoundTripAndLegacyFallback(t *testing.T) {
	tests := []struct {
		name    string
		failure *voice.Failure
	}{
		{name: "nil"},
		{name: "empty params", failure: &voice.Failure{Reason: voice.FailureReasonUnknown, TechnicalDetail: "provider timeout"}},
		{name: "allowlisted params", failure: &voice.Failure{Reason: "MODEL_RATE_LIMITED", Params: map[string]string{"retry_after_seconds": "2"}, TechnicalDetail: "429"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason, params, detail, err := encodeFailure(test.failure)
			if err != nil {
				t.Fatal(err)
			}
			if test.failure != nil && !test.failure.Empty() && (!params.Valid || params.String == "") {
				t.Fatal("structured failure did not persist a JSON object")
			}
			got, err := decodeFailure(reason, params, detail, sql.NullString{})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.failure) {
				// A nil Params map is canonically stored as an empty JSON object.
				if test.failure == nil || got == nil || test.failure.Reason != got.Reason || test.failure.TechnicalDetail != got.TechnicalDetail || len(got.Params) != 0 || len(test.failure.Params) != 0 {
					t.Fatalf("round trip = %#v, want %#v", got, test.failure)
				}
			}
		})
	}

	legacy, err := decodeFailure(sql.NullString{}, sql.NullString{}, sql.NullString{}, sql.NullString{String: "legacy provider prose", Valid: true})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Reason != voice.FailureReasonUnknown || legacy.TechnicalDetail != "legacy provider prose" || len(legacy.Params) != 0 {
		t.Fatalf("legacy fallback = %#v", legacy)
	}
}

func TestFailureCodecRejectsMalformedColumns(t *testing.T) {
	validReason := sql.NullString{String: voice.FailureReasonUnknown, Valid: true}
	tests := []struct {
		name   string
		reason sql.NullString
		params sql.NullString
		detail sql.NullString
	}{
		{name: "params without reason", params: sql.NullString{String: "{}", Valid: true}},
		{name: "detail without reason", detail: sql.NullString{String: "detail", Valid: true}},
		{name: "reason without params", reason: validReason},
		{name: "array params", reason: validReason, params: sql.NullString{String: "[]", Valid: true}},
		{name: "non string params", reason: validReason, params: sql.NullString{String: `{"attempt":2}`, Valid: true}},
		{name: "invalid reason", reason: sql.NullString{String: "provider_timeout", Valid: true}, params: sql.NullString{String: "{}", Valid: true}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := decodeFailure(test.reason, test.params, test.detail, sql.NullString{}); err == nil {
				t.Fatal("malformed failure was accepted")
			}
		})
	}
	if _, _, _, err := encodeFailure(&voice.Failure{Reason: "bad reason"}); err == nil {
		t.Fatal("invalid failure reason was encoded")
	}
}
