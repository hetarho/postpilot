package clip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
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

// The worker logs Error(), so redaction must apply here as well as to the public
// Failure projection. Unwrap retains the original cause for classification/tests.
func (e *StageFailure) Error() string        { return fmt.Sprintf("clip %s: %s", e.Stage, e.Failure().Reason) }
func (e *StageFailure) Unwrap() error        { return e.Cause }
func (e *StageFailure) FailureStage() string { return e.Stage }
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
	case errors.Is(e.Cause, ErrWorkspaceLimit):
		f = llm.Failure{Reason: "CLIP_WORKSPACE_LIMIT"}
	case errors.Is(e.Cause, ErrAnalysisTooLarge):
		f = llm.Failure{Reason: "CLIP_ANALYSIS_TOO_LARGE"}
	case errors.Is(e.Cause, ErrModelInputUnsupported):
		f = llm.Failure{Reason: "CLIP_MODEL_INPUT_UNSUPPORTED"}
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
	b, err := s.store.BatchForJob(ctx, user, job)
	if err != nil {
		return err
	}
	var p generationPayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	if p.Version != 2 || p.Approval == nil {
		return ErrQuoteRequired
	}
	if p.ProjectID != project || p.Batch.ProjectID != project || p.Batch.ID != b.ID || p.Batch.UserID != user || !reflect.DeepEqual(p.Batch.Sources, b.Sources) {
		return ErrInvalid
	}
	if b.State != "consuming" || b.ProjectID != project {
		return ErrSourceState
	}
	pricing := p.Approval.Pricing
	if !pricing.Valid() || pricing.Observe.Ref != modelRef(p.Observe) || pricing.Plan.Ref != modelRef(p.Write) || p.Approval.MaxCredits != pricing.MaxCredits {
		return ErrPricingUnavailable
	}
	quoteStore, ok := s.store.(QuoteStore)
	if !ok {
		return ErrQuoteRequired
	}
	q, err := quoteStore.GetQuote(ctx, user, p.Approval.QuoteID)
	if err != nil {
		return err
	}
	// Expiry is enforced when the quote is consumed, not after a long, already
	// accepted preparation. Reconfirm the durable ownership and exact approval.
	if q.ConsumedJobID != job || q.ProjectID != project || q.BatchID != b.ID || q.Pricing != pricing {
		return ErrQuoteChanged
	}
	set := func(name string, done, total int) {
		stage = name
		if progress != nil {
			progress(name, done, total)
		}
	}
	var analysisJSON []byte
	var planJSON string
	var result Result
	err = s.media.WithWorkspace(ctx, job, func(ws MediaWorkspace) error {
		set("prepare", 0, len(b.Sources))
		sources, prepared, err := s.prepareBatch(ctx, ws, b, func(n int) { set("prepare", n, len(b.Sources)) })
		if err != nil {
			return err
		}
		count := len(prepared)
		if count > pricing.ObservationCalls {
			return ErrQuoteChanged
		}
		if err = s.planner.ValidateModels(pricing.Observe.Ref, pricing.Plan.Ref); err != nil {
			return err
		}
		if s.planner.Budgets() != (CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Plan: pricing.Plan.CompletionTokens}) {
			return ErrQuoteChanged
		}
		if err = s.planner.ValidatePreparation(pricing.Observe.Ref, PlanningInput{Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Policy: pricing.Plan}, sources); err != nil {
			return err
		}
		if s.pricing == nil {
			return ErrPricingUnavailable
		}
		current, err := s.pricing.Freeze(ctx, pricing.Observe.Ref, pricing.Plan.Ref, count)
		if err != nil {
			return err
		}
		if !current.Valid() || current.ObservationCalls != count || current.Observe != pricing.Observe || current.Plan != pricing.Plan || current.MaxCredits > p.Approval.MaxCredits {
			return ErrQuoteChanged
		}
		// All source/proxy validation and price/profile checks precede the only
		// reservation. Subsequent calls can consume only this exact allowance.
		admitted, err := s.jobs.ReserveApproved(ctx, user, job, *p.Approval, count)
		if err != nil {
			return err
		}
		ctx = admitted
		set("analyze", 0, count)
		chunks := make([]ChunkAnalysis, 0, count)
		for _, v := range prepared {
			observation, err := s.observe(ctx, v.chunk, sources[v.source], pricing.Observe)
			if err != nil {
				return err
			}
			chunks = append(chunks, observation)
			set("analyze", len(chunks), count)
		}
		analyses, err := MergeAnalyses(s.cfg.Analysis, sources, chunks)
		if err != nil {
			return err
		}
		set("plan", 0, 1)
		edit, _, err := s.planner.Plan(ctx, pricing.Plan.Ref, PlanningInput{Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Analyses: analyses, Policy: pricing.Plan})
		if err != nil {
			return err
		}
		set("render", 0, 1)
		renderSources := make([]RenderSource, len(sources))
		for i, v := range sources {
			renderSources[i] = v.RenderSource
		}
		video, err := s.renderer.Render(ctx, ws, edit, renderSources, func(ctx context.Context, id string, fn func(MediaSource) error) error {
			for i, v := range b.Sources {
				if v.ID == id {
					return s.withSource(ctx, ws, v, sources[i].Info, fn)
				}
			}
			return ErrNotFound
		})
		if err != nil {
			return err
		}
		set("save", 0, 1)
		key := ResultPrefix + url.PathEscape(user) + "/" + url.PathEscape(project) + "/" + newID() + ".mp4"
		if err = s.uploadPath(ctx, key, video.Path, video.Bytes); err != nil {
			return err
		}
		analysisJSON, err = json.Marshal(analyses)
		if err != nil {
			return err
		}
		planJSON, err = EncodeEditPlan(edit, p.Template.CopyStyles)
		if err != nil {
			return err
		}
		result = Result{Key: key, ContentType: "video/mp4", Bytes: video.Bytes, DurationMS: video.Info.DurationMS, CreatedAt: s.now()}
		return nil
	})
	if err != nil {
		return err
	}
	// A failed attempt never replaces the prior result, including a local cleanup
	// failure. Uploaded but unpublished output remains recoverable by the sweep.
	if err = s.store.SaveGeneration(ctx, user, project, string(analysisJSON), planJSON, result); err != nil {
		return err
	}
	set("cleanup", 0, 1)
	return nil
}

type preparedChunk struct {
	source int
	chunk  AnalysisChunk
}

func (s *GenerationService) prepareBatch(ctx context.Context, ws MediaWorkspace, b SourceBatch, progress func(int)) ([]AnalysisSource, []preparedChunk, error) {
	if len(b.Sources) < 1 || len(b.Sources) > s.cfg.Media.Sources.MaxCount {
		return nil, nil, ErrInvalidMedia
	}
	var sources []AnalysisSource
	var probed []ProbedSource
	var prepared []preparedChunk
	var totalBytes int64
	seen := map[string]bool{}
	count := 0
	for i, v := range b.Sources {
		err := s.withSource(ctx, ws, v, MediaInfo{}, func(source MediaSource) error {
			info, err := s.media.Probe(ctx, ws, source.Path)
			if err != nil {
				return err
			}
			probed = append(probed, ProbedSource{Metadata: v.SourceMetadata, Info: info})
			count, err = ValidateProbedSources(s.cfg.Media, probed)
			if err != nil {
				return err
			}
			source.Info = info
			input := AnalysisSource{RenderSource: RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: info}, Filename: v.Filename}
			sources = append(sources, input)
			next := 0
			return s.media.PrepareAnalysisChunks(ctx, ws, source, func(c AnalysisChunk) error {
				if c.SourceID != v.ID || c.Fingerprint != v.Fingerprint || c.Index != next || ValidateChunkInput(s.cfg.Analysis, ChunkInput{Source: input, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}) != nil || filepath.Dir(c.Path) != ws.Path || filepath.Clean(c.Path) != c.Path || seen[c.Path] {
					return ErrInvalidMedia
				}
				if c.Bytes <= 0 || c.Bytes > s.cfg.Media.AnalysisMaxBytes {
					return ErrAnalysisTooLarge
				}
				if c.Bytes > s.cfg.Media.PreparedMaxBytes-totalBytes {
					return ErrWorkspaceLimit
				}
				if c.Info.ContainerDurationMS <= 0 || c.Info.ContainerDurationMS > 60000 || c.Info.Width <= 0 || c.Info.Height <= 0 || max(c.Info.Width, c.Info.Height) > 720 {
					return ErrInvalidMedia
				}
				file, err := os.Lstat(c.Path)
				if err != nil || !file.Mode().IsRegular() || file.Size() != c.Bytes {
					return ErrInvalidMedia
				}
				seen[c.Path] = true
				totalBytes += c.Bytes
				prepared = append(prepared, preparedChunk{source: i, chunk: c})
				next++
				return nil
			})
		})
		if err != nil {
			return nil, nil, err
		}
		if len(prepared) != count || count > 49 {
			return nil, nil, ErrInvalidMedia
		}
		progress(i + 1)
	}
	// Before charging, ensure the remaining entire bounded workspace can fit,
	// including a reloaded original and render intermediates. This checks disk
	// availability, not an allocation. Other filesystem users can still consume
	// space later, so actual writes retain the live guard too.
	for _, p := range prepared {
		file, err := os.Lstat(p.chunk.Path)
		if err != nil || !file.Mode().IsRegular() || file.Size() != p.chunk.Bytes {
			return nil, nil, ErrInvalidMedia
		}
	}
	if err := ws.CheckCapacity(s.cfg.Media.WorkspaceMaxBytes - totalBytes); err != nil {
		return nil, nil, err
	}
	return sources, prepared, nil
}
