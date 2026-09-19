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
	// The owner's own placement, size and style for this caption (CDS-82).
	// Unlike the fields above it is not a projection of the frozen document:
	// ② writes it, and an empty value is a caption placed automatically.
	Owner                          OwnerCaption
	StartMS, EndMS                 *int
	Pace, Accent, Keyword          string
	ResolvedStartMS, ResolvedEndMS int
	GroupID, ItemID                string
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
		Owner: t.Owner, Narration: t.Scope == NarrationScope}
	if t.Placement != nil {
		a, b := t.Placement.StartMS, t.Placement.EndMS
		result.EffectiveStartMS, result.EffectiveEndMS = &a, &b
	}
	return result
}

// nativeCorrection carries one correction through its steps: the portable plan
// being rewritten, the cuts the owner submitted, and what the archive already
// knew about the plan's text.
type nativeCorrection struct {
	cfg      RenderConfig
	p        Project
	old      EditPlan
	in       CorrectionPlan
	next     EditPlan
	portable PortablePlan
	// changed is the set of source associations this correction moved; a text
	// grounded on one of them is marked stale unless the owner reviewed it.
	changed   []SourceAssociation
	created   []string
	knownText map[string]PortableText
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
	c := &nativeCorrection{cfg: cfg, p: p, old: old, in: in, next: old}
	if err := c.rewritePortableInputs(); err != nil {
		return EditPlan{}, err
	}
	if err := c.applyCuts(); err != nil {
		return EditPlan{}, err
	}
	if err := c.applyElements(); err != nil {
		return EditPlan{}, err
	}
	return c.validate(sources)
}

// rewritePortableInputs copies the portable plan, applies the owner's source
// associations and retains the observations a geometry change needs.
func (c *nativeCorrection) rewritePortableInputs() error {
	old, in := c.old, c.in
	c.portable = *old.Portable
	c.portable.NativeEditing = c.portable.NativeEditing || c.portable.Snapshot.Legacy
	c.changed = []SourceAssociation{}
	if in.Associations != nil {
		c.portable.Inputs.Associations = slices.Clone(*in.Associations)
		limits := c.cfg.Composition
		if c.portable.Snapshot.Legacy {
			limits = LegacyCompositionLimits(limits)
		}
		doc, problem := composition.ReadStored(c.portable.Snapshot.Body, limits)
		if problem != nil {
			return problem
		}
		if err := ValidateCompositionInputs(doc, c.portable.Inputs, limits, false); err != nil {
			return err
		}
		if err := ValidateSourceAssociations(c.p, c.portable.Inputs.Associations); err != nil {
			return err
		}
		c.changed = changedAssociations(old.Portable.Inputs.Associations, c.portable.Inputs.Associations)
	}
	c.portable.Elements = nil
	c.portable.TargetDurationMS = 0
	geometryChanged := len(old.Cuts) != len(in.Cuts)
	if !geometryChanged {
		for i, cut := range in.Cuts {
			prior := old.Cuts[i]
			if cut.ID != prior.ID || cut.StartMS != prior.StartMS || cut.EndMS != prior.EndMS {
				geometryChanged = true
				break
			}
		}
	}
	if len(c.portable.Observations) == 0 && geometryChanged {
		var err error
		if c.portable.Observations, err = RetainedObservations(c.p); err != nil {
			return err
		}
	}
	c.next.Portable, c.next.Cuts, c.next.DurationMS = &c.portable, nil, in.DurationMS
	return nil
}

// applyCuts admits every created cut and corrects every known one; a cut the
// plan already approved is corrected, never created (CLIP-97).
func (c *nativeCorrection) applyCuts() error {
	old, in := c.old, c.in
	var known map[string]Cut
	known, c.knownText = correctionArchive(old, in, &c.portable)
	bindings := map[string]composition.Cut{}
	for _, cut := range append(slices.Clone(old.Portable.RetiredBindings), old.Portable.Cuts...) {
		bindings[cut.ID] = cut
	}
	c.created = []string{}
	for _, cut := range in.Cuts {
		prior, ok := known[cut.ID]
		if ok == (cut.Creation != nil) {
			return cutRefusal(cut.ID, "cut_identity")
		}
		if !ok {
			origin, exists := known[cut.Creation.OriginID]
			if !exists {
				return cutRefusal(cut.ID, "cut_origin")
			}
			admitted, binding, err := admitOwnerCut(cut, origin, bindings[cut.Creation.OriginID], &c.portable, in.Cuts)
			if err != nil {
				return err
			}
			c.created = append(c.created, cut.ID)
			c.portable.Cuts = append(c.portable.Cuts, binding)
			c.next.Cuts = append(c.next.Cuts, admitted)
			continue
		}
		if cut.SourceID != prior.SourceID || cut.Fingerprint != prior.Fingerprint || cut.VolumePermille < 0 || cut.VolumePermille > 1000 {
			return ErrInvalid
		}
		if cut.Focal != nil {
			f := *cut.Focal
			if math.IsNaN(f.X) || math.IsNaN(f.Y) || math.IsInf(f.X, 0) || math.IsInf(f.Y, 0) || f.X < 0 || f.X > 1 || f.Y < 0 || f.Y > 1 {
				return ErrInvalid
			}
			prior.Focal = f
		}
		volume := float64(cut.VolumePermille) / 1000
		prior.StartMS, prior.EndMS, prior.TransitionMS, prior.Volume = cut.StartMS, cut.EndMS, cut.TransitionMS, &volume
		// One fixed rate per cut; the plan validator below decides whether this
		// source's verified cadence actually admits it (CDS-68).
		prior.PlaybackRatePermille = cut.Rate()
		// The portable declarations are the only visible content authority.
		prior.Copies, prior.Chips = nil, nil
		c.next.Cuts = append(c.next.Cuts, prior)
	}
	// A created cut carries no text of its own: a split leaves the parent's
	// cut-bound content on the left, where the owner authored it (CDS-64).
	if err := ValidateOwnerCutContent(c.created, in.Elements); err != nil {
		return err
	}
	if len(c.next.Cuts) > 0 {
		c.next.Cuts[0].TransitionMS = 0
	}
	c.next.SourceAudio = ReconcileSourceAudio(old.SourceAudio, c.next.Cuts)
	trackNoticeCutEdits(old, &c.next)
	return nil
}

// applyElements applies every text edit onto the archived element it names,
// minting the narration captions the owner created (CLIP-134).
func (c *nativeCorrection) applyElements() error {
	seen := map[string]bool{}
	for _, edit := range c.in.Elements {
		if edit.Creation != nil {
			caption, minted, e := mintNarrationCaption(edit, c.knownText)
			if e != nil {
				return e
			}
			c.knownText[minted.InstanceID], edit = caption, minted
		}
		t, ok := c.knownText[edit.InstanceID]
		r := t.Resolved
		if !ok || seen[edit.InstanceID] || edit.ElementID != r.Element.ID || edit.CutID != r.CutID || edit.Kind != r.Element.Kind || edit.Role != r.Element.Role || edit.GroupID != r.GroupID || edit.ItemID != r.ItemID || edit.Narration != (t.Scope == NarrationScope) {
			return ErrInvalid
		}
		seen[edit.InstanceID] = true
		if err := c.validateElementEdit(edit); err != nil {
			return err
		}
		// The owner's own placement is admitted and clamped BEFORE the edit is
		// compared with what the plan holds, so a drag that the safe area pulls
		// back to where the caption already stood is not an edit at all.
		// The allowed styles are the PROJECT's own selection (CLIP-142), not the
		// plan's: a stored plan carries the design only as a render input, and
		// a save must be judged against what the project allows today.
		owner, err := ValidateOwnerCaption(edit.Owner, edit.Role, c.next.Ratio, c.p.DesignSelection().AllowedCaptionStyles())
		if err != nil {
			return err
		}
		edit.Owner = owner
		c.portable.Elements = append(c.portable.Elements, applyTextEdit(t, edit, c.changed))
	}
	return nil
}

// validateElementEdit is the shape check on one submitted text edit: bounded
// text and rows, known roles, positions, alignments, paces and accents.
func (c *nativeCorrection) validateElementEdit(edit CorrectionText) error {
	if len([]rune(edit.Text)) > c.cfg.Composition.CopyChars || len(edit.Rows) > c.cfg.Composition.Nodes {
		return ErrInvalid
	}
	for _, row := range edit.Rows {
		if len([]rune(row.Text)) > c.cfg.Composition.CopyChars || !slices.Contains([]string{"", "label", "hook", "body", "caption"}, row.Role) {
			return ErrInvalid
		}
	}
	if !slices.Contains([]string{"auto", "header", "top", "upper_mid", "lower_mid", "bottom", "center"}, edit.Position) || !slices.Contains([]string{"left", "center", "right"}, edit.Align) || !slices.Contains([]string{"steady", "rapid", ""}, edit.Pace) {
		return ErrInvalid
	}
	if !ValidAccent(edit.Accent) || edit.Keyword != "" && !strings.Contains(edit.Text, edit.Keyword) {
		return ErrInvalid
	}
	return nil
}

// applyTextEdit writes one admitted edit onto its archived text. Provenance and
// warnings stay server projections, and an edit that changes nothing leaves the
// placement and the stale-evidence mark untouched.
func applyTextEdit(t PortableText, edit CorrectionText, changed []SourceAssociation) PortableText {
	before := correctionText(t)
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
	t.Pace, t.Accent, t.Keyword, t.Owner = edit.Pace, edit.Accent, edit.Keyword, edit.Owner
	t.Phrases = slices.Clone(edit.Phrases)
	contentChanged := before.Text != edit.Text || !reflect.DeepEqual(before.Rows, edit.Rows) || !slices.Equal(before.Phrases, edit.Phrases)
	t.StaleEvidence = (t.StaleEvidence || associationAffectsText(t, changed)) && !reviewed && !contentChanged
	if !reflect.DeepEqual(before, edit) {
		t.OwnerEdited = true
	}
	return t
}

// validate resolves the corrected plan's intervals and runs the plan
// validators over it; the result is the plan the store saves.
func (c *nativeCorrection) validate(sources []AnalysisSource) (EditPlan, error) {
	resolved, err := ResolvePortableIntervals(c.next, c.cfg.Composition)
	if err != nil {
		return EditPlan{}, err
	}
	if resolved.DurationMS != c.in.DurationMS {
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
	if err := ValidateEditPlan(c.cfg, geometry, refs); err != nil {
		return EditPlan{}, err
	}
	// A legacy plan's existing overlap stays exactly as saved; a correction may
	// neither create a new one nor enlarge it (CLIP-98).
	if err := ValidateSourceRanges(resolved, SourceOverlaps(c.old.Cuts)); err != nil {
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
