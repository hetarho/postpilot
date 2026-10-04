package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
)

// attachedPost is generation's post port answering one post's attachments and nothing else.
type attachedPost struct{ input generation.PostInput }

func (p attachedPost) AttachedImages(context.Context, string, string) (generation.PostInput, error) {
	return p.input, nil
}
func (attachedPost) SetObservations(context.Context, string, string, []generation.Observation) error {
	return errors.New("a snapshot writes nothing")
}
func (attachedPost) SetGeneratedContent(context.Context, string, string, generation.PostContent, generation.Language, *generation.WriteAnnotations) error {
	return errors.New("a snapshot writes nothing")
}
func (attachedPost) SetStoryline(context.Context, string, string, generation.Storyline) error {
	return errors.New("a snapshot writes nothing")
}

// Generation's write-only readers, which an observe snapshot never reaches.
type (
	unreadTemplates    struct{ generation.TemplateBriefs }
	unreadGuidelines   struct{ generation.GuidelinesForPrompt }
	unreadQualityRules struct {
		generation.QualityRulesForPrompt
	}
)

// MODEL-30, VIDEO-13: an observe comparison over 9 photos and a video holds every call each
// candidate makes — ⌈9/batch⌉ photo batches and one call for the video — counted by generation's
// own arithmetic, not one call per candidate.
func TestAnObserveComparisonHoldsEveryObserveCall(t *testing.T) {
	const batch = 4
	images := make([]generation.Image, 0, 10)
	for i := range 9 {
		images = append(images, generation.Image{Filename: fmt.Sprintf("IMG_%d.jpg", i+1), Key: fmt.Sprintf("key-%d", i+1), Kind: generation.AttachmentPhoto, ContentType: "image/jpeg"})
	}
	images = append(images, generation.Image{Filename: "clip.mp4", Key: "key-clip", Kind: generation.AttachmentVideo, ContentType: "video/mp4", DurationMs: 4000})
	generations := generation.NewService(
		attachedPost{input: generation.PostInput{Slug: "post", UserID: "alice", Images: images}}, freezeProfiles{}, &recordingModels{}, freezeImages{}, nil,
		batch, generation.DefaultReasoningPolicy(), testCompletionBudget(),
		generation.Deps{
			Experiments: freezeExperiments{}, Templates: unreadTemplates{}, Guidelines: unreadGuidelines{}, Memories: freezeMemories{},
			Candidates: freezeCandidates{}, Videos: freezeLinker{}, VideoURLTTL: time.Minute, QualityRules: unreadQualityRules{},
		},
	)
	ctx := context.Background()
	runner := experimentRunner{generation: generations}
	snapshot, err := runner.Snapshot(ctx, experiment.StartRequest{UserID: "alice", PostSlug: "post", Stage: experiment.StageObserve})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := runner.ObserveCalls(experiment.StageObserve, snapshot.Content)
	if want := (9+batch-1)/batch + 1; err != nil || calls != want {
		t.Fatalf("observe calls = %d err=%v, want %d", calls, err, want)
	}
	if _, err := runner.ObserveCalls(experiment.Stage("analyze"), snapshot.Content); !errors.Is(err, experiment.ErrInvalidStage) {
		t.Fatalf("an unknown stage = %v", err)
	}

	handle, err := db.Open(filepath.Join(t.TempDir(), "experiment-hold.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	admission := &capturedAdmission{}
	queue.Admit(admission)
	if _, err := (experimentJobs{queue: queue}).EnqueueExperiment(ctx, experiment.JobRequest{
		UserID: "alice", ExperimentID: "exp-observe", Stage: experiment.StageObserve,
		Models: []string{"p/a", "p/b"}, ObserveCalls: calls,
	}); err != nil {
		t.Fatal(err)
	}
	if len(admission.starts) != 1 || len(admission.starts[0].Calls) != 2 {
		t.Fatalf("admitted %+v", admission.starts)
	}
	for _, call := range admission.starts[0].Calls {
		if call.Stage != "observe" || call.Count != calls {
			t.Fatalf("planned call %+v, want observe %d times", call, calls)
		}
	}
}
