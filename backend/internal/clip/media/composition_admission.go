package media

import (
	"context"
	"fmt"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// ValidateAuthoredInput checks content already known before source work. Each
// authored section is bound independently: no hypothetical cut allocation may
// reject a valid unknown AI choice. Actual cut windows are checked after planning.
func (r *Rendering) ValidateAuthoredInput(ctx context.Context, in clip.PlanningInput) error {
	if in.Composition == nil {
		return nil
	}
	limits := r.cfg.Composition
	doc, problem := composition.Parse(in.Composition.Snapshot.Body, limits)
	if problem != nil {
		return problem
	}
	canvas, err := clip.ClipCanvas(in.Ratio)
	if err != nil {
		return err
	}
	return r.media.WithWorkspace(ctx, "clip-authored-admission", func(ws clip.MediaWorkspace) error {
		sections := append([]composition.Section{{}}, doc.Sections...)
		for i, section := range sections {
			items := []composition.Item{{}}
			if section.Repeat != "" && section.Repeat != "scenes" {
				items = in.Composition.Inputs.Items[section.Repeat]
			}
			for j, item := range items {
				d := *doc
				d.Sections = nil
				if i > 0 {
					d.Elements = nil
					d.Sections = []composition.Section{section}
				}
				// A full-length placeholder checks only constraints that are impossible even
				// across the entire requested output. It is not a selected source or plan.
				cut := composition.Cut{ID: fmt.Sprintf("check%d_%d", i, j), SourceID: "admission", SectionID: section.ID, EndMS: in.TargetDurationMS}
				if item.ID != "" {
					cut.GroupID, cut.ItemID = section.Repeat, item.ID
				}
				timeline, problem := composition.Resolve(&d, composition.Inputs{Values: in.Composition.Inputs.Values, Items: in.Composition.Inputs.Items, Cuts: []composition.Cut{cut}}, limits, clip.AttemptCheckpointMaxBytes)
				if problem != nil {
					return problem
				}
				for _, element := range timeline.Elements {
					// The intro and the outro draw the project's slots, whose words
					// are checked as the slots they are (CLIP-147, CDS-77); a
					// template entry of either only seeds them and draws nothing.
					if clip.RegionRole(element.Element.Role) || element.Element.Kind != "fixed" {
						continue
					}
					text := clip.PortableText{Resolved: element, Pace: doc.Pace, Accent: doc.Accent}
					if element.Element.Role != "caption" {
						_, err := r.layoutDeclaredRole(ctx, ws, canvas, in.Ratio, declaredVisual{text: text, manifest: declaredManifest(text)}, in.Design.RegionPresets(), clip.RegionPlacement{}, timeline.Elements)
						if err != nil {
							return err
						}
						continue
					}
					c := clip.Copy{Text: element.Text, Style: in.Design.AllowedCaptionStyles()[0], Align: element.Element.Align, Anchor: element.Element.Position, Accent: doc.Accent}
					// "auto" names no anchor of its own: the renderer walks the style's
					// anchor and then its alternate, so the caption is admitted where
					// either holds it. A template caption declares no position (CLIP-112),
					// so without this every fixed caption was refused before a quote.
					anchors := []string{c.Anchor}
					if c.Anchor == "auto" {
						rule := captionStyle(c.Style).Rule()
						anchors = []string{rule.Anchor}
						if rule.AnchorAlt != "" && rule.AnchorAlt != rule.Anchor {
							anchors = append(anchors, rule.AnchorAlt)
						}
					}
					fits := false
					for _, anchor := range anchors {
						c.Anchor = anchor
						// layoutCopy returns only a layout PlaceCopy accepted at this anchor.
						if _, err := r.layoutCopy(ctx, ws, canvas, c); err == nil {
							fits = true
							break
						}
					}
					if !fits {
						return elementProblem(text, "copy_limit")
					}
				}
			}
		}
		return nil
	})
}
