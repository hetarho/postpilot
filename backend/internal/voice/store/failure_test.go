package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
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

func TestLearningFailureWritesStructuredColumnsAndRetryClearsAllFailures(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-failure.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const at = "2026-08-31T00:00:00Z"
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + at + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at,source_language) VALUES('voice','alice','English',1,'` + at + `','` + at + `','en')`,
		`INSERT INTO posts(slug,user_id,voice_id,title,memo,status,created_at,updated_at,target_language,content_language) VALUES('post','alice','voice','','','finalized','` + at + `','` + at + `','en','en')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	store := New(handle.Writer, handle.Reader)
	createdAt, _ := time.Parse(time.RFC3339, at)
	failure := &voice.Failure{Reason: "MODEL_RATE_LIMITED", Params: map[string]string{"retry_after_seconds": "3"}, TechnicalDetail: "provider 429"}
	event := voice.LearningEvent{ID: "event", UserID: "alice", VoiceID: "voice", PostSlug: "post", BaselineRevision: 1, InputHash: "hash", BaselineJSON: "{}", FinalJSON: "{}", ModelRef: "provider/model", Status: "retryable", Error: "must not be written", Failure: failure, CreatedAt: createdAt, ContentLanguage: voice.LanguageEnglish, SourceLanguage: voice.LanguageEnglish}
	if err := store.InsertLearningEvent(ctx, event); err != nil {
		t.Fatal(err)
	}
	assertFailureColumns(t, handle.Writer, "event", false, true)
	got, err := store.GetLearningEvent(ctx, "alice", "event")
	if err != nil || !reflect.DeepEqual(got.Failure, failure) || got.Error != "" {
		t.Fatalf("stored event = %#v, err = %v", got, err)
	}

	// Simulate a row produced by a legacy binary, then verify retry clears both
	// compatibility prose and the structured columns atomically.
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE voice_learning_events SET error='legacy raw',error_reason='UNKNOWN_FAILURE',error_params='{}',technical_detail='legacy detail' WHERE id='event'`); err != nil {
		t.Fatal(err)
	}
	if err := store.SetLearningEventStatus(ctx, "alice", "event", "queued", nil, nil); err != nil {
		t.Fatal(err)
	}
	assertFailureColumns(t, handle.Writer, "event", false, false)

	for _, statement := range []string{
		`INSERT INTO voice_authored_sources(id,user_id,voice_id,post_slug,learning_event_id,title,tags,body,excerpt,created_at) VALUES('source','alice','voice','post','event','source','[]','body','body','` + at + `')`,
		`INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('rule','alice','voice','rule','rule','syntax',1,'candidate','manual','` + at + `','` + at + `')`,
		`INSERT INTO voice_rule_comparisons(id,user_id,voice_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,created_at,source_language) VALUES('comparison','alice','voice','rule','source',1,'p/m',1200,'{}','left','failed','` + at + `','en')`,
		`INSERT INTO voice_rule_comparison_candidates(id,comparison_id,display_side,status,error) VALUES('candidate','comparison','left','failed','legacy candidate raw')`,
		`INSERT INTO voice_profile_validations(id,user_id,voice_id,profile_version,analyze_model_ref,write_model_ref,judge_enabled,status,created_at,source_language) VALUES('validation','alice','voice',1,'p/a','p/w',0,'failed','` + at + `','en')`,
		`INSERT INTO voice_profile_validation_items(id,validation_id,source_id,voice_id,user_id,position,status,error) VALUES('item','validation','source','voice','alice',0,'failed','legacy item raw')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	comparison := voice.RuleComparison{ID: "comparison", UserID: "alice", VoiceID: "voice", Status: "failed", Candidates: []voice.ComparisonCandidate{{ID: "candidate", ComparisonID: "comparison", Status: "failed", Failure: failure, Error: "must not be written"}}}
	if err := store.UpdateRuleComparison(ctx, comparison); err != nil {
		t.Fatal(err)
	}
	assertAggregateFailureColumns(t, handle.Writer, "voice_rule_comparison_candidates", "candidate", false, true)
	comparison.Status = "review"
	comparison.Candidates[0].Status = "succeeded"
	comparison.Candidates[0].Failure = nil
	if err := store.UpdateRuleComparison(ctx, comparison); err != nil {
		t.Fatal(err)
	}
	assertAggregateFailureColumns(t, handle.Writer, "voice_rule_comparison_candidates", "candidate", false, false)

	validation := voice.ProfileValidation{ID: "validation", UserID: "alice", VoiceID: "voice", Status: "failed", Items: []voice.ValidationItem{{ID: "item", ValidationID: "validation", Status: "failed", Failure: failure, Error: "must not be written"}}}
	if err := store.UpdateProfileValidation(ctx, validation); err != nil {
		t.Fatal(err)
	}
	assertAggregateFailureColumns(t, handle.Writer, "voice_profile_validation_items", "item", false, true)
	validation.Status = "done"
	validation.Items[0].Status = "scored"
	validation.Items[0].Failure = nil
	if err := store.UpdateProfileValidation(ctx, validation); err != nil {
		t.Fatal(err)
	}
	assertAggregateFailureColumns(t, handle.Writer, "voice_profile_validation_items", "item", false, false)
}

func assertFailureColumns(t *testing.T, database *sql.DB, eventID string, wantRaw, wantStructured bool) {
	t.Helper()
	var raw, reason, params, detail sql.NullString
	if err := database.QueryRow(`SELECT error,error_reason,error_params,technical_detail FROM voice_learning_events WHERE id=?`, eventID).Scan(&raw, &reason, &params, &detail); err != nil {
		t.Fatal(err)
	}
	if raw.Valid != wantRaw || reason.Valid != wantStructured || params.Valid != wantStructured || detail.Valid != wantStructured {
		t.Fatalf("columns raw=%#v reason=%#v params=%#v detail=%#v", raw, reason, params, detail)
	}
}

func assertAggregateFailureColumns(t *testing.T, database *sql.DB, table, id string, wantRaw, wantStructured bool) {
	t.Helper()
	var raw, reason, params, detail sql.NullString
	query := `SELECT error,error_reason,error_params,technical_detail FROM ` + table + ` WHERE id=?`
	if err := database.QueryRow(query, id).Scan(&raw, &reason, &params, &detail); err != nil {
		t.Fatal(err)
	}
	if raw.Valid != wantRaw || reason.Valid != wantStructured || params.Valid != wantStructured || detail.Valid != wantStructured {
		t.Fatalf("%s columns raw=%#v reason=%#v params=%#v detail=%#v", table, raw, reason, params, detail)
	}
}
