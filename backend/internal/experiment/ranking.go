package experiment

import (
	"math"
	"sort"
)

type Match struct {
	Winner ModelRef
	Loser  ModelRef
}

type LeaderboardEntry struct {
	Rank              int
	Model             ModelRef
	ModelLabel        string
	Rating            int
	Matches           int
	Wins              int
	Losses            int
	SuccessfulCalls   int
	TotalLatencyMS    int64
	PromptTokens      int64
	CompletionTokens  int64
	TotalCostMicrousd int64
	CostQuality       CostSource
	Provisional       bool
	Active            bool
	Recommended       bool
	Disappeared       bool
}

func (e LeaderboardEntry) WinRate() float64 {
	if e.Matches == 0 {
		return 0
	}
	return float64(e.Wins) / float64(e.Matches)
}

func (e LeaderboardEntry) AverageLatencyMS() int64 {
	if e.SuccessfulCalls == 0 {
		return 0
	}
	return e.TotalLatencyMS / int64(e.SuccessfulCalls)
}

func BuildLeaderboard(matches []Match, candidates []Candidate, labels map[ModelRef]string) []LeaderboardEntry {
	entries := map[ModelRef]*LeaderboardEntry{}
	entry := func(ref ModelRef) *LeaderboardEntry {
		if entries[ref] == nil {
			entries[ref] = &LeaderboardEntry{Model: ref, ModelLabel: labels[ref], Rating: LeaderboardInitialRating}
		}
		return entries[ref]
	}
	for _, candidate := range candidates {
		current := entry(candidate.Model)
		if current.ModelLabel == "" {
			current.ModelLabel = candidate.ModelLabel
		}
		if candidate.Status == CandidateSucceeded {
			current.SuccessfulCalls++
			current.TotalLatencyMS += candidate.Usage.LatencyMS
			current.PromptTokens += candidate.Usage.PromptTokens
			current.CompletionTokens += candidate.Usage.CompletionTokens
			if candidate.Usage.CostSource != CostUnavailable {
				current.TotalCostMicrousd += candidate.Usage.CostMicrousd
			}
			current.CostQuality = mergeCostQuality(current.CostQuality, candidate.Usage.CostSource)
		}
	}
	for _, match := range matches {
		winner := entry(match.Winner)
		loser := entry(match.Loser)
		expectedWinner := 1 / (1 + math.Pow(10, float64(loser.Rating-winner.Rating)/400))
		delta := int(math.Round(LeaderboardKFactor * (1 - expectedWinner)))
		winner.Rating += delta
		loser.Rating -= delta
		winner.Matches++
		winner.Wins++
		loser.Matches++
		loser.Losses++
	}
	out := make([]LeaderboardEntry, 0, len(entries))
	for _, current := range entries {
		current.Provisional = current.Matches < LeaderboardMinMatches
		out = append(out, *current)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provisional != out[j].Provisional {
			return !out[i].Provisional
		}
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		return out[i].Model.String() < out[j].Model.String()
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func mergeCostQuality(current, next CostSource) CostSource {
	if next == "" {
		return current
	}
	if current == "" {
		return next
	}
	if current != next {
		return CostMixed
	}
	return current
}
