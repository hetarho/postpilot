package clip

import (
	"context"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"log/slog"
	"strings"
	"time"
)

var ErrBusy = errors.New("clip busy")

type GenerationService struct {
	store      GenerationStore
	projects   *Service
	sources    *SourceService
	objects    ProcessingObjects
	media      Media
	planner    Planner
	renderer   Renderer
	jobs       GenerationJobs
	cfg        GenerationConfig
	pricing    QuotePricing
	accounting AccountingReader
	now        func() time.Time
}

func NewGenerationService(store GenerationStore, projects *Service, sources *SourceService, objects ProcessingObjects, media Media, planner Planner, renderer Renderer, jobs GenerationJobs, cfg GenerationConfig) *GenerationService {
	if cfg.ReadTTL <= 0 || cfg.CleanupTimeout <= 0 || cfg.OrphanMinAge <= 0 {
		panic("invalid clip generation configuration")
	}
	return &GenerationService{store: store, projects: projects, sources: sources, objects: objects, media: media, planner: planner, renderer: renderer, jobs: jobs, cfg: cfg, now: time.Now}
}

// This is the durable application snapshot, not the public project projection. No URL
// or source byte is serialized. Model choices and every recipe input are frozen here.
type generationPayload struct {
	Version                          int
	ProjectID, Ratio, Observe, Write string
	TargetDurationMS                 int
	Template                         Recipe
	Answers                          []Answer
	Batch                            SourceBatch
	Approval                         *GenerationApproval
}

func modelRef(s string) llm.ModelRef {
	p, m, _ := strings.Cut(s, "/")
	return llm.ModelRef{ProviderID: p, ModelID: m}
}
func (s *GenerationService) Start(ctx context.Context, user, id, batch, observe, write string, approval ...QuoteApproval) (string, error) {
	if len(approval) != 1 {
		return "", ErrQuoteRequired
	}
	return s.startApproved(ctx, user, id, batch, observe, write, approval[0])
}

func (s *GenerationService) enqueue(ctx context.Context, input GenerationStart, batch string, revision int) (string, error) {
	user := input.UserID
	job, err := s.jobs.Enqueue(ctx, input)
	if err != nil {
		return "", err
	}
	// A queued row is invisible to the dispatcher until its source lease is linked.
	if input.RenderOnly {
		err = s.store.LinkRenderSourceJob(ctx, user, batch, job, revision, time.Now())
	} else if input.Quote != nil {
		store, ok := s.store.(QuoteStore)
		if !ok {
			err = ErrQuoteRequired
		} else {
			err = store.LinkApprovedSourceJob(ctx, *input.Quote, job, s.now())
		}
	} else {
		err = ErrQuoteRequired
	}
	if err == nil {
		err = s.jobs.Activate(ctx, user, job)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		failed, fail := s.jobs.FailQueued(cleanup, user, job)
		if fail != nil {
			slog.Error("clip enqueue compensation pending recovery", "job", job)
		}
		// Activation may have committed before the caller lost its context. Never
		// delete inputs unless the queued-to-failed compare-and-swap succeeded.
		if failed {
			if linked, lookup := s.store.BatchForJob(cleanup, user, job); lookup == nil {
				if finish := s.sources.Finish(cleanup, user, linked.ID); finish != nil {
					slog.Warn("clip source cleanup pending recovery", "job", job)
				}
			}
		} else if current, lookup := s.jobs.Get(cleanup, user, job); lookup == nil && current != nil && (current.Status == "running" || current.Status == "done") {
			return job, nil
		}
		return "", err
	}
	return job, nil
}

type StageFailure struct {
	Stage string
	Cause error
}

func (e *StageFailure) Error() string { return fmt.Sprintf("clip %s: %v", e.Stage, e.Cause) }
func (e *StageFailure) Unwrap() error { return e.Cause }
func (e *StageFailure) Failure() llm.Failure {
	f := llm.NormalizeFailure(e.Cause)
	var credits *plan.InsufficientCreditsError
	switch {
	case errors.Is(e.Cause, ErrQuoteRequired):
		f = llm.Failure{Reason: "CLIP_QUOTE_REQUIRED"}
	case errors.Is(e.Cause, ErrQuoteExpired):
		f = llm.Failure{Reason: "CLIP_QUOTE_EXPIRED"}
	case errors.Is(e.Cause, ErrQuoteChanged):
		f = llm.Failure{Reason: "CLIP_QUOTE_CHANGED"}
	case errors.Is(e.Cause, ErrPricingUnavailable):
		f = llm.Failure{Reason: "CLIP_MODEL_PRICING_UNAVAILABLE"}
	case errors.As(e.Cause, &credits):
		f = llm.Failure{Reason: "INSUFFICIENT_CREDITS", Params: map[string]string{"required": fmt.Sprint(credits.Required), "balance": fmt.Sprint(credits.Balance), "renews_at": credits.RenewsAt.Format(time.RFC3339)}}
	case errors.Is(e.Cause, ErrInvalidMedia):
		f = llm.Failure{Reason: "CLIP_INVALID_MEDIA", TechnicalDetail: "Source verification failed before analysis."}
	case errors.Is(e.Cause, ErrCopyTooLong):
		f = llm.Failure{Reason: "CLIP_COPY_TOO_LONG"}
	case errors.Is(e.Cause, ErrInvalid):
		f = llm.Failure{Reason: "CLIP_INVALID_INPUT"}
	case errors.Is(e.Cause, ErrPlanConflict):
		f = llm.Failure{Reason: "CLIP_PLAN_CONFLICT"}
	case errors.Is(e.Cause, ErrSourceState), errors.Is(e.Cause, ErrNotFound):
		f = llm.Failure{Reason: "CLIP_SOURCE_UNAVAILABLE"}
	}
	if f.Reason == "UNKNOWN_FAILURE" {
		f = llm.Failure{Reason: "CLIP_PROCESSING_FAILED", TechnicalDetail: "Clip processing failed in stage " + e.Stage + "."}
	}
	if f.Params == nil {
		f.Params = map[string]string{}
	}
	// The durable job already owns Stage. Adding it to reason-specific params
	// would violate the frontend's strict failure allowlist (including credits).
	return f
}
func (s *GenerationService) Run(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	stage := "prepare"
	// Resolve cleanup from the durable linkage, even if payload decoding fails.
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		b, lookup := s.store.BatchForJob(cleanup, user, job)
		if lookup == nil {
			if e := s.sources.Finish(cleanup, user, b.ID); e != nil {
				slog.Warn("clip cleanup deferred", "job", job)
			}
		}
		if err != nil {
			err = &StageFailure{stage, err}
		}
	}()
	// T082 intentionally closes both legacy signed-proxy jobs and approved v2
	// jobs until T084 installs the complete prepare/admit/inline-call pipeline.
	// Keep durable-link cleanup above; no local media, hold or provider call occurs.
	return ErrQuoteRequired
}
