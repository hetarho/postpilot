package generation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// fakeCandidates is the guideline context's candidate recorder. It records nothing but what
// it was handed: this port is the whole seam, and the instruction crosses it as opaque text.
type fakeCandidates struct {
	recorded []struct{ userID, postSlug, instruction string }
	err      error
}

func (f *fakeCandidates) Record(_ context.Context, userID, postSlug, instruction string) error {
	f.recorded = append(f.recorded, struct{ userID, postSlug, instruction string }{userID, postSlug, instruction})
	return f.err
}

func candidateAwareService(t *testing.T, posts *fakePosts, models *fakeModels, candidates *fakeCandidates) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
	svc.SetGuidelineCandidates(candidates)
	return svc
}

func revisingModels() *fakeModels {
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"제목","summary":"요약","tags":["a","b","c"],"blocks":[{"type":"TEXT","content":"after"}]}`}, nil
	}
	return models
}

// A1: a completed revision records exactly one candidate, verbatim, against the post it ran
// on. The instruction is the payload's, not a re-read of any live row.
func TestCompletedRevisionRecordsTheInstructionVerbatim(t *testing.T) {
	const instruction = "여기  너무 광고 같아!! (특히 마지막 문단)"
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	candidates := &fakeCandidates{}
	svc := candidateAwareService(t, posts, revisingModels(), candidates)

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, instruction, false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(candidates.recorded) != 1 {
		t.Fatalf("recorded %d candidates", len(candidates.recorded))
	}
	got := candidates.recorded[0]
	if got.instruction != instruction {
		t.Fatalf("recorded %q, want the instruction verbatim", got.instruction)
	}
	if got.userID != "alice" || got.postSlug != "post" {
		t.Fatalf("recorded against %q/%q", got.userID, got.postSlug)
	}
}

// A1: a failed revision records nothing. That follows from the call position alone — the
// recording sits after the content is persisted — so this test guards the position.
func TestFailedRevisionRecordsNoCandidate(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	models := newFakeModels()
	models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
		return llm.Response{}, errors.New("provider down")
	}
	candidates := &fakeCandidates{}
	svc := candidateAwareService(t, posts, models, candidates)

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "광고 같아", false),
	}, func(string, int, int) {}); err == nil {
		t.Fatal("the failing revision reported success")
	}
	if len(candidates.recorded) != 0 {
		t.Fatalf("a failed revision recorded %d candidates", len(candidates.recorded))
	}
}

// A recording failure must never fail the revision: the revised content is already persisted
// and the user is looking at it, so a bookkeeping error cannot be allowed to discard it.
func TestRecordingFailureLeavesTheRevisionSuccessful(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	candidates := &fakeCandidates{err: errors.New("candidate store down")}
	svc := candidateAwareService(t, posts, revisingModels(), candidates)

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "광고 같아", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatalf("a recording failure failed the revision: %v", err)
	}
	if len(posts.contents) != 1 {
		t.Fatalf("content writes = %d, want the revision's result persisted", len(posts.contents))
	}
	if got := posts.contents[0].Blocks[0].Content; got != "after" {
		t.Fatalf("persisted content = %q", got)
	}
}

// An unwired recorder is the same outcome as a failed recording: the revision is the product.
func TestRevisionWithoutACandidateRecorderStillCompletes(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, revisingModels(), fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "광고 같아", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
}

// A12/[I5]: recording rides the revision job that is already running. It adds no provider
// call and no enqueue — the recorder is handed text and nothing else, and the model call
// count is exactly the revision's one.
func TestRecordingAddsNoProviderCallAndNoEnqueue(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
	models := revisingModels()
	jobs := &fakeJobs{}
	candidates := &fakeCandidates{}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget)
	svc.SetGuidelineCandidates(candidates)

	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
		Payload: mustRevisionPayload(t, "광고 같아", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatalf("provider calls = %d, want the revision's one", len(models.calls))
	}
	if jobs.enqueues != 0 {
		t.Fatalf("recording enqueued %d jobs", jobs.enqueues)
	}
	if len(candidates.recorded) != 1 {
		t.Fatalf("recorded %d candidates", len(candidates.recorded))
	}
}

// A4/[I4]: recording changes no prompt byte. The same revision is run twice over the same
// input — once with the recorder wired and once without — and the two provider requests must
// be identical. This is the test that keeps the feature on the right side of the boundary
// plan 16 drew: a candidate is recorded, never learned, and never injected.
func TestRecordingChangesNoPromptByte(t *testing.T) {
	const instruction = "여기 너무 광고 같아"
	run := func(recorder GuidelineCandidates) llm.Request {
		t.Helper()
		posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("before")}}
		models := revisingModels()
		svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
		if recorder != nil {
			svc.SetGuidelineCandidates(recorder)
		}
		if err := svc.Revise(context.Background(), RevisionJob{
			UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(),
			Payload: mustRevisionPayload(t, instruction, false),
		}, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		return models.calls[0].request
	}
	candidates := &fakeCandidates{}
	with, without := run(candidates), run(nil)
	if with.System != without.System {
		t.Fatal("recording changed the system prompt")
	}
	if with.Messages[0].Parts[0].Text != without.Messages[0].Parts[0].Text {
		t.Fatal("recording changed the per-post material")
	}
	if with.MaxTokens != without.MaxTokens || with.Reasoning != without.Reasoning {
		t.Fatal("recording changed the request envelope")
	}
	if len(candidates.recorded) != 1 {
		t.Fatalf("recorded %d candidates", len(candidates.recorded))
	}
	// And nothing a candidate would inhabit appears: no 지침 was frozen into this payload.
	if strings.Contains(with.System, "[작문 지침]") {
		t.Fatal("a 지침 section appeared with no guideline frozen into the payload")
	}
}
