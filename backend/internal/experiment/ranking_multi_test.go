package experiment

import "testing"

func TestThreeWayRankingTiesAndCountsOneEvaluation(t *testing.T) {
	a, b, c := ModelRef{"p", "a"}, ModelRef{"p", "b"}, ModelRef{"p", "c"}
	participants := []RankedParticipant{{Model: a, Rank: 1, DisplaySide: SideLeft}, {Model: b, Rank: 1, DisplaySide: SideRight}, {Model: c, Rank: 2, DisplaySide: SideC}}
	calls := []Candidate{{Model: a, Status: CandidateSucceeded, Usage: Usage{PromptTokens: 10, LatencyMS: 50}},
		{Model: b, Status: CandidateSucceeded, Usage: Usage{PromptTokens: 20, LatencyMS: 70}},
		{Model: c, Status: CandidateSucceeded, Usage: Usage{PromptTokens: 30, LatencyMS: 90}}}
	board := BuildLeaderboard([]Match{{Ranked: participants}}, calls, nil, nil)
	rows := map[ModelRef]LeaderboardEntry{}
	total := 0
	for _, row := range board {
		rows[row.Model] = row
		total += row.Rating - LeaderboardInitialRating
	}
	if len(rows) != 3 || total != 0 {
		t.Fatalf("board=%+v sum=%d", board, total)
	}
	for _, ref := range []ModelRef{a, b} {
		row := rows[ref]
		if row.Rating != 1508 || row.EvaluatedComparisons != 1 || row.Matches != 2 ||
			row.Wins != 1 || row.Draws != 1 || row.Losses != 0 || !row.Provisional || row.SuccessfulCalls != 1 {
			t.Fatalf("%s=%+v", ref, row)
		}
	}
	if row := rows[c]; row.Rating != 1484 || row.EvaluatedComparisons != 1 || row.Losses != 2 || row.PromptTokens != 30 {
		t.Fatalf("last=%+v", row)
	}
}

func TestFiveWayEventHasBoundedZeroSumMovement(t *testing.T) {
	participants := []RankedParticipant{}
	for i, side := range []DisplaySide{SideLeft, SideRight, SideC, SideD, SideE} {
		participants = append(participants, RankedParticipant{Model: ModelRef{"p", string(rune('a' + i))}, Rank: i + 1, DisplaySide: side})
	}
	board := BuildLeaderboard([]Match{{Ranked: participants}}, nil, nil, nil)
	want := map[string]int{"a": 1516, "b": 1508, "c": 1500, "d": 1492, "e": 1484}
	sum := 0
	for _, row := range board {
		if row.Rating != want[row.Model.ModelID] || row.EvaluatedComparisons != 1 || row.Matches != 4 || !row.Provisional {
			t.Fatalf("%s=%+v", row.Model, row)
		}
		sum += row.Rating - 1500
	}
	if sum != 0 {
		t.Fatalf("integer changes sum to %d", sum)
	}
}

func TestAllTiedFiveWayDrawsFourPairsEach(t *testing.T) {
	participants := []RankedParticipant{}
	for i, side := range []DisplaySide{SideE, SideD, SideC, SideRight, SideLeft} {
		participants = append(participants, RankedParticipant{Model: ModelRef{"p", string(rune('a' + i))}, Rank: 1, DisplaySide: side})
	}
	for _, row := range BuildLeaderboard([]Match{{Ranked: participants}}, nil, nil, nil) {
		if row.Rating != 1500 || row.Draws != 4 || row.Matches != 4 || row.EvaluatedComparisons != 1 ||
			row.Wins != 0 || row.Losses != 0 {
			t.Fatalf("tie=%+v", row)
		}
	}
}

func TestMultiwayRoundingIsZeroSumAndStable(t *testing.T) {
	a, b, c := ModelRef{"p", "a"}, ModelRef{"p", "b"}, ModelRef{"p", "c"}
	seed := []Match{{Winner: a, Loser: b}, {Winner: a, Loser: c}, {Winner: b, Loser: c}}
	ranked := Match{Ranked: []RankedParticipant{{Model: c, Rank: 1, DisplaySide: SideC}, {Model: a, Rank: 2, DisplaySide: SideLeft}, {Model: b, Rank: 2, DisplaySide: SideRight}}}
	first := BuildLeaderboard(append(append([]Match{}, seed...), ranked), nil, nil, nil)
	second := BuildLeaderboard(append(append([]Match{}, seed...), ranked), nil, nil, nil)
	if len(first) != len(second) {
		t.Fatalf("nondeterministic length: %d %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Model != second[i].Model || first[i].Rating != second[i].Rating {
			t.Fatalf("nondeterministic: %+v %+v", first, second)
		}
	}
	before := BuildLeaderboard(seed, nil, nil, nil)
	sum := 0
	for _, after := range first {
		for _, previous := range before {
			if previous.Model == after.Model {
				sum += after.Rating - previous.Rating
			}
		}
	}
	if sum != 0 {
		t.Fatalf("ranked event gained %d points", sum)
	}
}

func TestUpsetAgainstStrongerOpponentEarnsMoreThanAnEvenWin(t *testing.T) {
	a, b := ModelRef{"p", "a"}, ModelRef{"p", "b"}
	seed := []Match{{Winner: a, Loser: b}, {Winner: a, Loser: b}}
	before := BuildLeaderboard(seed, nil, nil, nil)
	upset := Match{Ranked: []RankedParticipant{{Model: a, Rank: 2, DisplaySide: SideLeft}, {Model: b, Rank: 1, DisplaySide: SideRight}}}
	after := BuildLeaderboard(append(append([]Match{}, seed...), upset), nil, nil, nil)
	rating := func(board []LeaderboardEntry, ref ModelRef) int {
		for _, entry := range board {
			if entry.Model == ref {
				return entry.Rating
			}
		}
		return 0
	}
	gain := rating(after, b) - rating(before, b)
	if gain <= LeaderboardKFactor/2 {
		t.Fatalf("upset earned %d points, want more than even win", gain)
	}
	if rating(after, a)-rating(before, a) != -gain {
		t.Fatalf("two-way event not zero sum: %+v", after)
	}
}
