package media

import (
	"context"
	"fmt"
	"slices"

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
	if in.Composition.Snapshot.Legacy {
		limits = clip.LegacyCompositionLimits(limits)
	}
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
				// The lines each region entry draws under the project's own
				// presets, so admission checks the layout the render will do
				// (CLIP-147).
				placements := clip.RegionPlacements(timeline.Elements, in.Design.RegionPresets())
				for _, element := range timeline.Elements {
					region := element.Element.Role == "hook" || element.Element.Role == "ending"
					if !region && element.Element.Kind != "fixed" {
						continue
					}
					text := clip.PortableText{Resolved: element, Pace: doc.Pace, Accent: doc.Accent}
					if region {
						text.Resolved.Rows = slices.Clone(element.Rows)
						for i, row := range element.Element.Rows {
							if composition.RowKind(element.Element, row) == "ai" {
								text.Resolved.Rows[i].Text = ""
							}
						}
					}
					if element.Element.Role != "caption" {
						_, err := r.layoutDeclaredRole(ctx, ws, canvas, in.Ratio, declaredVisual{text: text, manifest: declaredManifest(text)}, in.Design.RegionPresets(), placements[element.InstanceID])
						if err != nil {
							return err
						}
						continue
					}
					c := clip.Copy{Text: element.Text, Style: in.Design.AllowedCaptionStyles()[0], Align: element.Element.Align, Anchor: element.Element.Position, Accent: doc.Accent}
					layout, err := r.layoutCopy(ctx, ws, canvas, c)
					if err != nil {
						return elementProblem(text, "copy_limit")
					}
					if c.Anchor != "auto" {
						if _, err := clip.PlaceCopy(canvas, c.Anchor, c.Align, layout.Region.Width, layout.Region.Height); err != nil {
							return elementProblem(text, "copy_limit")
						}
					}
				}
			}
		}
		return nil
	})
}
