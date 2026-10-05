package main

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/platform/config"
)

func testCompletionBudget() config.LLMCompletionBudget {
	return config.LLMCompletionBudget{Observe: 1024, WriteFloor: 8192, WritePerChar: 4, Ceiling: 32768}
}

func TestGenerationPricingCallsUseTheBudgetsTheStagesWillSend(t *testing.T) {
	target := 6000
	_, calls := generationCalls(generation.StartRequest{
		ObserveModel: "openrouter/shared", WriteModel: "openrouter/shared",
		ObserveCalls: 2, TargetLength: &target,
	}, testCompletionBudget())
	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want separate observe and write entries", calls)
	}
	if calls[0].Count != 2 || calls[0].CompletionTokens != 1024 {
		t.Errorf("observe call = %+v", calls[0])
	}
	// The write is priced with the storyline's allowance, the budget it will send (GEN-67).
	if calls[1].Count != 1 || calls[1].CompletionTokens != 24000+config.StorylineCompletionAllowance {
		t.Errorf("write call = %+v", calls[1])
	}
	_, native := generationCalls(generation.StartRequest{
		WriteModel: "openrouter/reasoner", WriteNativeEffort: true,
	}, testCompletionBudget())
	if len(native) != 1 || native[0].CompletionTokens != 2*(8192+config.StorylineCompletionAllowance) {
		t.Fatalf("native-effort pricing calls = %+v, want frozen headroom", native)
	}

	_, reused := generationCalls(generation.StartRequest{
		ObserveModel: "openrouter/shared", WriteModel: "openrouter/shared", ObserveCalls: 0,
	}, testCompletionBudget())
	if len(reused) != 1 || reused[0].CompletionTokens != 8192+config.StorylineCompletionAllowance {
		t.Fatalf("reuse-everything calls = %+v, want only the write floor", reused)
	}
}

func TestRevisionPricingUsesTheLargerFrozenLength(t *testing.T) {
	target := 1200
	calls := revisionPricingCalls(generation.StartRevisionRequest{
		WriteModel: "openrouter/writer", TargetLength: &target, ContentChars: 6000,
	}, testCompletionBudget())
	if len(calls) != 1 || calls[0].CompletionTokens != 24000 {
		t.Fatalf("revision calls = %+v, want the 6000-character budget", calls)
	}
}

// QUOTA-13: a storyline job is priced over its frozen set — the observe calls it will make and
// one storyline call at the short budget — and the storyline request as that one call. A frozen
// native-effort flag doubles the storyline call's budget, as the call doubles it (GEN-22).
func TestStorylinePricingCallsUseTheBudgetsTheCallsWillSend(t *testing.T) {
	_, calls := storylineCalls(generation.StartStorylineRequest{
		ObserveModel: "openrouter/shared", WriteModel: "openrouter/writer", ObserveCalls: 2,
	}, testCompletionBudget())
	if len(calls) != 2 || calls[0].Count != 2 || calls[0].CompletionTokens != 1024 ||
		calls[1].Ref != "openrouter/writer" || calls[1].Count != 1 || calls[1].CompletionTokens != 8192 {
		t.Fatalf("storyline calls = %+v", calls)
	}
	_, reused := storylineCalls(generation.StartStorylineRequest{ObserveModel: "openrouter/shared", WriteModel: "openrouter/writer"}, testCompletionBudget())
	if len(reused) != 1 || reused[0].Ref != "openrouter/writer" {
		t.Fatalf("a storyline reusing every observation = %+v, want the one call", reused)
	}
	_, native := storylineCalls(generation.StartStorylineRequest{WriteModel: "openrouter/reasoner", WriteNativeEffort: true}, testCompletionBudget())
	if len(native) != 1 || native[0].CompletionTokens != 16384 {
		t.Fatalf("a native-effort storyline = %+v, want the doubled floor", native)
	}
	request := storylineRevisionPricingCalls(generation.StartStorylineRevisionRequest{WriteModel: "openrouter/writer"}, testCompletionBudget())
	if len(request) != 1 || request[0].Count != 1 || request[0].CompletionTokens != 8192 {
		t.Fatalf("storyline request calls = %+v", request)
	}
	nativeRequest := storylineRevisionPricingCalls(generation.StartStorylineRevisionRequest{WriteModel: "openrouter/reasoner", WriteNativeEffort: true}, testCompletionBudget())
	if len(nativeRequest) != 1 || nativeRequest[0].CompletionTokens != 16384 {
		t.Fatalf("a native-effort storyline request = %+v, want the doubled floor", nativeRequest)
	}
}

// One block states every observe-and-write job's call counts (QUOTA-13): per model across the
// whole job, the observe count stated even at zero, and the write call counted on a model that
// serves both stages.
func TestObserveWriteCallCountsArePerModelAcrossTheJob(t *testing.T) {
	shared, _ := generationCalls(generation.StartRequest{
		ObserveModel: "openrouter/shared", WriteModel: "openrouter/shared", ObserveCalls: 2,
	}, testCompletionBudget())
	if !reflect.DeepEqual(shared, map[string]int{"openrouter/shared": 3}) {
		t.Fatalf("a model serving both stages counts = %v", shared)
	}
	reused, _ := storylineCalls(generation.StartStorylineRequest{
		ObserveModel: "openrouter/observer", WriteModel: "openrouter/writer",
	}, testCompletionBudget())
	if !reflect.DeepEqual(reused, map[string]int{"openrouter/observer": 0}) {
		t.Fatalf("a storyline reusing every observation counts = %v, want the zero stated", reused)
	}
	none, _ := generationCalls(generation.StartRequest{WriteModel: "openrouter/writer"}, testCompletionBudget())
	if len(none) != 0 {
		t.Fatalf("a zero-photo generation counts = %v, want none", none)
	}
}

// Every generation enqueue answers the queue's refusals in generation's words.
func TestGenerationEnqueueErrorMapsTheQueueRefusals(t *testing.T) {
	var active *generation.JobAlreadyInProgressError
	if err := generationEnqueueError(fmt.Errorf("enqueue: %w", &job.ErrAlreadyInProgress{ActiveID: "active"})); !errors.As(err, &active) || active.ActiveID != "active" {
		t.Fatalf("an active job = %v", err)
	}
	if err := generationEnqueueError(job.ErrVoiceUnavailable); !errors.Is(err, generation.ErrVoiceDeleted) {
		t.Fatalf("an unavailable voice = %v", err)
	}
	other := errors.New("disk full")
	if err := generationEnqueueError(other); err != other {
		t.Fatalf("another error = %v", err)
	}
	if err := generationEnqueueError(nil); err != nil {
		t.Fatalf("no error = %v", err)
	}
}
