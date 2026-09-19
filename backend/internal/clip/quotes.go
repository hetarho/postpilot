package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"time"

	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/llm"
)

var (
	ErrQuoteRequired      = errors.New("clip credit quote and exact approval required")
	ErrQuoteExpired       = errors.New("clip credit quote expired")
	ErrQuoteChanged       = errors.New("clip credit quote inputs changed")
	ErrPricingUnavailable = errors.New("clip model pricing unavailable")
	ErrCancellationPolicy = errors.New("clip cancellation policy requires a supported client approval")
)

const PricingPolicyVersion = 3
const CancellationPolicyVersion = 1

type GenerationPricing struct {
	CancellationPolicyVersion int
	Version                   int
	// The observation, then the two writing calls a generation makes: `Plan`
	// prices the flow call and `Narration` the narration over it (CLIP-135).
	// The flow keeps the field name it has always had, so the quote, the
	// reservation and the durable job read one shape; the label, not the field,
	// tells the owner which call is which.
	Observe, Plan, Narration     llm.CallPolicy
	ObservationCalls, MaxCredits int
	// A resumed generation skips the writing calls whose result it already
	// holds. The narration cannot be kept over a flow that is being rewritten,
	// so skipping it implies skipping the flow.
	SkipFlow, SkipNarration bool
	RecoveryDigest          string
	ReusedChunks            int
}

// Both stages must have the complete enforceable profile, not a legacy pair of
// catalog token prices. This value is frozen in the quote and checked at admission.
func (p GenerationPricing) Valid() bool {
	writing := func(c llm.CallPolicy) bool {
		return c.Valid() && c.Pricing.Valid() && c.Pricing.Delivery == llm.ExecutionTextOnly && c.Stage == llm.StageNameWrite && c.CompletionTokens == 32768
	}
	return p.Version == PricingPolicyVersion && p.CancellationPolicyVersion >= 0 && p.CancellationPolicyVersion <= CancellationPolicyVersion && p.ObservationCalls >= 0 && p.ObservationCalls <= 49 && p.MaxCredits >= 0 && p.ReusedChunks >= 0 && p.ReusedChunks <= 49 && (!p.SkipFlow || p.ObservationCalls == 0) && (!p.SkipNarration || p.SkipFlow) &&
		p.Observe.Valid() && p.Observe.Pricing.Valid() && p.Observe.Pricing.Delivery == llm.ExecutionInlineStatic && p.Observe.Stage == llm.StageNameObserve && p.Observe.CompletionTokens == 8192 &&
		writing(p.Plan) && writing(p.Narration) && p.Plan.Ref == p.Narration.Ref
}

// FlowCalls and NarrationCalls are what each writing call may cost, its own
// response corrections included. A call whose result the recovery already
// holds costs nothing.
func (p GenerationPricing) FlowCalls() int {
	if p.SkipFlow {
		return 0
	}
	return 1 + p.Plan.ResponseRetries
}
func (p GenerationPricing) NarrationCalls() int {
	if p.SkipNarration {
		return 0
	}
	return 1 + p.Narration.ResponseRetries
}

// RenderOnly is a resume that writes nothing at all: both calls are answered by
// the plan the recovery already holds.
func (p GenerationPricing) RenderOnly() bool { return p.SkipFlow && p.SkipNarration }

// PlanCalls is every writing call this generation still has to pay for. Both
// are the same model at the same budget, so they reserve as one priced line.
func (p GenerationPricing) PlanCalls() int { return p.FlowCalls() + p.NarrationCalls() }
func (p GenerationPricing) ObserveCalls(chunks int) int {
	return chunks * (1 + p.Observe.ResponseRetries)
}

type GenerationQuote struct {
	ID, UserID, ProjectID, BatchID, InputDigest, ConsumedJobID string
	Pricing                                                    GenerationPricing
	ExpiresAt                                                  time.Time
}
type GenerationApproval struct {
	QuoteID    string
	MaxCredits int
	Pricing    GenerationPricing
}
type QuoteApproval struct {
	CancellationPolicyVersion int
	QuoteID                   string
	MaxCredits                *int
}

// Pricing has no media or provider-call capability. Its output is server-owned.
type QuotePricing interface {
	Freeze(context.Context, llm.ModelRef, llm.ModelRef, int) (GenerationPricing, error)
}
type QuoteStore interface {
	SaveQuote(context.Context, GenerationQuote, time.Time) error
	GetQuote(context.Context, string, string) (GenerationQuote, error)
	LinkApprovedSourceJob(context.Context, GenerationQuote, string, time.Time) error
}

type remainingQuotePricing interface {
	FreezeWork(context.Context, llm.ModelRef, llm.ModelRef, int, bool, bool, int) (GenerationPricing, error)
}

func (s *GenerationService) freezeWork(ctx context.Context, o, w llm.ModelRef, n int, skipFlow, skipNarration bool, retries int) (GenerationPricing, error) {
	if p, ok := s.pricing.(remainingQuotePricing); ok {
		return p.FreezeWork(ctx, o, w, n, skipFlow, skipNarration, retries)
	}
	if skipFlow || skipNarration {
		return GenerationPricing{}, ErrPricingUnavailable
	}
	return s.pricing.Freeze(ctx, o, w, n)
}

// ConservativeObservationCount covers the probe tolerance and independent chunk
// rounding per source. Its global bound still respects verified total duration.
func ConservativeObservationCount(cfg MediaConfig, batch SourceBatch) (int, error) {
	n := len(batch.Sources)
	if n < 1 || n > cfg.Sources.MaxCount || cfg.DurationToleranceMS < 0 || cfg.Sources.MaxDurationMS <= 0 || cfg.ChunkDurationMS != 60_000 {
		return 0, ErrInvalidMedia
	}
	ceil := func(v int) int { return (v-1)/60_000 + 1 }
	bound := ceil(cfg.Sources.MaxDurationMS)
	if bound > 49 || n-1 > 49-bound {
		return 0, ErrInvalidMedia
	}
	bound += n - 1
	total, count := 0, 0
	for _, v := range batch.Sources {
		d := v.DurationMS
		if d <= 0 || d > cfg.Sources.MaxDurationMS-total || cfg.DurationToleranceMS > math.MaxInt-d {
			return 0, ErrInvalidMedia
		}
		total += d
		// Saturate at the global bound without overflowing the sum.
		count += min(ceil(d+cfg.DurationToleranceMS), bound-count)
	}
	return count, nil
}

func requiredQuoteAnswers(p Project, t VideoTemplate) []Answer {
	answers := make([]Answer, 0, len(t.InformationFields))
	for _, f := range t.InformationFields {
		for _, a := range p.Answers {
			if a.Label == f.Label {
				answers = append(answers, a)
				break
			}
		}
	}
	return answers
}

// QuoteInputDigest is also recomputed inside the source-link transaction. Only
// semantic inputs participate: no mutable lease state, timestamps, URL or bytes.
func QuoteInputDigest(p Project, t VideoTemplate, b SourceBatch, pricing GenerationPricing) string {
	type source struct {
		ID       string
		Metadata SourceMetadata
	}
	sources := make([]source, len(b.Sources))
	for i, v := range b.Sources {
		sources[i] = source{v.ID, v.SourceMetadata}
	}
	input := struct {
		User, Project, Batch, Title, TemplateID, Ratio string
		Disclosure, CTA, Language                      string
		Instruction                                    string
		HideDisclosure                                 bool
		Target                                         int
		Recipe                                         Recipe
		Answers                                        []Answer
		Sources                                        []source
		Pricing                                        GenerationPricing
		Composition                                    *ProjectComposition
	}{p.UserID, p.ID, b.ID, p.Title, p.VideoTemplateID, p.Ratio, p.Disclosure, p.CTA, p.Language, p.Instruction, p.HideDisclosure, p.TargetDurationMS, t.Recipe, requiredQuoteAnswers(p, t), sources, pricing, p.Composition}
	data, _ := json.Marshal(input) // All fields are concrete JSON-safe values.
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func ValidQuoteBatch(b SourceBatch, user, project string, now time.Time) bool {
	if b.AccessDenied || b.UserID != user || b.ProjectID != project || b.State != "ready" || !now.Before(b.ExpiresAt) || len(b.Sources) == 0 {
		return false
	}
	for _, v := range b.Sources {
		if v.CleanupPending || v.State != "ready" || v.ActualBytes != v.Bytes || v.Bytes <= 0 {
			return false
		}
	}
	return true
}

func (s *GenerationService) quoteInputs(ctx context.Context, user, id, batch, observe, write string) (Project, VideoTemplate, SourceBatch, GenerationPricing, error) {
	var p Project
	var t VideoTemplate
	var b SourceBatch
	var pricing GenerationPricing
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return p, t, b, pricing, err
	}
	if p.Finalized != nil {
		return p, t, b, pricing, ErrFinalized
	}
	// A project generates with no template (CLIP-5) and with one it has since
	// lost (CLIP-25): both leave the zero recipe standing and no declared
	// structure to satisfy.
	if p.VideoTemplateID != "" {
		if t, err = s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID); err != nil {
			return p, t, b, pricing, err
		}
	}
	if err = RequiredAnswers(t, p, s.projects.limits.Composition); err != nil {
		return p, t, b, pricing, err
	}
	if !s.projects.duration(p.TargetDurationMS) {
		return p, t, b, pricing, ErrTargetDurationRequired
	}
	c, err := GenerationComposition(t, p, s.projects.limits.Composition)
	if err != nil {
		return p, t, b, pricing, err
	}
	if err = s.checkComposition(c); err != nil {
		return p, t, b, pricing, err
	}
	if validator, ok := s.renderer.(interface {
		ValidateAuthoredInput(context.Context, PlanningInput) error
	}); ok {
		if err := validator.ValidateAuthoredInput(ctx, PlanningInput{Composition: c, Template: t.Recipe, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Design: p.DesignSelection()}); err != nil {
			return p, t, b, pricing, err
		}
	}
	b, err = s.sources.AvailableBatch(ctx, user, batch)
	if err != nil {
		return p, t, b, pricing, err
	}
	if !ValidQuoteBatch(b, user, id, s.now()) {
		return p, t, b, pricing, ErrSourceState
	}
	if err := MatchCompositionSources(c, b); err != nil {
		return p, t, b, pricing, err
	}
	count, err := ConservativeObservationCount(s.cfg.Media, b)
	if err != nil {
		return p, t, b, pricing, err
	}
	if s.pricing == nil || s.cfg.QuoteTTL <= 0 {
		return p, t, b, pricing, ErrPricingUnavailable
	}
	rawRecovery, err := s.loadRecovery(ctx, user, id)
	if err != nil {
		return p, t, b, pricing, err
	}
	upgraded, err := s.upgradeRecovery(ctx, user, id, rawRecovery)
	if err != nil {
		return p, t, b, pricing, err
	}
	recovered := s.selectRecovery(upgraded, b, modelRef(observe), p.Language)
	seed := generationPayload{Language: p.Language, Batch: b, Composition: c, Template: t.Recipe, Answers: requiredQuoteAnswers(p, t), Ratio: p.Ratio, Write: write, TargetDurationMS: p.TargetDurationMS, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: design.DefaultCTA(t.Preset, p.CTA), Instruction: p.Instruction}
	// A resume reuses exactly what the recovery holds: a complete plan answers
	// both writing calls, a written flow answers only the first. A recovery
	// saved before the flow was its own call carries a complete plan, so it
	// still resumes at rendering.
	written := recovered.Plan != "" && recovered.PlanDigest == planRecoveryDigest(seed)
	skipFlow := written && (recovered.FlowReady || recovered.PlanReady)
	skipNarration := skipFlow && recovered.PlanReady
	upperCount := count
	// Verified source durations supersede conservative browser estimates only for
	// the exact retained source identity; count only missing chunks.
	count = 0
	for _, v := range b.Sources {
		one := b
		one.Sources = []SourceLease{v}
		n, e := ConservativeObservationCount(s.cfg.Media, one)
		if e != nil {
			return p, t, b, pricing, e
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
			return p, t, b, pricing, admissionRefusal(modelRef(observe), err)
		}
		pricing, err = s.freezeWork(ctx, modelRef(observe), modelRef(write), count, skipFlow, skipNarration, 3)
	}
	pricing.RecoveryDigest = RecoveryDigest(rawRecovery)
	pricing.ReusedChunks = len(recovered.Chunks)
	if err != nil {
		return p, t, b, pricing, admissionRefusal(modelRef(observe), err)
	}
	pricing.CancellationPolicyVersion = CancellationPolicyVersion
	if pricing.Version != PricingPolicyVersion || pricing.ObservationCalls != count || pricing.MaxCredits < 0 || pricing.Observe.Ref != modelRef(observe) || pricing.Plan.Ref != modelRef(write) || !pricing.Observe.Valid() || !pricing.Plan.Valid() {
		return p, t, b, pricing, ErrPricingUnavailable
	}
	return p, t, b, pricing, nil
}

func (s *GenerationService) Quote(ctx context.Context, user, id, batch, observe, write string) (GenerationQuote, error) {
	p, t, b, pricing, err := s.quoteInputs(ctx, user, id, batch, observe, write)
	if err != nil {
		return GenerationQuote{}, err
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return GenerationQuote{}, err
	}
	if active != nil {
		return GenerationQuote{}, ErrBusy
	}
	store, ok := s.store.(QuoteStore)
	if !ok {
		return GenerationQuote{}, ErrPricingUnavailable
	}
	now := s.now()
	expires := now.Add(s.cfg.QuoteTTL)
	if b.ExpiresAt.Before(expires) {
		expires = b.ExpiresAt
	}
	q := GenerationQuote{ID: newID(), UserID: user, ProjectID: id, BatchID: batch, InputDigest: QuoteInputDigest(p, t, b, pricing), Pricing: pricing, ExpiresAt: expires}
	if err = store.SaveQuote(ctx, q, now); err != nil {
		return GenerationQuote{}, err
	}
	return q, nil
}

func (s *GenerationService) startApproved(ctx context.Context, user, id, batch, observe, write string, a QuoteApproval) (string, error) {
	if a.QuoteID == "" || a.MaxCredits == nil || *a.MaxCredits < 0 {
		return "", ErrQuoteRequired
	}
	current, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return "", err
	}
	if current.Finalized != nil {
		return "", ErrFinalized
	}
	// The durable snapshot survives transient batch/quote cleanup, including an
	// ambiguous start response. Returning it never dispatches or reserves again.
	if existing, err := s.acceptedJob(ctx, user, id, batch, observe, write, a); err != nil || existing != "" {
		return existing, err
	}
	if a.CancellationPolicyVersion != CancellationPolicyVersion {
		return "", ErrCancellationPolicy
	}
	store, ok := s.store.(QuoteStore)
	if !ok {
		return "", ErrQuoteRequired
	}
	q, err := store.GetQuote(ctx, user, a.QuoteID)
	if errors.Is(err, ErrNotFound) {
		return "", ErrQuoteChanged
	}
	if err != nil {
		return "", err
	}
	if q.ProjectID != id || q.BatchID != batch || q.Pricing.MaxCredits != *a.MaxCredits || q.Pricing.Observe.Ref != modelRef(observe) || q.Pricing.Plan.Ref != modelRef(write) {
		return "", ErrQuoteChanged
	}
	if q.ConsumedJobID != "" {
		return q.ConsumedJobID, nil
	}
	if !s.now().Before(q.ExpiresAt) {
		return "", ErrQuoteExpired
	}
	p, t, b, pricing, err := s.quoteInputs(ctx, user, id, batch, observe, write)
	if err != nil {
		return "", err
	}
	if !reflect.DeepEqual(q.Pricing, pricing) || q.InputDigest != QuoteInputDigest(p, t, b, pricing) {
		return "", ErrQuoteChanged
	}
	c, err := GenerationComposition(t, p, s.projects.limits.Composition)
	if err != nil {
		return "", err
	}
	rawRecovery, err := s.loadRecovery(ctx, user, id)
	if err != nil {
		return "", err
	}
	if RecoveryDigest(rawRecovery) != q.Pricing.RecoveryDigest {
		return "", ErrQuoteChanged
	}
	upgraded, err := s.upgradeRecovery(ctx, user, id, rawRecovery)
	if err != nil {
		return "", err
	}
	recovery := s.selectRecovery(upgraded, b, modelRef(observe), p.Language)
	payload, err := json.Marshal(generationPayload{Language: p.Language, Recovery: &recovery, Composition: c, Version: generationPayloadVersion, ProjectID: id, Ratio: p.Ratio, Observe: observe, Write: write, TargetDurationMS: p.TargetDurationMS, Template: t.Recipe, Answers: requiredQuoteAnswers(p, t), Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: design.DefaultCTA(t.Preset, p.CTA), Instruction: p.Instruction, CaptionPace: p.CaptionPace, Accent: p.Accent, IntroPreset: p.IntroPreset, OutroPreset: p.OutroPreset, CaptionStyles: p.CaptionStyles, Batch: b, Approval: &GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing}})
	if err != nil {
		return "", err
	}
	// The instruction this generation froze, kept verbatim beside the project.
	// An empty one is recorded as the empty instruction it was: a clip written
	// without direction is a thing the record has to be able to explain.
	frozen := ProjectRequest{Kind: RequestInstruction, Body: p.Instruction, CreatedAt: s.now()}
	job, err := s.enqueue(ctx, GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Quote: &q, Request: &frozen}, batch, 0)
	if err != nil {
		// A racing request may have won the unique active-project/source linkage.
		if existing, lookup := s.acceptedJob(ctx, user, id, batch, observe, write, a); lookup == nil && existing != "" {
			return existing, nil
		}
	}
	return job, err
}

type generationJobReader interface {
	Latest(context.Context, string, string) (*ClipJob, error)
}

func (s *GenerationService) acceptedJob(ctx context.Context, user, id, batch, observe, write string, a QuoteApproval) (string, error) {
	r, ok := s.jobs.(generationJobReader)
	if !ok {
		return "", nil
	}
	j, err := r.Latest(ctx, user, id)
	if err != nil || j == nil {
		return "", err
	}
	var p generationPayload
	if j.Kind != "generate_clip" || json.Unmarshal(j.Payload, &p) != nil || !supportedGenerationPayload(p.Version) || p.ProjectID != id || p.Batch.UserID != user || p.Batch.ID != batch || p.Observe != observe || p.Write != write || p.Approval == nil || p.Approval.QuoteID != a.QuoteID {
		return "", nil
	}
	if a.MaxCredits == nil || p.Approval.MaxCredits != *a.MaxCredits || a.CancellationPolicyVersion != p.Approval.Pricing.CancellationPolicyVersion {
		return "", ErrQuoteChanged
	}
	// An unlinked queued row is not an accepted quote yet.
	if j.Status == "queued" && !j.DispatchReady {
		return "", ErrBusy
	}
	if !j.DispatchReady {
		return "", nil
	}
	return j.ID, nil
}
