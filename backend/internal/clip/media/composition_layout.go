package media

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

type declaredVisual struct {
	text      clip.PortableText
	manifest  clip.CompositionElement
	copy      clip.Copy
	caption   copyLayout
	card      cardLayout
	furniture furniture
	info      overlay.CopyView
	ground    Luminance
	cues      []declaredVisual
}
type declaredLayout struct {
	plan    clip.EditPlan
	visuals []declaredVisual
}

func (layout declaredLayout) elements() []clip.CompositionElement {
	result := make([]clip.CompositionElement, 0, len(layout.visuals))
	indices := map[string]int{}
	for _, visual := range layout.visuals {
		m := visual.manifest
		if index, found := indices[m.InstanceID]; found {
			result[index].Parts = append(result[index].Parts, m.Parts...)
			result[index].Cues = append(result[index].Cues, clip.CompositionCue{Text: visual.copy.Text, Style: m.Style, Position: m.Position, Region: m.Region, StartMS: m.StartMS, EndMS: m.EndMS})
			result[index].Region = unionRegion(result[index].Region, m.Region)
			continue
		}
		indices[m.InstanceID] = len(result)
		if m.Role == "caption" {
			m.Cues = []clip.CompositionCue{{Text: visual.copy.Text, Style: m.Style, Position: m.Position, Region: m.Region, StartMS: m.StartMS, EndMS: m.EndMS}}
			m.StartMS, m.EndMS = visual.text.Resolved.StartMS, visual.text.Resolved.EndMS
		}
		m.Parts = slices.Clone(m.Parts)
		result = append(result, m)
	}
	return result
}

func unionRegion(a, b clip.Region) clip.Region {
	x, y := min(a.X, b.X), min(a.Y, b.Y)
	return clip.Region{X: x, Y: y, Width: max(a.X+a.Width, b.X+b.Width) - x, Height: max(a.Y+a.Height, b.Y+b.Height) - y}
}

// LayoutComposition is the shared render/preview measurement boundary. It uses
// bundled glyphs and owned declarations without loading any original footage.
func (r *Rendering) LayoutComposition(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (clip.EditPlan, []clip.CompositionElement, error) {
	resolved, err := clip.ResolvePortableIntervals(plan, r.cfg.Composition)
	if err != nil {
		return plan, nil, err
	}
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(resolved), sources); err != nil {
		return plan, nil, err
	}
	var result declaredLayout
	err = r.media.WithWorkspace(ctx, "clip-composition-layout", func(ws clip.MediaWorkspace) error {
		var err error
		result, err = r.layoutComposition(ctx, ws, resolved)
		return err
	})
	if err != nil {
		return plan, nil, err
	}
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(result.plan), sources); err != nil {
		return plan, nil, err
	}
	return result.plan, result.elements(), nil
}

func compositionGeometry(plan clip.EditPlan) clip.EditPlan {
	plan.Hook = ""
	plan.Cuts = slices.Clone(plan.Cuts)
	for i := range plan.Cuts {
		plan.Cuts[i].Copies = nil
		plan.Cuts[i].Chips = nil
	}
	return plan
}

func elementProblem(text clip.PortableText, reason string) error {
	e := text.Resolved.Element
	return &composition.Problem{ElementID: e.ID, Line: e.Span.Line, Reason: reason}
}

// Native composition layout consumes declared text only. Legacy helpers may
// measure or draw a visual role, but never choose its content or time interval.
func (r *Rendering) layoutComposition(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan) (declaredLayout, error) {
	limits := r.cfg.Composition
	if plan.Portable == nil {
		return declaredLayout{}, clip.ErrInvalid
	}
	if plan.Portable.Snapshot.Legacy {
		limits = clip.LegacyCompositionLimits(limits)
	}
	doc, problem := composition.Parse(plan.Portable.Snapshot.Body, limits)
	if problem != nil {
		return declaredLayout{}, problem
	}
	plan, err := clip.ResolvePortableIntervals(plan, limits)
	if err != nil {
		return declaredLayout{}, err
	}
	plan, err = clip.ExtendCompositionReadingWindows(plan, limits)
	if err != nil {
		return declaredLayout{}, err
	}
	plan.Styles = slices.Clone(doc.Styles)
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return declaredLayout{}, err
	}
	result := declaredLayout{plan: plan}
	portable := *plan.Portable
	// Reading extension is a generation-time choice. Subsequent correction
	// must not undo a deliberate trim by spending the original target again.
	portable.TargetDurationMS = 0
	result.plan.Portable = &portable
	result.plan.Portable.Elements = nil
	history := design.StyleHistory{}
	ordered, timingFallbacks := scheduleDeclaredCaptions(plan.Portable.Elements)
	result.plan.Portable.Fallbacks = append(result.plan.Portable.Fallbacks, timingFallbacks...)
	rank := func(role string) int {
		switch role {
		case "badge":
			return 0
		case "hook", "ending":
			return 1
		case "info":
			return 2
		default:
			return 3
		}
	}
	slices.SortStableFunc(ordered, func(a, b clip.PortableText) int { return rank(a.Resolved.Element.Role) - rank(b.Resolved.Element.Role) })
	byID := map[string]declaredVisual{}
	previous := ""
	headerReady := false
	for _, text := range ordered {
		if text.Resolved.Element.Role == "caption" && !headerReady {
			arrangeDeclaredHeader(plan.Ratio, result.visuals)
			headerReady = true
		}
		if (text.Resolved.StartMS*r.cfg.FPS+999)/1000 >= (text.Resolved.EndMS*r.cfg.FPS+999)/1000 {
			return declaredLayout{}, elementProblem(text, "invalid_interval")
		}
		placed := clip.Manifest{}
		for _, prior := range result.visuals {
			if prior.manifest.Role != "caption" && prior.manifest.StartMS < text.Resolved.EndMS && prior.manifest.EndMS > text.Resolved.StartMS {
				placed = append(placed, prior.manifest.Parts...)
			}
		}
		visual, err := r.layoutDeclaredElement(ctx, ws, canvas, plan, text, doc.Styles, history, placed, previous, false)
		if err != nil {
			var problem *composition.Problem
			if !clip.AutomaticCompositionRepair(text) || !errors.As(err, &problem) {
				return declaredLayout{}, err
			}
			result.plan.Portable.Fallbacks = append(result.plan.Portable.Fallbacks, clip.CopyFallback{ElementID: text.Resolved.Element.ID, CutID: text.Resolved.CutID, Reason: problem.Reason})
			continue
		}
		result.visuals = append(result.visuals, visual)
		if visual.text.FallbackReason != "" && visual.text.FallbackReason != text.FallbackReason {
			result.plan.Portable.Fallbacks = append(result.plan.Portable.Fallbacks, clip.CopyFallback{ElementID: text.Resolved.Element.ID, CutID: text.Resolved.CutID, Reason: visual.text.FallbackReason})
		}
		if text.Resolved.Element.Role == "caption" {
			history = append(history, visual.manifest.Style)
			previous = visual.manifest.Position
		}
	}
	if !headerReady {
		arrangeDeclaredHeader(plan.Ratio, result.visuals)
	}
	for _, visual := range result.visuals {
		byID[visual.manifest.InstanceID] = visual
	}
	// Measurement priority is independent of authored display order.
	result.visuals = nil
	cutStarts := map[string]int{}
	offset := 0
	for _, cut := range plan.Cuts {
		offset -= cut.TransitionMS
		cutStarts[cut.ID] = offset
		offset += cut.EndMS - cut.StartMS
	}
	for _, text := range plan.Portable.Elements {
		if visual, exists := byID[text.Resolved.InstanceID]; exists {
			if visual.text.Resolved.Element.Kind == "ai" {
				visual.text.Placement = &clip.CompositionPlacement{Style: visual.manifest.Style, Position: visual.manifest.Position, StartMS: visual.text.Resolved.StartMS - cutStarts[visual.text.Resolved.CutID], EndMS: visual.text.Resolved.EndMS - cutStarts[visual.text.Resolved.CutID]}
				for i := range visual.cues {
					visual.cues[i].text = visual.text
				}
			}
			if len(visual.cues) > 0 {
				result.visuals = append(result.visuals, visual.cues...)
			} else {
				result.visuals = append(result.visuals, visual)
			}
			result.plan.Portable.Elements = append(result.plan.Portable.Elements, visual.text)
		}
	}
	for i := range result.visuals {
		a := &result.visuals[i].manifest
		for j := i + 1; j < len(result.visuals); j++ {
			b := &result.visuals[j].manifest
			if a.StartMS >= b.EndMS || b.StartMS >= a.EndMS || a.Region.X >= b.Region.X+b.Region.Width || b.Region.X >= a.Region.X+a.Region.Width || a.Region.Y >= b.Region.Y+b.Region.Height || b.Region.Y >= a.Region.Y+a.Region.Height {
				continue
			}
			if !slices.Contains(a.Advisories, "overlap") {
				a.Advisories = append(a.Advisories, "overlap")
			}
			if !slices.Contains(b.Advisories, "overlap") {
				b.Advisories = append(b.Advisories, "overlap")
			}
		}
	}
	if err := clip.VerifyCompositionManifest(result.plan, result.elements(), r.cfg.Composition); err != nil {
		return declaredLayout{}, err
	}
	return result, nil
}

func declaredManifest(text clip.PortableText) clip.CompositionElement {
	r, e := text.Resolved, text.Resolved.Element
	result := clip.CompositionElement{InstanceID: r.InstanceID, ElementID: e.ID, CutID: r.CutID, Kind: e.Kind, Role: e.Role, Text: r.Text, Rows: slices.Clone(r.Rows), Facts: slices.Clone(r.Facts), Evidence: slices.Clone(text.Evidence), Style: e.Style, Position: e.Position, Align: e.Align, StartMS: r.StartMS, EndMS: r.EndMS, AuthoredStyle: e.Style != "auto", AuthoredPosition: e.Position != "auto", AuthoredTiming: r.AuthoredTiming, FallbackReason: text.FallbackReason, Layer: clip.CompositionLayer(e.Role)}
	motion := elementMotion(result, text.Pace)
	result.InMS, result.OutMS, result.DY = motion.InMS, motion.OutMS, motion.DY
	return result
}

func (r *Rendering) layoutDeclaredElement(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, plan clip.EditPlan, text clip.PortableText, styles []string, history design.StyleHistory, placed clip.Manifest, previous string, phrase bool) (declaredVisual, error) {
	visual := declaredVisual{text: text, manifest: declaredManifest(text)}
	e := text.Resolved.Element
	if e.Role == "caption" && e.Kind == "ai" && text.Pace == "rapid" && !phrase {
		return r.layoutDeclaredRapid(ctx, ws, canvas, plan, text, styles, history, placed, previous)
	}
	if e.Role != "caption" {
		result, err := r.layoutDeclaredRole(ctx, ws, canvas, plan.Ratio, visual)
		var problem *composition.Problem
		if err == nil || !clip.AutomaticCompositionRepair(text) || !errors.As(err, &problem) {
			return result, err
		}
		for _, alternative := range text.Alternatives {
			candidate := visual
			candidate.text.Resolved.Text, candidate.text.Resolved.Rows = alternative.Text, slices.Clone(alternative.Rows)
			candidate.manifest.Text, candidate.manifest.Rows = alternative.Text, slices.Clone(alternative.Rows)
			result, alternativeError := r.layoutDeclaredRole(ctx, ws, canvas, plan.Ratio, candidate)
			if alternativeError == nil {
				result.text.FallbackReason = "shorter_copy"
				result.manifest.FallbackReason = "shorter_copy"
				return result, nil
			}
			if !errors.As(alternativeError, &problem) {
				return result, alternativeError
			}
		}
		return visual, err
	}
	scene := ""
	readable := false
	subject := clip.Region{}
	for _, cut := range plan.Cuts {
		if cut.ID != text.Resolved.CutID {
			continue
		}
		for _, a := range plan.Portable.Observations {
			if a.Source.ID == cut.SourceID {
				scene, readable = clip.CutScene(cut, a)
				subject = clip.CutSubject(canvas, cut, a)
			}
		}
	}
	style := e.Style
	if style == "auto" && text.Placement != nil {
		style = text.Placement.Style
	}
	if style == "auto" {
		keyword := 0
		if text.Keyword != "" {
			keyword = 1
		}
		style = design.SelectStyle(scene, design.Classify(text.Resolved.Text), styles, history, keyword)
	}
	candidates := []string{style}
	if clip.AutomaticCompositionRepair(text) {
		for _, other := range styles {
			if other != style {
				candidates = append(candidates, other)
			}
		}
	}
	texts := []clip.CopyAlternative{{Text: text.Resolved.Text, Rows: text.Resolved.Rows}}
	if clip.AutomaticCompositionRepair(text) {
		texts = append(texts, text.Alternatives...)
	}
	last := "copy_limit"
	for index, candidate := range texts {
		if strings.TrimSpace(candidate.Text) == "" {
			continue
		}
		if text.Pace != "rapid" && text.Resolved.EndMS-text.Resolved.StartMS < clip.MinExposureMS(candidate.Text) {
			last = "readability"
			continue
		}
		for _, style := range candidates {
			rule, known := design.Styles[style]
			if !known {
				return visual, elementProblem(text, "invalid_style")
			}
			anchors := []string{e.Position}
			pinned := e.Position != "auto" || text.Placement != nil
			if text.Placement != nil && e.Position == "auto" {
				anchors = []string{text.Placement.Position}
			}
			if !pinned {
				anchors = []string{rule.Anchor}
				if rule.AnchorAlt != "" && rule.AnchorAlt != rule.Anchor {
					anchors = append(anchors, rule.AnchorAlt)
				}
			}
			fits := []declaredVisual{}
			placements := []design.Candidate{}
			for _, anchor := range anchors {
				if !pinned && readable && anchor != "top" && anchor != "bottom" {
					continue
				}
				copy := clip.Copy{Text: candidate.Text, Style: style, Anchor: anchor, Align: e.Align, Accent: text.Accent, Keyword: text.Keyword, Pace: text.Pace}
				if !strings.Contains(copy.Text, copy.Keyword) {
					copy.Keyword = ""
				}
				layout, err := r.layoutCopy(ctx, ws, canvas, copy)
				if err != nil {
					if errors.Is(err, clip.ErrCopyTooLong) || errors.Is(err, clip.ErrInvalid) {
						last = "copy_limit"
						continue
					}
					return visual, err
				}
				tooLong := false
				for _, line := range layout.Lines {
					tooLong = tooLong || design.Chars(line) > rule.Chars
				}
				if tooLong || text.Pace == "rapid" && len(layout.Lines) != 1 {
					last = "copy_limit"
					continue
				}
				visual.copy, visual.caption = copy, layout
				visual.text.Resolved.Text = candidate.Text
				visual.manifest.Text, visual.manifest.Style, visual.manifest.Position, visual.manifest.Region = candidate.Text, style, anchor, layout.Region
				visual.manifest.Parts = layout.Elements(0, 0, copy, text.Resolved.StartMS, text.Resolved.EndMS)
				if index > 0 {
					visual.text.FallbackReason = "shorter_copy"
					visual.manifest.FallbackReason = "shorter_copy"
				}
				if pinned {
					return visual, nil
				}
				fits = append(fits, visual)
				placements = append(placements, design.Candidate{Anchor: anchor, Align: e.Align, Plate: design.Region(layout.Region), Fits: true})
			}
			chosen := design.SelectAnchor(placements, design.Region(subject), placed, readable, previous)
			if chosen >= 0 {
				return fits[chosen], nil
			}
			if len(fits) > 0 && !clip.AutomaticCompositionRepair(text) {
				fits[0].manifest.Advisories = append(fits[0].manifest.Advisories, "overlap")
				return fits[0], nil
			}
			if len(fits) > 0 {
				last = "automatic_placement"
			}
		}
	}
	return visual, elementProblem(text, last)
}
