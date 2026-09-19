package app

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// A revision is a clip job like a generation: approved before it runs, charged
// for the writing calls its target actually needs, cancellable, and settled the
// same way (CLIP-131, CLIP-19). It does no media work at all — it rewrites the
// saved plan and leaves the rendered result exactly where it is, which is what
// makes the result read as stale and 다시 렌더 stay credit-free (CLIP-132).
const revisionPayloadVersion = 1

type revisionJobPayload struct {
	Version                  int
	ProjectID, Write         string
	Revision                 int
	Request, Target          string
	PlanJSON                 string
	Language                 string
	Composition              *clip.ProjectComposition
	Template                 clip.Recipe
	Answers                  []clip.Answer
	Disclosure, CTA          string
	Instruction              string
	CaptionPace, Accent      string
	IntroPreset, OutroPreset string
	CaptionStyles            []string
	HideDisclosure           bool
	TargetDurationMS         int
	SourceAudio              []clip.SourceAudioSetting
	Approval                 *clip.GenerationApproval
	Batch                    clip.SourceBatch
}

func (p revisionJobPayload) Design() clip.ProjectDesign {
	return clip.ProjectDesign{CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles}
}

// revisionPricing prices exactly the writing calls the target needs: one for a
// narration request, two where the footage is rewritten and the narration
// follows it. No observation is repaid — the recorded ones are the evidence
// this rewrite is bound to (CLIP-93, CLIP-131).
func (s *GenerationService) revisionPricing(ctx context.Context, write, observe string, target string) (clip.GenerationPricing, error) {
	if s.pricing == nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	pricing, err := s.freezeWork(ctx, modelRef(observe), modelRef(write), 0, target == clip.RevisionNarration, false, 3)
	if err != nil {
		return clip.GenerationPricing{}, admissionRefusal(modelRef(observe), err)
	}
	pricing.CancellationPolicyVersion = clip.CancellationPolicyVersion
	if !pricing.Valid() || pricing.ObservationCalls != 0 {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	return pricing, nil
}

func (s *GenerationService) revisionInputs(ctx context.Context, user, id, request, target string) (clip.Project, clip.SourceBatch, error) {
	if !clip.ValidRevisionTarget(target) || strings.TrimSpace(request) == "" || !clip.BoundedText(request, 1, s.projects.limits.InstructionChars) {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrInvalid
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	if p.Finalized != nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrFinalized
	}
	// A revision rewrites a plan that exists, on a composition this build can
	// execute, against the observations already stored. A frozen legacy snapshot
	// qualifies: the two writing calls read it under its own limits.
	if p.EditPlan == "" || p.EditPlanRevision <= 0 || p.Composition == nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrCompositionUnavailable
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	if plan.Portable == nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrCompositionUnavailable
	}
	// The batch is the project's own current one: a revision is about the plan,
	// and the batch is here only to renew the originals' retention and to state
	// the owner's sound choice (CLIP-73, CLIP-100).
	batches, err := s.sources.GetSources(ctx, user, id)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	for _, candidate := range batches {
		if candidate.State != "ready" || candidate.ProjectID != id {
			continue
		}
		b, err := s.sources.AvailableBatch(ctx, user, candidate.ID, plan)
		if err != nil {
			return clip.Project{}, clip.SourceBatch{}, err
		}
		return p, b, nil
	}
	return clip.Project{}, clip.SourceBatch{}, clip.ErrSourceState
}

// QuoteRevision prices one revision of the plan the owner is looking at.
func (s *GenerationService) QuoteRevision(ctx context.Context, user, id, request, target, observe, write string) (clip.GenerationQuote, error) {
	// One clip job at a time, answered before anything else: a batch held by a
	// running job would otherwise refuse as an unavailable source (CLIP-79).
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	if active != nil {
		return clip.GenerationQuote{}, clip.ErrBusy
	}
	p, b, err := s.revisionInputs(ctx, user, id, request, target)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	if err := s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
		return clip.GenerationQuote{}, admissionRefusal(modelRef(observe), err)
	}
	pricing, err := s.revisionPricing(ctx, write, observe, target)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	store, ok := s.store.(clip.RevisionQuoteStore)
	if !ok || s.cfg.QuoteTTL <= 0 {
		return clip.GenerationQuote{}, clip.ErrPricingUnavailable
	}
	now := s.now()
	expires := now.Add(s.cfg.QuoteTTL)
	if b.ExpiresAt.Before(expires) {
		expires = b.ExpiresAt
	}
	q := clip.GenerationQuote{ID: newID(), UserID: user, ProjectID: id, BatchID: b.ID, InputDigest: clip.RevisionInputDigest(p, request, target, write, pricing), Pricing: pricing, ExpiresAt: expires}
	if err := store.SaveRevisionQuote(ctx, q, now); err != nil {
		return clip.GenerationQuote{}, err
	}
	return q, nil
}

// StartRevision accepts one approved revision and enqueues it.
func (s *GenerationService) StartRevision(ctx context.Context, user, id, request, target, observe, write string, approval clip.QuoteApproval) (string, error) {
	if approval.QuoteID == "" || approval.MaxCredits == nil || *approval.MaxCredits < 0 {
		return "", clip.ErrQuoteRequired
	}
	if approval.CancellationPolicyVersion != clip.CancellationPolicyVersion {
		return "", clip.ErrCancellationPolicy
	}
	store, ok := s.store.(clip.QuoteStore)
	if !ok {
		return "", clip.ErrQuoteRequired
	}
	q, err := store.GetQuote(ctx, user, approval.QuoteID)
	if err != nil {
		return "", err
	}
	if q.ConsumedJobID != "" {
		return q.ConsumedJobID, nil
	}
	if !s.now().Before(q.ExpiresAt) {
		return "", clip.ErrQuoteExpired
	}
	p, b, err := s.revisionInputs(ctx, user, id, request, target)
	if err != nil {
		return "", err
	}
	pricing, err := s.revisionPricing(ctx, write, observe, target)
	if err != nil {
		return "", err
	}
	if q.ProjectID != id || q.BatchID != b.ID || q.Pricing.MaxCredits != *approval.MaxCredits ||
		!reflect.DeepEqual(q.Pricing, pricing) || q.InputDigest != clip.RevisionInputDigest(p, request, target, write, pricing) {
		return "", clip.ErrQuoteChanged
	}
	// The template's recipe rides the payload the way a generation freezes it:
	// a template edited mid-flight changes nothing in flight.
	recipe := clip.Recipe{}
	if t, err := s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID); err == nil {
		recipe = t.Recipe
	}
	payload, err := json.Marshal(revisionJobPayload{
		Version: revisionPayloadVersion, ProjectID: id, Write: write, Revision: p.EditPlanRevision,
		Request: request, Target: target, PlanJSON: p.EditPlan, Language: p.Language,
		Composition: p.Composition, Template: recipe, Answers: p.Answers,
		Disclosure: p.Disclosure, CTA: p.CTA, Instruction: p.Instruction,
		CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles, HideDisclosure: p.HideDisclosure,
		TargetDurationMS: p.TargetDurationMS, SourceAudio: batchSourceAudio(b), Batch: b,
		Approval: &clip.GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing},
	})
	if err != nil {
		return "", err
	}
	// The owner's words and the document they named, verbatim (CLIP-133).
	asked := clip.ProjectRequest{Kind: clip.RevisionRequestKind(target), Body: request, CreatedAt: s.now()}
	return s.enqueue(ctx, clip.GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Revise: true, Quote: &q, Request: &asked}, b.ID, p.EditPlanRevision)
}

// RunRevision is the whole job: check that the plan is still the one that was
// approved, reserve the writing calls, rewrite the targeted document, lay the
// result out and save it. A failure leaves the saved plan and the rendered
// result exactly as they were.
func (s *GenerationService) RunRevision(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	defer func() {
		if err != nil {
			err = &clip.StageFailure{Stage: stage, Cause: err}
		}
	}()
	var frozen revisionJobPayload
	if clip.StrictJSON(string(payload), &frozen) != nil || frozen.Version != revisionPayloadVersion || frozen.ProjectID != project || frozen.Approval == nil {
		return clip.ErrInvalid
	}
	if !clip.ValidRevisionTarget(frozen.Target) || strings.TrimSpace(frozen.Request) == "" {
		return clip.ErrInvalid
	}
	pricing := frozen.Approval.Pricing
	if !pricing.Valid() || pricing.ObservationCalls != 0 || pricing.Plan.Ref != modelRef(frozen.Write) {
		return clip.ErrPricingUnavailable
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	// The plan the owner approved a revision of is the plan this rewrites.
	if p.EditPlanRevision != frozen.Revision || p.EditPlan != frozen.PlanJSON {
		return clip.ErrPlanConflict
	}
	current, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		return err
	}
	if s.planner.Budgets() != (clip.CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Flow: pricing.Plan.CompletionTokens, Narration: pricing.Narration.CompletionTokens}) {
		return clip.ErrQuoteChanged
	}
	analyses, err := clip.RetainedObservations(p)
	if err != nil {
		return err
	}
	if len(analyses) == 0 {
		return clip.ErrInsufficientFootage
	}
	// CLIP-19: nothing is asked of a model before the reservation succeeds.
	ctx, err = s.jobs.ReserveApproved(ctx, user, job, *frozen.Approval, 0)
	if err != nil {
		return err
	}
	in := clip.PlanningInput{Language: frozen.Language, Composition: frozen.Composition, Template: frozen.Template,
		Answers: frozen.Answers, Ratio: p.Ratio, TargetDurationMS: frozen.TargetDurationMS, Analyses: analyses,
		Policy: pricing.Plan, Disclosure: frozen.Disclosure, HideDisclosure: frozen.HideDisclosure,
		CTA: frozen.CTA, Instruction: frozen.Instruction, Design: frozen.Design(), SourceAudio: frozen.SourceAudio}
	stage = "flow"
	if frozen.Target == clip.RevisionNarration {
		stage = "narrate"
	}
	if progress != nil {
		progress(stage, 0, 1)
	}
	next, _, err := s.planner.Revise(ctx, pricing.Plan.Ref, clip.RevisionInput{PlanningInput: in, Current: current, Request: frozen.Request, Target: frozen.Target})
	if err != nil {
		return err
	}
	next = next.WithDesign(frozen.Design())
	next.SourceAudio = clip.FreezeSourceAudio(frozen.Batch, next.Cuts)
	if layout, ok := s.renderer.(clip.CompositionLayouter); ok {
		refs := make([]clip.RenderSource, 0, len(analyses))
		for _, a := range analyses {
			refs = append(refs, a.Source.RenderSource)
		}
		next, _, err = layout.LayoutComposition(ctx, next, refs)
		if err != nil {
			return err
		}
		next.SourceAudio = clip.FreezeSourceAudio(frozen.Batch, next.Cuts)
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		return err
	}
	stage = "save"
	// The save advances the plan revision alone: the rendered revision stays
	// behind, which is exactly the stale result CLIP-132 asks for. It is this
	// job's own save, so this job's being active does not refuse it.
	store, ok := s.store.(clip.RevisionPlanStore)
	if !ok {
		return clip.ErrCompositionUnavailable
	}
	if _, err := store.SaveRevisedPlan(ctx, user, project, job, frozen.Revision, raw); err != nil {
		return err
	}
	return nil
}
