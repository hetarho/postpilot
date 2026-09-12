package clip

import (
	"slices"

	"github.com/postpilot/backend/internal/clip/composition"
)

// A reading extension spends only unused target duration and remains inside
// one already-observed scene. It never borrows new footage, another item's
// scene, or time needed by the next selected range of the same source.
func ExtendCompositionReadingWindows(plan EditPlan, limits composition.Limits) (EditPlan, error) {
	if plan.Portable == nil {
		return plan, ErrInvalid
	}
	target := min(plan.Portable.TargetDurationMS, limits.MaxDurationMS)
	if target <= plan.DurationMS {
		return plan, nil
	}
	plan.Cuts = slices.Clone(plan.Cuts)
	changed := false
	for i := range plan.Cuts {
		cut := &plan.Cuts[i]
		need, count := 0, 0
		for _, text := range plan.Portable.Elements {
			if text.Resolved.CutID != cut.ID || text.Resolved.Element.Role != "caption" || text.Pace == "rapid" || !AutomaticCompositionRepair(text) || count >= 2 {
				continue
			}
			n := MinExposureMS(text.Resolved.Text)
			for _, alternative := range text.Alternatives {
				if alternative.Text != "" {
					n = min(n, MinExposureMS(alternative.Text))
				}
			}
			need += n
			count++
		}
		extra := need + 2*limits.AutoInsetMS - (cut.EndMS - cut.StartMS)
		if need == 0 || extra <= 0 || extra > target-plan.DurationMS {
			continue
		}
		end := cut.EndMS
		for _, analysis := range plan.Portable.Observations {
			if analysis.Source.ID != cut.SourceID || analysis.Source.Fingerprint != cut.Fingerprint {
				continue
			}
			for _, segment := range analysis.Segments {
				if segment.StartMS <= cut.StartMS && segment.EndMS >= cut.EndMS {
					end = max(end, segment.EndMS)
				}
			}
			end = min(end, analysis.Source.Info.DurationMS)
		}
		for j, other := range plan.Cuts {
			if j != i && other.SourceID == cut.SourceID && other.StartMS >= cut.EndMS {
				end = min(end, other.StartMS)
			}
		}
		// Explicit owner associations are also a boundary: extending into the
		// adjacent item's range is not a same-scene reading repair.
		for _, a := range plan.Portable.Inputs.Associations {
			if a.SourceID == cut.SourceID && a.Fingerprint == cut.Fingerprint {
				if a.StartMS <= cut.StartMS && a.EndMS >= cut.EndMS {
					end = min(end, a.EndMS)
				} else if a.StartMS >= cut.EndMS {
					end = min(end, a.StartMS)
				}
			}
		}
		if extra > end-cut.EndMS {
			continue
		}
		cut.EndMS += extra
		plan.DurationMS += extra
		changed = true
	}
	if !changed {
		return plan, nil
	}
	return ResolvePortableIntervals(plan, limits)
}
