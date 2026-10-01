package experiment

import (
	"math"
	"sort"
)

// Match is one counted event, replayed in decision order (MODEL-38). A completed ranking
// names every participant. Historical winner verdicts retain Winner/Loser, and a legacy
// two-delivery dismissal carries both in Dismissed against the fixed reference.
type Match struct {
	Winner    ModelRef
	Loser     ModelRef
	Dismissed []ModelRef
	Ranked    []RankedParticipant
}

// RankedParticipant is one model in a completed comparison. DisplaySide breaks
// equal fractional remainders without depending on request or map iteration order.
type RankedParticipant struct {
	Model       ModelRef
	Rank        int
	DisplaySide DisplaySide
}

// BadgeTally is how often one model earned one badge inside a board's own scope, stage and
// window. It carries no note and names no account (MODEL-41, MODEL-63).
type BadgeTally struct {
	Model ModelRef
	Badge Badge
	Count int
}

type LeaderboardEntry struct {
	Rank                 int
	Model                ModelRef
	ModelLabel           string
	Rating               int
	Matches              int
	Wins                 int
	Losses               int
	Draws                int
	EvaluatedComparisons int
	SuccessfulCalls      int
	TotalLatencyMS       int64
	PromptTokens         int64
	CompletionTokens     int64
	TotalCostMicrousd    int64
	CostQuality          CostSource
	Provisional          bool
	Active               bool
	Recommended          bool
	Disappeared          bool
	// What this model's verdicts said about it in the same span, most often first. It
	// explains a rank rather than producing one.
	BadgeTallies []BadgeTally
}

// ComparisonCostRow is the operator-only cost projection of one counted leaderboard model.
// It deliberately carries no account, experiment, output or note (MODEL-39, MODEL-41).
type ComparisonCostRow struct {
	Model                ModelRef
	ModelLabel           string
	EvaluatedComparisons int
	TotalCostMicrousd    int64
	CostQuality          CostSource
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

func BuildLeaderboard(matches []Match, candidates []Candidate, labels map[ModelRef]string, tallies []BadgeTally) []LeaderboardEntry {
	entries := map[ModelRef]*LeaderboardEntry{}
	entry := func(ref ModelRef) *LeaderboardEntry {
		if entries[ref] == nil {
			entries[ref] = &LeaderboardEntry{Model: ref, ModelLabel: labels[ref], Rating: LeaderboardInitialRating}
		}
		return entries[ref]
	}
	// A model reaches the board only through a counted outcome; the calls beside it are
	// accounting for a model already there (MODEL-38).
	for _, match := range matches {
		if len(match.Ranked) >= 2 {
			replayRanking(match.Ranked, entry)
			continue
		}
		if len(match.Dismissed) > 0 {
			for _, ref := range match.Dismissed {
				loser := entry(ref)
				expected := 1 / (1 + math.Pow(10, float64(LeaderboardDismissalReference-loser.Rating)/400))
				loser.Rating += int(math.Round(LeaderboardKFactor * (0 - expected)))
				loser.Matches++
				loser.Losses++
				loser.EvaluatedComparisons++
			}
			continue
		}
		winner := entry(match.Winner)
		loser := entry(match.Loser)
		expectedWinner := 1 / (1 + math.Pow(10, float64(loser.Rating-winner.Rating)/400))
		delta := int(math.Round(LeaderboardKFactor * (1 - expectedWinner)))
		winner.Rating += delta
		loser.Rating -= delta
		winner.Matches++
		winner.Wins++
		winner.EvaluatedComparisons++
		loser.Matches++
		loser.Losses++
		loser.EvaluatedComparisons++
	}
	for _, candidate := range candidates {
		current := entries[candidate.Model]
		if current == nil {
			continue
		}
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
	out := make([]LeaderboardEntry, 0, len(entries))
	for _, current := range entries {
		current.Provisional = current.EvaluatedComparisons < LeaderboardMinEvaluations
		out = append(out, *current)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provisional != out[j].Provisional {
			return !out[i].Provisional
		}
		if out[i].Rating != out[j].Rating {
			return out[i].Rating > out[j].Rating
		}
		if out[i].EvaluatedComparisons != out[j].EvaluatedComparisons {
			return out[i].EvaluatedComparisons > out[j].EvaluatedComparisons
		}
		return out[i].Model.String() < out[j].Model.String()
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	// Attached after the ranking, and only to a model the board already holds: a badge given
	// to a model whose verdicts all aged out of this window has nothing to explain here.
	byModel := map[ModelRef][]BadgeTally{}
	for _, tally := range tallies {
		byModel[tally.Model] = append(byModel[tally.Model], tally)
	}
	order := map[Badge]int{}
	for index, badge := range catalogOrder() {
		order[badge] = index
	}
	for i := range out {
		attached := byModel[out[i].Model]
		sort.Slice(attached, func(a, b int) bool {
			if attached[a].Count != attached[b].Count {
				return attached[a].Count > attached[b].Count
			}
			return order[attached[a].Badge] < order[attached[b].Badge]
		})
		out[i].BadgeTallies = attached
	}
	return out
}

func replayRanking(participants []RankedParticipant, entry func(ModelRef) *LeaderboardEntry) {
	n := len(participants)
	before := make([]int, n)
	raw := make([]float64, n)
	for i, candidate := range participants {
		current := entry(candidate.Model)
		before[i] = current.Rating
		current.EvaluatedComparisons++
	}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			left, right := participants[i], participants[j]
			a, b := entry(left.Model), entry(right.Model)
			score := 0.5
			switch {
			case left.Rank < right.Rank:
				score = 1
				a.Wins++
				b.Losses++
			case left.Rank > right.Rank:
				score = 0
				a.Losses++
				b.Wins++
			default:
				a.Draws++
				b.Draws++
			}
			a.Matches++
			b.Matches++
			expected := 1 / (1 + math.Pow(10, float64(before[j]-before[i])/400))
			raw[i] += score - expected
			raw[j] += expected - score
		}
	}
	for i := range raw {
		raw[i] *= float64(LeaderboardKFactor) / float64(n-1)
	}
	changes := make([]int, n)
	if n == 2 {
		changes[0] = int(math.Round(raw[0]))
		changes[1] = -changes[0]
	} else {
		// Every event is zero sum. Floor each value and award the outstanding
		// integer points to the largest remainders, then persisted position.
		remainders := make([]float64, n)
		total := 0
		for i, value := range raw {
			nearest := math.Round(value)
			if math.Abs(value-nearest) < 1e-9 {
				value = nearest
			}
			changes[i] = int(math.Floor(value))
			remainders[i] = value - float64(changes[i])
			total += changes[i]
		}
		order := make([]int, n)
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool {
			i, j := order[a], order[b]
			if math.Abs(remainders[i]-remainders[j]) > 1e-9 {
				return remainders[i] > remainders[j]
			}
			return sideOrder(participants[i].DisplaySide) < sideOrder(participants[j].DisplaySide)
		})
		for i := 0; i < -total; i++ {
			changes[order[i]]++
		}
	}
	for i, candidate := range participants {
		entry(candidate.Model).Rating += changes[i]
	}
}

func sideOrder(side DisplaySide) int {
	switch side {
	case SideLeft:
		return 0
	case SideRight:
		return 1
	case SideC:
		return 2
	case SideD:
		return 3
	case SideE:
		return 4
	default:
		return 5
	}
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
