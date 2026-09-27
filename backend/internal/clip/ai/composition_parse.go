package ai

import (
	"regexp"

	"github.com/postpilot/backend/internal/clip"
)

// generatedJSON is one generated region entry's answer: its rows, and the shorter rows it
// falls back to. It cites nothing (CLIP-134).
type generatedJSON struct {
	ElementID string
	Text      string
	ShortText string
	Rows      []string
	ShortRows []string
}

var writerIdentity = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// A reference must identify an actual overlapping observation. For cut selection
// all covering observations are mandatory; copy may cite a subset of that cut.
func selectedReferences(ids []string, evidence []clip.ObservedEvidence, complete bool) ([]clip.SourceEvidence, bool) {
	if len(ids) > 120 || complete && len(ids) == 0 {
		return nil, false
	}
	seen := map[string]bool{}
	var out []clip.SourceEvidence
	for _, id := range ids {
		if seen[id] {
			return nil, false
		}
		seen[id] = true
		found := false
		for _, observation := range evidence {
			if observation.ID == id {
				out = append(out, observation.Source)
				found = true
			}
		}
		if !found {
			return nil, false
		}
	}
	if complete {
		for _, observation := range evidence {
			if !seen[observation.ID] {
				return nil, false
			}
		}
	}
	return out, true
}
