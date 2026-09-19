package clip

import (
	"slices"

	"github.com/postpilot/backend/internal/clip/design"
)

// SequenceCaptionCost is what the sequence-rendered captions of a clip add to
// its render (CDS-81): a static style is rasterised once however long it is on
// screen, while a sequence style draws one layer per output frame, so the two
// are quoted separately rather than as one caption count.
//
// CLIP-145 sets no caption ceiling: this informs approval and refuses nothing.
// Before narration exists, quote the whole target as sequence-rendered whenever
// the selection permits it. Rendering itself costs no credits (CLIP-20).
type SequenceCaptionCost struct {
	// Whether the captions were counted from a plan the project actually holds.
	// Before the first generation, frames and time describe the longest case
	// the selection admits; the eventual caption count remains unknown.
	FromPlan bool
	// The plan's sequence captions (zero before narration), their frames or
	// the whole target's frames, and the measured added render time.
	Captions, Frames, AddedRenderMS int
	// Sequence-rendered styles in the project's own selection (CLIP-142).
	SelectedStyles int
}

// SequenceCostOf counts what a project's CURRENT plan and selection imply. The
// style is the owner's choice, then the narration's, then the selection's
// first entry, as in layout. With no plan, non-overlapping captions can cover
// at most the whole target timeline, regardless of how many styles are selected.
func SequenceCostOf(p Project, plan EditPlan, hasPlan bool, cfg RenderConfig) SequenceCaptionCost {
	allowed := p.DesignSelection().AllowedCaptionStyles()
	out := SequenceCaptionCost{}
	for _, id := range allowed {
		if style, ok := design.LookupCaptionStyle(id); ok && !style.Static() {
			out.SelectedStyles++
		}
	}
	if !hasPlan || plan.Portable == nil {
		if out.SelectedStyles > 0 {
			out.Frames = (p.TargetDurationMS*cfg.FPS + 999) / 1000
			out.AddedRenderMS = out.Frames * cfg.SequenceFrameCostMS
		}
		return out
	}
	out.FromPlan = true
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role != "caption" {
			continue
		}
		id := allowed[0]
		if !plan.Portable.Snapshot.Legacy && text.Resolved.Element.Style != "" && text.Resolved.Element.Style != "auto" {
			id = text.Resolved.Element.Style
		}
		if text.Owner.Style != "" {
			id = text.Owner.Style
		}
		if !slices.Contains(allowed, id) {
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
