package media

import (
	"context"
	"errors"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// Only automatic sentence elements share a cut's reading window. Output text,
// authored offsets and explicit visuals are independent and never rescheduled.
func scheduleDeclaredCaptions(elements []clip.PortableText) ([]clip.PortableText, []clip.CopyFallback) {
	result := slices.Clone(elements)
	groups := map[string][]int{}
	order := []string{}
	for i, text := range result {
		if text.Resolved.Element.Role == "caption" && text.Pace != "rapid" && clip.AutomaticCompositionRepair(text) {
			if _, seen := groups[text.Resolved.CutID]; !seen {
				order = append(order, text.Resolved.CutID)
			}
			groups[text.Resolved.CutID] = append(groups[text.Resolved.CutID], i)
		}
	}
	dropped, fallbacks := map[int]bool{}, []clip.CopyFallback{}
	for _, id := range order {
		indices := groups[id]
		for _, i := range indices[min(2, len(indices)):] {
			dropped[i] = true
			fallbacks = append(fallbacks, clip.CopyFallback{ElementID: result[i].Resolved.Element.ID, CutID: result[i].Resolved.CutID, Reason: "sentence_count"})
		}
		if len(indices) < 2 {
			continue
		}
		a, b := &result[indices[0]], &result[indices[1]]
		start, end := max(a.Resolved.StartMS, b.Resolved.StartMS), min(a.Resolved.EndMS, b.Resolved.EndMS)
		minimum := func(t clip.PortableText) int {
			n := clip.MinExposureMS(t.Resolved.Text)
			for _, alternative := range t.Alternatives {
				if alternative.Text != "" {
					n = min(n, clip.MinExposureMS(alternative.Text))
				}
			}
			return n
		}
		first, second := minimum(*a), minimum(*b)
		if first+second > end-start {
			dropped[indices[1]] = true
			fallbacks = append(fallbacks, clip.CopyFallback{ElementID: b.Resolved.Element.ID, CutID: b.Resolved.CutID, Reason: "readability"})
			continue
		}
		// Allocate excess in proportion to the full sentence's reading need.
		weightA, weightB := clip.MinExposureMS(a.Resolved.Text), clip.MinExposureMS(b.Resolved.Text)
		boundary := start + first + (end-start-first-second)*weightA/(weightA+weightB)
		a.Resolved.StartMS, a.Resolved.EndMS = start, boundary
		b.Resolved.StartMS, b.Resolved.EndMS = boundary, end
	}
	kept := result[:0]
	for i, text := range result {
		if !dropped[i] {
			kept = append(kept, text)
		}
	}
	return kept, fallbacks
}

func (r *Rendering) layoutDeclaredRapid(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, plan clip.EditPlan, text clip.PortableText, styles []string, history design.StyleHistory, placed clip.Manifest, previous string) (declaredVisual, error) {
	candidates := []clip.CopyAlternative{{Text: text.Resolved.Text}}
	if clip.AutomaticCompositionRepair(text) {
		candidates = append(candidates, text.Alternatives...)
	}
	for index, candidate := range candidates {
		phrases, ok := clip.SplitRapid(clip.Caption{Text: candidate.Text, Keyword: text.Keyword}, text.Resolved.StartMS, text.Resolved.EndMS)
		if !ok {
			continue
		}
		parent := text
		parent.Resolved.Text = candidate.Text
		if index > 0 {
			parent.FallbackReason = "shorter_copy"
		}
		root := declaredVisual{text: parent, manifest: declaredManifest(parent)}
		style, position := "", ""
		for _, phrase := range phrases {
			part := parent
			part.Resolved.Text, part.Resolved.StartMS, part.Resolved.EndMS = phrase.Text, phrase.StartMS, phrase.EndMS
			part.Keyword, part.Alternatives = phrase.Keyword, nil
			if style != "" {
				part.Resolved.Element.Style, part.Resolved.Element.Position = style, position
			}
			visual, err := r.layoutDeclaredElement(ctx, ws, canvas, plan, part, styles, history, placed, previous, true)
			if err != nil {
				var problem *composition.Problem
				if !errors.As(err, &problem) {
					return root, err
				}
				ok = false
				break
			}
			style, position = visual.manifest.Style, visual.manifest.Position
			visual.text = parent
			visual.manifest.Text = parent.Resolved.Text
			visual.manifest.AuthoredStyle, visual.manifest.AuthoredPosition = parent.Resolved.Element.Style != "auto", parent.Resolved.Element.Position != "auto"
			root.cues = append(root.cues, visual)
			if len(root.cues) == 1 {
				root.manifest.Region = visual.manifest.Region
			} else {
				root.manifest.Region = unionRegion(root.manifest.Region, visual.manifest.Region)
			}
		}
		if ok {
			root.manifest.Style, root.manifest.Position = style, position
			return root, nil
		}
	}
	if clip.AutomaticCompositionRepair(text) {
		text.Pace = "steady"
		visual, err := r.layoutDeclaredElement(ctx, ws, canvas, plan, text, styles, history, placed, previous, true)
		if err == nil {
			visual.text.FallbackReason, visual.manifest.FallbackReason = "steady_copy", "steady_copy"
		}
		return visual, err
	}
	return declaredVisual{}, elementProblem(text, "rapid_readability")
}
