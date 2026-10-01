package experiment

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestComparisonCostsReuseCountedAllScopeAndWindow(t *testing.T) {
	svc, store, catalog, _, _ := newTestService()
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a, b, c := ModelRef{"p", "a"}, ModelRef{"p", "b"}, ModelRef{"p", "c"}
	catalog.models[c] = Model{Ref: c, Label: "C", Enabled: true, Stages: allStages}
	catalog.activeErr = errors.New("admin cost read must not inspect a synthetic user's selection")
	catalog.recommendedErr = errors.New("admin cost read must not inspect recommendation display state")
	seedRankedBoard(store, "alice-day", "alice", now.Add(-time.Hour), []ModelRef{a, b, c}, []int{1, 2, 3})
	seedRankedBoard(store, "bob-day", "bob", now.Add(-2*time.Hour), []ModelRef{a, b, c}, []int{2, 1, 3})
	seedRankedBoard(store, "bob-week", "bob", now.Add(-3*24*time.Hour), []ModelRef{a, b, c}, []int{2, 3, 1})
	store.mu.Lock()
	for _, input := range []struct {
		id      string
		model   ModelRef
		cost    int64
		quality CostSource
	}{
		{"alice-day", a, 100, CostReported}, {"bob-day", a, 50, CostEstimated},
		{"alice-day", b, 250, CostReported}, {"bob-day", b, 0, CostUnavailable},
		{"alice-day", c, 0, CostUnavailable}, {"bob-day", c, 0, CostUnavailable},
		{"bob-week", a, 999, CostReported},
	} {
		row := store.rows[input.id]
		for i := range row.Candidates {
			if row.Candidates[i].Model == input.model {
				row.Candidates[i].Usage.CostMicrousd = input.cost
				row.Candidates[i].Usage.CostSource = input.quality
			}
		}
		store.rows[input.id] = row
	}
	store.mu.Unlock()

	day, err := svc.ComparisonCosts(context.Background(), StageWrite, WindowDay)
	if err != nil {
		t.Fatal(err)
	}
	if len(day) != 3 || day[0].Model != b || day[0].TotalCostMicrousd != 250 || day[0].CostQuality != CostMixed ||
		day[1].Model != a || day[1].TotalCostMicrousd != 150 || day[1].CostQuality != CostMixed ||
		day[2].Model != c || day[2].CostQuality != CostUnavailable {
		t.Fatalf("day cost rows = %+v", day)
	}
	for _, row := range day {
		if row.EvaluatedComparisons != 2 {
			t.Fatalf("one three-way ranking should count once per model: %+v", row)
		}
	}
	week, err := svc.ComparisonCosts(context.Background(), StageWrite, WindowWeek)
	if err != nil || len(week) != 3 || week[0].Model != a || week[0].TotalCostMicrousd != 1149 || week[0].EvaluatedComparisons != 3 {
		t.Fatalf("week cost rows = %+v, %v", week, err)
	}
	if _, err := svc.ComparisonCosts(context.Background(), Stage("analyze"), WindowDay); err == nil {
		t.Fatal("unsupported comparison stage accepted")
	}
}
