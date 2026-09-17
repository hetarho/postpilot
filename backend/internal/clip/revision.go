package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"
	"time"
)

// A revision is a clip job like a generation: approved before it runs, charged
// for the writing calls its target actually needs, cancellable, and settled the
// same way (CLIP-131, CLIP-19). It does no media work at all — it rewrites the
// saved plan and leaves the rendered result exactly where it is, which is what
// makes the result read as stale and 다시 렌더 stay credit-free (CLIP-132).
const revisionPayloadVersion = 1

// RevisionQuoteStore is what a revision needs beyond the shared quote store: a
// quote whose digest binds the saved plan, and a link that consumes it.
// RevisionPlanStore saves what a revision wrote.
type RevisionPlanStore interface {
	SaveRevisedPlan(ctx context.Context, user, project, job string, revision int, raw string) (Project, error)
}

type RevisionQuoteStore interface {
	SaveRevisionQuote(ctx context.Context, quote GenerationQuote, now time.Time) error
	LinkRevisionJob(ctx context.Context, quote GenerationQuote, job string, now time.Time) error
}

type revisionJobPayload struct {
	Version                  int
	ProjectID, Write         string
	Revision                 int
	Request, Target          string
	PlanJSON                 string
	Language                 string
	Composition              *ProjectComposition
	Template                 Recipe
	Answers                  []Answer
	Disclosure, CTA          string
	Instruction              string
	CaptionPace, Accent      string
	IntroPreset, OutroPreset string
	CaptionStyles            []string
	HideDisclosure           bool
	TargetDurationMS         int
	SourceAudio              []SourceAudioSetting
	Approval                 *GenerationApproval
	Batch                    SourceBatch
}

func (p revisionJobPayload) design() ProjectDesign {
	return ProjectDesign{CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles}
}

// RevisionInputDigest binds a quote to the exact plan the owner was looking at
// and the exact words they wrote. A plan saved after the quote — their own edit,
// or another revision — invalidates it rather than rewriting something they
// never approved (CLIP-131).
func RevisionInputDigest(p Project, request, target, write string, pricing GenerationPricing) string {
	input := struct {
		User, Project, Write   string
		Revision               int
		Request, Target        string
		PlanJSON               string
		Pricing                GenerationPricing
		Composition            *ProjectComposition
		Instruction            string
		CaptionPace, Accent    string
		Disclosure, CTA, Ratio string
		HideDisclosure         bool
		Target_                int
		Answers                []Answer
	}{p.UserID, p.ID, write, p.EditPlanRevision, request, target, p.EditPlan, pricing, p.Composition,
		p.Instruction, p.CaptionPace, p.Accent, p.Disclosure, p.CTA, p.Ratio, p.HideDisclosure, p.TargetDurationMS, p.Answers}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// revisionPricing prices exactly the writing calls the target needs: one for a
// narration request, two where the footage is rewritten and the narration
// follows it. No observation is repaid — the recorded ones are the evidence
// this rewrite is bound to (CLIP-93, CLIP-131).
func (s *GenerationService) revisionPricing(ctx context.Context, write, observe string, target string) (GenerationPricing, error) {
	if s.pricing == nil {
		return GenerationPricing{}, ErrPricingUnavailable
	}
	pricing, err := s.freezeWork(ctx, modelRef(observe), modelRef(write), 0, target == RevisionNarration, false, 3)
	if err != nil {
		return GenerationPricing{}, admissionRefusal(modelRef(observe), err)
	}
	pricing.CancellationPolicyVersion = CancellationPolicyVersion
	if !pricing.Valid() || pricing.ObservationCalls != 0 {
		return GenerationPricing{}, ErrPricingUnavailable
	}
	return pricing, nil
}

func (s *GenerationService) revisionInputs(ctx context.Context, user, id, request, target string) (Project, SourceBatch, error) {
	if !ValidRevisionTarget(target) || strings.TrimSpace(request) == "" || !bounded(request, 1, s.projects.limits.InstructionChars) {
		return Project{}, SourceBatch{}, ErrInvalid
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return Project{}, SourceBatch{}, err
	}
	if p.Finalized != nil {
		return Project{}, SourceBatch{}, ErrFinalized
	}
	// A revision rewrites a plan that exists, on a composition this build can
	// execute, against the observations already stored. A frozen legacy snapshot
	// qualifies: the two writing calls read it under its own limits.
	if p.EditPlan == "" || p.EditPlanRevision <= 0 || p.Composition == nil {
		return Project{}, SourceBatch{}, ErrCompositionUnavailable
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return Project{}, SourceBatch{}, err
	}
	plan, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return Project{}, SourceBatch{}, err
	}
	if plan.Portable == nil {
		return Project{}, SourceBatch{}, ErrCompositionUnavailable
	}
	// The batch is the project's own current one: a revision is about the plan,
	// and the batch is here only to renew the originals' retention and to state
	// the owner's sound choice (CLIP-73, CLIP-100).
	batches, err := s.sources.GetSources(ctx, user, id)
	if err != nil {
		return Project{}, SourceBatch{}, err
	}
	for _, candidate := range batches {
		if candidate.State != "ready" || candidate.ProjectID != id {
			continue
		}
		b, err := s.sources.AvailableBatch(ctx, user, candidate.ID, plan)
		if err != nil {
			return Project{}, SourceBatch{}, err
		}
		return p, b, nil
	}
	return Project{}, SourceBatch{}, ErrSourceState
}

// QuoteRevision prices one revision of the plan the owner is looking at.
func (s *GenerationService) QuoteRevision(ctx context.Context, user, id, request, target, observe, write string) (GenerationQuote, error) {
	// One clip job at a time, answered before anything else: a batch held by a
	// running job would otherwise refuse as an unavailable source (CLIP-79).
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return GenerationQuote{}, err
	}
	if active != nil {
		return GenerationQuote{}, ErrBusy
	}
	p, b, err := s.revisionInputs(ctx, user, id, request, target)
	if err != nil {
		return GenerationQuote{}, err
	}
	if err := s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
		return GenerationQuote{}, admissionRefusal(modelRef(observe), err)
	}
	pricing, err := s.revisionPricing(ctx, write, observe, target)
	if err != nil {
		return GenerationQuote{}, err
	}
	store, ok := s.store.(RevisionQuoteStore)
	if !ok || s.cfg.QuoteTTL <= 0 {
		return GenerationQuote{}, ErrPricingUnavailable
	}
	now := s.now()
	expires := now.Add(s.cfg.QuoteTTL)
	if b.ExpiresAt.Before(expires) {
		expires = b.ExpiresAt
	}
	q := GenerationQuote{ID: newID(), UserID: user, ProjectID: id, BatchID: b.ID, InputDigest: RevisionInputDigest(p, request, target, write, pricing), Pricing: pricing, ExpiresAt: expires}
	if err := store.SaveRevisionQuote(ctx, q, now); err != nil {
		return GenerationQuote{}, err
	}
	return q, nil
}

// StartRevision accepts one approved revision and enqueues it.
func (s *GenerationService) StartRevision(ctx context.Context, user, id, request, target, observe, write string, approval QuoteApproval) (string, error) {
	if approval.QuoteID == "" || approval.MaxCredits == nil || *approval.MaxCredits < 0 {
		return "", ErrQuoteRequired
	}
	if approval.CancellationPolicyVersion != CancellationPolicyVersion {
		return "", ErrCancellationPolicy
	}
	store, ok := s.store.(QuoteStore)
	if !ok {
		return "", ErrQuoteRequired
	}
	q, err := store.GetQuote(ctx, user, approval.QuoteID)
	if err != nil {
		return "", err
	}
	if q.ConsumedJobID != "" {
		return q.ConsumedJobID, nil
	}
	if !s.now().Before(q.ExpiresAt) {
		return "", ErrQuoteExpired
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
		!reflect.DeepEqual(q.Pricing, pricing) || q.InputDigest != RevisionInputDigest(p, request, target, write, pricing) {
		return "", ErrQuoteChanged
	}
	// The template's recipe rides the payload the way a generation freezes it:
	// a template edited mid-flight changes nothing in flight.
	recipe := Recipe{}
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
		Approval: &GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing},
	})
	if err != nil {
		return "", err
	}
	// The owner's words and the document they named, verbatim (CLIP-133).
	asked := ProjectRequest{Kind: RevisionRequestKind(target), Body: request, CreatedAt: s.now()}
	return s.enqueue(ctx, GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Revise: true, Quote: &q, Request: &asked}, b.ID, p.EditPlanRevision)
}

// RunRevision is the whole job: check that the plan is still the one that was
// approved, reserve the writing calls, rewrite the targeted document, lay the
// result out and save it. A failure leaves the saved plan and the rendered
// result exactly as they were.
func (s *GenerationService) RunRevision(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	defer func() {
		if err != nil {
			err = &StageFailure{stage, err}
		}
	}()
	var frozen revisionJobPayload
	if strictJSON(string(payload), &frozen) != nil || frozen.Version != revisionPayloadVersion || frozen.ProjectID != project || frozen.Approval == nil {
		return ErrInvalid
	}
	if !ValidRevisionTarget(frozen.Target) || strings.TrimSpace(frozen.Request) == "" {
		return ErrInvalid
	}
	pricing := frozen.Approval.Pricing
	if !pricing.Valid() || pricing.ObservationCalls != 0 || pricing.Plan.Ref != modelRef(frozen.Write) {
		return ErrPricingUnavailable
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	if p.Finalized != nil {
		return ErrFinalized
	}
	// The plan the owner approved a revision of is the plan this rewrites.
	if p.EditPlanRevision != frozen.Revision || p.EditPlan != frozen.PlanJSON {
		return ErrPlanConflict
	}
	current, err := DecodeEditPlan(p.EditPlan)
	if err != nil {
		return err
	}
	if s.planner.Budgets() != (CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Flow: pricing.Plan.CompletionTokens, Narration: pricing.Narration.CompletionTokens}) {
		return ErrQuoteChanged
	}
	analyses, err := RetainedObservations(p)
	if err != nil {
		return err
	}
	if len(analyses) == 0 {
		return ErrInsufficientFootage
	}
	// CLIP-19: nothing is asked of a model before the reservation succeeds.
	ctx, err = s.jobs.ReserveApproved(ctx, user, job, *frozen.Approval, 0)
	if err != nil {
		return err
	}
	in := PlanningInput{Language: frozen.Language, Composition: frozen.Composition, Template: frozen.Template,
		Answers: frozen.Answers, Ratio: p.Ratio, TargetDurationMS: frozen.TargetDurationMS, Analyses: analyses,
		Policy: pricing.Plan, Disclosure: frozen.Disclosure, HideDisclosure: frozen.HideDisclosure,
		CTA: frozen.CTA, Instruction: frozen.Instruction, Design: frozen.design(), SourceAudio: frozen.SourceAudio}
	stage = "flow"
	if frozen.Target == RevisionNarration {
		stage = "narrate"
	}
	if progress != nil {
		progress(stage, 0, 1)
	}
	next, _, err := s.planner.Revise(ctx, pricing.Plan.Ref, RevisionInput{PlanningInput: in, Current: current, Request: frozen.Request, Target: frozen.Target})
	if err != nil {
		return err
	}
	next = next.WithDesign(frozen.design())
	next.SourceAudio = FreezeSourceAudio(frozen.Batch, next.Cuts)
	if layout, ok := s.renderer.(CompositionLayouter); ok {
		refs := make([]RenderSource, 0, len(analyses))
		for _, a := range analyses {
			refs = append(refs, a.Source.RenderSource)
		}
		next, _, err = layout.LayoutComposition(ctx, next, refs)
		if err != nil {
			return err
		}
		next.SourceAudio = FreezeSourceAudio(frozen.Batch, next.Cuts)
	}
	raw, err := EncodeEditPlan(next)
	if err != nil {
		return err
	}
	stage = "save"
	// The save advances the plan revision alone: the rendered revision stays
	// behind, which is exactly the stale result CLIP-132 asks for. It is this
	// job's own save, so this job's being active does not refuse it.
	store, ok := s.store.(RevisionPlanStore)
	if !ok {
		return ErrCompositionUnavailable
	}
	if _, err := store.SaveRevisedPlan(ctx, user, project, job, frozen.Revision, raw); err != nil {
		return err
	}
	return nil
}
