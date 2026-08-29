package experiment

import (
	"bytes"
	"errors"
	"testing"
)

func TestFreezeSnapshotCopiesAndHashesCanonicalBytes(t *testing.T) {
	raw := []byte(`{"memo":"same","photos":["a.jpg","b.jpg"]}`)
	frozen, hash, err := FreezeSnapshot(Snapshot{Content: raw, PromptVersion: "write-v2"})
	if err != nil {
		t.Fatal(err)
	}
	raw[0] = 'x'
	if bytes.Equal(raw, frozen.Content) || string(frozen.Content) != `{"memo":"same","photos":["a.jpg","b.jpg"]}` {
		t.Fatalf("snapshot was not defensively copied: %q", frozen.Content)
	}
	if hash != "1b6dae75dd07c98d63b39f1ba3f3dc6249ecdc41fb4d441df3296450ca39cf46" {
		t.Fatalf("hash = %q", hash)
	}
	if _, _, err := FreezeSnapshot(Snapshot{}); err == nil {
		t.Fatal("accepted an empty snapshot")
	}
}

func TestCandidateLifecycleAndVerdicts(t *testing.T) {
	success := Candidate{ID: "good", Status: CandidateSucceeded}
	failure := Candidate{ID: "bad", Status: CandidateFailed}
	cases := []struct {
		name       string
		candidates []Candidate
		want       Status
		wantErr    bool
	}{
		{"review", []Candidate{success, success}, StatusReview, false},
		{"partial", []Candidate{success, failure}, StatusPartial, false},
		{"failed", []Candidate{failure, failure}, StatusFailed, false},
		{"not terminal", []Candidate{success, {Status: CandidateRunning}}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := StatusAfterCandidates(tc.candidates)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("status = %q, err = %v", got, err)
			}
		})
	}

	review := Experiment{Status: StatusReview, Candidates: []Candidate{success, failure}}
	if got, err := ValidateVerdict(review, "good", false); err != nil || got.ID != "good" {
		t.Fatalf("paired verdict = %+v, %v", got, err)
	}
	if _, err := ValidateVerdict(review, "good", true); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("review accepted use-single: %v", err)
	}
	partial := Experiment{Status: StatusPartial, Candidates: []Candidate{success, failure}}
	if _, err := ValidateVerdict(partial, "good", false); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("partial accepted paired verdict: %v", err)
	}
	if _, err := ValidateVerdict(partial, "bad", true); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("failed candidate was selectable: %v", err)
	}
}

func TestResolveCostQuality(t *testing.T) {
	model := Model{InputUSDPerMillion: "0.375", OutputUSDPerMillion: "1.875"}
	reported := ResolveCost(UsageReport{PromptTokens: 10, CompletionTokens: 2, CostMicrousd: 9, CostReported: true}, model)
	if reported.CostSource != CostReported || reported.CostMicrousd != 9 {
		t.Fatalf("reported = %+v", reported)
	}
	estimated := ResolveCost(UsageReport{PromptTokens: 10, CompletionTokens: 2}, model)
	if estimated.CostSource != CostEstimated || estimated.CostMicrousd != 8 {
		t.Fatalf("estimated = %+v", estimated)
	}
	unavailable := ResolveCost(UsageReport{PromptTokens: 10}, Model{})
	if unavailable.CostSource != CostUnavailable || unavailable.CostMicrousd != 0 {
		t.Fatalf("unavailable = %+v", unavailable)
	}
	missingUsage := ResolveCost(UsageReport{}, model)
	if missingUsage.CostSource != CostUnavailable {
		t.Fatalf("missing usage became a zero estimate: %+v", missingUsage)
	}
}

func TestLeaderboardReplaysInOrderAndKeepsUnavailableCostExplicit(t *testing.T) {
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	calls := []Candidate{
		{Model: a, ModelLabel: "A", Status: CandidateSucceeded, Usage: Usage{PromptTokens: 10, CostSource: CostUnavailable, LatencyMS: 100}},
		{Model: b, ModelLabel: "B", Status: CandidateSucceeded, Usage: Usage{PromptTokens: 20, CostMicrousd: 5, CostSource: CostReported, LatencyMS: 300}},
	}
	forward := BuildLeaderboard([]Match{{Winner: a, Loser: b}, {Winner: a, Loser: b}, {Winner: b, Loser: a}}, calls, nil)
	reversed := BuildLeaderboard([]Match{{Winner: b, Loser: a}, {Winner: a, Loser: b}, {Winner: a, Loser: b}}, calls, nil)
	byModel := func(entries []LeaderboardEntry, ref ModelRef) LeaderboardEntry {
		for _, entry := range entries {
			if entry.Model == ref {
				return entry
			}
		}
		return LeaderboardEntry{}
	}
	if byModel(forward, a).Rating == byModel(reversed, a).Rating {
		t.Fatal("changing decision order did not change Elo replay")
	}
	if byModel(forward, a).Provisional || byModel(forward, a).Matches != 3 {
		t.Fatalf("three-match entry should be non-provisional: %+v", byModel(forward, a))
	}
	if byModel(forward, a).CostQuality != CostUnavailable {
		t.Fatalf("missing cost became zero/unspecified: %+v", byModel(forward, a))
	}
	if byModel(forward, b).CostQuality != CostReported || byModel(forward, b).AverageLatencyMS() != 300 {
		t.Fatalf("reported accounting = %+v", byModel(forward, b))
	}
}
