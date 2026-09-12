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
	doc, problem := composition.Parse(plan.Portable.Snapshot.Body, limits)
	if problem != nil {
		return problem
	}
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
		if r.Element.Style != "auto" && element.Style != r.Element.Style {
			return fail("invalid_style")
		}
		if r.Element.Position != "auto" && element.Position != r.Element.Position {
			return fail("invalid_position")
		}
		if element.Align != r.Element.Align {
			return fail("invalid_align")
		}
		if element.Role == "caption" && !slices.Contains(doc.Styles, element.Style) {
			return fail("invalid_style")
		}
		if element.Layer != CompositionLayer(element.Role) {
			return fail("invalid_visual")
		}
		if element.Role == "caption" {
			motion := design.CaptionMotion(text.Pace)
			if element.InMS != motion.InMS || element.OutMS != motion.OutMS || element.DY != motion.InDY {
				return fail("motion")
			}
		}
		safe := canvas.Safe
		if element.Role == "badge" && element.Position == "header" {
			layout, _ := design.Layout(plan.Ratio)
			safe.X, safe.Width = layout.Chip.X, layout.Badge.Right-layout.Chip.X
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
				if element.Role == "caption" {
					minimum = design.Styles[element.Style].Role().Min
				}
				if element.Role == "badge" {
					minimum = design.Type["badge"].Size
				}
				if part.FontSize < minimum || element.Role == "badge" && part.FontSize != minimum {
					return fail("text_size")
				}
			}
			if part.Fill != "" && part.Background != "" {
				contrast, valid := design.Contrast(part.Fill, part.Background)
				if !valid || contrast < 4.5 {
					return fail("contrast")
				}
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
	}
	return nil
}

func compositionInside(box, safe Region) bool {
	for _, number := range []float64{box.X, box.Y, box.Width, box.Height} {
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return false
		}
	}
	return box.Width > 0 && box.Height > 0 && box.X >= safe.X && box.Y >= safe.Y && box.X+box.Width <= safe.X+safe.Width && box.Y+box.Height <= safe.Y+safe.Height
}
