package clip

import "fmt"

type ObservedEvidence struct {
	ID      string
	Source  SourceEvidence
	Segment Segment
}

// CutEvidence covers every frame with recorded observations. It never bridges
// an unobserved gap or treats a filename as an observation.
func CutEvidence(analyses []SourceAnalysis, cut Cut) ([]ObservedEvidence, bool) {
	for _, analysis := range analyses {
		if analysis.Source.ID != cut.SourceID || analysis.Source.Fingerprint != cut.Fingerprint {
			continue
		}
		end := cut.StartMS
		var out []ObservedEvidence
		for i, segment := range analysis.Segments {
			start, stop := max(segment.StartMS, cut.StartMS), min(segment.EndMS, cut.EndMS)
			if start >= stop {
				continue
			}
			if start > end {
				return nil, false
			}
			out = append(out, ObservedEvidence{ID: ObservationID(cut.SourceID, i), Source: SourceEvidence{cut.SourceID, cut.Fingerprint, start, stop}, Segment: segment})
			end = max(end, stop)
		}
		return out, end == cut.EndMS && len(out) > 0
	}
	return nil, false
}

func ObservationID(source string, index int) string { return fmt.Sprintf("%s/%d", source, index) }

// CutScenes reports the observed scenes a cut touches: exactly one for a valid
// selection, more when it crosses a boundary, none when nothing observed it.
func CutScenes(analyses []SourceAnalysis, cut Cut) []Segment {
	var out []Segment
	for _, analysis := range analyses {
		if analysis.Source.ID != cut.SourceID || analysis.Source.Fingerprint != cut.Fingerprint {
			continue
		}
		for _, segment := range analysis.Segments {
			if max(segment.StartMS, cut.StartMS) < min(segment.EndMS, cut.EndMS) {
				out = append(out, segment)
			}
		}
	}
	return out
}

// ContainedScene is the ONE observed scene a cut may be taken from. A cut that
// spans two scenes is refused even when their union leaves no time gap: the
// scenes were recorded separately because they show different things, and a
// selection that mixes them has no single subject, status or scene to answer
// for (CLIP-7, CLIP-98).
func ContainedScene(analyses []SourceAnalysis, cut Cut) (Segment, bool) {
	scenes := CutScenes(analyses, cut)
	if len(scenes) != 1 {
		return Segment{}, false
	}
	s := scenes[0]
	return s, s.StartMS <= cut.StartMS && cut.EndMS <= s.EndMS
}

// SelectableScene answers whether AUTOMATIC assembly may use this footage at a
// given rate. Uncertain footage is usable but only as it was recorded — at 1x —
// and unknown or unusable footage is not selectable at all (CLIP-99). A legacy
// scene carries no status and keeps its previous freedom.
func SelectableScene(s Segment, ratePermille int) bool {
	if !s.Observed() {
		return true
	}
	if s.Unknowable() {
		return false
	}
	return s.Confident() || ratePermille == RateUnitPermille
}
