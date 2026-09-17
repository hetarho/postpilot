package clip

import (
	"context"
	"slices"

	"github.com/postpilot/backend/internal/clip/design"
)

// SequenceCaptionCost is what the sequence-rendered captions of a clip add to
// its render (CDS-81): a static style is rasterised once however long it is on
// screen, while a sequence style draws one layer per output frame, so the two
// are quoted separately rather than as one caption count.
//
// Nothing here refuses anything. How many such captions a project may hold, and
// where that would be checked, is CLIP-145 and still open — this is the surface
// the numbers such a ceiling needs accumulate on (CLIP-19, CLIP-20: rendering
// itself costs no credits).
type SequenceCaptionCost struct {
	// Whether the captions were counted from a plan the project actually holds.
	// Before the first generation there is none, and only the selection is
	// known: how many of the styles the clip may use draw a frame at a time.
	FromPlan bool
	// The plan's captions whose style is sequence-rendered, the output frames
	// they cover, and what those frames add to the render at the measured
	// per-frame cost.
	Captions, Frames, AddedRenderMS int
	// Sequence-rendered styles in the project's own selection (CLIP-142).
	SelectedStyles int
}

// SequenceCostOf counts what a project's CURRENT plan and selection imply. The
// style a caption is drawn in is the owner's where they chose one and the
// project's first allowed style otherwise — the same resolution the layout
// makes, so the number the owner is shown is the number the render produces.
func SequenceCostOf(p Project, plan EditPlan, hasPlan bool, cfg RenderConfig) SequenceCaptionCost {
	allowed := p.DesignSelection().AllowedCaptionStyles()
	out := SequenceCaptionCost{}
	for _, id := range allowed {
		if style, ok := design.LookupCaptionStyle(id); ok && !style.Static() {
			out.SelectedStyles++
		}
	}
	if !hasPlan || plan.Portable == nil {
		return out
	}
	out.FromPlan = true
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role != "caption" {
			continue
		}
		id := text.Owner.Style
		if id == "" || !slices.Contains(allowed, id) {
			id = allowed[0]
		}
		style, ok := design.LookupCaptionStyle(id)
		if !ok || style.Static() {
			continue
		}
		out.Captions++
		out.Frames += sequenceFrames(text, cfg.FPS)
	}
	out.AddedRenderMS = out.Frames * cfg.SequenceFrameCostMS
	return out
}

// sequenceFrames is how many output frames one caption is drawn for: its rapid
// phrases where it has them, each drawn over its own interval (CDS-59), and its
// own interval otherwise. The count is the renderer's — first frame to last.
func sequenceFrames(text PortableText, fps int) int {
	frames := func(startMS, endMS int) int {
		if endMS <= startMS || fps <= 0 {
			return 0
		}
		return (endMS*fps+999)/1000 - startMS*fps/1000
	}
	if len(text.Phrases) > 0 {
		total := 0
		for _, phrase := range text.Phrases {
			total += frames(phrase.StartMS, phrase.EndMS)
		}
		return total
	}
	return frames(text.Resolved.StartMS, text.Resolved.EndMS)
}

// SequenceCost answers what one project's approval surfaces show. A project
// whose stored plan cannot be read is quoted from its selection alone rather
// than refused: nothing here gates an action.
func (s *GenerationService) SequenceCost(ctx context.Context, user, id string) (SequenceCaptionCost, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return SequenceCaptionCost{}, err
	}
	return s.sequenceCostOf(p), nil
}

func (s *GenerationService) sequenceCostOf(p Project) SequenceCaptionCost {
	plan, err := DecodeEditPlan(p.EditPlan)
	return SequenceCostOf(p, plan, err == nil && p.EditPlan != "", s.cfg.Render)
}
