package app

import (
	"context"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

type spokenScriptWriter interface {
	SpokenScript(context.Context, llm.ModelRef, clip.PlanningInput) (clip.SpokenDraft, llm.Usage, error)
}

func (r *generationRun) keepSpoken() error {
	r.recovery.PlanDigest = planRecoveryDigest(r.p)
	r.recovery.FlowReady = true
	return r.s.saveRecovery(r.ctx, r.user, r.project, r.recovery)
}

func (r *generationRun) writeSpoken() error {
	if r.s.speech == nil {
		return clip.ErrCompositionUnavailable
	}
	in := r.planningInput()
	in.Analyses, in.SourceAudio = r.analyses, batchSourceAudio(r.b)
	if r.pricing.SkipNarration {
		var e error
		r.edit, e = clip.DecodeEditPlan(r.recovery.Plan)
		r.storyline = r.recovery.Storyline
		if e != nil {
			return e
		}
		r.edit = r.edit.WithDesign(r.p.Design())
		r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
		if e = r.drawRegions(); e != nil {
			return e
		}
		return r.layout()
	}
	if !r.pricing.SkipFlow {
		writer, ok := r.s.planner.(spokenScriptWriter)
		if !ok {
			return clip.ErrCompositionUnavailable
		}
		r.set("script", 0, 1)
		draft, _, e := writer.SpokenScript(r.correcting("script"), r.pricing.Plan.Ref, in)
		if e != nil && (draft.Version != 1 || len(draft.Narration.Segments) == 0) {
			return e
		}
		draft.Narration.VoiceID, draft.Narration.BindingDigest = r.pricing.Dubbing.Voice.Binding.ID, r.pricing.Dubbing.Voice.Binding.Digest
		r.recovery.Spoken = &draft
		r.storyline = draft.Storyline
		r.recovery.Storyline = draft.Storyline
		if saved := r.keepSpoken(); saved != nil {
			return saved
		}
		if e != nil {
			return e
		}
	}
	draft := r.recovery.Spoken
	if draft == nil {
		return clip.ErrQuoteChanged
	}
	r.storyline = draft.Storyline
	r.set("speech", 0, len(draft.Narration.Segments))
	if e := r.s.speech.AssembleInitial(r.ctx, r.user, r.project, r.job, r.p.Approval.QuoteID, r.current.EditPlanRevision, r.pricing.Dubbing, draft, r.keepSpoken, r.progress); e != nil {
		return e
	}
	// Resolve intro/outro exclusions with the exact regions and reviewed/generated slot words.
	skeleton := clip.EditPlan{Ratio: r.p.Ratio, DurationMS: r.p.TargetDurationMS, Portable: &clip.PortablePlan{Snapshot: in.Composition.Snapshot, Inputs: in.Composition.Inputs}}
	regions := clip.WrittenRegions(r.current, r.drafts())
	skeleton, _, e := clip.ProjectPlanRegions(skeleton, regions, r.p.Design().RegionPresets())
	if e != nil {
		return e
	}
	intro, bodyEnd := clip.CaptionBodyWindow(skeleton)
	outro := r.p.TargetDurationMS - bodyEnd
	duration, e := clip.MeasuredSpokenDraft(draft, intro, outro, r.p.TargetDurationMS)
	if saved := r.keepSpoken(); saved != nil {
		return saved
	}
	if e != nil {
		return e
	}
	in.TargetDurationMS = duration
	in.FollowStoryline = draft.Storyline
	in.MeasuredNarration = &draft.Narration
	in.Policy = r.pricing.Narration
	r.set("flow", 0, 1)
	r.edit, _, e = r.s.planner.Flow(r.correcting("flow"), r.pricing.Narration.Ref, in)
	if e != nil {
		return e
	}
	r.edit = r.edit.WithDesign(r.p.Design())
	r.edit.Narration = &draft.Narration
	r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
	if e = r.drawRegions(); e != nil {
		return e
	}
	start, end := clip.CaptionBodyWindow(r.edit)
	for _, seg := range draft.Narration.Segments {
		if seg.StartMS < start || seg.EndMS > end {
			return &clip.SpokenError{SegmentID: seg.ID, Reason: "spoken_timing_conflict"}
		}
	}
	if r.edit.DurationMS > r.p.TargetDurationMS {
		return &clip.SpokenError{Reason: "spoken_timing_conflict"}
	}
	if e = clip.NarrationReadiness(r.edit); e != nil {
		return e
	}
	clip.InitialSpokenCaptions(&r.edit)
	clip.RestrictGeneratedCaptionStyles(&r.edit, r.p.Design().AllowedCaptionStyles())
	if e = r.keep(true, true); e != nil {
		return e
	}
	return r.layout()
}

func (s *GenerationService) FinishInitialSpeech(ctx context.Context, j job.Job) error {
	if s.speech == nil {
		return nil
	}
	return s.speech.OnTerminal(ctx, j)
}
