package app

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type remainingQuotePricing interface {
	FreezeWork(context.Context, llm.ModelRef, llm.ModelRef, int, bool, bool, int) (clip.GenerationPricing, error)
}

func (s *GenerationService) freezeWork(ctx context.Context, o, w llm.ModelRef, n int, skipFlow, skipNarration bool, retries int) (clip.GenerationPricing, error) {
	if p, ok := s.pricing.(remainingQuotePricing); ok {
		return p.FreezeWork(ctx, o, w, n, skipFlow, skipNarration, retries)
	}
	if skipFlow || skipNarration {
		return clip.GenerationPricing{}, clip.ErrPricingUnavailable
	}
	return s.pricing.Freeze(ctx, o, w, n)
}

// quoteMode is the work a clip quote prices: 바로 만들기's flow and narration, 이 스토리로
// 만들기's (the same two calls along the stored storyline), or the storyline call alone.
type quoteMode int

const (
	quoteGenerate quoteMode = iota
	quoteFromStoryline
	quoteStoryline
)

func (m quoteMode) kind() string {
	if m == quoteStoryline {
		return clip.JobKindStoryline
	}
	return ""
}

// refreeze prices the approved work again on the current documents, in the shape it was
// approved in: the storyline call is ONE writing call with no narration after it (CLIP-177).
func (s *GenerationService) refreeze(ctx context.Context, pricing clip.GenerationPricing, count int) (clip.GenerationPricing, error) {
	if pricing.Storyline {
		current, err := s.freezeWork(ctx, pricing.Observe.Ref, pricing.Plan.Ref, count, false, true, pricing.Plan.ResponseRetries)
		current.SkipNarration, current.Storyline = false, true
		return current, err
	}
	return s.freezeWork(ctx, pricing.Observe.Ref, pricing.Plan.Ref, count, pricing.SkipFlow, pricing.SkipNarration, pricing.Plan.ResponseRetries)
}

func (s *GenerationService) quoteInputs(ctx context.Context, user, id, batch, observe, write string, mode quoteMode) (clip.Project, clip.VideoTemplate, clip.SourceBatch, clip.GenerationPricing, clip.VideoGuidelines, error) {
	var p clip.Project
	var t clip.VideoTemplate
	var b clip.SourceBatch
	var pricing clip.GenerationPricing
	var guidelines clip.VideoGuidelines
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if p.Finalized != nil {
		return p, t, b, pricing, guidelines, clip.ErrFinalized
	}
	// Building from the storyline needs one (CLIP-178).
	if mode == quoteFromStoryline && p.Storyline == nil {
		return p, t, b, pricing, guidelines, clip.ErrStorylineMissing
	}
	// A project generates with no template (CLIP-5) and with one it has since
	// lost (CLIP-25): both leave the zero recipe standing and no declared
	// structure to satisfy.
	if p.VideoTemplateID != "" {
		if t, err = s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID); err != nil {
			return p, t, b, pricing, guidelines, err
		}
	}
	if err = clip.RequiredAnswers(t, p, s.projects.limits.Composition); err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if !s.projects.duration(p.TargetDurationMS) {
		return p, t, b, pricing, guidelines, clip.ErrTargetDurationRequired
	}
	c, err := clip.GenerationComposition(t, p, s.projects.limits.Composition)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if err = s.checkComposition(c); err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if validator, ok := s.renderer.(interface {
		ValidateAuthoredInput(context.Context, clip.PlanningInput) error
	}); ok {
		if err := validator.ValidateAuthoredInput(ctx, clip.PlanningInput{Composition: c, Template: t.Recipe, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Design: p.DesignSelection()}); err != nil {
			return p, t, b, pricing, guidelines, err
		}
	}
	// The intro/outro words the owner or an answer supplies are drawn as written, so words
	// their slot cannot draw are refused by that slot before anything is paid (CDS-77).
	if err := clip.ValidateAuthoredRegions(clip.EffectiveProjectRegions(p), p.DesignSelection().RegionPresets(), p.Ratio); err != nil {
		return p, t, b, pricing, guidelines, err
	}
	b, err = s.sources.AvailableBatch(ctx, user, batch)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if !clip.ValidQuoteBatch(b, user, id, s.now()) {
		return p, t, b, pricing, guidelines, clip.ErrSourceState
	}
	if err := clip.MatchCompositionSources(c, b); err != nil {
		return p, t, b, pricing, guidelines, err
	}
	count, err := clip.ConservativeObservationCount(s.cfg.Media, b)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	if s.pricing == nil || s.cfg.QuoteTTL <= 0 {
		return p, t, b, pricing, guidelines, clip.ErrPricingUnavailable
	}
	rawRecovery, err := s.loadRecovery(ctx, user, id)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	upgraded, err := s.upgradeRecovery(ctx, user, id, rawRecovery)
	if err != nil {
		return p, t, b, pricing, guidelines, err
	}
	recovered := s.selectRecovery(upgraded, b, modelRef(observe), p.Language)
	// The 영상 지침 are writer input like the instruction (CLIP-69, GUIDE-15): read once here, so the
	// value the approval binds is the value the start freezes.
	if guidelines, err = s.videoGuidelines(ctx, p); err != nil {
		return p, t, b, pricing, guidelines, err
	}
	regions := clip.EffectiveProjectRegions(p)
	seed := clip.GenerationPayload{Language: p.Language, Batch: b, Composition: c, Template: t.Recipe, Ratio: p.Ratio, Write: write, TargetDurationMS: p.TargetDurationMS, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, Instruction: p.Instruction, Guidelines: guidelines, Regions: &regions, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset}
	if mode == quoteFromStoryline {
		seed.FollowStoryline = p.Storyline
	}
	// A resume reuses exactly what the recovery holds: a complete plan answers
	// both writing calls, a written flow answers only the first. A recovery
	// saved before the flow was its own call carries a complete plan, so it
	// still resumes at rendering.
	// The storyline call writes no plan, so no kept plan answers it; the analysis it
	// still needs is counted below either way (CLIP-93).
	written := mode != quoteStoryline && recovered.Plan != "" && recovered.PlanDigest == planRecoveryDigest(seed)
	skipFlow := written && (recovered.FlowReady || recovered.PlanReady)
	skipNarration := skipFlow && recovered.PlanReady
	upperCount := count
	// Verified source durations supersede conservative browser estimates only for
	// the exact retained source identity; count only missing chunks.
	count = 0
	for _, v := range b.Sources {
		one := b
		one.Sources = []clip.SourceLease{v}
		n, e := clip.ConservativeObservationCount(s.cfg.Media, one)
		if e != nil {
			return p, t, b, pricing, guidelines, e
		}
		if a, ok := recoverySource(recovered, v.ID); ok {
			n = (a.Info.DurationMS + 59999) / 60000
		}
		for _, ch := range recovered.Chunks {
			if ch.SourceID == v.ID {
				n--
			}
		}
		count += max(0, n)
	}
	count = min(count, max(0, upperCount-len(recovered.Chunks)))
	if skipFlow && count != 0 {
		skipFlow, skipNarration = false, false
	}
	if skipNarration && recovered.Pricing.Valid() {
		pricing = recovered.Pricing
		pricing.SkipFlow, pricing.SkipNarration, pricing.ObservationCalls, pricing.MaxCredits = true, true, 0, 0
	} else {
		if err = s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
			return p, t, b, pricing, guidelines, admissionRefusal(modelRef(observe), err)
		}
		if mode == quoteStoryline {
			// The analysis still missing and ONE writing call (CLIP-177): priced as a flow
			// with no narration after it, then marked as the storyline call it is.
			pricing, err = s.freezeWork(ctx, modelRef(observe), modelRef(write), count, false, true, DefaultQuoteRetries)
			pricing.SkipNarration, pricing.Storyline = false, true
		} else {
			pricing, err = s.freezeWork(ctx, modelRef(observe), modelRef(write), count, skipFlow, skipNarration, 3)
		}
	}
	pricing.RecoveryDigest = clip.RecoveryDigest(rawRecovery)
	pricing.ReusedChunks = len(recovered.Chunks)
	if err != nil {
		return p, t, b, pricing, guidelines, admissionRefusal(modelRef(observe), err)
	}
	pricing.CancellationPolicyVersion = clip.CancellationPolicyVersion
	// The quote binds them through the pricing, and a change before the start invalidates the
	// approval (QUOTA-45).
	pricing.GuidelinesDigest = guidelines.Digest()
	// 이 스토리로 만들기 binds the storyline it builds along the same way (QUOTA-45).
	if mode == quoteFromStoryline {
		pricing.StorylineDigest = p.Storyline.Digest()
	}
	if pricing.Version != clip.PricingPolicyVersion || pricing.ObservationCalls != count || pricing.MaxCredits < 0 || pricing.Observe.Ref != modelRef(observe) || pricing.Plan.Ref != modelRef(write) || !pricing.Observe.Valid() || !pricing.Plan.Valid() {
		return p, t, b, pricing, guidelines, clip.ErrPricingUnavailable
	}
	return p, t, b, pricing, guidelines, nil
}

func (s *GenerationService) Quote(ctx context.Context, user, id, batch, observe, write string) (clip.GenerationQuote, error) {
	return s.quote(ctx, user, id, batch, observe, write, quoteGenerate)
}

// QuoteFromStoryline prices 이 스토리로 만들기 (CLIP-178): the flow and the narration along the
// stored storyline, which the quote binds.
func (s *GenerationService) QuoteFromStoryline(ctx context.Context, user, id, batch, observe, write string) (clip.GenerationQuote, error) {
	return s.quote(ctx, user, id, batch, observe, write, quoteFromStoryline)
}

// QuoteStoryline prices 스토리라인 먼저 and 다시 만들기 (CLIP-177): the analysis still missing
// and one storyline call.
func (s *GenerationService) QuoteStoryline(ctx context.Context, user, id, batch, observe, write string) (clip.GenerationQuote, error) {
	return s.quote(ctx, user, id, batch, observe, write, quoteStoryline)
}

func (s *GenerationService) quote(ctx context.Context, user, id, batch, observe, write string, mode quoteMode) (clip.GenerationQuote, error) {
	p, t, b, pricing, _, err := s.quoteInputs(ctx, user, id, batch, observe, write, mode)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return clip.GenerationQuote{}, err
	}
	if active != nil {
		return clip.GenerationQuote{}, clip.ErrBusy
	}
	store, ok := s.store.(clip.QuoteStore)
	if !ok {
		return clip.GenerationQuote{}, clip.ErrPricingUnavailable
	}
	now := s.now()
	expires := now.Add(s.cfg.QuoteTTL)
	if b.ExpiresAt.Before(expires) {
		expires = b.ExpiresAt
	}
	q := clip.GenerationQuote{ID: newID(), UserID: user, ProjectID: id, BatchID: batch, InputDigest: clip.QuoteInputDigest(p, t, b, pricing), Pricing: pricing, ExpiresAt: expires}
	if err = store.SaveQuote(ctx, q, now); err != nil {
		return clip.GenerationQuote{}, err
	}
	return q, nil
}

// StartFromStoryline accepts an approved 이 스토리로 만들기 (CLIP-178).
func (s *GenerationService) StartFromStoryline(ctx context.Context, user, id, batch, observe, write string, approval clip.QuoteApproval) (string, error) {
	return s.startMode(ctx, user, id, batch, observe, write, approval, quoteFromStoryline)
}

// StartStoryline accepts an approved 스토리라인 먼저 or 다시 만들기 (CLIP-177).
func (s *GenerationService) StartStoryline(ctx context.Context, user, id, batch, observe, write string, approval clip.QuoteApproval) (string, error) {
	return s.startMode(ctx, user, id, batch, observe, write, approval, quoteStoryline)
}

func (s *GenerationService) startApproved(ctx context.Context, user, id, batch, observe, write string, a clip.QuoteApproval) (string, error) {
	return s.startMode(ctx, user, id, batch, observe, write, a, quoteGenerate)
}

func (s *GenerationService) startMode(ctx context.Context, user, id, batch, observe, write string, a clip.QuoteApproval, mode quoteMode) (string, error) {
	if a.QuoteID == "" || a.MaxCredits == nil || *a.MaxCredits < 0 {
		return "", clip.ErrQuoteRequired
	}
	current, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return "", err
	}
	if current.Finalized != nil {
		return "", clip.ErrFinalized
	}
	// The durable snapshot survives transient batch/quote cleanup, including an
	// ambiguous start response. Returning it never dispatches or reserves again.
	if existing, err := s.acceptedJob(ctx, user, id, batch, observe, write, a); err != nil || existing != "" {
		return existing, err
	}
	if a.CancellationPolicyVersion != clip.CancellationPolicyVersion {
		return "", clip.ErrCancellationPolicy
	}
	store, ok := s.store.(clip.QuoteStore)
	if !ok {
		return "", clip.ErrQuoteRequired
	}
	q, err := store.GetQuote(ctx, user, a.QuoteID)
	if errors.Is(err, clip.ErrNotFound) {
		return "", clip.ErrQuoteChanged
	}
	if err != nil {
		return "", err
	}
	if q.ProjectID != id || q.BatchID != batch || q.Pricing.MaxCredits != *a.MaxCredits || q.Pricing.Observe.Ref != modelRef(observe) || q.Pricing.Plan.Ref != modelRef(write) {
		return "", clip.ErrQuoteChanged
	}
	if q.ConsumedJobID != "" {
		return q.ConsumedJobID, nil
	}
	if !s.now().Before(q.ExpiresAt) {
		return "", clip.ErrQuoteExpired
	}
	p, t, b, pricing, guidelines, err := s.quoteInputs(withFrozenQuoteRate(ctx, q.Pricing), user, id, batch, observe, write, mode)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(q.Pricing, pricing) || q.InputDigest != clip.QuoteInputDigest(p, t, b, pricing) {
		return "", clip.ErrQuoteChanged
	}
	c, err := clip.GenerationComposition(t, p, s.projects.limits.Composition)
	if err != nil {
		return "", err
	}
	rawRecovery, err := s.loadRecovery(ctx, user, id)
	if err != nil {
		return "", err
	}
	if clip.RecoveryDigest(rawRecovery) != q.Pricing.RecoveryDigest {
		return "", clip.ErrQuoteChanged
	}
	upgraded, err := s.upgradeRecovery(ctx, user, id, rawRecovery)
	if err != nil {
		return "", err
	}
	recovery := s.selectRecovery(upgraded, b, modelRef(observe), p.Language)
	regions := clip.EffectiveProjectRegions(p)
	payload, err := json.Marshal(clip.GenerationPayload{Language: p.Language, Recovery: &recovery, Composition: c, Version: clip.GenerationPayloadVersion, ProjectID: id, Ratio: p.Ratio, Observe: observe, Write: write, TargetDurationMS: p.TargetDurationMS, Template: t.Recipe, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, Instruction: p.Instruction, Guidelines: guidelines, FollowStoryline: followed(mode, p), Regions: &regions, CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles, Batch: b, Approval: &clip.GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing}})
	if err != nil {
		return "", err
	}
	// The instruction this generation froze, kept verbatim beside the project.
	// An empty one is recorded as the empty instruction it was: a clip written
	// without direction is a thing the record has to be able to explain.
	frozen := clip.ProjectRequest{Kind: clip.RequestInstruction, Body: p.Instruction, CreatedAt: s.now()}
	job, err := s.enqueue(ctx, clip.GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Quote: &q, Request: &frozen, Kind: mode.kind()}, batch, 0)
	if err != nil {
		// A racing request may have won the unique active-project/source linkage.
		if existing, lookup := s.acceptedJob(ctx, user, id, batch, observe, write, a); lookup == nil && existing != "" {
			return existing, nil
		}
	}
	return job, err
}

type generationJobReader interface {
	Latest(context.Context, string, string) (*clip.ClipJob, error)
}

func (s *GenerationService) acceptedJob(ctx context.Context, user, id, batch, observe, write string, a clip.QuoteApproval) (string, error) {
	r, ok := s.jobs.(generationJobReader)
	if !ok {
		return "", nil
	}
	j, err := r.Latest(ctx, user, id)
	if err != nil || j == nil {
		return "", err
	}
	var p clip.GenerationPayload
	if j.Kind != "generate_clip" || json.Unmarshal(j.Payload, &p) != nil || !clip.SupportedGenerationPayload(p.Version) || p.ProjectID != id || p.Batch.UserID != user || p.Batch.ID != batch || p.Observe != observe || p.Write != write || p.Approval == nil || p.Approval.QuoteID != a.QuoteID {
		return "", nil
	}
	if a.MaxCredits == nil || p.Approval.MaxCredits != *a.MaxCredits || a.CancellationPolicyVersion != p.Approval.Pricing.CancellationPolicyVersion {
		return "", clip.ErrQuoteChanged
	}
	// An unlinked queued row is not an accepted quote yet.
	if j.Status == "queued" && !j.DispatchReady {
		return "", clip.ErrBusy
	}
	if !j.DispatchReady {
		return "", nil
	}
	return j.ID, nil
}

// followed is the storyline a start freezes to build along: the stored one for 이 스토리로
// 만들기, none otherwise.
func followed(mode quoteMode, p clip.Project) *clip.Storyline {
	if mode == quoteFromStoryline {
		return p.Storyline
	}
	return nil
}
