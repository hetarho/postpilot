package clip

import (
	"maps"
	"reflect"
	"slices"
)

// PlanNotice extends the existing text fallback vocabulary to cut and plan
// targets. Stored records are retained; owner edits only filter the projection.
type PlanNotice struct {
	CopyFallback
	Action      string
	CutRevision int
}

func AddPlanNotice(plan *EditPlan, check, cut, element, action string) {
	n := PlanNotice{CopyFallback: CopyFallback{ElementID: element, CutID: cut, Reason: check}, Action: action, CutRevision: plan.NoticeCutRevisions[noticeRevisionKey(cut, element)]}
	if !slices.Contains(plan.Notices, n) {
		plan.Notices = append(plan.Notices, n)
	}
}

// RecomputePlanNotices runs at composition and rerender admission, never in the
// media executor. A shorter delivered timeline is a completed result.
func RecomputePlanNotices(plan *EditPlan, target, tolerance int) {
	plan.Notices = slices.DeleteFunc(slices.Clone(plan.Notices), func(n PlanNotice) bool {
		return n.CutID == "" && n.ElementID == "" && n.Reason == "plan_target_duration"
	})
	if plan.DurationMS < target-tolerance {
		AddPlanNotice(plan, "plan_target_duration", "", "", "shortfall")
	}
}

func ActivePlanNotices(plan EditPlan) []PlanNotice {
	notices := slices.Clone(plan.Notices)
	if plan.Portable != nil {
		for _, f := range plan.Portable.Fallbacks {
			n := PlanNotice{CopyFallback: f, Action: "removal"}
			for _, t := range plan.Portable.Elements {
				if t.Resolved.Element.ID == f.ElementID && t.Resolved.CutID == f.CutID {
					n.Action = "repair"
				}
			}
			if !slices.ContainsFunc(notices, func(v PlanNotice) bool { return v.CopyFallback == f }) {
				notices = append(notices, n)
			}
		}
	}
	return slices.DeleteFunc(notices, func(n PlanNotice) bool {
		if (n.CutID != "" || n.ElementID != "") && plan.NoticeCutRevisions[noticeRevisionKey(n.CutID, n.ElementID)] != n.CutRevision {
			return true
		}
		if n.ElementID != "" && plan.Portable != nil {
			for _, t := range plan.Portable.Elements {
				if t.Resolved.Element.ID == n.ElementID && t.Resolved.CutID == n.CutID && t.OwnerEdited {
					return true
				}
			}
			for _, t := range plan.Portable.RetiredElements {
				if t.Resolved.Element.ID == n.ElementID && t.Resolved.CutID == n.CutID {
					return true
				}
			}
		}
		return false
	})
}

func sameNoticeCut(a, b Cut) bool {
	return a.ID == b.ID && a.SourceID == b.SourceID && a.Fingerprint == b.Fingerprint && a.StartMS == b.StartMS && a.EndMS == b.EndMS && a.TransitionMS == b.TransitionMS && a.Focal == b.Focal && a.Rate() == b.Rate() && a.OriginalVolume() == b.OriginalVolume()
}

// A no-op save keeps its explanations. Editing or deleting a cut clears only
// that target from the read projection, while retaining its stored history.
func trackNoticeCutEdits(old EditPlan, next *EditPlan) {
	next.Notices = slices.Clone(old.Notices)
	next.NoticeCutRevisions = maps.Clone(old.NoticeCutRevisions)
	if old.Hook != next.Hook {
		if next.NoticeCutRevisions == nil {
			next.NoticeCutRevisions = map[string]int{}
		}
		next.NoticeCutRevisions[noticeRevisionKey("", "hook")]++
	}
	for i, prior := range old.Cuts {
		index := slices.IndexFunc(next.Cuts, func(c Cut) bool { return c.ID == prior.ID })
		changed := index < 0 || index != i || !sameNoticeCut(prior, next.Cuts[index])
		if !changed && old.Portable == nil {
			changed = !reflect.DeepEqual(prior.Copies, next.Cuts[index].Copies) || !slices.Equal(prior.Chips, next.Cuts[index].Chips)
		}
		if changed {
			if next.NoticeCutRevisions == nil {
				next.NoticeCutRevisions = map[string]int{}
			}
			next.NoticeCutRevisions[prior.ID]++
		}
	}
}

func noticeRevisionKey(cut, element string) string {
	if cut != "" {
		return cut
	}
	if element != "" {
		return ":element:" + element
	}
	return ""
}
