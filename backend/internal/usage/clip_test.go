package usage

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"testing"
	"time"
)

func TestClipSettlementNeverDebitsAboveReservation(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.lots = []Lot{openMonthly("alice", 0), {ID: "bonus", UserID: "alice", Kind: LotBonus, Granted: 100, Remaining: 100}}
	start := holdStart("alice", plan.Free, "clip", PlannedCall{Ref: cheapRef, Count: 3, CompletionTokens: 8192})
	start.Kind = "generate_clip"
	start.Clip = approvedTestClip()
	if err := svc.Hold(context.Background(), start); err != nil {
		t.Fatal(err)
	}
	held := store.admissions[0].HoldCredits
	remaining := store.balance("alice", seoulNoon)
	store.events = append(store.events, Event{UserID: "alice", JobID: "clip", CostMicrousd: 5_000_000, CostSource: llm.CostReported})
	for range 2 {
		if err := svc.Settle(context.Background(), "clip", OutcomeSucceeded); err != nil {
			t.Fatal(err)
		}
	}
	if store.balance("alice", seoulNoon) != remaining || store.settled["clip"] != held {
		t.Fatalf("extra credit debit: held=%d settled=%v remaining=%d", held, store.settled, store.balance("alice", seoulNoon))
	}
	if store.events[0].CostMicrousd != 5_000_000 {
		t.Fatal("actual provider cost must not be hidden")
	}
}

type clipRecordStore struct {
	Store
	event Event
}

func (s *clipRecordStore) InsertEvent(ctx context.Context, e Event) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	deadline, ok := ctx.Deadline()
	if !ok || time.Until(deadline) > 5*time.Second {
		return errors.New("record context is not bounded")
	}
	s.event = e
	return nil
}

func TestClipCanceledCallStillRecordsReportedUsage(t *testing.T) {
	svc, _ := newTestService(t, seoulNoon)
	store := &clipRecordStore{Store: svc.store}
	svc.store = store
	ctx, cancel := context.WithCancel(WithWork(context.Background(), Work{UserID: "alice", Kind: "generate_clip", JobID: "clip"}))
	cancel()
	if err := svc.RecordCall(ctx, cheapRef, "observe", llm.Usage{PromptTokens: 10, CostMicrousd: 100, CostReported: true}, ctx.Err()); err != nil {
		t.Fatal(err)
	}
	if store.event.JobID != "clip" || store.event.CostMicrousd != 100 || store.event.PromptTokens != 10 {
		t.Fatal(store.event)
	}
}
