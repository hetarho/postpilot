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
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// MODEL-31: a comparison's voice that is unknown or another account's answers
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
		if err := createTestVoice(ctx, handle, id); err != nil {
			t.Fatal(err)
		}
	}
	voices := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	active, err := firstTestVoice(ctx, voices, "alice")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := voices.CreateVoice(ctx, "alice", "옛 말투")
	if err != nil {
		t.Fatal(err)
	}
	// A tombstone, as DeleteVoice leaves one; its guards are the voice context's own tests.
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE voices SET deleted_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), gone.ID); err != nil {
		t.Fatal(err)
	}
	foreign, err := firstTestVoice(ctx, voices, "bob")
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

// MODEL-67, VOICE-13: a 말투 반영 비교's job names neither a post nor the voice — its two write
// calls pass one admission — so the voice it froze deletes while the comparison runs; and the
// voice context's refusals reach the experiment context in its own words.
func TestAReflectionJobNeverHoldsItsVoice(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "reflection-job.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := createTestVoice(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	// The real queue answers the voice's deletion check, so a job naming it would refuse.
	voices := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, voiceJobs{queue: queue})
	found, err := firstTestVoice(ctx, voices, "alice")
	if err != nil {
		t.Fatal(err)
	}
	admission := &capturedAdmission{}
	queue.Admit(admission)
	korean := experiment.LanguageKorean
	id, err := experimentJobs{queue: queue}.EnqueueExperiment(ctx, experiment.JobRequest{
		UserID: "alice", ExperimentID: "exp-voice", Stage: experiment.StageWrite, TargetLanguage: &korean,
		Models: []string{"p/a", "p/b"},
	})
	if err != nil || id == "" {
		t.Fatalf("enqueue = %q %v", id, err)
	}
	if len(admission.starts) != 1 || len(admission.starts[0].Calls) != 2 {
		t.Fatalf("admitted %+v", admission.starts)
	}
	var voiceID, postSlug any
	if err := handle.Reader.QueryRow(`SELECT voice_id, post_slug FROM generation_jobs WHERE id = ?`, id).Scan(&voiceID, &postSlug); err != nil || voiceID != nil || postSlug != nil {
		t.Fatalf("the job names voice=%v post=%v err=%v", voiceID, postSlug, err)
	}
	if _, err := voices.DeleteVoice(ctx, "alice", found.ID); err != nil {
		t.Fatalf("a running comparison held the voice: %v", err)
	}
	for from, want := range map[error]error{
		voice.ErrVoiceRequired:         experiment.ErrVoiceRequired,
		voice.ErrVoiceNotFound:         experiment.ErrVoiceNotFound,
		voice.ErrVoiceDeleted:          experiment.ErrVoiceUnavailable,
		voice.ErrVoiceNotMade:          experiment.ErrVoiceNotMade,
		voice.ErrPromptNotFound:        experiment.ErrPromptNotFound,
		voice.ErrCheckPromptUnanswered: experiment.ErrPromptUnanswered,
	} {
		if got := reflectionError(from); !errors.Is(got, want) {
			t.Fatalf("%v became %v, want %v", from, got, want)
		}
	}
}
