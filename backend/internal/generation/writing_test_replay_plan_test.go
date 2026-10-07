package generation

import (
	"context"
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func TestWritingTestDurableAnswerRecoveryIsFreeAndFencedAfterModelWithdrawal(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 4, nil)
	stored, save := checkpointStore()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
	result, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil || stored[0].Answer == nil {
		t.Fatalf("paid answer not durable: %v", err)
	}
	// Its consumer crashed after saving the answer and before marking the
	// candidate successful. Only that owner-confirmed failed row may select it.
	calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0})
	if err != nil || len(calls) != 0 {
		t.Fatalf("recovery would buy another answer: %v, %+v", err, calls)
	}
	models.infos[result.Checkpoint.WriteModel] = llm.ModelInfo{Ref: result.Checkpoint.WriteModel, Disabled: true}
	recovered, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &result.Checkpoint, RetryFailed: true, SaveCheckpoint: save})
	if err != nil || !recovered.Replayed || recovered.Usage != (CandidateUsage{}) || len(models.calls) != 1 {
		t.Fatalf("paid result repeated: %v, %#v, calls=%d", err, recovered, len(models.calls))
	}
	refusal := errors.New("owner purged the test")
	_, err = factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{Checkpoint: &result.Checkpoint, RetryFailed: true, SaveCheckpoint: func(context.Context, WritingTestCheckpoint) error { return refusal }})
	if !errors.Is(err, refusal) || len(models.calls) != 1 {
		t.Fatalf("replay bypassed purge fence: %v", err)
	}
}

func TestWritingTestMixedAnswerRecoveryPlansOnlyUnfinishedCandidates(t *testing.T) {
	models := newFakeModels()
	factory := runTestFactory(models, &fakePosts{})
	snapshot := runTestSnapshot(t, "model", "write", 4, nil)
	stored, save := checkpointStore()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return runTestAnswer(), nil }
	complete, err := factory.RunWritingTestCandidate(context.Background(), snapshot, 0, nil, WritingTestRunOptions{SaveCheckpoint: save})
	if err != nil {
		t.Fatal(err)
	}
	calls, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0, 1})
	if err != nil || len(calls) != 1 || calls[0].Count != 1 || calls[0].Stage != "write" || calls[0].Ref.ModelID != "writer-1" {
		t.Fatalf("finished answer entered the paid plan: %v, %+v", err, calls)
	}
	uncertain := cloneWritingTestCheckpoint(complete.Checkpoint)
	uncertain.InFlightStage = "write"
	if _, err := factory.PlanWritingTestRetry(snapshot, nil, map[int]WritingTestCheckpoint{0: uncertain}, []int{0}); !errors.Is(err, ErrWritingTestCheckpointInvalid) {
		t.Fatalf("inconsistent answered checkpoint accepted: %v", err)
	}
	if _, err := factory.PlanWritingTestRetry(snapshot, nil, stored, []int{0, 0}); !errors.Is(err, ErrWritingTestReference) {
		t.Fatalf("repeated recovery accepted: %v", err)
	}
}
