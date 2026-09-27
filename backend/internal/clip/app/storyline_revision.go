package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// A storyline request (CLIP-181) is a clip job like a revision: approved before it runs,
// charged for its ONE writing call, cancellable, and settled the same way. It reads the
// observations already stored, rewrites the storyline as the owner asked, and touches neither
// the plan nor the result.
const storylineRevisionPayloadVersion = 1

type storylineRevisionPayload struct {
	Version          int
	ProjectID, Write string
	Request          string
	Current          clip.Storyline
	Language         string
	Composition      *clip.ProjectComposition
	Template         clip.Recipe
	Instruction      string
	Guidelines       clip.VideoGuidelines `json:",omitzero"`
	Ratio            string
	TargetDurationMS int
	Approval         *clip.GenerationApproval
}

// storylineRevisionInputs is what a storyline request is about: the stored storyline, the
// observations it names and the project's current batch, whose originals it keeps retained.
func (s *GenerationService) storylineRevisionInputs(ctx context.Context, user, id, request string) (clip.Project, clip.SourceBatch, error) {
	if strings.TrimSpace(request) == "" || !clip.BoundedText(request, 1, s.projects.limits.InstructionChars) {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrInvalid
	}
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	if p.Finalized != nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrFinalized
	}
	if p.Storyline == nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrStorylineMissing
	}
	if p.Composition == nil {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrCompositionUnavailable
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	analyses, err := clip.RetainedObservations(p)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	if len(analyses) == 0 {
		return clip.Project{}, clip.SourceBatch{}, clip.ErrInsufficientFootage
	}
	batches, err := s.sources.GetSources(ctx, user, id)
	if err != nil {
		return clip.Project{}, clip.SourceBatch{}, err
	}
	for _, b := range batches {
		if b.State == "ready" && b.ProjectID == id && b.Current {
			return p, b, nil
		}
	}
	return clip.Project{}, clip.SourceBatch{}, clip.ErrSourceState
}

// storylineRevisionPricing prices the one writing call a storyline request makes and binds
// the storyline it rewrites and the 영상 지침 it reads (QUOTA-45).
func (s *GenerationService) storylineRevisionPricing(ctx context.Context, observe, write string, p clip.Project, guidelines clip.VideoGuidelines) (clip.GenerationPricing, error) {
	if s.pricing == nil {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	pricing, err := s.freezeWork(ctx, modelRef(observe), modelRef(write), 0, false, true, DefaultQuoteRetries)
	if err != nil {
		return clip.GenerationPricing{}, admissionRefusal(modelRef(observe), err)
	}
	pricing.SkipNarration, pricing.Storyline = false, true
	pricing.CancellationPolicyVersion = clip.CancellationPolicyVersion
	pricing.GuidelinesDigest = guidelines.Digest()
	pricing.StorylineDigest = p.Storyline.Digest()
	if !pricing.Valid() || pricing.ObservationCalls != 0 || pricing.PlanCalls() != 1+pricing.Plan.ResponseRetries {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	return pricing, nil
}

// StorylineRevisionInputDigest binds a quote to the storyline the owner was looking at and the
// exact words they wrote: an edit or another storyline saved after the quote invalidates it.
func StorylineRevisionInputDigest(p clip.Project, request, write string, pricing clip.GenerationPricing) string {
	input := struct {
		User, Project, Write, Request string
		Storyline                     *clip.Storyline
		Pricing                       clip.GenerationPricing
		Composition                   *clip.ProjectComposition
		Instruction, Language         string
		Target                        int
	}{p.UserID, p.ID, write, request, p.Storyline, pricing, p.Composition, p.Instruction, p.Language, p.TargetDurationMS}
	raw, _ := json.Marshal(input)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// QuoteStorylineRevision prices one storyline request (CLIP-181).
func (s *GenerationService) QuoteStorylineRevision(ctx context.Context, user, id, request, observe, write string) (clip.GenerationQuote, error) {
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	if active != nil {
		return clip.GenerationQuote{}, clip.ErrBusy
	}
	p, b, err := s.storylineRevisionInputs(ctx, user, id, request)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	if err := s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
		return clip.GenerationQuote{}, admissionRefusal(modelRef(observe), err)
	}
	guidelines, err := s.videoGuidelines(ctx, p)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	pricing, err := s.storylineRevisionPricing(ctx, observe, write, p, guidelines)
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
	q := clip.GenerationQuote{ID: newID(), UserID: user, ProjectID: id, BatchID: b.ID, InputDigest: StorylineRevisionInputDigest(p, request, write, pricing), Pricing: pricing, ExpiresAt: expires}
	if err := store.SaveRevisionQuote(ctx, q, now); err != nil {
		return clip.GenerationQuote{}, err
	}
	return q, nil
}

// StartStorylineRevision accepts one approved storyline request and enqueues it, recording the
// owner's words as the storyline request they were (CLIP-133).
func (s *GenerationService) StartStorylineRevision(ctx context.Context, user, id, request, observe, write string, approval clip.QuoteApproval) (string, error) {
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
	p, b, err := s.storylineRevisionInputs(ctx, user, id, request)
	if err != nil {
		return "", err
	}
	// Read once: the value the digest check compares is the value the payload freezes.
	guidelines, err := s.videoGuidelines(ctx, p)
	if err != nil {
		return "", err
	}
	pricing, err := s.storylineRevisionPricing(ctx, observe, write, p, guidelines)
	if err != nil {
		return "", err
	}
	if q.ProjectID != id || q.BatchID != b.ID || q.Pricing.MaxCredits != *approval.MaxCredits ||
		!reflect.DeepEqual(q.Pricing, pricing) || q.InputDigest != StorylineRevisionInputDigest(p, request, write, pricing) {
		return "", clip.ErrQuoteChanged
	}
	recipe := clip.Recipe{}
	if t, err := s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID); err == nil {
		recipe = t.Recipe
	}
	payload, err := json.Marshal(storylineRevisionPayload{
		Version: storylineRevisionPayloadVersion, ProjectID: id, Write: write, Request: request, Current: *p.Storyline,
		Language: p.Language, Composition: p.Composition, Template: recipe, Instruction: p.Instruction, Guidelines: guidelines,
		Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS,
		Approval: &clip.GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing},
	})
	if err != nil {
		return "", err
	}
	asked := clip.ProjectRequest{Kind: clip.RequestStoryline, Body: request, CreatedAt: s.now()}
	return s.enqueue(ctx, clip.GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Revise: true, Kind: clip.JobKindReviseStoryline, Quote: &q, Request: &asked}, b.ID, p.EditPlanRevision)
}

// RunStorylineRevision is the whole storyline request job: check the storyline is still the
// one the request was approved against, reserve the one writing call, rewrite the storyline and
// save it. A failure leaves the stored storyline exactly as it was.
func (s *GenerationService) RunStorylineRevision(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	defer func() {
		if err != nil {
			err = &clip.StageFailure{Stage: stage, Cause: err}
		}
	}()
	var frozen storylineRevisionPayload
	if clip.StrictJSON(string(payload), &frozen) != nil || frozen.Version != storylineRevisionPayloadVersion || frozen.ProjectID != project || frozen.Approval == nil || strings.TrimSpace(frozen.Request) == "" {
		return clip.ErrInvalid
	}
	pricing := frozen.Approval.Pricing
	if !pricing.Valid() || !pricing.Storyline || pricing.ObservationCalls != 0 || pricing.Plan.Ref != modelRef(frozen.Write) {
		return clip.ErrPricingUnavailable
	}
	p, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	if p.Finalized != nil {
		return clip.ErrFinalized
	}
	// The storyline the owner approved a request about is the storyline this rewrites.
	if p.Storyline == nil || !reflect.DeepEqual(*p.Storyline, frozen.Current) {
		return clip.ErrPlanConflict
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
	stage = "storyline"
	if progress != nil {
		progress(stage, 0, 1)
	}
	in := clip.PlanningInput{Language: frozen.Language, Composition: frozen.Composition, Template: frozen.Template,
		Ratio: frozen.Ratio, TargetDurationMS: frozen.TargetDurationMS, Analyses: analyses, Policy: pricing.Plan,
		Instruction: frozen.Instruction, Guidelines: frozen.Guidelines}
	current := frozen.Current
	written, _, err := s.planner.Storyline(ctx, pricing.Plan.Ref, clip.StorylineInput{PlanningInput: in, Current: &current, Request: frozen.Request})
	if err != nil {
		return err
	}
	made := make([]string, 0, len(analyses))
	for _, a := range analyses {
		made = append(made, a.Source.ID)
	}
	raw, err := clip.EncodeStoryline(&clip.Storyline{Paragraphs: written.Paragraphs, MadeWithSources: made})
	if err != nil {
		return err
	}
	stage = "save"
	store, ok := s.store.(clip.StorylineStore)
	if !ok {
		return clip.ErrCompositionUnavailable
	}
	_, err = store.SaveStoryline(ctx, user, project, "", raw, s.now())
	return err
}
