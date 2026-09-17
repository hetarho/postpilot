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
	// V20 is checked against the presets the PROJECT chose, which is what the
	// layout was given: the frozen document declares the slot text, not which
	// preset holds it (CLIP-139).
	selection := plan.Design().RegionPresets()
	canvas, err := ClipCanvas(plan.Ratio)
	if err != nil {
		return err
	}
	expected := map[string]PortableText{}
	for _, text := range plan.Portable.Elements {
		id := text.Resolved.InstanceID
		if _, exists := expected[id]; exists || id == "" {
			return ErrInvalid
		}
		expected[id] = text
	}
	if len(elements) != len(expected) {
		return ErrInvalid
	}
	seen := map[string]bool{}
	captionWindows := map[string][]CompositionElement{}
	// V18 on the whole timeline: a narration caption belongs to no cut, so its
	// window is checked against every other caption (CDS-43).
	narrationWindows := []CompositionElement{}
	facts := collectedCompositionFacts(plan.Portable.Inputs)
	totalCues := 0
	for _, element := range elements {
		text, exists := expected[element.InstanceID]
		if !exists || seen[element.InstanceID] {
			return ErrInvalid
		}
		seen[element.InstanceID] = true
		r := text.Resolved
		fail := func(reason string) error {
			return &composition.Problem{ElementID: r.Element.ID, Line: r.Element.Span.Line, Reason: reason}
		}
		if element.ElementID != r.Element.ID || element.CutID != r.CutID || element.Kind != r.Element.Kind || element.Role != r.Element.Role || element.Text != r.Text || !slices.Equal(element.Rows, r.Rows) {
			return fail("authored_text_changed")
		}
		if element.StartMS < r.StartMS || element.EndMS > r.EndMS || element.StartMS >= element.EndMS || r.AuthoredTiming && (element.StartMS != r.StartMS || element.EndMS != r.EndMS) {
			return fail("interval_outside")
		}
		if r.Element.Position != "auto" && element.Position != r.Element.Position {
			return fail("invalid_position")
		}
		if element.Align != r.Element.Align && !(element.Role == "info" && element.Align == "center") {
			return fail("invalid_align")
		}
		if element.Layer != CompositionLayer(element.Role) {
			return fail("invalid_visual")
		}
		if element.Role == "caption" {
			// A frozen legacy plan's captions belong to cuts and keep the
			// per-cut reading they were rendered under; a narration caption
			// holds its window against every caption on the timeline.
			if text.Scope == NarrationScope {
				for _, other := range narrationWindows {
					if element.StartMS < other.EndMS && other.StartMS < element.EndMS {
						return fail(NoticeCaptionOverlap)
					}
				}
				narrationWindows = append(narrationWindows, element)
			} else if element.CutID != "" {
				for _, other := range captionWindows[element.CutID] {
					if element.StartMS < other.EndMS && other.StartMS < element.EndMS {
						return fail(NoticeCaptionOverlap)
					}
				}
				captionWindows[element.CutID] = append(captionWindows[element.CutID], element)
			}
			motion := design.CaptionMotion(element.Style, text.Pace)
			if element.InMS != motion.InMS || element.OutMS != motion.OutMS || element.DY != motion.InDY {
				return fail("motion")
			}
		}
		region := element.Role == "hook" || element.Role == "ending"
		if region {
			kind, preset := "intro", selection.Intro
			if element.Role == "ending" {
				kind, preset = "outro", selection.Outro
			}
			rows := []string{}
			for _, row := range r.Rows {
				rows = append(rows, row.Text)
			}
			if len(rows) == 0 {
				rows = []string{r.Text}
			}
			if err := design.VerifyRegion(kind, preset, plan.Ratio, rows, element.Parts); err != nil {
				return fail("preset_mismatch")
			}
			if len(element.Parts) == 0 {
				continue
			}
		}
		safe := canvas.Safe
		if element.Role == "badge" && element.Position == "header" {
			layout, _ := design.Layout(plan.Ratio)
			safe.X, safe.Width = layout.Anchor.Left, layout.Badge.Right-layout.Anchor.Left
		}
		if !compositionInside(element.Region, safe) {
			return fail("safe_area")
		}
		if len(element.Parts) == 0 {
			return fail("missing_visual")
		}
		for _, part := range element.Parts {
			if !slices.Contains(design.ElementKinds, part.Kind) {
				return fail("invalid_visual")
			}
			if part.Kind != "scrim" && !compositionInside(Region(part.Region), safe) {
				return fail("safe_area")
			}
			if part.StartMS < element.StartMS || part.EndMS > element.EndMS || part.EndMS <= part.StartMS || element.Role != "caption" && (part.StartMS != element.StartMS || part.EndMS != element.EndMS) {
				return fail("interval_outside")
			}
			if part.FontSize > 0 {
				minimum := design.MinTypeSize()
				if part.TypeRole != "" {
					role, known := design.Type[part.TypeRole]
					if !known {
						return fail("text_size")
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
					return fail("text_size")
				}
			}
			if !part.ContrastNotice && !design.Legible(design.Manifest{part}) {
				return fail("contrast")
			}
		}
		if element.Role == "caption" {
			totalCues += len(element.Cues)
			if len(element.Cues) == 0 || totalCues > limits.Cues {
				return fail("cue_limit")
			}
			phrases := []string{}
			previous := element.StartMS
			for _, cue := range element.Cues {
				if cue.StartMS < previous || cue.StartMS < element.StartMS || cue.EndMS > element.EndMS || cue.EndMS <= cue.StartMS || cue.Style != element.Style || cue.Position != element.Position || !compositionInside(cue.Region, safe) {
					return fail("interval_outside")
				}
				if text.Pace == "rapid" && (cue.EndMS-cue.StartMS < design.Rapid.MinMS || cue.EndMS-cue.StartMS > design.Rapid.MaxMS || design.Chars(cue.Text) > design.Rapid.MaxChars) {
					return fail("rapid_readability")
				}
				previous = cue.EndMS
				phrases = append(phrases, cue.Text)
			}
			if text.Pace == "rapid" && len(element.Cues) > design.Rapid.MaxPerCut {
				return fail("cue_limit")
			}
			if strings.Join(strings.Fields(strings.Join(phrases, "")), "") != strings.Join(strings.Fields(element.Text), "") {
				return fail("authored_text_changed")
			}
		}
		if element.Role == "caption" && text.Pace != "rapid" && element.EndMS-element.StartMS < MinExposureMS(element.Text) {
			return fail("readability")
		}
		// V16: every interval lies ON the output timeline this plan produces.
		if element.StartMS < 0 || element.EndMS > plan.DurationMS {
			return fail("interval_outside")
		}
		// V11: a collected fact behind every number the narration states, from
		// any item — a caption belongs to none (CLIP-137). Owner-written text is
		// the owner's own claim and is not ground checked (CLIP-122).
		if text.Scope == NarrationScope && !text.OwnerEdited {
			// `instructed` is true here: whether an experiential sentence was
			// asked for was settled when it was written, and the instruction
			// itself is not part of a rendered plan.
			if reason := GroundNarration(element.Text, facts, true); reason != "" {
				return fail(reason)
			}
		}
	}
	return nil
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
