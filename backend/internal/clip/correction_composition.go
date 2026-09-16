package clip

import (
	"maps"
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
	// Whether this caption is narration (CLIP-134). A read projection like the
	// ids beside it: a draft carries it back unchanged, and only `Creation`
	// makes a new one.
	Narration bool
	// Request-only provenance for a caption the plan does not yet contain. It
	// is never stored and never projected back, so one caption is created
	// exactly once — the rule a created cut follows.
	Creation *TextCreation
}

// TextCreation authorizes one caption the plan does not hold. `add` is its only
// kind and narration its only scope: every other caption on the timeline is a
// template declaration, and a client cannot mint a declaration (CLIP-97).
type TextCreation struct{ Kind string }

func correctionText(t PortableText) CorrectionText {
	r, e := t.Resolved, t.Resolved.Element
	result := CorrectionText{InstanceID: r.InstanceID, ElementID: e.ID, CutID: r.CutID, Kind: e.Kind, Role: e.Role,
		Text: r.Text, Rows: slices.Clone(r.Rows), Style: e.Style, Position: e.Position, Align: e.Align, Basis: e.Basis,
		StartMS: e.StartMS, EndMS: e.EndMS, Pace: t.Pace, Accent: t.Accent, Keyword: t.Keyword,
		ResolvedStartMS: r.StartMS, ResolvedEndMS: r.EndMS, GroupID: r.GroupID, ItemID: r.ItemID,
		Phrases: slices.Clone(t.Phrases), StaleEvidence: t.StaleEvidence, Evidence: slices.Clone(t.Evidence), FallbackReason: t.FallbackReason,
		Narration: t.Scope == NarrationScope}
	if t.Placement != nil {
		a, b := t.Placement.StartMS, t.Placement.EndMS
		result.EffectiveStartMS, result.EffectiveEndMS = &a, &b
	}
	return result
}

func applyNativeCorrection(cfg RenderConfig, p Project, old EditPlan, sources []AnalysisSource, in CorrectionPlan) (EditPlan, error) {
	// An older client must never silently replace the portable plan with its
	// legacy caption projection. Reload with the native editing contract.
	if !in.NativeComposition {
		return EditPlan{}, ErrCompositionUnavailable
	}
	if err := matchOwnerAudio(old, in); err != nil {
		return EditPlan{}, err
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
		doc, problem := composition.ReadStored(portable.Snapshot.Body, limits)
		if problem != nil {
			return EditPlan{}, problem
		}
		if err := ValidateCompositionInputs(doc, portable.Inputs, limits, false); err != nil {
			return EditPlan{}, err
		}
		if err := ValidateSourceAssociations(p, portable.Inputs.Associations); err != nil {
			return EditPlan{}, err
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
			return EditPlan{}, err
		}
	}
	next.Portable, next.Cuts, next.DurationMS = &portable, nil, in.DurationMS
	known, knownText := correctionArchive(old, in, &portable)
	bindings := map[string]composition.Cut{}
	for _, c := range append(slices.Clone(old.Portable.RetiredBindings), old.Portable.Cuts...) {
		bindings[c.ID] = c
	}
	created := []string{}
	for _, c := range in.Cuts {
		prior, ok := known[c.ID]
		// An id the server already approved is corrected, never created: only a
		// cut the plan does not contain may carry creation provenance, and only
		// with it (CLIP-97).
		if ok == (c.Creation != nil) {
			return EditPlan{}, cutRefusal(c.ID, "cut_identity")
		}
		if !ok {
			origin, exists := known[c.Creation.OriginID]
			if !exists {
				return EditPlan{}, cutRefusal(c.ID, "cut_origin")
			}
			admitted, binding, err := admitOwnerCut(c, origin, bindings[c.Creation.OriginID], &portable, in.Cuts)
			if err != nil {
				return EditPlan{}, err
			}
			created = append(created, c.ID)
			portable.Cuts = append(portable.Cuts, binding)
			next.Cuts = append(next.Cuts, admitted)
			continue
		}
		if c.SourceID != prior.SourceID || c.Fingerprint != prior.Fingerprint || c.VolumePermille < 0 || c.VolumePermille > 1000 {
			return EditPlan{}, ErrInvalid
		}
		if c.Focal != nil {
			f := *c.Focal
			if math.IsNaN(f.X) || math.IsNaN(f.Y) || math.IsInf(f.X, 0) || math.IsInf(f.Y, 0) || f.X < 0 || f.X > 1 || f.Y < 0 || f.Y > 1 {
				return EditPlan{}, ErrInvalid
			}
			prior.Focal = f
		}
		volume := float64(c.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Volume = c.StartMS, c.EndMS, c.TransitionMS, &volume
		// One fixed rate per cut; the plan validator below decides whether this
		// source's verified cadence actually admits it (CDS-68).
		prior.PlaybackRatePermille = c.Rate()
		// The portable declarations are the only visible content authority.
		prior.Copies, prior.Chips = nil, nil
		next.Cuts = append(next.Cuts, prior)
	}
	// A created cut carries no text of its own: a split leaves the parent's
	// cut-bound content on the left, where the owner authored it (CDS-64).
	if err := ValidateOwnerCutContent(created, in.Elements); err != nil {
		return EditPlan{}, err
	}
	if len(next.Cuts) > 0 {
		next.Cuts[0].TransitionMS = 0
	}
	next.SourceAudio = ReconcileSourceAudio(old.SourceAudio, next.Cuts)
	trackNoticeCutEdits(old, &next)
	seen := map[string]bool{}
	for _, edit := range in.Elements {
		if edit.Creation != nil {
			caption, minted, e := mintNarrationCaption(edit, knownText)
			if e != nil {
				return EditPlan{}, e
			}
			knownText[minted.InstanceID], edit = caption, minted
		}
		t, ok := knownText[edit.InstanceID]
		r := t.Resolved
		if !ok || seen[edit.InstanceID] || edit.ElementID != r.Element.ID || edit.CutID != r.CutID || edit.Kind != r.Element.Kind || edit.Role != r.Element.Role || edit.GroupID != r.GroupID || edit.ItemID != r.ItemID || edit.Narration != (t.Scope == NarrationScope) {
			return EditPlan{}, ErrInvalid
		}
		seen[edit.InstanceID] = true
		if len([]rune(edit.Text)) > cfg.Composition.CopyChars || len(edit.Rows) > cfg.Composition.Nodes {
			return EditPlan{}, ErrInvalid
		}
		for _, row := range edit.Rows {
			if len([]rune(row.Text)) > cfg.Composition.CopyChars || !slices.Contains([]string{"", "label", "hook", "body", "caption"}, row.Role) {
				return EditPlan{}, ErrInvalid
			}
		}
		if !slices.Contains([]string{"auto", "header", "top", "upper_mid", "lower_mid", "bottom", "center"}, edit.Position) || !slices.Contains([]string{"left", "center", "right"}, edit.Align) || !slices.Contains([]string{"steady", "rapid", ""}, edit.Pace) {
			return EditPlan{}, ErrInvalid
		}
		if !ValidAccent(edit.Accent) || edit.Keyword != "" && !strings.Contains(edit.Text, edit.Keyword) {
			return EditPlan{}, ErrInvalid
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
		return EditPlan{}, err
	}
	if resolved.DurationMS != in.DurationMS {
		return EditPlan{}, ErrInvalid
	}
	if err := ValidateEditablePhrases(resolved); err != nil {
		return EditPlan{}, err
	}
	geometry := resolved
	geometry.Hook = ""
	geometry.SourceAudio = nil
	refs := make([]RenderSource, 0, len(sources))
	for _, s := range sources {
		refs = append(refs, s.RenderSource)
	}
	if err := ValidateEditPlan(cfg, geometry, refs); err != nil {
		return EditPlan{}, err
	}
	// A legacy plan's existing overlap stays exactly as saved; a correction may
	// neither create a new one nor enlarge it (CLIP-98).
	if err := ValidateSourceRanges(resolved, SourceOverlaps(old.Cuts)); err != nil {
		return EditPlan{}, err
	}
	return resolved, nil
}

// mintNarrationCaption admits one caption the plan does not yet contain and
// returns the edit the ordinary correction path then applies to it. Only the
// sentence, its interval and its phrasing come from the owner: the identity,
// the scope, the automatic placement and the absence of a cut are the server's,
// exactly as they are for a caption the narration call wrote (CLIP-134).
func mintNarrationCaption(edit CorrectionText, known map[string]PortableText) (PortableText, CorrectionText, error) {
	// An id the plan already knows is corrected, never created, and a caption
	// bound to a cut or to an item is a template declaration rather than
	// narration — neither is something a client may mint (CLIP-97).
	_, exists := known[edit.InstanceID]
	if exists || edit.Creation.Kind != CutAdd || !edit.Narration || edit.CutID != "" || edit.GroupID != "" || edit.ItemID != "" || edit.StartMS == nil || edit.EndMS == nil {
		return PortableText{}, CorrectionText{}, ErrInvalid
	}
	caption := NarrationCaption(NextNarrationID(slices.Collect(maps.Keys(known))), "", *edit.StartMS, *edit.EndMS)
	// The owner wrote this sentence, so it is theirs before it is applied:
	// nothing grounds an owner caption and no automatic repair rewrites it.
	caption.OwnerEdited = true
	out := correctionText(caption)
	out.Text, out.Phrases, out.Pace, out.Accent, out.Keyword = edit.Text, slices.Clone(edit.Phrases), edit.Pace, edit.Accent, edit.Keyword
	return caption, out, nil
}
