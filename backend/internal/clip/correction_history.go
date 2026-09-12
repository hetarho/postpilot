package clip

import (
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"slices"
)

// The archive contains only identities produced by this generation. Deleted
// cuts/text can be restored after autosave without accepting a client-made cut.
func correctionArchive(old EditPlan, in CorrectionPlan, portable *PortablePlan) (map[string]Cut, map[string]PortableText) {
	knownCuts, knownText := map[string]Cut{}, map[string]PortableText{}
	bindings := map[string]composition.Cut{}
	for _, c := range append(slices.Clone(old.Portable.RetiredCuts), old.Cuts...) {
		knownCuts[c.ID] = c
	}
	for _, c := range append(slices.Clone(old.Portable.RetiredBindings), old.Portable.Cuts...) {
		bindings[c.ID] = c
	}
	for _, t := range append(slices.Clone(old.Portable.RetiredElements), old.Portable.Elements...) {
		knownText[t.Resolved.InstanceID] = t
	}
	retainedCuts, retainedText := map[string]bool{}, map[string]bool{}
	portable.Cuts, portable.RetiredCuts, portable.RetiredBindings, portable.RetiredElements = nil, nil, nil, nil
	for _, c := range in.Cuts {
		retainedCuts[c.ID] = true
		if binding, ok := bindings[c.ID]; ok {
			portable.Cuts = append(portable.Cuts, binding)
		}
	}
	for _, t := range in.Elements {
		retainedText[t.InstanceID] = true
	}
	// Stable ordering keeps no-op saves and encoded snapshots deterministic.
	cutIDs, textIDs := []string{}, []string{}
	for id := range knownCuts {
		cutIDs = append(cutIDs, id)
	}
	for id := range knownText {
		textIDs = append(textIDs, id)
	}
	slices.Sort(cutIDs)
	slices.Sort(textIDs)
	for _, id := range cutIDs {
		if !retainedCuts[id] {
			portable.RetiredCuts = append(portable.RetiredCuts, knownCuts[id])
			portable.RetiredBindings = append(portable.RetiredBindings, bindings[id])
		}
	}
	for _, id := range textIDs {
		if !retainedText[id] {
			portable.RetiredElements = append(portable.RetiredElements, knownText[id])
		}
	}
	return knownCuts, knownText
}

func changedAssociations(before, after []SourceAssociation) []SourceAssociation {
	var changed []SourceAssociation
	for _, a := range before {
		if !slices.Contains(after, a) {
			changed = append(changed, a)
		}
	}
	for _, a := range after {
		if !slices.Contains(before, a) {
			changed = append(changed, a)
		}
	}
	return changed
}
func associationAffectsText(text PortableText, changed []SourceAssociation) bool {
	if text.Resolved.Element.Kind != "ai" {
		return false
	}
	for _, a := range changed {
		if text.Resolved.GroupID == a.GroupID && text.Resolved.ItemID == a.ItemID {
			return true
		}
		for _, e := range text.Evidence {
			if e.SourceID == a.SourceID && e.Fingerprint == a.Fingerprint && e.StartMS < a.EndMS && e.EndMS > a.StartMS {
				return true
			}
		}
	}
	return false
}

// Phrase windows use cut time even when the containing text has output timing.
func ValidateEditablePhrases(plan EditPlan) error {
	if plan.Portable == nil {
		return nil
	}
	starts := map[string]int{}
	offset := 0
	for _, c := range plan.Cuts {
		offset -= c.TransitionMS
		starts[c.ID] = offset
		offset += c.EndMS - c.StartMS
	}
	for _, t := range plan.Portable.Elements {
		if len(t.Phrases) == 0 {
			continue
		}
		r := t.Resolved
		problem := func() error {
			return &composition.Problem{ElementID: r.Element.ID, Line: r.Element.Span.Line, Reason: "rapid_readability"}
		}
		if t.Pace != "rapid" || r.Element.Role != "caption" || len(t.Phrases) > design.Rapid.MaxPerCut {
			return problem()
		}
		previous := r.StartMS
		for _, p := range t.Phrases {
			a, b := starts[r.CutID]+p.StartMS, starts[r.CutID]+p.EndMS
			if p.Text == "" || design.Chars(p.Text) > design.Rapid.MaxChars || a < previous || a < r.StartMS || b > r.EndMS || b-a < design.Rapid.MinMS || b-a > design.Rapid.MaxMS {
				return problem()
			}
			previous = b
		}
	}
	return nil
}

func ValidateCompositionEvidence(plan EditPlan) error {
	if plan.Portable == nil {
		return nil
	}
	for _, t := range plan.Portable.Elements {
		if t.StaleEvidence {
			return &composition.Problem{ElementID: t.Resolved.Element.ID, Line: t.Resolved.Element.Span.Line, Reason: "source_association"}
		}
	}
	return nil
}
