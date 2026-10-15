package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/template"
	"slices"
	"testing"
)

type plainHeadingVoice struct{}

func (plainHeadingVoice) DefaultVoiceStyle(context.Context, string) (string, bool, error) {
	return "", false, nil
}
func realHeadingService(h *requestHarness) *template.HeadingSuggestions {
	return template.NewHeadingSuggestions(h.models, plainHeadingVoice{}, templateHeadingJobs{queue: h.queue}, testCompletionBudget(), templateEstimates{rates: stubRates{}}, h.service.Limits())
}
func realHeadingStart() template.HeadingSuggestionStart {
	return template.HeadingSuggestionStart{WriteModel: "p/m", TargetKind: "clip", TargetPath: []string{"맛집", "식사"}, Sections: []template.HeadingSection{{Role: "후킹", Description: "상호와 이유"}}}
}

func TestHeadingSuggestionAdmissionPricesOneFrozenWriteCallPerAccount(t *testing.T) {
	for _, native := range []bool{false, true} {
		h := newRequestHarness(t)
		h.models.nativeEffort = native
		s := realHeadingService(h)
		if _, err := s.Start(t.Context(), "alice", realHeadingStart()); err != nil {
			t.Fatal(err)
		}
		want := testCompletionBudget().Short(native)
		hold := h.admit.holds[0]
		if hold.Kind != template.HeadingSuggestionJobKind || len(hold.Calls) != 1 || hold.Calls[0] != (job.PlannedCall{Ref: "p/m", Stage: llm.StageNameWrite, Count: 1, CompletionTokens: want}) {
			t.Fatalf("wrong hold: %+v", hold)
		}
		if _, err := s.Start(t.Context(), "alice", realHeadingStart()); !errors.Is(err, template.ErrRequestRunning) {
			t.Fatalf("duplicate account admitted: %v", err)
		}
		if _, err := s.Start(t.Context(), "bob", realHeadingStart()); err != nil {
			t.Fatal("another owner blocked", err)
		}
		if _, err := h.start(t, "alice"); err != nil {
			t.Fatal("legacy kind collided", err)
		}
		kinds := jobKinds()
		if !slices.Contains(kinds.Authorized, template.HeadingSuggestionJobKind) || !slices.Contains(kinds.Cancellable, template.HeadingSuggestionJobKind) || kinds.FirstStages[template.HeadingSuggestionJobKind] != "write" || !(jobReporting{}).Redacted(template.HeadingSuggestionJobKind) {
			t.Fatal("heading dispatch/reporting policy is missing")
		}
	}
}

func TestHeadingSuggestionsUseDurableOwnerKindAndTerminalFences(t *testing.T) {
	h := newRequestHarness(t)
	s := realHeadingService(h)
	h.models.answers = []string{`{"sections":[{"headings":["여기를 고른 이유","오늘의 한 장면","처음 마주한 순간"]}]}`}
	id, err := s.Start(t.Context(), "alice", realHeadingStart())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Result(t.Context(), "alice", id); !errors.Is(err, template.ErrRequestNotReady) {
		t.Fatalf("read nonterminal input as output: %v", err)
	}
	h.queue.Register(template.HeadingSuggestionJobKind, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return s.Run(ctx, template.RequestRun{ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload}, progress)
	})
	ctx, stop := context.WithCancel(t.Context())
	defer stop()
	go h.queue.Run(ctx)
	if status := h.waitTerminal(t, id); status != job.StatusDone {
		t.Fatal(status)
	}
	groups, err := s.Result(t.Context(), "alice", id)
	if err != nil || len(groups) != 1 || len(groups[0]) != 3 || h.models.calls != 1 {
		t.Fatalf("result=%v calls=%d err=%v", groups, h.models.calls, err)
	}
	if _, err := s.Result(t.Context(), "bob", id); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("foreign result disclosed: %v", err)
	}
	if _, err := h.service.RequestResult(t.Context(), "alice", id); !errors.Is(err, template.ErrNotFound) {
		t.Fatalf("kind fence absent: %v", err)
	}
}

func TestHeadingSuggestionQueuedCancellationMakesNoCall(t *testing.T) {
	h := newRequestHarness(t)
	s := realHeadingService(h)
	id, err := s.Start(t.Context(), "alice", realHeadingStart())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.CancelOwned(t.Context(), "alice", id); err != nil {
		t.Fatal(err)
	}
	if found, err := h.queue.Get(t.Context(), id, "alice"); err != nil || found.Status != job.StatusCancelled || h.models.calls != 0 {
		t.Fatalf("cancellation=%+v %v", found, err)
	}
	if _, err := s.Result(t.Context(), "alice", id); !errors.Is(err, template.ErrRequestNotReady) {
		t.Fatalf("cancelled result read: %v", err)
	}
}
