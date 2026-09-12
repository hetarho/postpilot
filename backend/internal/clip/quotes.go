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
)

const PricingPolicyVersion = 2

type GenerationPricing struct {
	Version                      int
	Observe, Plan                llm.CallPolicy
	ObservationCalls, MaxCredits int
}

// Both stages must have the complete enforceable profile, not a legacy pair of
// catalog token prices. This value is frozen in the quote and checked at admission.
func (p GenerationPricing) Valid() bool {
	return p.Version == PricingPolicyVersion && p.ObservationCalls >= 1 && p.ObservationCalls <= 49 && p.MaxCredits >= 0 &&
		p.Observe.Valid() && p.Observe.Pricing.Valid() && p.Observe.Pricing.Delivery == llm.ExecutionInlineStatic && p.Observe.Stage == llm.StageNameObserve && p.Observe.CompletionTokens == 8192 &&
		p.Plan.Valid() && p.Plan.Pricing.Valid() && p.Plan.Pricing.Delivery == llm.ExecutionTextOnly && p.Plan.Stage == llm.StageNameWrite && p.Plan.CompletionTokens == 32768
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
	QuoteID    string
	MaxCredits *int
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

func (s *GenerationService) WithCredits(pricing QuotePricing, accounting AccountingReader) *GenerationService {
	s.pricing, s.accounting = pricing, accounting
	return s
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
		Disclosure, CTA                                string
		HideDisclosure                                 bool
		Target                                         int
		Recipe                                         Recipe
		Answers                                        []Answer
		Sources                                        []source
		Pricing                                        GenerationPricing
		Composition                                    *ProjectComposition
	}{p.UserID, p.ID, b.ID, p.Title, p.VideoTemplateID, p.Ratio, p.Disclosure, p.CTA, p.HideDisclosure, p.TargetDurationMS, t.Recipe, requiredQuoteAnswers(p, t), sources, pricing, p.Composition}
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
	t, err = s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID)
	if err != nil {
		return p, t, b, pricing, err
	}
	if err = RequiredAnswers(t, p, s.projects.limits.Composition); err != nil {
		return p, t, b, pricing, err
	}
	c, err := GenerationComposition(t, p, s.projects.limits.Composition)
	if err != nil {
		return p, t, b, pricing, err
	}
	if err = s.checkComposition(c); err != nil {
		return p, t, b, pricing, err
	}
	if err = s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
		return p, t, b, pricing, admissionRefusal(modelRef(observe), err)
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
	pricing, err = s.pricing.Freeze(ctx, modelRef(observe), modelRef(write), count)
	if err != nil {
		return p, t, b, pricing, admissionRefusal(modelRef(observe), err)
	}
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
	if _, err := s.projects.store.GetProject(ctx, user, id); err != nil {
		return "", err
	}
	// The durable snapshot survives transient batch/quote cleanup, including an
	// ambiguous start response. Returning it never dispatches or reserves again.
	if existing, err := s.acceptedJob(ctx, user, id, batch, observe, write, a); err != nil || existing != "" {
		return existing, err
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
	payload, err := json.Marshal(generationPayload{Composition: c, Version: generationPayloadVersion, ProjectID: id, Ratio: p.Ratio, Observe: observe, Write: write, TargetDurationMS: p.TargetDurationMS, Template: t.Recipe, Answers: requiredQuoteAnswers(p, t), Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: design.DefaultCTA(t.Preset, p.CTA), Batch: b, Approval: &GenerationApproval{QuoteID: q.ID, MaxCredits: q.Pricing.MaxCredits, Pricing: q.Pricing}})
	if err != nil {
		return "", err
	}
	job, err := s.enqueue(ctx, GenerationStart{UserID: user, ProjectID: id, Observe: observe, Write: write, Payload: payload, Quote: &q}, batch, 0)
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
	if j.Kind != "generate_clip" || json.Unmarshal(j.Payload, &p) != nil || (p.Version != generationPayloadVersion && p.Version != 3) || p.ProjectID != id || p.Batch.UserID != user || p.Batch.ID != batch || p.Observe != observe || p.Write != write || p.Approval == nil || p.Approval.QuoteID != a.QuoteID {
		return "", nil
	}
	if a.MaxCredits == nil || p.Approval.MaxCredits != *a.MaxCredits {
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
