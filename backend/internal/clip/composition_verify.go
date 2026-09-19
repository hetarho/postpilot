package clip

import (
	"math"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// VerifyCompositionManifest checks only declared elements. It has no mandatory
// badge, card, fact label, clean-style fallback or first/last-cut prerequisite.
// Layout preserves the draft and returns a stable element failure on rejection.
func VerifyCompositionManifest(plan EditPlan, elements []CompositionElement, limits composition.Limits) error {
	if plan.Portable == nil {
		return ErrInvalid
	}
	if plan.Portable.Snapshot.Legacy {
		limits = LegacyCompositionLimits(limits)
	}
	if _, problem := composition.ReadStored(plan.Portable.Snapshot.Body, limits); problem != nil {
		return problem
	}
	expected, err := expectedManifestTexts(plan, elements)
	if err != nil {
		return err
	}
	v := &manifestVerifier{plan: plan, limits: limits, captionWindows: map[string][]CompositionElement{}, facts: collectedCompositionFacts(plan.Portable.Inputs)}
	// V20 is checked against the presets the PROJECT chose, which is what the
	// layout was given: the frozen document declares the slot text, not which
	// preset holds it (CLIP-139).
	v.selection = plan.Design().RegionPresets()
	// Where each region entry's lines land in that preset: the lines the
	// region's earlier entries took, and the ones this preset cannot draw at
	// all (CLIP-147).
	v.placements = RegionPlacements(ResolvedElements(plan.Portable.Elements), v.selection)
	if v.canvas, err = ClipCanvas(plan.Ratio); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, element := range elements {
		text, exists := expected[element.InstanceID]
		if !exists || seen[element.InstanceID] {
			return ErrInvalid
		}
		seen[element.InstanceID] = true
		if err := v.verify(element, text); err != nil {
			return err
		}
	}
	return nil
}

// expectedManifestTexts is the plan's texts by instance id; the manifest must
// declare exactly those, each once.
func expectedManifestTexts(plan EditPlan, elements []CompositionElement) (map[string]PortableText, error) {
	expected := map[string]PortableText{}
	for _, text := range plan.Portable.Elements {
		id := text.Resolved.InstanceID
		if _, exists := expected[id]; exists || id == "" {
			return nil, ErrInvalid
		}
		expected[id] = text
	}
	if len(elements) != len(expected) {
		return nil, ErrInvalid
	}
	return expected, nil
}

// manifestVerifier holds what every element check reads and the windows the
// caption checks accumulate across the manifest.
type manifestVerifier struct {
	plan       EditPlan
	limits     composition.Limits
	selection  composition.DesignSelection
	placements map[string]RegionPlacement
	canvas     Canvas
	facts      []composition.Fact
	// V18 on the whole timeline: a narration caption belongs to no cut, so its
	// window is checked against every other caption (CDS-43).
	captionWindows   map[string][]CompositionElement
	narrationWindows []CompositionElement
	totalCues        int
}

func (v *manifestVerifier) verify(element CompositionElement, text PortableText) error {
	r := text.Resolved
	fail := func(reason string) error {
		return &composition.Problem{ElementID: r.Element.ID, Line: r.Element.Span.Line, Reason: reason}
	}
	if reason := verifyElementIdentity(element, r); reason != "" {
		return fail(reason)
	}
	if element.Role == "caption" {
		if reason := v.verifyCaptionWindow(element, text); reason != "" {
			return fail(reason)
		}
	}
	if element.Role == "hook" || element.Role == "ending" {
		if reason := v.verifyRegion(element, r); reason != "" {
			return fail(reason)
		}
		if len(element.Parts) == 0 {
			return nil
		}
	}
	safe := v.canvas.Safe
	if element.Role == "badge" && element.Position == "header" {
		layout, _ := design.Layout(v.plan.Ratio)
		safe.X, safe.Width = layout.Anchor.Left, layout.Badge.Right-layout.Anchor.Left
	}
	if reason := verifyElementParts(element, safe); reason != "" {
		return fail(reason)
	}
	if element.Role == "caption" {
		if reason := v.verifyCaptionCues(element, text, safe); reason != "" {
			return fail(reason)
		}
		if text.Pace != "rapid" && element.EndMS-element.StartMS < MinExposureMS(element.Text) {
			return fail("readability")
		}
	}
	// V16: every interval lies ON the output timeline this plan produces.
	if element.StartMS < 0 || element.EndMS > v.plan.DurationMS {
		return fail("interval_outside")
	}
	// V11: a collected fact behind every number the narration states, from
	// any item — a caption belongs to none (CLIP-137). Owner-written text is
	// the owner's own claim and is not ground checked (CLIP-122).
	if text.Scope == NarrationScope && !text.OwnerEdited && !text.Authored {
		// `instructed` is true here: whether an experiential sentence was
		// asked for was settled when it was written, and the instruction
		// itself is not part of a rendered plan.
		if reason := GroundNarration(element.Text, v.facts, true); reason != "" {
			return fail(reason)
		}
	}
	return nil
}

// verifyElementIdentity checks that the rendered element is the plan's text,
// inside its interval, at its position and alignment, on its layer.
func verifyElementIdentity(element CompositionElement, r composition.ResolvedElement) string {
	if element.ElementID != r.Element.ID || element.CutID != r.CutID || element.Kind != r.Element.Kind || element.Role != r.Element.Role || element.Text != r.Text || !slices.Equal(element.Rows, r.Rows) {
		return "authored_text_changed"
	}
	if element.StartMS < r.StartMS || element.EndMS > r.EndMS || element.StartMS >= element.EndMS || r.AuthoredTiming && (element.StartMS != r.StartMS || element.EndMS != r.EndMS) {
		return "interval_outside"
	}
	if r.Element.Position != "auto" && element.Position != r.Element.Position {
		return "invalid_position"
	}
	if element.Align != r.Element.Align && !(element.Role == "info" && element.Align == "center") {
		return "invalid_align"
	}
	if element.Layer != CompositionLayer(element.Role) {
		return "invalid_visual"
	}
	return ""
}

// verifyCaptionWindow rejects overlapping caption windows and a motion that is
// not the style's own. A frozen legacy plan's captions belong to cuts and keep
// the per-cut reading they were rendered under; a narration caption holds its
// window against every caption on the timeline.
func (v *manifestVerifier) verifyCaptionWindow(element CompositionElement, text PortableText) string {
	if text.Scope == NarrationScope {
		for _, other := range v.narrationWindows {
			if element.StartMS < other.EndMS && other.StartMS < element.EndMS {
				return NoticeCaptionOverlap
			}
		}
		v.narrationWindows = append(v.narrationWindows, element)
	} else if element.CutID != "" {
		for _, other := range v.captionWindows[element.CutID] {
			if element.StartMS < other.EndMS && other.StartMS < element.EndMS {
				return NoticeCaptionOverlap
			}
		}
		v.captionWindows[element.CutID] = append(v.captionWindows[element.CutID], element)
	}
	motion := design.CaptionMotion(element.Style, text.Pace)
	if element.InMS != motion.InMS || element.OutMS != motion.OutMS || element.DY != motion.InDY {
		return "motion"
	}
	return ""
}

// verifyRegion checks a hook or ending against the preset the project chose
// and the lines that preset can draw (V20).
func (v *manifestVerifier) verifyRegion(element CompositionElement, r composition.ResolvedElement) string {
	kind := "intro"
	if element.Role == "ending" {
		kind = "outro"
	}
	rows := []string{}
	for _, row := range r.Rows {
		rows = append(rows, row.Text)
	}
	if len(rows) == 0 {
		rows = []string{r.Text}
	}
	placement := v.placements[r.InstanceID]
	rows = rows[:min(len(rows), max(0, placement.Drawn))]
	if err := design.VerifyRegion(kind, RegionPresetID(v.selection, kind), v.plan.Ratio, placement.Offset, placement.Rules, rows, element.Parts); err != nil {
		return "preset_mismatch"
	}
	return ""
}

// verifyElementParts checks the drawn parts: inside the safe area, within the
// element's interval, at a legible type size and contrast.
func verifyElementParts(element CompositionElement, safe Region) string {
	if !compositionInside(element.Region, safe) {
		return "safe_area"
	}
	if len(element.Parts) == 0 {
		return "missing_visual"
	}
	for _, part := range element.Parts {
		if !slices.Contains(design.ElementKinds, part.Kind) {
			return "invalid_visual"
		}
		if part.Kind != "scrim" && !compositionInside(Region(part.Region), safe) {
			return "safe_area"
		}
		if part.StartMS < element.StartMS || part.EndMS > element.EndMS || part.EndMS <= part.StartMS || element.Role != "caption" && (part.StartMS != element.StartMS || part.EndMS != element.EndMS) {
			return "interval_outside"
		}
		if part.FontSize > 0 {
			minimum := design.MinTypeSize()
			if part.TypeRole != "" {
				role, known := design.Type[part.TypeRole]
				if !known {
					return "text_size"
				}
				minimum = role.Min
			}
			if element.Role == "caption" {
				minimum = design.Caption().Role().Min
			}
			if element.Role == "badge" {
				minimum = design.Type["badge"].Size
			}
			if part.FontSize < minimum || element.Role == "badge" && part.FontSize != minimum {
				return "text_size"
			}
		}
		if !part.ContrastNotice && !part.OwnerPlaced && !design.Legible(design.Manifest{part}) {
			return "contrast"
		}
	}
	return ""
}

// verifyCaptionCues checks a caption's cues: ordered inside the element, under
// the cue limit, readable at the rapid pace, and spelling the caption's text.
func (v *manifestVerifier) verifyCaptionCues(element CompositionElement, text PortableText, safe Region) string {
	v.totalCues += len(element.Cues)
	if len(element.Cues) == 0 || v.totalCues > v.limits.Cues {
		return "cue_limit"
	}
	phrases := []string{}
	previous := element.StartMS
	for _, cue := range element.Cues {
		if cue.StartMS < previous || cue.StartMS < element.StartMS || cue.EndMS > element.EndMS || cue.EndMS <= cue.StartMS || cue.Style != element.Style || cue.Position != element.Position || !compositionInside(cue.Region, safe) {
			return "interval_outside"
		}
		if text.Pace == "rapid" && (cue.EndMS-cue.StartMS < design.Rapid.MinMS || cue.EndMS-cue.StartMS > design.Rapid.MaxMS || design.Chars(cue.Text) > design.Rapid.MaxChars) {
			return "rapid_readability"
		}
		previous = cue.EndMS
		phrases = append(phrases, cue.Text)
	}
	if text.Pace == "rapid" && len(element.Cues) > design.Rapid.MaxPerCut {
		return "cue_limit"
	}
	if strings.Join(strings.Fields(strings.Join(phrases, "")), "") != strings.Join(strings.Fields(element.Text), "") {
		return "authored_text_changed"
	}
	return ""
}

// collectedCompositionFacts is every fact the project collected, which is what
// a narration number is checked against.
func collectedCompositionFacts(inputs CompositionInputs) []composition.Fact {
	var out []composition.Fact
	for id, value := range inputs.Values {
		if strings.TrimSpace(value) != "" {
			out = append(out, composition.Fact{FieldID: id, Value: value})
		}
	}
	for group, items := range inputs.Items {
		for _, item := range items {
			for field, value := range item.Values {
				if strings.TrimSpace(value) != "" {
					out = append(out, composition.Fact{FieldID: field, GroupID: group, ItemID: item.ID, Value: value})
				}
			}
		}
	}
	return out
}

func compositionInside(box, safe Region) bool {
	for _, number := range []float64{box.X, box.Y, box.Width, box.Height} {
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return false
		}
	}
	return box.Width > 0 && box.Height > 0 && box.X >= safe.X && box.Y >= safe.Y && box.X+box.Width <= safe.X+safe.Width && box.Y+box.Height <= safe.Y+safe.Height
}
