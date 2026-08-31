package plan

import (
	"errors"
	"testing"
	"time"
)

func TestParseRefusesAnythingOffTheLadder(t *testing.T) {
	for _, value := range []string{"free", " basic ", "max", "master"} {
		if _, err := Parse(value); err != nil {
			t.Errorf("Parse(%q) = %v, want a known plan", value, err)
		}
	}
	for _, value := range []string{"", "FREE", "pro", "free-tier"} {
		if got, err := Parse(value); err == nil {
			t.Errorf("Parse(%q) = %q, want an error", value, got)
		}
	}
}

func TestAllowsFailsClosedAtBothEnds(t *testing.T) {
	cases := []struct {
		acting, floor Plan
		want          bool
	}{
		{Free, Free, true},
		{Free, Basic, false},
		{Basic, Free, true},
		{Max, Basic, true},
		{Master, Max, true},
		// An undeclared floor is satisfied by nothing, and an unknown tier satisfies nothing:
		// the registry refuses to boot without min_plan, so either case is corrupted state.
		{Master, "", false},
		{"", Free, false},
	}
	for _, tc := range cases {
		if got := tc.acting.Allows(tc.floor); got != tc.want {
			t.Errorf("%q.Allows(%q) = %v, want %v", tc.acting, tc.floor, got, tc.want)
		}
	}
}

func TestLimitsAreTheShippedLadder(t *testing.T) {
	cases := map[Plan]Limits{
		Free:   {DailyJobStarts: 10, DailyBudgetMicrousd: 100_000, MonthlyBudgetMicrousd: 2_000_000},
		Basic:  {DailyJobStarts: 30, DailyBudgetMicrousd: 500_000, MonthlyBudgetMicrousd: 12_000_000},
		Max:    {DailyJobStarts: 100, DailyBudgetMicrousd: 1_000_000, MonthlyBudgetMicrousd: 25_000_000},
		Master: {},
	}
	for tier, want := range cases {
		if got := LimitsFor(tier); got != want {
			t.Errorf("LimitsFor(%q) = %+v, want %+v", tier, got, want)
		}
	}
	if !LimitsFor(Master).Unlimited() {
		t.Error("master must be unlimited on every axis")
	}
	// A corrupted row must not buy free spend: an unknown tier gets the strictest known one.
	if got, want := LimitsFor("pro"), LimitsFor(Free); got != want {
		t.Errorf("LimitsFor(unknown) = %+v, want the free limits %+v", got, want)
	}
}

func TestWindowsAreSeoulCalendarWindows(t *testing.T) {
	// 15:30 UTC on 31 August is already 1 September in Seoul — the case a UTC window gets
	// wrong for nine hours every day.
	at := time.Date(2026, 8, 31, 15, 30, 0, 0, time.UTC)

	dayStart, dayEnd := DayWindow(at)
	if got, want := dayStart.UTC(), time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("day start = %v, want %v", got, want)
	}
	if got, want := dayEnd.UTC(), time.Date(2026, 9, 1, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("day end = %v, want %v", got, want)
	}

	monthStart, monthEnd := MonthWindow(at)
	if got, want := monthStart.UTC(), time.Date(2026, 8, 31, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("month start = %v, want September's first Seoul midnight %v", got, want)
	}
	if got, want := monthEnd.UTC(), time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("month end = %v, want %v", got, want)
	}
}

func TestEnsureAllowedNamesEveryOffenderAndTheOneUpgrade(t *testing.T) {
	err := EnsureAllowed(Free, []ModelFloor{
		{Ref: "p/cheap", MinPlan: Free},
		{Ref: "p/mid", MinPlan: Basic},
		{Ref: "p/top", MinPlan: Max},
	})
	var locked *ModelLockedError
	if !errors.As(err, &locked) {
		t.Fatalf("error = %v, want a ModelLockedError", err)
	}
	if got, want := len(locked.Models), 2; got != want {
		t.Fatalf("locked = %v, want %d refs", locked.Models, want)
	}
	if locked.Required != Max {
		t.Errorf("required = %q, want the highest floor among the offenders", locked.Required)
	}
	params := locked.Params()
	if params["model"] != "p/mid" || params["models"] != "p/mid, p/top" || params["required_plan"] != "max" {
		t.Errorf("params = %#v", params)
	}

	if err := EnsureAllowed(Max, []ModelFloor{{Ref: "p/top", MinPlan: Max}}); err != nil {
		t.Errorf("a satisfied floor = %v, want nil", err)
	}
}

func TestQuotaErrorCarriesItsWholeExplanation(t *testing.T) {
	resets := time.Date(2026, 9, 2, 0, 0, 0, 0, seoul)
	err := &QuotaError{Axis: AxisDailyCost, Limit: 100_000, Used: 100_001, ResetsAt: resets}
	if err.Reason() != "DAILY_COST" {
		t.Errorf("reason = %q", err.Reason())
	}
	params := err.Params()
	if params["limit"] != "100000" || params["used"] != "100001" {
		t.Errorf("params = %#v", params)
	}
	// The instant is normalized to UTC so a client parses one shape, whatever the server's
	// own clock offset happens to be.
	if got, want := params["resets_at"], "2026-09-01T15:00:00Z"; got != want {
		t.Errorf("resets_at = %q, want %q", got, want)
	}
}
