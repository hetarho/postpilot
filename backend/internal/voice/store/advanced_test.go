package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// Comparison and validation services precheck the voice, but deletion can commit before
// their aggregate insert. The store translates the durable trigger refusal back into the
// voice context's lifecycle sentinel instead of exposing a SQLite error.
func TestInactiveVoiceAggregateTriggersMapToDeleted(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-store.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	const now = "2026-08-30T00:00:00Z"
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)", now); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('default','alice','기본 말투',1,?,?),('gone','alice','삭제 말투',0,?,?)",
		now, now, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx,
		"UPDATE voices SET deleted_at = ? WHERE id = 'gone'", now); err != nil {
		t.Fatal(err)
	}

	store := voicestore.New(handle.Writer, handle.Reader)
	createdAt := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	comparisonErr := store.InsertRuleComparison(ctx, voice.RuleComparison{
		ID: "comparison", UserID: "alice", VoiceID: "gone", RuleID: "rule", SourceID: "source",
		ProfileVersion: 1, ModelRef: "provider/model", InputSnapshot: "{}", RuleOnSide: "left",
		Status: "queued", CreatedAt: createdAt,
	})
	if !errors.Is(comparisonErr, voice.ErrVoiceDeleted) {
		t.Fatalf("comparison insert = %v, want ErrVoiceDeleted", comparisonErr)
	}
	validationErr := store.InsertProfileValidation(ctx, voice.ProfileValidation{
		ID: "validation", UserID: "alice", VoiceID: "gone", ProfileVersion: 1,
		AnalyzeModelRef: "provider/analyze", WriteModelRef: "provider/write", Status: "queued", CreatedAt: createdAt,
	})
	if !errors.Is(validationErr, voice.ErrVoiceDeleted) {
		t.Fatalf("validation insert = %v, want ErrVoiceDeleted", validationErr)
	}
}
