package experiment

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A board reads one (scope, stage, window). The window is rolling and measured at the moment
// of the request, so a verdict that has aged out stops counting without anything being
// deleted, and a board asked for a longer window sees it again.
func TestLeaderboardReadsOnlyItsWindow(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	seedVerdict(store, "fresh", "alice", StageWrite, a, b, now.Add(-2*time.Hour))
	seedVerdict(store, "week-old", "alice", StageWrite, b, a, now.Add(-3*24*time.Hour))
	seedVerdict(store, "month-old", "alice", StageWrite, b, a, now.Add(-20*24*time.Hour))

	cases := []struct {
		window  Window
		matches int
	}{
		{WindowDay, 1},
		{WindowWeek, 2},
		{WindowMonth, 3},
	}
	for _, sample := range cases {
		entries, err := svc.Leaderboard(context.Background(), "alice", StageWrite, sample.window, ScopeMe)
		if err != nil {
			t.Fatalf("%s: %v", sample.window, err)
		}
		played := 0
		for _, entry := range entries {
			played += entry.Matches
		}
		// Each match is counted on both competitors.
		if played != sample.matches*2 {
			t.Fatalf("%s replayed %d competitor-matches, want %d", sample.window, played, sample.matches*2)
		}
	}
}

// The board a window opens is a replay, not a slice of a bigger one: dropping the oldest
// verdict changes the ratings the remaining ones produce.
func TestANarrowerWindowReplaysRatingsFromScratch(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	seedVerdict(store, "old-a-wins", "alice", StageWrite, a, b, now.Add(-10*24*time.Hour))
	seedVerdict(store, "new-b-wins", "alice", StageWrite, b, a, now.Add(-1*time.Hour))

	month, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowMonth, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	day, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowDay, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	// The day holds one verdict between equal competitors, so K/2 separates them exactly.
	if ratingOf(day, b)-ratingOf(day, a) != LeaderboardKFactor {
		t.Fatalf("the day should hold only b's win: %+v", day)
	}
	// The month holds both, which very nearly cancel — the point being that it is replayed
	// from 1500 rather than sliced out of a longer-running rating.
	if spread := ratingOf(month, b) - ratingOf(month, a); spread < -4 || spread > 4 {
		t.Fatalf("one win each should nearly cancel over the month: %+v", month)
	}
	if ratingOf(month, b) == ratingOf(day, b) {
		t.Fatalf("the two windows produced the same rating for b: %+v vs %+v", month, day)
	}
}

// ScopeAll is a flag, not an account: it reads every account's verdicts and the entries it
// returns carry model-level figures only.
func TestAllScopeAggregatesEveryAccountAndNamesNone(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	seedVerdict(store, "mine", "alice", StageWrite, a, b, now.Add(-time.Hour))
	seedVerdict(store, "theirs", "bob", StageWrite, a, b, now.Add(-2*time.Hour))

	mine, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if winsOf(mine, a) != 1 {
		t.Fatalf("my board should hold my one verdict: %+v", mine)
	}
	all, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if winsOf(all, a) != 2 || winsOf(all, b) != 0 {
		t.Fatalf("the shared board should hold both verdicts: %+v", all)
	}
	// Anyone else's board is the same board: it is keyed by nothing the caller owns.
	fromBob, err := svc.Leaderboard(context.Background(), "bob", StageWrite, WindowWeek, ScopeAll)
	if err != nil {
		t.Fatal(err)
	}
	if ratingOf(fromBob, a) != ratingOf(all, a) {
		t.Fatalf("the shared board differed by caller: %+v vs %+v", fromBob, all)
	}
}

// Every value the wire can carry is named. Nothing offers an all-time board, and a value
// outside the vocabulary is refused rather than silently defaulted.
func TestLeaderboardRefusesAWindowOrScopeItDoesNotName(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	if _, err := svc.Leaderboard(context.Background(), "alice", StageWrite, Window("all-time"), ScopeMe); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("window = %v, want ErrInvalidWindow", err)
	}
	if _, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, Scope("everyone-i-follow")); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("scope = %v, want ErrInvalidScope", err)
	}
}

func seedVerdict(store *memoryStore, id, user string, stage Stage, winner, loser ModelRef, at time.Time) {
	decided := at
	store.mu.Lock()
	defer store.mu.Unlock()
	store.rows[id] = Experiment{
		ID: id, UserID: user, Stage: stage, Origin: OriginLab, Status: StatusDecided,
		Outcome: OutcomeWinner, WinnerCandidateID: id + "-w", DecidedAt: &decided,
		Candidates: []Candidate{
			{ID: id + "-w", ExperimentID: id, Model: winner, ModelLabel: winner.ModelID, DisplaySide: SideLeft, Status: CandidateSucceeded},
			{ID: id + "-l", ExperimentID: id, Model: loser, ModelLabel: loser.ModelID, DisplaySide: SideRight, Status: CandidateSucceeded},
		},
	}
}

func ratingOf(entries []LeaderboardEntry, ref ModelRef) int {
	for _, entry := range entries {
		if entry.Model == ref {
			return entry.Rating
		}
	}
	return 0
}

func winsOf(entries []LeaderboardEntry, ref ModelRef) int {
	for _, entry := range entries {
		if entry.Model == ref {
			return entry.Wins
		}
	}
	return -1
}

// A tally explains a rank; it never produces one. The same verdicts rank identically whether
// or not badges came with them, and the tallies land on the model that earned them.
func TestBadgeTalliesAttachWithoutMovingTheRanking(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	seedVerdict(store, "one", "alice", StageWrite, a, b, now.Add(-time.Hour))
	store.tallies = []BadgeTally{
		{Model: a, Badge: BadgeConcise, Count: 1},
		{Model: a, Badge: BadgeFast, Count: 4},
		{Model: b, Badge: BadgeSlow, Count: 2},
		// A model whose verdicts all aged out of this window has nothing to explain here.
		{Model: ModelRef{ProviderID: "p", ModelID: "gone"}, Badge: BadgeFast, Count: 9},
	}
	entries, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("a tally put a model on the board: %+v", entries)
	}
	if entries[0].Model != a || entries[0].Rating <= entries[1].Rating {
		t.Fatalf("the ranking followed the tallies: %+v", entries)
	}
	winner := entries[0]
	if len(winner.BadgeTallies) != 2 {
		t.Fatalf("winner tallies = %+v", winner.BadgeTallies)
	}
	// Most often first, then catalog order.
	if winner.BadgeTallies[0].Badge != BadgeFast || winner.BadgeTallies[0].Count != 4 {
		t.Fatalf("tallies out of order: %+v", winner.BadgeTallies)
	}
	if winner.BadgeTallies[1].Badge != BadgeConcise {
		t.Fatalf("tallies out of order: %+v", winner.BadgeTallies)
	}
	if entries[1].BadgeTallies[0].Badge != BadgeSlow {
		t.Fatalf("loser tallies = %+v", entries[1].BadgeTallies)
	}
}

// Two badges earned equally often fall back on the catalog's own order, so a board reads the
// same way twice in a row.
func TestEquallyEarnedTalliesFallBackOnCatalogOrder(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	seedVerdict(store, "one", "alice", StageWrite, a, b, now.Add(-time.Hour))
	store.tallies = []BadgeTally{
		{Model: a, Badge: BadgeConcise, Count: 2},
		{Model: a, Badge: BadgeNatural, Count: 2},
		{Model: a, Badge: BadgeFast, Count: 2},
	}
	entries, err := svc.Leaderboard(context.Background(), "alice", StageWrite, WindowWeek, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	got := []Badge{}
	for _, tally := range entries[0].BadgeTallies {
		got = append(got, tally.Badge)
	}
	want := []Badge{BadgeFast, BadgeNatural, BadgeConcise}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("catalog order = %v, want %v", got, want)
		}
	}
}
