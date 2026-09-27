package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// MODEL-31: an analyze comparison's voice that is unknown or another account's answers
// NotFound, one of the owner's that is deleted FailedPrecondition, and an active one passes.
func TestExperimentVoicesTellAnUnknownVoiceFromADeletedOne(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "experiment-voice.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(ctx, auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
		if err := defaultVoiceBootstrap(ctx, handle, id); err != nil {
			t.Fatal(err)
		}
	}
	voices := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	active, err := voices.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	gone, _, err := voices.CreateVoice(ctx, "alice", "옛 말투", voice.LanguageKorean, nil)
	if err != nil {
		t.Fatal(err)
	}
	// A tombstone, as DeleteVoice leaves one; its guards are the voice context's own tests.
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE voices SET deleted_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), gone.ID); err != nil {
		t.Fatal(err)
	}
	foreign, err := voices.DefaultVoice(ctx, "bob")
	if err != nil {
		t.Fatal(err)
	}
	adapter := experimentVoices{service: voices}
	for name, tc := range map[string]struct {
		voiceID string
		want    error
	}{
		"active":  {active.ID, nil},
		"deleted": {gone.ID, experiment.ErrVoiceUnavailable},
		"unknown": {"voice-nobody", experiment.ErrVoiceNotFound},
		"foreign": {foreign.ID, experiment.ErrVoiceNotFound},
	} {
		if err := adapter.ActiveVoice(ctx, "alice", tc.voiceID); !errors.Is(err, tc.want) || (tc.want == nil && err != nil) {
			t.Fatalf("%s voice = %v, want %v", name, err, tc.want)
		}
	}
}
