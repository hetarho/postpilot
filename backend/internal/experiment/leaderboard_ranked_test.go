package experiment

import (
	"context"
	"testing"
	"time"
)

func TestRankedLeaderboardReplaysWindowAndScopeBesideLegacy(t *testing.T) {
	svc, store, catalog, _, _ := newTestService()
	ctx := context.Background()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a, b, c := ModelRef{"p", "a"}, ModelRef{"p", "b"}, ModelRef{"p", "c"}
	catalog.models[c] = Model{Ref: c, Label: "C", Enabled: true, Stages: allStages}
	seedVerdict(store, "legacy", "alice", StageWrite, a, b, now.Add(-2*time.Hour))
	seedRankedBoard(store, "ranked", "alice", now.Add(-time.Hour), []ModelRef{a, b, c}, []int{2, 1, 1})
	seedRankedBoard(store, "other-owner", "bob", now.Add(-30*time.Minute), []ModelRef{a, b, c}, []int{3, 2, 1})
	mine, err := svc.Leaderboard(ctx, "alice", StageWrite, WindowDay, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	all, err := svc.Leaderboard(ctx, "alice", StageWrite, WindowDay, ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	byRef := func(rows []LeaderboardEntry, ref ModelRef) LeaderboardEntry {
		for _, row := range rows {
			if row.Model == ref {
				return row
			}
		}
		return LeaderboardEntry{}
	}
	if row := byRef(mine, c); row.EvaluatedComparisons != 1 || row.Wins != 1 || row.Draws != 1 || row.Matches != 2 ||
		row.SuccessfulCalls != 1 || row.PromptTokens != 11 {
		t.Fatalf("mine C=%+v", row)
	}
	if row := byRef(all, c); row.EvaluatedComparisons != 2 || row.SuccessfulCalls != 2 || row.Wins != 3 ||
		row.Draws != 1 {
		t.Fatalf("all C=%+v", row)
	}
	if row := byRef(mine, a); row.EvaluatedComparisons != 2 || row.Wins != 1 {
		t.Fatalf("legacy plus ranked A=%+v", row)
	}
	// Every window starts from 1500; a month-old event is not silently carried in.
	day := byRef(mine, b).Rating
	seedRankedBoard(store, "older", "alice", now.Add(-3*24*time.Hour), []ModelRef{a, b, c}, []int{3, 2, 1})
	repeated, err := svc.Leaderboard(ctx, "alice", StageWrite, WindowDay, ScopeMe)
	if err != nil || byRef(repeated, b).Rating != day {
		t.Fatalf("day changed by old event: %+v %v", repeated, err)
	}
	week, err := svc.Leaderboard(ctx, "alice", StageWrite, WindowWeek, ScopeMe)
	if err != nil || byRef(week, c).EvaluatedComparisons != 2 {
		t.Fatalf("week=%+v %v", week, err)
	}
}

func seedRankedBoard(store *memoryStore, id, user string, when time.Time, refs []ModelRef, ranks []int) {
	sides := []DisplaySide{SideLeft, SideRight, SideC, SideD, SideE}
	found := Experiment{ID: id, UserID: user, Stage: StageWrite, Origin: OriginLab, ReviewMode: ReviewCandidateRanking,
		Status: StatusCompleted, CompletedAt: &when, CreatedAt: when}
	for i, ref := range refs {
		found.Candidates = append(found.Candidates, Candidate{ID: id + "-" + ref.ModelID, ExperimentID: id,
			Model: ref, ModelLabel: ref.ModelID, DisplaySide: sides[i], Status: CandidateSucceeded,
			Rank: ranks[i], Usage: Usage{PromptTokens: 11, LatencyMS: 25}})
	}
	store.mu.Lock()
	store.rows[id] = found
	store.mu.Unlock()
}
