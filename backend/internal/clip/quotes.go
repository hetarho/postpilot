package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"time"

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

func RequiredQuoteAnswers(p Project, t VideoTemplate) []Answer {
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
	}{p.UserID, p.ID, b.ID, p.Title, p.VideoTemplateID, p.Ratio, p.Disclosure, p.CTA, p.Language, p.Instruction, p.HideDisclosure, p.TargetDurationMS, t.Recipe, RequiredQuoteAnswers(p, t), sources, pricing, p.Composition}
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
