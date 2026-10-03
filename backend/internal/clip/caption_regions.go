package clip

// CaptionBodyWindow is the output span left for captions by enabled project
// regions. A disabled region has no element in the plan. The two regions stay
// attached to the beginning and end of the output as their lengths are edited.
func CaptionBodyWindow(plan EditPlan) (start, end int) {
	end = plan.DurationMS
	if plan.Portable == nil {
		return start, end
	}
	for _, text := range plan.Portable.Elements {
		switch text.Resolved.Element.Role {
		case "hook":
			start = max(start, text.Resolved.EndMS)
		case "ending":
			end = min(end, text.Resolved.StartMS)
		}
	}
	return start, end
}

// An owner-set caption interval is never silently moved when a region changes.
func ValidateOwnerCaptionRegions(plan EditPlan) error {
	start, end := CaptionBodyWindow(plan)
	if plan.Portable == nil {
		return nil
	}
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role != "caption" || !text.OwnerEdited {
			continue
		}
		if text.Resolved.StartMS < start || text.Resolved.EndMS > end {
			return narrationRefusal(text.Resolved.Element.ID, NoticeCaptionRegionOverlap)
		}
	}
	return nil
}
