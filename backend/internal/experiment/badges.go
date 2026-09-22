package experiment

// The badge catalog is fixed and code-owned (MODEL-62): the operator does not edit it, and
// the ids are stable because a leaderboard tallies them across releases. Copy lives in the
// UI locale resources, never here.
type Badge string

const (
	// Positive.
	BadgeFast       Badge = "fast"
	BadgeNatural    Badge = "natural"
	BadgeOnBrief    Badge = "on_brief"
	BadgeStructured Badge = "structured"
	BadgeAccurate   Badge = "accurate"
	BadgeInVoice    Badge = "in_voice"
	BadgeConcise    Badge = "concise"
	// Negative.
	BadgeSlow         Badge = "slow"
	BadgeAILike       Badge = "ai_like"
	BadgeOffBrief     Badge = "off_brief"
	BadgeVerbose      Badge = "verbose"
	BadgeInaccurate   Badge = "inaccurate"
	BadgeOffVoice     Badge = "off_voice"
	BadgeRepetitive   Badge = "repetitive"
	BadgeBrokenFormat Badge = "broken_format"
	// Its own group, and the only one that carries a note.
	BadgeOther Badge = "other"
)

// BadgeNoteMaxLength bounds the free note beside `other`. Code-owned for the same reason the
// catalog is: the browser counts against it while typing and the server refuses past it.
const BadgeNoteMaxLength = 200

// PositiveBadges and NegativeBadges are the two groups the sheet presents, in the order it
// presents them. `other` belongs to neither and is rendered below both.
var (
	PositiveBadges = []Badge{BadgeFast, BadgeNatural, BadgeOnBrief, BadgeStructured, BadgeAccurate, BadgeInVoice, BadgeConcise}
	NegativeBadges = []Badge{BadgeSlow, BadgeAILike, BadgeOffBrief, BadgeVerbose, BadgeInaccurate, BadgeOffVoice, BadgeRepetitive, BadgeBrokenFormat}
)

// AppliesTo reports whether a stage offers this badge. Only the voice pair is conditional:
// an observe comparison produces no prose, so neither judgement about voice can be made of
// it (MODEL-62).
func (b Badge) AppliesTo(stage Stage) bool {
	if b == BadgeInVoice || b == BadgeOffVoice {
		return stage == StageWrite || stage == StageAnalyze
	}
	return true
}

func (b Badge) valid() bool {
	switch b {
	case BadgeFast, BadgeNatural, BadgeOnBrief, BadgeStructured, BadgeAccurate, BadgeInVoice, BadgeConcise,
		BadgeSlow, BadgeAILike, BadgeOffBrief, BadgeVerbose, BadgeInaccurate, BadgeOffVoice, BadgeRepetitive,
		BadgeBrokenFormat, BadgeOther:
		return true
	default:
		return false
	}
}

// CandidateBadges is what one candidate was given at the verdict.
type CandidateBadges struct {
	CandidateID string
	Badges      []Badge
	OtherNote   string
}

// NormalizeBadges checks the offered badges against the comparison they are being written
// onto and returns them deduplicated, in catalog order. Zero badges is a legal answer for
// every candidate — a badge is evidence offered, not a toll on the verdict (MODEL-61).
func NormalizeBadges(found Experiment, input []CandidateBadges) ([]CandidateBadges, error) {
	if len(input) == 0 {
		return nil, nil
	}
	known := map[string]bool{}
	for _, candidate := range found.Candidates {
		known[candidate.ID] = true
	}
	seenCandidate := map[string]bool{}
	out := make([]CandidateBadges, 0, len(input))
	for _, offered := range input {
		if !known[offered.CandidateID] || seenCandidate[offered.CandidateID] {
			return nil, ErrBadgesInvalid
		}
		seenCandidate[offered.CandidateID] = true
		chosen := map[Badge]bool{}
		for _, badge := range offered.Badges {
			if !badge.valid() || !badge.AppliesTo(found.Stage) {
				return nil, ErrBadgesInvalid
			}
			chosen[badge] = true
		}
		note := offered.OtherNote
		if note != "" && !chosen[BadgeOther] {
			return nil, ErrBadgesInvalid
		}
		if len([]rune(note)) > BadgeNoteMaxLength {
			return nil, ErrBadgesInvalid
		}
		ordered := make([]Badge, 0, len(chosen))
		for _, badge := range catalogOrder() {
			if chosen[badge] {
				ordered = append(ordered, badge)
			}
		}
		if len(ordered) == 0 && note == "" {
			continue
		}
		out = append(out, CandidateBadges{CandidateID: offered.CandidateID, Badges: ordered, OtherNote: note})
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func catalogOrder() []Badge {
	order := make([]Badge, 0, len(PositiveBadges)+len(NegativeBadges)+1)
	order = append(order, PositiveBadges...)
	order = append(order, NegativeBadges...)
	return append(order, BadgeOther)
}
