package clip

import (
	"slices"

	"github.com/postpilot/backend/internal/clip/composition"
)

// CompositionElement describes one declared element, including all the visual
// parts that draw it. Its bounds and times are shared with preview preparation.
type CompositionElement struct {
	InstanceID, ElementID, CutID, Kind, Role        string
	Text, Style, Position, Align                    string
	Rows                                            []composition.ResolvedRow
	Facts                                           []composition.Fact
	Evidence                                        []SourceEvidence
	Region                                          Region
	StartMS, EndMS                                  int
	InMS, OutMS, Layer                              int
	DY                                              float64
	AuthoredStyle, AuthoredPosition, AuthoredTiming bool
	// Whether the owner placed this caption themselves (CDS-82): V1 still holds
	// it inside the safe area, V3 records a contrast shortfall under it as a
	// notice and V13 does not measure its anchor step.
	OwnerPlaced    bool
	FallbackReason string
	Advisories     []string
	Parts          Manifest
	Cues           []CompositionCue
}

func CompositionLayer(role string) int {
	switch role {
	case "badge":
		return 3
	case "hook", "ending":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

type CompositionCue struct {
	Text, Style, Position string
	Region                Region
	StartMS, EndMS        int
}

// ResolvePortableIntervals projects retained element declarations onto the
// edited cut order. Text and authored offsets remain untouched, and invalid
// windows return the element identity for correction before any source download.
func ResolvePortableIntervals(plan EditPlan, limits composition.Limits) (EditPlan, error) {
	if plan.Portable == nil || plan.Portable.Snapshot.Version != CompositionVersion {
		return plan, ErrInvalid
	}
	v := *plan.Portable
	v.Cuts, v.Elements, v.Fallbacks = nil, nil, slices.Clone(v.Fallbacks)
	oldCuts := map[string]composition.Cut{}
	for _, c := range plan.Portable.Cuts {
		if _, exists := oldCuts[c.ID]; exists {
			return plan, ErrInvalid
		}
		oldCuts[c.ID] = c
	}
	starts, lengths, seen := map[string]int{}, map[string]int{}, map[string]bool{}
	duration := 0
	for i, cut := range plan.Cuts {
		c, exists := oldCuts[cut.ID]
		length := cut.OutputDurationMS()
		if !exists || seen[cut.ID] || c.SourceID != cut.SourceID || cut.StartMS < 0 || cut.EndMS <= cut.StartMS || length > limits.MaxDurationMS || !ValidTransition(cut.TransitionMS) || i == 0 && cut.TransitionMS != 0 || cut.TransitionMS >= length {
			return plan, ErrInvalid
		}
		seen[cut.ID] = true
		start := duration - cut.TransitionMS
		if start < 0 || length > limits.MaxDurationMS-start {
			return plan, ErrInvalid
		}
		starts[cut.ID], lengths[cut.ID], duration = start, length, start+length
		c.StartMS, c.EndMS, c.TransitionMS, c.PlaybackRatePermille = cut.StartMS, cut.EndMS, cut.TransitionMS, cut.Rate()
		v.Cuts = append(v.Cuts, c)
	}
	if duration <= 0 || len(v.Cuts) > limits.Cuts || len(plan.Portable.Elements) > limits.Cues {
		return plan, ErrInvalid
	}
	// Narration is measured against the output this edit produces, before any
	// interval is resolved: creating, splitting, trimming, reordering or
	// re-rating footage moves the cuts under a caption and never the caption
	// itself, so a caption the new output cannot hold is named here rather than
	// silently retimed to fit it (CLIP-67).
	if err := ValidateNarrationIntervals(plan.Portable.Elements, duration); err != nil {
		return plan, err
	}
	for _, text := range plan.Portable.Elements {
		r := &text.Resolved
		if r.Element.Basis == "cut" && r.CutID != "" && !seen[r.CutID] {
			continue
		}
		a, b, problem := composition.ResolveInterval(r.Element, duration, lengths[r.CutID], starts[r.CutID], limits.AutoInsetMS)
		if problem != nil {
			return plan, problem
		}
		r.StartMS, r.EndMS = a, b
		r.AuthoredTiming = r.Element.Basis != "cut" || r.Element.StartMS != nil
		if text.Placement != nil && !r.AuthoredTiming {
			choice := text.Placement
			e := r.Element
			e.StartMS, e.EndMS = &choice.StartMS, &choice.EndMS
			a, b, problem := composition.ResolveInterval(e, duration, lengths[r.CutID], starts[r.CutID], limits.AutoInsetMS)
			if problem != nil {
				return plan, problem
			}
			r.StartMS, r.EndMS = a, b
		}
		v.Elements = append(v.Elements, text)
	}
	plan.DurationMS, plan.Portable = duration, &v
	return plan, nil
}

func AutomaticCompositionRepair(text PortableText) bool {
	e := text.Resolved.Element
	// An owner placement is the last word on where a caption sits (CDS-38), so
	// the repair ladder — which moves a caption to make it fit — never reaches
	// one, even before the owner has touched anything else about it.
	return !text.OwnerEdited && text.Placement == nil && text.Owner == (OwnerCaption{}) && e.Kind == "ai" && e.Style == "auto" && e.Position == "auto" && e.Basis == "cut" && e.StartMS == nil && e.EndMS == nil
}
