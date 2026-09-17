package clip

import (
	"slices"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// ProjectDesign is everything about a clip's look the PROJECT owns (CLIP-139):
// the intro and outro presets, the caption styles this clip may use (CLIP-142),
// and the caption pace and accent that moved here first. A template supplies
// the starting values only (CLIP-14), so all five reach a render from the
// project and never from the frozen template composition.
type ProjectDesign struct {
	CaptionPace, Accent      string
	IntroPreset, OutroPreset string
	CaptionStyles            []string
}

// DesignSelection is what the project currently renders with.
func (p Project) DesignSelection() ProjectDesign {
	return ProjectDesign{CaptionPace: p.CaptionPace, Accent: p.Accent,
		IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: slices.Clone(p.CaptionStyles)}
}

// RegionPresets is what the intro and the outro render in: the selection where
// the project made one, the shared defaults where it did not (CLIP-111). The
// frozen template composition is never consulted for it.
func (d ProjectDesign) RegionPresets() composition.DesignSelection {
	selection := composition.DefaultDesign()
	if d.IntroPreset != "" {
		selection.Intro = d.IntroPreset
	}
	if d.OutroPreset != "" {
		selection.Outro = d.OutroPreset
	}
	return selection
}

// AllowedCaptionStyles is the set a caption may be assigned from: the project's
// selection, or the default style alone where it selected none (CLIP-142).
func (d ProjectDesign) AllowedCaptionStyles() []string { return ResolvedCaptionStyles(d.CaptionStyles) }

func ValidIntroPreset(id string) bool {
	_, ok := design.Region("intro", id)
	return ok
}
func ValidOutroPreset(id string) bool {
	_, ok := design.Region("outro", id)
	return ok
}

// ValidCaptionStyles admits an empty selection — which resolves to the default
// style alone (CLIP-142) — and refuses an unapproved or repeated id.
func ValidCaptionStyles(styles []string) bool {
	for i, id := range styles {
		if _, ok := design.CaptionStyle(id); !ok || slices.Index(styles, id) != i {
			return false
		}
	}
	return true
}

// ResolvedCaptionStyles is the set a caption may actually be assigned from: the
// project's own selection, or the default style alone where it selected none.
func ResolvedCaptionStyles(styles []string) []string {
	if len(styles) == 0 {
		return []string{design.DefaultCaptionStyle}
	}
	return slices.Clone(styles)
}

// TemplateDesign is the starting values a template carries for the project's
// selection (CLIP-14): the root attributes its own body declares, and the
// shared defaults where it declares none — a project made without a template
// included, whose body is no body at all.
func TemplateDesign(t VideoTemplate, limits composition.Limits) composition.DesignSelection {
	if t.CompositionLegacy {
		limits = LegacyCompositionLimits(limits)
	}
	doc, problem := composition.ReadStored(t.CompositionBody, limits)
	if problem != nil {
		return composition.DefaultDesign()
	}
	return doc.Design
}
