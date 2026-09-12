package clip

import (
	"math"
	"reflect"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
)

// Editable values reference a saved identity. Evidence, binding ownership and
// the template source remain server-owned even when text is corrected.
type CorrectionText struct {
	EffectiveStartMS, EffectiveEndMS         *int
	Phrases                                  []EditablePhrase
	StaleEvidence, EvidenceReviewed          bool
	Evidence                                 []SourceEvidence
	FallbackReason                           string
	InstanceID, ElementID, CutID, Kind, Role string
	Text                                     string
	Rows                                     []composition.ResolvedRow
	Style, Position, Align, Basis            string
	StartMS, EndMS                           *int
	Pace, Accent, Keyword                    string
	ResolvedStartMS, ResolvedEndMS           int
	GroupID, ItemID                          string
}

func correctionText(t PortableText) CorrectionText {
	r, e := t.Resolved, t.Resolved.Element
	result := CorrectionText{InstanceID: r.InstanceID, ElementID: e.ID, CutID: r.CutID, Kind: e.Kind, Role: e.Role,
		Text: r.Text, Rows: slices.Clone(r.Rows), Style: e.Style, Position: e.Position, Align: e.Align, Basis: e.Basis,
		StartMS: e.StartMS, EndMS: e.EndMS, Pace: t.Pace, Accent: t.Accent, Keyword: t.Keyword,
		ResolvedStartMS: r.StartMS, ResolvedEndMS: r.EndMS, GroupID: r.GroupID, ItemID: r.ItemID,
		Phrases: slices.Clone(t.Phrases), StaleEvidence: t.StaleEvidence, Evidence: slices.Clone(t.Evidence), FallbackReason: t.FallbackReason}
	if t.Placement != nil {
		a, b := t.Placement.StartMS, t.Placement.EndMS
		result.EffectiveStartMS, result.EffectiveEndMS = &a, &b
	}
	return result
}

func applyNativeCorrection(cfg RenderConfig, p Project, old EditPlan, styles []string, sources []AnalysisSource, in CorrectionPlan) (EditPlan, []string, error) {
	// An older client must never silently replace the portable plan with its
	// legacy caption projection. Reload with the native editing contract.
	if !in.NativeComposition {
		return EditPlan{}, nil, ErrCompositionUnavailable
	}
	next := old
	portable := *old.Portable
	portable.NativeEditing = portable.NativeEditing || portable.Snapshot.Legacy

	changed := []SourceAssociation{}
	if in.Associations != nil {
		portable.Inputs.Associations = slices.Clone(*in.Associations)
		limits := cfg.Composition
		if portable.Snapshot.Legacy {
			limits = LegacyCompositionLimits(limits)
		}
		doc, problem := composition.Parse(portable.Snapshot.Body, limits)
		if problem != nil {
			return EditPlan{}, nil, problem
		}
		if err := ValidateCompositionInputs(doc, portable.Inputs, limits, false); err != nil {
			return EditPlan{}, nil, err
		}
		if err := ValidateSourceAssociations(p, portable.Inputs.Associations); err != nil {
			return EditPlan{}, nil, err
		}
		changed = changedAssociations(old.Portable.Inputs.Associations, portable.Inputs.Associations)
	}
	portable.Elements = nil
	portable.TargetDurationMS = 0
	geometryChanged := len(old.Cuts) != len(in.Cuts)
	if !geometryChanged {
		for i, c := range in.Cuts {
			prior := old.Cuts[i]
			if c.ID != prior.ID || c.StartMS != prior.StartMS || c.EndMS != prior.EndMS {
				geometryChanged = true
				break
			}
		}
	}
	if len(portable.Observations) == 0 && geometryChanged {
		var err error
		portable.Observations, err = RetainedObservations(p)
		if err != nil {
			return EditPlan{}, nil, err
		}
	}
	next.Portable, next.Cuts, next.DurationMS = &portable, nil, in.DurationMS
	known, knownText := correctionArchive(old, in, &portable)
	for _, c := range in.Cuts {
		prior, ok := known[c.ID]
		if !ok || c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, nil, ErrInvalid
		}
		if c.Focal != nil {
			f := *c.Focal
			if math.IsNaN(f.X) || math.IsNaN(f.Y) || math.IsInf(f.X, 0) || math.IsInf(f.Y, 0) || f.X < 0 || f.X > 1 || f.Y < 0 || f.Y > 1 {
				return EditPlan{}, nil, ErrInvalid
			}
			prior.Focal = f
		}
		volume := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Volume = c.StartMS, c.EndMS, c.TransitionMS, &volume
		// The portable declarations are the only visible content authority.
		prior.Copies, prior.Chips = nil, nil
		next.Cuts = append(next.Cuts, prior)
	}
	if len(next.Cuts) > 0 {
		next.Cuts[0].TransitionMS = 0
	}
	seen := map[string]bool{}
	for _, edit := range in.Elements {
		t, ok := knownText[edit.InstanceID]
		r := t.Resolved
		if !ok || seen[edit.InstanceID] || edit.ElementID != r.Element.ID || edit.CutID != r.CutID || edit.Kind != r.Element.Kind || edit.Role != r.Element.Role || edit.GroupID != r.GroupID || edit.ItemID != r.ItemID {
			return EditPlan{}, nil, ErrInvalid
		}
		seen[edit.InstanceID] = true
		if len([]rune(edit.Text)) > cfg.Composition.CopyChars || len(edit.Rows) > cfg.Composition.Nodes {
			return EditPlan{}, nil, ErrInvalid
		}
		for _, row := range edit.Rows {
			if len([]rune(row.Text)) > cfg.Composition.CopyChars || !slices.Contains([]string{"label", "hook", "body", "caption"}, row.Role) {
				return EditPlan{}, nil, ErrInvalid
			}
		}
		if !slices.Contains(append(slices.Clone(styles), "auto"), edit.Style) || !slices.Contains([]string{"auto", "header", "top", "upper_mid", "lower_mid", "bottom", "center"}, edit.Position) || !slices.Contains([]string{"left", "center", "right"}, edit.Align) || !slices.Contains([]string{"steady", "rapid", ""}, edit.Pace) {
			return EditPlan{}, nil, ErrInvalid
		}
		if !ValidAccent(edit.Accent) || edit.Keyword != "" && !strings.Contains(edit.Text, edit.Keyword) {
			return EditPlan{}, nil, ErrInvalid
		}
		before := correctionText(t)
		// Provenance and warnings are server projections, never client authority.
		edit.Evidence, edit.FallbackReason, edit.StaleEvidence = before.Evidence, before.FallbackReason, before.StaleEvidence
		edit.EffectiveStartMS, edit.EffectiveEndMS = before.EffectiveStartMS, before.EffectiveEndMS
		reviewed := edit.EvidenceReviewed
		edit.EvidenceReviewed = false
		// Derived intervals are output only and never make a content edit.
		before.ResolvedStartMS, before.ResolvedEndMS = edit.ResolvedStartMS, edit.ResolvedEndMS
		if !reflect.DeepEqual(before, edit) {
			t.Placement, t.Alternatives = nil, nil
			t.FallbackReason = ""
		}
		t.Resolved.Text, t.Resolved.Rows = edit.Text, slices.Clone(edit.Rows)
		e := &t.Resolved.Element
		e.Style, e.Position, e.Align, e.Basis, e.StartMS, e.EndMS = edit.Style, edit.Position, edit.Align, edit.Basis, edit.StartMS, edit.EndMS
		t.Resolved.AuthoredTiming = edit.Basis != "cut" || edit.StartMS != nil || edit.EndMS != nil
		t.Pace, t.Accent, t.Keyword = edit.Pace, edit.Accent, edit.Keyword
		t.Phrases = slices.Clone(edit.Phrases)
		contentChanged := before.Text != edit.Text || !reflect.DeepEqual(before.Rows, edit.Rows) || !slices.Equal(before.Phrases, edit.Phrases)
		t.StaleEvidence = (t.StaleEvidence || associationAffectsText(t, changed)) && !reviewed && !contentChanged
		if !reflect.DeepEqual(before, edit) {
			t.OwnerEdited = true
		}
		portable.Elements = append(portable.Elements, t)
	}
	resolved, err := ResolvePortableIntervals(next, cfg.Composition)
	if err != nil {
		return EditPlan{}, nil, err
	}
	if resolved.DurationMS != in.DurationMS {
		return EditPlan{}, nil, ErrInvalid
	}
	if err := ValidateEditablePhrases(resolved); err != nil {
		return EditPlan{}, nil, err
	}
	geometry := resolved
	geometry.Hook = ""
	refs := make([]RenderSource, 0, len(sources))
	for _, s := range sources {
		refs = append(refs, s.RenderSource)
	}
	if err := ValidateEditPlan(cfg, geometry, refs); err != nil {
		return EditPlan{}, nil, err
	}
	return resolved, styles, nil
}
