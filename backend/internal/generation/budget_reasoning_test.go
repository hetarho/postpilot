package generation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// A1 · A5 — one run, two purposes, two efforts. The generation context sets the STAGE on
// every request, which is what lets the registry resolve the operator's override for the
// task being performed rather than for the model as a whole (the 2026-09-03 defect).
func TestEveryStageNamesItselfOnItsRequest(t *testing.T) {
	images, _ := storedSnapshot(2, "")
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Images: images}}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)

	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	var stages []string
	for _, call := range models.calls {
		stages = append(stages, call.request.Stage)
	}
	// One observation batch for two photos, then the write.
	if len(stages) != 2 || stages[0] != llm.StageNameObserve || stages[1] != llm.StageNameWrite {
		t.Fatalf("stages = %v, want [observe write]", stages)
	}
}

// A6 · A7 — the stages stop sharing one constant: the writer's budget follows the post's
// requested length and the observation stage's does not.
func TestEachStageSendsItsOwnCompletionBudget(t *testing.T) {
	images, _ := storedSnapshot(1, "")
	target := 6000
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Images: images, TargetLength: &target,
	}}
	models := observingModels(t)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)

	if err := svc.Generate(context.Background(), GenerateJob{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
		TargetLength: &target,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	budgets := map[string]int{}
	for _, call := range models.calls {
		budgets[call.request.Stage] = call.request.MaxTokens
	}
	if got, want := budgets[llm.StageNameObserve], testBudget.Observation(); got != want {
		t.Errorf("observe budget = %d, want %d", got, want)
	}
	if got, want := budgets[llm.StageNameWrite], testBudget.Write(&target); got != want {
		t.Errorf("write budget = %d, want %d", got, want)
	}
	// The point of the split: the writer's headroom is not handed to observation, whose
	// per-photo output is small and bounded.
	if budgets[llm.StageNameObserve] >= budgets[llm.StageNameWrite] {
		t.Errorf("observation was given the writer's headroom: %v", budgets)
	}
	// And the writer's own budget moves with what was asked for.
	if testBudget.Write(&target) <= testBudget.Write(nil) {
		t.Errorf("a longer requested draft was not given more room")
	}
}

// A12 — the truncation failure says which half consumed the budget when the provider
// reported it, so the remedy follows from the message rather than from a database query.
// A14 — and it is still ErrOutputTruncated either way, on both paths.
func TestTruncationNamesTheReasoningSplitWhenReported(t *testing.T) {
	for _, path := range []struct {
		name string
		run  func(*Service) error
	}{
		{name: "write", run: func(svc *Service) error {
			_, _, err := svc.writeCandidate(context.Background(), PostInput{}, Profile{}, nil, writeRef)
			return err
		}},
		{name: "observe", run: func(svc *Service) error {
			post := PostInput{Images: []Image{{Filename: "IMG.jpg", Key: "key"}}}
			_, _, err := svc.observeCandidate(context.Background(), post, post.Images, nil, observeRef, func(string, int, int) {}, false)
			return err
		}},
	} {
		for _, test := range []struct {
			name       string
			usage      llm.Usage
			wantDetail string
		}{
			{
				name:       "reported",
				usage:      llm.Usage{CompletionTokens: 8192, ReasoningTokens: 8100},
				wantDetail: "completion budget exhausted: 8100 of 8192 completion tokens went to reasoning, 92 to visible output",
			},
			// A provider that reports no split keeps today's message: nothing is invented.
			{name: "not reported", usage: llm.Usage{CompletionTokens: 8192}},
		} {
			t.Run(fmt.Sprintf("%s/%s", path.name, test.name), func(t *testing.T) {
				models := newFakeModels()
				models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
					return llm.Response{Text: `{"partial":`, FinishReason: "length", Usage: test.usage}, nil
				}
				svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget)
				err := path.run(svc)
				// A14: the sentinel and the reason are unchanged whichever half is at fault.
				if !errors.Is(err, llm.ErrOutputTruncated) || errors.Is(err, llm.ErrBadOutput) {
					t.Fatalf("err = %v, want only ErrOutputTruncated", err)
				}
				if got := llm.NormalizeFailure(err); got.Reason != llm.FailureReasonOutputTruncated {
					t.Fatalf("reason = %q", got.Reason)
				}
				if got := llm.NormalizeFailure(err).TechnicalDetail; got != test.wantDetail {
					t.Fatalf("technical detail = %q, want %q", got, test.wantDetail)
				}
			})
		}
	}
}
