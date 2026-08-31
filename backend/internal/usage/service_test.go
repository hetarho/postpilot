package usage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
)

// fakeStore is an in-memory usage.Store. These tests are about the admission rules and the
// window arithmetic, not about SQL, so the windows are applied here exactly as the real
// store's half-open BETWEEN does.
type fakeStore struct {
	admissions []Admission
	events     []Event
	countErr   error
}

// InWriteTx is a pass-through here: these tests assert the rules, and the real store's
// transaction is what makes them hold under concurrency.
func (f *fakeStore) InWriteTx(_ context.Context, fn func(Store) error) error { return fn(f) }

func (f *fakeStore) CountAdmissions(_ context.Context, userID string, from, to time.Time) (int64, error) {
	if f.countErr != nil {
		return 0, f.countErr
	}
	var n int64
	for _, admission := range f.admissions {
		if admission.UserID == userID && !admission.CreatedAt.Before(from) && admission.CreatedAt.Before(to) {
			n++
		}
	}
	return n, nil
}

func (f *fakeStore) InsertAdmission(_ context.Context, admission Admission) error {
	f.admissions = append(f.admissions, admission)
	return nil
}

func (f *fakeStore) DeleteAdmissionForJob(_ context.Context, jobID string) error {
	kept := f.admissions[:0]
	for _, admission := range f.admissions {
		if admission.JobID != jobID {
			kept = append(kept, admission)
		}
	}
	f.admissions = kept
	return nil
}

func (f *fakeStore) SumCost(_ context.Context, userID string, from, to time.Time) (int64, error) {
	var total int64
	for _, event := range f.events {
		if event.UserID == userID && !event.CreatedAt.Before(from) && event.CreatedAt.Before(to) {
			total += event.CostMicrousd
		}
	}
	return total, nil
}

func (f *fakeStore) InsertEvent(_ context.Context, event Event) error {
	f.events = append(f.events, event)
	return nil
}

type fakeModels map[llm.ModelRef]llm.ModelInfo

func (m fakeModels) Lookup(ref llm.ModelRef) (llm.ModelInfo, bool) {
	info, ok := m[ref]
	return info, ok
}

// seoulNoon is a fixed instant inside one Asia/Seoul day, far from either boundary — and
// mid-month, so a test can place a row in the same month but on a different day.
var seoulNoon = time.Date(2026, 9, 15, 3, 0, 0, 0, time.UTC) // 15 Sep, 12:00 KST

// earlierSameMonth is 2 Sep 03:00 KST: the same Seoul month as seoulNoon, a different day.
var earlierSameMonth = time.Date(2026, 9, 1, 18, 0, 0, 0, time.UTC)

func newTestService(t *testing.T, now time.Time) (*Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{}
	svc := NewService(store, fakeModels{})
	svc.now = func() time.Time { return now }
	return svc, store
}

func start(userID string, tier plan.Plan, jobID string) Start {
	return Start{UserID: userID, Plan: tier, Kind: "generate", JobID: jobID}
}

func TestAdmitRefusesAtTheDailyStartCount(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	for i := range 10 {
		if err := svc.Admit(context.Background(), start("alice", plan.Free, string(rune('a'+i)))); err != nil {
			t.Fatalf("admission %d: %v", i, err)
		}
	}

	err := svc.Admit(context.Background(), start("alice", plan.Free, "eleventh"))
	var quota *plan.QuotaError
	if !errors.As(err, &quota) {
		t.Fatalf("error = %v, want a QuotaError", err)
	}
	if quota.Axis != plan.AxisDailyCount || quota.Limit != 10 || quota.Used != 10 {
		t.Errorf("quota = %+v", quota)
	}
	// The refusal must leave no trace: the count axis counts admissions, so a refused start
	// that recorded one would consume the allowance it was just denied.
	if got, want := len(store.admissions), 10; got != want {
		t.Errorf("admissions = %d, want %d", got, want)
	}
	if _, resets := plan.DayWindow(seoulNoon); !quota.ResetsAt.Equal(resets) {
		t.Errorf("resets_at = %v, want the next Seoul midnight %v", quota.ResetsAt, resets)
	}
}

func TestAdmitCountsOnlyTheCurrentSeoulDay(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	// 14 Sep 23:30 KST — the previous Seoul day, but the same UTC day as `seoulNoon` by only
	// nine hours' difference in the other direction: counting in UTC would slide these rows
	// into a different window than the one the user experiences.
	yesterday := time.Date(2026, 9, 14, 14, 30, 0, 0, time.UTC)
	for i := range 10 {
		store.admissions = append(store.admissions, Admission{
			UserID: "alice", Kind: "generate", JobID: string(rune('a' + i)), CreatedAt: yesterday,
		})
	}

	if err := svc.Admit(context.Background(), start("alice", plan.Free, "today")); err != nil {
		t.Fatalf("yesterday's starts must not count against today: %v", err)
	}
}

func TestAdmitRefusesOnBothBudgetAxes(t *testing.T) {
	for name, tc := range map[string]struct {
		spentAt time.Time
		cost    int64
		axis    plan.Axis
	}{
		"daily":   {seoulNoon, 100_000, plan.AxisDailyCost},
		"monthly": {earlierSameMonth, 2_000_000, plan.AxisMonthlyCost},
	} {
		t.Run(name, func(t *testing.T) {
			svc, store := newTestService(t, seoulNoon)
			store.events = append(store.events, Event{
				UserID: "alice", CostMicrousd: tc.cost, CreatedAt: tc.spentAt,
			})

			err := svc.Admit(context.Background(), start("alice", plan.Free, "next"))
			var quota *plan.QuotaError
			if !errors.As(err, &quota) || quota.Axis != tc.axis {
				t.Fatalf("error = %v, want a %s refusal", err, tc.axis)
			}
			if quota.Used != tc.cost {
				t.Errorf("used = %d, want %d", quota.Used, tc.cost)
			}
		})
	}
}

func TestMasterIsNeverRefusedButIsStillRecorded(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.events = append(store.events, Event{UserID: "root", CostMicrousd: 900_000_000, CreatedAt: seoulNoon})
	for i := range 200 {
		store.admissions = append(store.admissions, Admission{
			UserID: "root", JobID: string(rune(i)), CreatedAt: seoulNoon,
		})
	}

	if err := svc.Admit(context.Background(), start("root", plan.Master, "next")); err != nil {
		t.Fatalf("master admission = %v, want nil", err)
	}
	if got, want := len(store.admissions), 201; got != want {
		t.Fatalf("admissions = %d, want %d — master skips the checks, not the ledger", got, want)
	}
}

func TestAdmitRefusesAModelAboveTheTier(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	request := start("alice", plan.Free, "job")
	request.Models = []plan.ModelFloor{
		{Ref: "openrouter/free", MinPlan: plan.Free},
		{Ref: "openrouter/anthropic/claude-opus-5", MinPlan: plan.Max},
	}

	err := svc.Admit(context.Background(), request)
	var locked *plan.ModelLockedError
	if !errors.As(err, &locked) {
		t.Fatalf("error = %v, want a ModelLockedError", err)
	}
	if len(locked.Models) != 1 || locked.Models[0] != "openrouter/anthropic/claude-opus-5" {
		t.Errorf("locked models = %v", locked.Models)
	}
	if locked.Required != plan.Max {
		t.Errorf("required = %q, want max", locked.Required)
	}
	if len(store.admissions) != 0 {
		t.Error("a model refusal must not consume a start")
	}
}

func TestReleaseUndoesAnAdmission(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	if err := svc.Admit(context.Background(), start("alice", plan.Free, "job")); err != nil {
		t.Fatal(err)
	}
	if err := svc.Release(context.Background(), "job"); err != nil {
		t.Fatal(err)
	}
	if len(store.admissions) != 0 {
		t.Fatalf("admissions = %+v, want the released row gone", store.admissions)
	}
}

func TestRecordCallPricesAndAttributesFromContext(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "z-ai/glm-5.3-flash"}
	store := &fakeStore{}
	svc := NewService(store, fakeModels{ref: {
		Ref: ref, InputUSDPerMillion: "0.075", OutputUSDPerMillion: "0.25",
	}})
	svc.now = func() time.Time { return seoulNoon }
	ctx := WithWork(context.Background(), Work{
		UserID: "alice", Kind: "generate", JobID: "job", ObserveModel: "openrouter/observer",
	})

	if err := svc.RecordCall(ctx, ref, llm.Usage{PromptTokens: 1_000_000, CompletionTokens: 0}, false); err != nil {
		t.Fatal(err)
	}
	if got, want := len(store.events), 1; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	event := store.events[0]
	if event.CostMicrousd != 75_000 || event.CostSource != llm.CostEstimated {
		t.Errorf("cost = %d (%s), want 75000 estimated", event.CostMicrousd, event.CostSource)
	}
	if event.UserID != "alice" || event.JobID != "job" || event.Stage != "generate" {
		t.Errorf("attribution = %+v", event)
	}
}

func TestRecordCallKeepsFailedCallsWithReportedUsage(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	ctx := WithWork(context.Background(), Work{UserID: "alice", Kind: "revise", JobID: "job"})
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "gone"}

	// A failure that never reached a model has nothing to account for.
	if err := svc.RecordCall(ctx, ref, llm.Usage{}, true); err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %+v, want none for a failure with no usage", store.events)
	}

	// A failure the provider still billed is the case job 23 preserves usage for.
	if err := svc.RecordCall(ctx, ref, llm.Usage{PromptTokens: 12, CostMicrousd: 4, CostReported: true}, true); err != nil {
		t.Fatal(err)
	}
	if got, want := len(store.events), 1; got != want {
		t.Fatalf("events = %d, want %d", got, want)
	}
	if store.events[0].CostSource != llm.CostReported || store.events[0].CostMicrousd != 4 {
		t.Errorf("event = %+v", store.events[0])
	}
}

func TestRecordCallDropsAnUnattributableCall(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	err := svc.RecordCall(context.Background(), llm.ModelRef{ProviderID: "p", ModelID: "m"}, llm.Usage{PromptTokens: 5}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.events) != 0 {
		t.Fatalf("events = %+v, want none without a job in context", store.events)
	}
}

func TestStageMarksTheObserveCallOnly(t *testing.T) {
	work := Work{Kind: "generate", ObserveModel: "openrouter/vision"}
	if got := work.StageFor(llm.ModelRef{ProviderID: "openrouter", ModelID: "vision"}); got != "observe" {
		t.Errorf("observe stage = %q", got)
	}
	if got := work.StageFor(llm.ModelRef{ProviderID: "openrouter", ModelID: "writer"}); got != "generate" {
		t.Errorf("write stage = %q, want the job kind", got)
	}
}

func TestSummaryReportsBothWindowsAndTheirResets(t *testing.T) {
	svc, store := newTestService(t, seoulNoon)
	store.admissions = append(store.admissions, Admission{UserID: "alice", CreatedAt: seoulNoon})
	store.events = append(store.events,
		Event{UserID: "alice", CostMicrousd: 30, CreatedAt: seoulNoon},
		// Earlier in the same Seoul month but a different day.
		Event{UserID: "alice", CostMicrousd: 70, CreatedAt: earlierSameMonth},
	)

	summary, err := svc.Summary(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if summary.JobsStartedToday != 1 || summary.CostTodayMicrousd != 30 || summary.CostMonthMicrousd != 100 {
		t.Fatalf("summary = %+v", summary)
	}
	_, dayEnd := plan.DayWindow(seoulNoon)
	_, monthEnd := plan.MonthWindow(seoulNoon)
	if !summary.DayResetsAt.Equal(dayEnd) || !summary.MonthResetsAt.Equal(monthEnd) {
		t.Errorf("resets = %v / %v, want %v / %v",
			summary.DayResetsAt, summary.MonthResetsAt, dayEnd, monthEnd)
	}
}
