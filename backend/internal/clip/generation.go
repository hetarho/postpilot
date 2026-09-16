package clip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrBusy = errors.New("clip busy")

type GenerationService struct {
	previewOwners sync.Map
	store         GenerationStore
	projects      *Service
	sources       *SourceService
	objects       ProcessingObjects
	media         Media
	planner       Planner
	renderer      Renderer
	jobs          GenerationJobs
	finisher      ClipFinisher
	cfg           GenerationConfig
	pricing       QuotePricing
	accounting    AccountingReader
	admission     AnalysisAdmission
	now           func() time.Time
}

func (s *GenerationService) WithFinisher(finisher ClipFinisher) *GenerationService {
	s.finisher = finisher
	return s
}

func NewGenerationService(store GenerationStore, projects *Service, sources *SourceService, objects ProcessingObjects, media Media, planner Planner, renderer Renderer, jobs GenerationJobs, cfg GenerationConfig) *GenerationService {
	if cfg.ReadTTL <= 0 || cfg.CleanupTimeout <= 0 || cfg.OrphanMinAge <= 0 {
		panic("invalid clip generation configuration")
	}
	return &GenerationService{store: store, projects: projects, sources: sources, objects: objects, media: media, planner: planner, renderer: renderer, jobs: jobs, cfg: cfg, now: time.Now}
}

// This is the durable application snapshot, not the public project projection. No URL
// or source byte is serialized. Model choices and every recipe input are frozen here.
// generationPayloadVersion is bumped whenever the frozen shape changes, and
// every reader checks it: an approval frozen under an older shape is refused
// rather than run with fields it never carried. Version 3 added the disclosure
// and the resolved CTA, so a job approved before the owner could choose a
// campaign type cannot render a clip that carries one. Version 4 freezes the
// project composition. Version 5 freezes the observation language; accepted
// version-3/4 jobs retain Korean, the legacy project language.
const generationPayloadVersion = 5

type generationPayload struct {
	Language                         string
	Recovery                         *RecoveryState
	Composition                      *ProjectComposition
	Version                          int
	ProjectID, Ratio, Observe, Write string
	TargetDurationMS                 int
	Template                         Recipe
	Answers                          []Answer
	// Frozen with the approval, like the template and the answers: the badge's
	// campaign type and the resolved closing CTA. Version 3 exists because of
	// them — a job approved before the disclosure was a choice cannot render a
	// clip that carries one, and is refused rather than rendered without it.
	Disclosure, CTA string
	// The project's own instruction (CLIP-121), frozen with the answers and the
	// composition so editing it mid-flight changes nothing in flight. A payload
	// written before it existed decodes as none, which is today's behaviour.
	Instruction string
	// The project's caption pace and accent (CLIP-139), frozen with the rest of
	// the render inputs. They are deliberately absent from planRecoveryDigest
	// and from the quote: changing either re-renders the same plan and costs no
	// writing call. A payload written before they existed decodes as empty,
	// which is the frozen document's own value — today's behaviour.
	CaptionPace, Accent string
	HideDisclosure      bool
	Batch               SourceBatch
	Approval            *GenerationApproval
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
			if current, lookup := s.jobs.Get(cleanup, user, job); lookup == nil && current != nil && current.FinishedAt != nil {
				if finish := s.sources.ReleaseAttempt(cleanup, user, job, *current.FinishedAt); finish != nil {
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
	var layout interface{ LayoutReason() string }
	var facts *MissingFactsError
	var admission *ModelAdmissionError
	var element *composition.Problem
	switch {
	case errors.As(e.Cause, &element):
		f = llm.Failure{Reason: "CLIP_COMPOSITION_INVALID", Params: element.FailureParams()}
	case errors.Is(e.Cause, ErrCompositionUnavailable):
		f = llm.Failure{Reason: "CLIP_COMPOSITION_UNAVAILABLE"}
	// The model's own admission answer comes first: it unwraps to the generic
	// unsupported error, which must not swallow the specific reason.
	case errors.As(e.Cause, &admission):
		f = admission.Failure()
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
	case errors.Is(e.Cause, ErrInputTooLarge):
		f = llm.Failure{Reason: "CLIP_INPUT_TOO_LARGE"}
	case errors.Is(e.Cause, ErrAnalysisTooLarge):
		f = llm.Failure{Reason: "CLIP_ANALYSIS_TOO_LARGE"}
	case errors.Is(e.Cause, ErrModelInputUnsupported):
		f = llm.Failure{Reason: "CLIP_MODEL_INPUT_UNSUPPORTED"}
	case errors.As(e.Cause, &credits):
		f = llm.Failure{Reason: "INSUFFICIENT_CREDITS", Params: map[string]string{"required": fmt.Sprint(credits.Required), "balance": fmt.Sprint(credits.Balance), "renews_at": credits.RenewsAt.Format(time.RFC3339)}}
	case errors.Is(e.Cause, ErrInvalidMedia):
		f = llm.Failure{Reason: "CLIP_INVALID_MEDIA", TechnicalDetail: "Source verification failed before analysis."}
	case errors.Is(e.Cause, ErrDisclosureRequired):
		f = llm.Failure{Reason: reasonDisclosureRequired}
	case errors.Is(e.Cause, ErrTargetDurationRequired):
		f = llm.Failure{Reason: reasonTargetDuration}
	case errors.As(e.Cause, &facts):
		f = llm.Failure{Reason: reasonFactsRequired, Params: map[string]string{"labels": strings.Join(facts.Labels, ", ")}}
	case errors.Is(e.Cause, ErrCopyTooLong):
		f = llm.Failure{Reason: "CLIP_COPY_TOO_LONG"}
	// A plan that was read and validated but cannot fill the floor is the
	// footage's shortfall, not the model's (CLIP-120).
	case errors.Is(e.Cause, ErrInsufficientFootage):
		f = llm.Failure{Reason: reasonInsufficientFootage}
	// One reason per verifier check, so the correction step can point at what
	// failed instead of saying the plan is invalid (CDS-52, LANG-21).
	case errors.As(e.Cause, &layout) && layout.LayoutReason() != "":
		f = llm.Failure{Reason: layout.LayoutReason()}
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
	if s.finisher == nil {
		return ErrCompositionUnavailable
	}
	currentProject, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	stage := "prepare"
	checkpoint := AttemptCheckpoint{Version: 1, JobID: job, Stage: stage}
	defer func() {
		if err == nil {
			return
		}
		checkpoint.Stage = stage
		if d, ok := DiagnosticFromError(err); ok {
			d.Values = SafeAttemptValues(d.Values)
			if len(d.Ranges) == 0 {
				d.Ranges = checkpoint.Diagnostic.Ranges
			}
			for _, key := range []string{"target_ms", "after_ms", "cut_count", "transition_ms", "reused_chunks", "remaining_chunks", "reused_plan"} {
				if n, ok := checkpoint.Diagnostic.Values[key]; ok {
					d.Values[key] = n
				}
			}
			if stage == "analyze" {
				// The worker owns the global source/chunk position; parser indexes
				// describe a segment inside that chunk and cannot replace it.
				for _, key := range []string{"source", "chunk"} {
					if value, exists := checkpoint.Diagnostic.Values[key]; exists {
						d.Values[key] = value
					}
				}
			}
			checkpoint.Diagnostic = d
		} else if err != nil {
			var output interface{ OutputValidationCode() string }
			if errors.As(err, &output) {
				checkpoint.Diagnostic.Check = output.OutputValidationCode()
			}
		}
		if err != nil {
			logAttemptDiagnostic(job, stage, checkpoint.Diagnostic)
		}
		recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.cfg.CleanupTimeout)
		defer cancel()
		s.checkpoint(recordCtx, user, project, checkpoint)
	}()
	// Resolve cleanup from the durable linkage, even if payload decoding fails.
	defer func() {
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
	if err := s.checkComposition(p.Composition); err != nil {
		return err
	}
	if p.Template.CompositionBody != "" && !p.Template.CompositionLegacy && p.Composition == nil {
		return ErrInvalid
	}
	if !supportedGenerationPayload(p.Version) || (p.Version >= 4 && p.Composition == nil) || p.Approval == nil {
		return ErrQuoteRequired
	}
	if p.ProjectID != project || p.Batch.ProjectID != project || p.Batch.ID != b.ID || p.Batch.UserID != user || !SameSourceManifest(p.Batch.Sources, b.Sources) {
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
	recovery := s.selectRecovery(p.Recovery, b, pricing.Observe.Ref, p.Language)
	recovery.JobID = job
	if p.Recovery != nil && len(recovery.Chunks) != pricing.ReusedChunks {
		return ErrQuoteChanged
	}
	if pricing.SkipNarration && (!recovery.PlanReady || recovery.Plan == "" || recovery.PlanDigest != planRecoveryDigest(p)) {
		return ErrQuoteChanged
	}
	// A narration-only resume still needs the flow it will write over.
	if pricing.SkipFlow && (recovery.Plan == "" || recovery.PlanDigest != planRecoveryDigest(p) || !recovery.FlowReady && !recovery.PlanReady) {
		return ErrQuoteChanged
	}
	if !pricing.SkipFlow {
		recovery.Plan, recovery.PlanDigest, recovery.PlanReady, recovery.FlowReady = "", "", false, false
	}
	checkpoint.TotalSources = len(b.Sources)
	s.checkpoint(ctx, user, project, checkpoint)
	set := func(name string, done, total int) {
		stage = name
		checkpoint.Stage = name
		slog.Info("clip work progress", "job", job, "stage", name, "completed", done, "total", total)
		if name == "prepare" {
			checkpoint.Diagnostic.Values = map[string]int{"source": min(done+1, total)}
		}
		if name != "cleanup" {
			s.checkpoint(ctx, user, project, checkpoint)
		}
		if progress != nil {
			progress(name, done, total)
		}
	}
	if !pricing.RenderOnly() {
		if err := s.planner.ValidateModels(pricing.Observe.Ref, pricing.Plan.Ref); err != nil {
			return admissionRefusal(pricing.Observe.Ref, err)
		}
		if s.planner.Budgets() != (CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Flow: pricing.Plan.CompletionTokens, Narration: pricing.Narration.CompletionTokens}) {
			return ErrQuoteChanged
		}
		declared := make([]AnalysisSource, 0, len(b.Sources))
		for _, v := range b.Sources {
			declared = append(declared, AnalysisSource{RenderSource: RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: MediaInfo{DurationMS: v.DurationMS, Width: v.Width, Height: v.Height}}, Filename: v.Filename})
		}
		if err := s.planner.ValidatePreparation(pricing.Observe.Ref, PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA, Instruction: p.Instruction, Policy: pricing.Plan}, declared); err != nil {
			return err
		}
	}
	if validator, ok := s.renderer.(interface {
		ValidateAuthoredInput(context.Context, PlanningInput) error
	}); ok {
		if err := validator.ValidateAuthoredInput(ctx, PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS}); err != nil {
			return err
		}
	}
	var analysisJSON []byte
	var planJSON string
	var result Result
	// Every media command is timed, the successful ones included, and each
	// record is attributed to the stage running when it started, so a stage
	// total is explained by the operations inside it (CLIP-88).
	ctx = WithMediaStageObserver(ctx, func(r MediaRecord) {
		if r.Operation != "" {
			slog.Info("clip media operation", "job", job, "stage", stage, "operation", r.Operation, "outcome", r.Outcome, "elapsed_ms", r.Elapsed.Milliseconds())
			return
		}
		slog.Info("clip render substage", "job", job, "stage", stage, "substage", r.Substage, "elapsed_ms", r.Elapsed.Milliseconds())
	})
	err = s.media.WithWorkspace(ctx, job, func(ws MediaWorkspace) error {
		set("prepare", 0, len(b.Sources))
		sources, prepared, err := s.prepareRecoveredBatch(ctx, ws, b, recovery, func(n int) { set("prepare", n, len(b.Sources)) })
		if err != nil {
			return err
		}
		count := 0
		for _, v := range prepared {
			if v.reused == nil {
				count++
			}
		}
		checkpoint.TotalChunks = len(prepared)
		recovery.Sources, recovery.Pricing = sources, pricing
		if err := s.saveRecovery(ctx, user, project, recovery); err != nil {
			return err
		}
		checkpoint.Observations = make([]SourceAnalysis, len(sources))
		for i, source := range sources {
			checkpoint.Observations[i].Source = source
		}
		if count > pricing.ObservationCalls {
			return ErrQuoteChanged
		}
		if !pricing.RenderOnly() {
			if err = s.planner.ValidateModels(pricing.Observe.Ref, pricing.Plan.Ref); err != nil {
				return admissionRefusal(pricing.Observe.Ref, err)
			}
			if s.planner.Budgets() != (CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Flow: pricing.Plan.CompletionTokens, Narration: pricing.Narration.CompletionTokens}) {
				return ErrQuoteChanged
			}
			if err = s.planner.ValidatePreparation(pricing.Observe.Ref, PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA, Instruction: p.Instruction, Policy: pricing.Plan}, sources); err != nil {
				return err
			}
			if s.pricing == nil {
				return ErrPricingUnavailable
			}
			// The same qualification the quote ran, on the current document: a leaf
			// that drifted refuses here, before the reservation and any model call.
			current, err := s.freezeWork(ctx, pricing.Observe.Ref, pricing.Plan.Ref, count, pricing.SkipFlow, pricing.SkipNarration, pricing.Plan.ResponseRetries)
			if err != nil {
				return admissionRefusal(pricing.Observe.Ref, err)
			}
			if !current.Valid() || current.ObservationCalls != count || current.Observe != pricing.Observe || current.Plan != pricing.Plan || current.Narration != pricing.Narration || current.MaxCredits > p.Approval.MaxCredits {
				return ErrQuoteChanged
			}
		}
		// All source/proxy validation and price/profile checks precede the only
		// reservation. Subsequent calls can consume only this exact allowance.
		admitted, err := s.jobs.ReserveApproved(ctx, user, job, *p.Approval, count)
		if err != nil {
			return err
		}
		ctx = admitted
		set("analyze", 0, len(prepared))
		chunks := make([]ChunkAnalysis, 0, len(prepared))
		for index, v := range prepared {
			checkpoint.Diagnostic.Values = map[string]int{"source": v.source + 1, "chunk": index + 1}
			if err := s.checkpoint(ctx, user, project, checkpoint); err != nil {
				return err
			}
			slog.Info("clip analysis started", "job", job, "source", v.source+1, "chunk", index+1, "total", len(prepared), "reused", v.reused != nil)
			var observation ChunkAnalysis
			var err error
			if v.reused != nil {
				observation = *v.reused
			} else {
				observeCtx := WithResponseCorrectionObserver(ctx, func(retry, limit int, d AttemptDiagnostic) error {
					checkpoint.Diagnostic = d
					checkpoint.Diagnostic.Values["retry"] = retry
					logAttemptDiagnostic(job, "analyze", d)
					if progress != nil {
						progress("analyze_retry", retry, limit)
					}
					return s.checkpoint(ctx, user, project, checkpoint)
				})
				observation, err = s.observe(observeCtx, v.chunk, sources[v.source], pricing.Observe, p.Language)
			}
			if err != nil {
				return admissionRefusal(pricing.Observe.Ref, err)
			}
			// The observer validates its chunk; check the identity and range again
			// before making it durable or allowing the next paid call.
			source := sources[v.source]
			if observation.SourceID != source.ID || observation.Fingerprint != source.Fingerprint || observation.Index != v.chunk.Index || observation.OffsetMS != v.chunk.OffsetMS || observation.DurationMS != v.chunk.DurationMS {
				return ErrInvalid
			}
			if err := ValidateSegments(s.cfg.Analysis, observation.Segments, observation.OffsetMS, observation.OffsetMS+observation.DurationMS); err != nil {
				return err
			}
			if v.reused == nil {
				recovery.Chunks = append(recovery.Chunks, observation)
				if err := s.saveRecovery(ctx, user, project, recovery); err != nil {
					return err
				}
			}
			chunks = append(chunks, observation)
			checkpoint.Observations[v.source].Segments = append(checkpoint.Observations[v.source].Segments, observation.Segments...)
			checkpoint.CompletedChunks = len(chunks)
			if index+1 == len(prepared) || prepared[index+1].source != v.source {
				checkpoint.CompletedSources++
			}
			set("analyze", len(chunks), len(prepared))
		}
		analyses, err := MergeAnalyses(s.cfg.Analysis, sources, chunks)
		if err != nil {
			return err
		}
		checkpoint.Diagnostic = AttemptDiagnostic{}
		// The writer's own context: one retry observer per stage, so a
		// correction says which of the two calls is being corrected.
		writing := func(stage string) context.Context {
			return WithResponseCorrectionObserver(ctx, func(retry, limit int, d AttemptDiagnostic) error {
				checkpoint.Diagnostic = d
				checkpoint.Diagnostic.Values["retry"] = retry
				logAttemptDiagnostic(job, stage, d)
				if progress != nil {
					progress(stage+"_retry", retry, limit)
				}
				return s.checkpoint(ctx, user, project, checkpoint)
			})
		}
		keep := func(edit *EditPlan, flowReady, planReady bool) error {
			raw, err := EncodeEditPlan(*edit)
			if err != nil {
				return err
			}
			recovery.Plan, recovery.PlanDigest = raw, planRecoveryDigest(p)
			recovery.FlowReady, recovery.PlanReady = flowReady, planReady
			return s.saveRecovery(ctx, user, project, recovery)
		}
		in := PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Analyses: analyses, Policy: pricing.Plan, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA, Instruction: p.Instruction, SourceAudio: batchSourceAudio(b)}
		set("flow", 0, 1)
		if err := s.checkpoint(ctx, user, project, checkpoint); err != nil {
			return err
		}
		var edit EditPlan
		if pricing.SkipFlow {
			edit, err = DecodeEditPlan(recovery.Plan)
		} else if p.Composition != nil {
			// A composition is written by the flow call and then the narration
			// over it (CLIP-135); the single writer is what a payload with no
			// composition snapshot still uses.
			edit, _, err = s.planner.Flow(writing("flow"), pricing.Plan.Ref, in)
		} else {
			edit, _, err = s.planner.Plan(writing("flow"), pricing.Plan.Ref, in)
		}
		if err != nil {
			return err
		}
		if edit.Portable == nil {
			edit = edit.WithFacts(p.Disclosure, p.Answers, p.Template.Preset, p.CTA, p.Template.Accent, p.HideDisclosure)
		}
		// The project's caption pace and accent are render inputs too: a plan
		// carries what was written, the project says how it is shown (CLIP-139).
		edit = edit.WithCaptions(p.CaptionPace, p.Accent)
		// The owner's source-sound choice is the SERVER's to state, and it is
		// stated only here — after the model's own output has been validated, so
		// no prompt, response or plan digest ever carried it, and a resumed plan
		// takes the setting as it stands now (CLIP-100, CDS-6).
		edit.SourceAudio = FreezeSourceAudio(b, edit.Cuts)
		// The written flow is kept before the narration is asked for, so a
		// failure or a cancellation in the narration resumes on it and pays for
		// one writing call rather than two (CLIP-87, CLIP-93).
		if !pricing.SkipFlow {
			// The flow is written either way: a payload with no composition gets
			// its whole plan from the one writer, and is ready to render at once.
			if err := keep(&edit, true, edit.Portable == nil); err != nil {
				return err
			}
		}
		if !pricing.SkipNarration && edit.Portable != nil {
			set("narrate", 0, 1)
			if err := s.checkpoint(ctx, user, project, checkpoint); err != nil {
				return err
			}
			edit, _, err = s.planner.Narrate(writing("narrate"), pricing.Narration.Ref, NarrationInput{PlanningInput: in, Flow: edit})
			if err != nil {
				return err
			}
			edit.SourceAudio = FreezeSourceAudio(b, edit.Cuts)
			if err := keep(&edit, true, false); err != nil {
				return err
			}
		}
		checkpoint.Diagnostic = AttemptDiagnostic{Ranges: AttemptRangeDiagnostics(edit, analyses), Values: map[string]int{"cut_count": len(edit.Cuts), "target_ms": p.TargetDurationMS, "after_ms": edit.DurationMS, "transition_ms": edit.TransitionTotal()}}
		renderSources := make([]RenderSource, len(sources))
		for i, v := range sources {
			renderSources[i] = v.RenderSource
		}
		if edit.Portable != nil {
			if layout, ok := s.renderer.(interface {
				LayoutComposition(context.Context, EditPlan, []RenderSource) (EditPlan, []CompositionElement, error)
			}); ok {
				set("narrate", 0, 1)
				edit, _, err = layout.LayoutComposition(ctx, edit, renderSources)
				if err != nil {
					return err
				}
				edit.SourceAudio = FreezeSourceAudio(b, edit.Cuts)
				recovery.Plan, err = EncodeEditPlan(edit)
				if err != nil {
					return err
				}
				recovery.PlanReady = true
				if err := s.saveRecovery(ctx, user, project, recovery); err != nil {
					return err
				}
			}
		}
		set("render", 0, 1)
		logAttemptDiagnostic(job, "render", checkpoint.Diagnostic)
		// The retained composition owns every visible element. Only old queued
		// payloads without that contract enter the compatibility compositor.
		load, releaseSource := s.renderLoader(ws, func(id string) (SourceLease, MediaInfo, bool) {
			for i, v := range b.Sources {
				if v.ID == id {
					return v, sources[i].Info, true
				}
			}
			return SourceLease{}, MediaInfo{}, false
		}, pricing.ReusedChunks > 0)
		if edit.Portable == nil {
			edit = edit.WithFacts(p.Disclosure, p.Answers, p.Template.Preset, p.CTA, p.Template.Accent, p.HideDisclosure)
		}
		video, err := s.renderer.Render(ctx, ws, edit, renderSources, load)
		if err = errors.Join(err, releaseSource()); err != nil {
			return err
		}
		recovery.PlanReady = true
		if video.Plan != nil {
			edit = *video.Plan
		}
		recovery.Plan, err = EncodeEditPlan(edit)
		if err != nil {
			return err
		}
		if err := s.saveRecovery(ctx, user, project, recovery); err != nil {
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
		if edit.Portable == nil && p.Composition != nil && p.Composition.Snapshot.Legacy {
			projectSnapshot := Project{VideoTemplateID: p.Composition.Snapshot.TemplateID, Answers: p.Answers, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA}
			edit.Portable, err = FreezeLegacyPlan(projectSnapshot, edit, p.Template, s.projects.limits.Composition)
			if err != nil {
				return err
			}
		}
		// The owner owns source sound, not the writer: the snapshot is taken
		// from the live leases, so a reselected source keeps the choice already
		// made about it and a new one stays silent (CLIP-18, CLIP-100).
		edit.SourceAudio = FreezeSourceAudio(b, edit.Cuts)
		planJSON, err = EncodeEditPlan(edit)
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
	if err = s.finisher.Complete(ctx, AttemptResult{JobID: job, UserID: user, ProjectID: project, ExpectedRevision: currentProject.EditPlanRevision, Analysis: string(analysisJSON), EditPlan: planJSON, Result: result}); err != nil {
		return err
	}
	set("cleanup", 0, 1)
	return nil
}

type preparedChunk struct {
	reused *ChunkAnalysis
	source int
	chunk  AnalysisChunk
}

func (s *GenerationService) prepareBatch(ctx context.Context, ws MediaWorkspace, b SourceBatch, progress func(int)) ([]AnalysisSource, []preparedChunk, error) {
	return s.prepareRecoveredBatch(ctx, ws, b, RecoveryState{}, progress)
}
func (s *GenerationService) prepareRecoveredBatch(ctx context.Context, ws MediaWorkspace, b SourceBatch, recovery RecoveryState, progress func(int)) ([]AnalysisSource, []preparedChunk, error) {
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
		if cached, ok := recoverySource(recovery, v.ID); ok {
			n := (cached.Info.DurationMS + 59999) / 60000
			complete := true
			for index := 0; index < n; index++ {
				complete = complete && recoveryChunk(recovery, v.ID, index) != nil
			}
			if complete {
				probed = append(probed, ProbedSource{Metadata: v.SourceMetadata, Info: cached.Info})
				var err error
				count, err = ValidateProbedSources(s.cfg.Media, probed)
				if err != nil {
					return nil, nil, err
				}
				sources = append(sources, cached)
				for index := 0; index < n; index++ {
					c := recoveryChunk(recovery, v.ID, index)
					prepared = append(prepared, preparedChunk{source: i, chunk: AnalysisChunk{SourceID: c.SourceID, Fingerprint: c.Fingerprint, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}, reused: c})
				}
				progress(i + 1)
				continue
			}
		}
		err := s.withSource(ctx, ws, v, MediaInfo{}, func(source MediaSource) error {
			// The container read is the cheap one and decides how many analysis
			// copies this source needs; producing those copies decodes it once and
			// settles its real length below (CLIP-33, CLIP-124).
			info, err := s.probeSource(ctx, ws, source.Path)
			if err != nil {
				return err
			}
			claimed := info.DurationMS
			probed = append(probed, ProbedSource{Metadata: v.SourceMetadata, Info: info})
			count, err = ValidateProbedSources(s.cfg.Media, probed)
			if err != nil {
				return err
			}
			source.Info = info
			input := AnalysisSource{RenderSource: RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: info}, Filename: v.Filename}
			sources = append(sources, input)
			next := 0
			consume := func(c AnalysisChunk) error {
				if old := recoveryChunk(recovery, v.ID, c.Index); old != nil && old.OffsetMS == c.OffsetMS && old.DurationMS == c.DurationMS {
					if c.Index != next {
						return ErrInvalidMedia
					}
					next++
					prepared = append(prepared, preparedChunk{source: i, chunk: c, reused: old})
					return nil
				}
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
			}
			verified := MediaInfo{}
			if media, ok := s.media.(interface {
				PrepareAnalysisChunksExcept(context.Context, MediaWorkspace, MediaSource, func(int) bool, func(AnalysisChunk) error) (MediaInfo, error)
			}); ok {
				verified, err = media.PrepareAnalysisChunksExcept(ctx, ws, source, func(i int) bool { return recoveryChunk(recovery, v.ID, i) != nil }, consume)
			} else {
				verified, err = s.media.PrepareAnalysisChunks(ctx, ws, source, consume)
			}
			if err != nil {
				return err
			}
			// The decode has now confirmed or refused the container's claim. A
			// claim that would have produced a different set of copies is refused
			// as invalid media, exactly as an unreadable source is; anything else
			// carries the measured length into observation and planning.
			if verified.DurationMS <= 0 || chunkCount(s.cfg.Media, verified.DurationMS) != chunkCount(s.cfg.Media, claimed) {
				return ErrInvalidMedia
			}
			probed[len(probed)-1].Info = verified
			if _, err = ValidateProbedSources(s.cfg.Media, probed); err != nil {
				return err
			}
			sources[len(sources)-1].Info = verified
			return nil
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
		if p.reused != nil {
			continue
		}
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

// probeSource reads only what a container claims when the adapter can, so the
// decode that produces the analysis copies is the source's only decode.
func (s *GenerationService) probeSource(ctx context.Context, ws MediaWorkspace, path string) (MediaInfo, error) {
	if media, ok := s.media.(interface {
		ProbeContainer(context.Context, MediaWorkspace, string) (MediaInfo, error)
	}); ok {
		return media.ProbeContainer(ctx, ws, path)
	}
	return s.media.Probe(ctx, ws, path)
}

// chunkCount is how many analysis copies a length of footage needs — the same
// walk PrepareAnalysisChunks takes.
func chunkCount(cfg MediaConfig, durationMS int) int {
	if cfg.ChunkDurationMS <= 0 {
		return 0
	}
	return (durationMS + cfg.ChunkDurationMS - 1) / cfg.ChunkDurationMS
}
