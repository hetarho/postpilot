package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/postpilot/backend/internal/clip"
	jobctx "github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

type GenerationService struct {
	previewOwners sync.Map
	remoteMedia   *MediaDispatch
	store         clip.GenerationStore
	projects      *Service
	sources       *SourceService
	objects       clip.ProcessingObjects
	media         clip.Media
	planner       clip.Planner
	renderer      clip.Renderer
	jobs          clip.GenerationJobs
	finisher      clip.ClipFinisher
	cfg           clip.GenerationConfig
	pricing       clip.QuotePricing
	accounting    clip.AccountingReader
	admission     clip.AnalysisAdmission
	now           func() time.Time
}

// GenerationDeps are the collaborators the generation side reaches other contexts
// through (ARCH-40). All four are required: the finisher commits a result with its job,
// the pricing and accounting sit on the credit path, and the admission answers
// eligibility — a service missing any of them refuses or misreports rather than runs.
type GenerationDeps struct {
	// Nil is the temporary embedded rollout path; removed by T387.
	RemoteMedia *MediaDispatch
	Finisher    clip.ClipFinisher
	Pricing     clip.QuotePricing
	Accounting  clip.AccountingReader
	Admission   clip.AnalysisAdmission
}

func NewGenerationService(store clip.GenerationStore, projects *Service, sources *SourceService, objects clip.ProcessingObjects, media clip.Media, planner clip.Planner, renderer clip.Renderer, jobs clip.GenerationJobs, cfg clip.GenerationConfig, deps GenerationDeps) *GenerationService {
	if cfg.ReadTTL <= 0 || cfg.CleanupTimeout <= 0 || cfg.OrphanMinAge <= 0 {
		panic("invalid clip generation configuration")
	}
	if deps.Finisher == nil || deps.Pricing == nil || deps.Accounting == nil || deps.Admission == nil {
		panic("clip: finisher, pricing, accounting and admission are required")
	}
	s := &GenerationService{remoteMedia: deps.RemoteMedia, store: store, projects: projects, sources: sources, objects: objects, media: media, planner: planner, renderer: renderer, jobs: jobs, cfg: cfg, now: time.Now,
		finisher: deps.Finisher, pricing: deps.Pricing, accounting: deps.Accounting, admission: deps.Admission}
	// The project service and its generation side need each other; the pair is closed
	// here, where both exist, instead of through a setter the composition root could forget.
	if projects != nil {
		projects.generation = s
	}
	return s
}

func modelRef(s string) llm.ModelRef {
	p, m, _ := strings.Cut(s, "/")
	return llm.ModelRef{ProviderID: p, ModelID: m}
}

func (s *GenerationService) Start(ctx context.Context, user, id, batch, observe, write string, approval ...clip.QuoteApproval) (string, error) {
	if len(approval) != 1 {
		return "", clip.ErrQuoteRequired
	}
	return s.startApproved(ctx, user, id, batch, observe, write, approval[0])
}

func (s *GenerationService) enqueue(ctx context.Context, input clip.GenerationStart, batch string, revision int) (string, error) {
	user := input.UserID
	job, err := s.jobs.Enqueue(ctx, input)
	if err != nil {
		return "", err
	}
	// A queued row is invisible to the dispatcher until its source lease is linked.
	if input.RenderOnly {
		err = s.store.LinkRenderSourceJob(ctx, user, batch, job, revision, s.now())
	} else if input.Revise && input.Quote != nil {
		// A revision's quote binds the saved plan and the owner's words, which
		// StartRevision has just checked; the link consumes it and renews the
		// originals' retention (CLIP-73).
		store, ok := s.store.(clip.RevisionQuoteStore)
		if !ok {
			err = clip.ErrQuoteRequired
		} else {
			err = store.LinkRevisionJob(ctx, *input.Quote, job, s.now())
		}
	} else if input.Quote != nil {
		store, ok := s.store.(clip.QuoteStore)
		if !ok {
			err = clip.ErrQuoteRequired
		} else {
			err = store.LinkApprovedSourceJob(ctx, *input.Quote, job, s.now())
		}
	} else {
		err = clip.ErrQuoteRequired
	}
	// Between the link and the activation: the job row is still invisible to the
	// dispatcher, so a record that cannot be written fails the start instead of
	// letting a clip run with nothing saying what it was asked for (CLIP-133).
	if err == nil && input.Request != nil {
		err = s.projects.store.RecordProjectRequest(ctx, newID(), user, input.ProjectID, *input.Request)
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

// generationRun is one attempt of the generation job: the accepted payload and batch,
// the quote it runs under, the recovery it resumes from, and the checkpoint the owner
// reads while it works. The stages run in order and each leaves its result on the run.
type generationRun struct {
	s                  *GenerationService
	ctx                context.Context
	user, job, project string
	progress           func(string, int, int)
	stage              string
	checkpoint         clip.AttemptCheckpoint
	p                  clip.GenerationPayload
	b                  clip.SourceBatch
	pricing            clip.GenerationPricing
	recovery           clip.RecoveryState
	sources            []clip.AnalysisSource
	prepared           []preparedChunk
	analyses           []clip.SourceAnalysis
	edit               clip.EditPlan
	analysisJSON       []byte
	planJSON           string
}

// Run is the generation job: accept the approved payload, prepare the sources, observe
// every chunk, write the flow and the narration over it, save the plan, and commit it
// with the job. The attempt ends on the validated plan — no media is rendered and no
// result file is produced; the owner starts the render they want (CLIP-151).
func (s *GenerationService) Run(ctx context.Context, user, job, project string, payload []byte, progress func(string, int, int)) (err error) {
	if s.finisher == nil {
		return clip.ErrCompositionUnavailable
	}
	currentProject, err := s.projects.store.GetProject(ctx, user, project)
	if err != nil {
		return err
	}
	r := &generationRun{s: s, ctx: ctx, user: user, job: job, project: project, progress: progress, stage: "prepare", checkpoint: clip.AttemptCheckpoint{Version: 1, JobID: job, Stage: "prepare"}}
	// The failure is recorded after it has been wrapped with its stage (defers run last
	// registered first), so the checkpoint names the stage the diagnostic belongs to.
	defer func() {
		if err != nil && !errors.Is(err, jobctx.ErrYield) {
			r.recordFailure(ctx, err)
		}
	}()
	defer func() {
		if err != nil && !errors.Is(err, jobctx.ErrYield) {
			err = &clip.StageFailure{Stage: r.stage, Cause: err}
		}
	}()
	if err := r.accept(payload); err != nil {
		return err
	}
	r.checkpoint.TotalSources = len(r.b.Sources)
	s.checkpoint(ctx, user, project, r.checkpoint)
	r.ctx = r.observeMedia(ctx)
	continueAI := func() error {
		if err := r.analyze(); err != nil {
			return err
		}
		if err := r.write(); err != nil {
			return err
		}
		return r.save()
	}
	if s.remoteMedia != nil {
		if err = r.prepareRemote(currentProject.EditPlanRevision); err == nil {
			err = continueAI()
		}
	} else {
		err = s.media.WithWorkspace(r.ctx, job, func(ws clip.MediaWorkspace) error {
			if err := r.prepare(ws); err != nil {
				return err
			}
			return continueAI()
		})
	}
	if err != nil {
		return err
	}
	return r.finish(currentProject)
}

// recordFailure folds the failing stage and its diagnostic into the durable checkpoint.
func (r *generationRun) recordFailure(ctx context.Context, err error) {
	r.checkpoint.Stage = r.stage
	if d, ok := clip.DiagnosticFromError(err); ok {
		d.Values = clip.SafeAttemptValues(d.Values)
		if len(d.Ranges) == 0 {
			d.Ranges = r.checkpoint.Diagnostic.Ranges
		}
		for _, key := range []string{"target_ms", "after_ms", "cut_count", "transition_ms", "reused_chunks", "remaining_chunks", "reused_plan"} {
			if n, ok := r.checkpoint.Diagnostic.Values[key]; ok {
				d.Values[key] = n
			}
		}
		if r.stage == "analyze" {
			// The worker owns the global source/chunk position; parser indexes
			// describe a segment inside that chunk and cannot replace it.
			for _, key := range []string{"source", "chunk"} {
				if value, exists := r.checkpoint.Diagnostic.Values[key]; exists {
					d.Values[key] = value
				}
			}
		}
		r.checkpoint.Diagnostic = d
	} else {
		var output interface{ OutputValidationCode() string }
		if errors.As(err, &output) {
			r.checkpoint.Diagnostic.Check = output.OutputValidationCode()
		}
	}
	logAttemptDiagnostic(r.job, r.stage, r.checkpoint.Diagnostic)
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.s.cfg.CleanupTimeout)
	defer cancel()
	r.s.checkpoint(recordCtx, r.user, r.project, r.checkpoint)
}

// set advances the stage the owner sees and the worker's progress.
func (r *generationRun) set(name string, done, total int) {
	r.stage = name
	r.checkpoint.Stage = name
	slog.Info("clip work progress", "job", r.job, "stage", name, "completed", done, "total", total)
	if name == "prepare" {
		r.checkpoint.Diagnostic.Values = map[string]int{"source": min(done+1, total)}
	}
	if name != "cleanup" {
		r.s.checkpoint(r.ctx, r.user, r.project, r.checkpoint)
	}
	if r.progress != nil {
		r.progress(name, done, total)
	}
}

// planningInput is the frozen brief every planner call reads.
func (r *generationRun) planningInput() clip.PlanningInput {
	p := r.p
	return clip.PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA, Instruction: p.Instruction, Design: p.Design(), Policy: r.pricing.Plan}
}

// validatePreparation is the planner's own check of the models, the budgets and the
// brief, run before the workspace opens and again over the prepared sources.
func (r *generationRun) validatePreparation(declared []clip.AnalysisSource) error {
	s, pricing := r.s, r.pricing
	if err := s.planner.ValidateModels(pricing.Observe.Ref, pricing.Plan.Ref); err != nil {
		return admissionRefusal(pricing.Observe.Ref, err)
	}
	if s.planner.Budgets() != (clip.CompletionBudgets{Observe: pricing.Observe.CompletionTokens, Flow: pricing.Plan.CompletionTokens, Narration: pricing.Narration.CompletionTokens}) {
		return clip.ErrQuoteChanged
	}
	return s.planner.ValidatePreparation(pricing.Observe.Ref, r.planningInput(), declared)
}

// accept decodes the durable payload and proves it is the approved work: the batch
// it names, the quote it consumed, the recovery it may resume, and a brief the planner
// and the renderer accept. Nothing here opens a workspace or calls a model.
func (r *generationRun) accept(payload []byte) error {
	s, ctx := r.s, r.ctx
	// Resolve cleanup from the durable linkage, even if payload decoding fails.
	b, err := s.store.BatchForJob(ctx, r.user, r.job)
	if err != nil {
		return err
	}
	var p clip.GenerationPayload
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.DisallowUnknownFields()
	if dec.Decode(&p) != nil || dec.Decode(new(any)) != io.EOF {
		return clip.ErrInvalid
	}
	if err := s.checkComposition(p.Composition); err != nil {
		return err
	}
	if p.Template.CompositionBody != "" && !p.Template.CompositionLegacy && p.Composition == nil {
		return clip.ErrInvalid
	}
	if !clip.SupportedGenerationPayload(p.Version) || (p.Version >= 4 && p.Composition == nil) || p.Approval == nil {
		return clip.ErrQuoteRequired
	}
	if p.ProjectID != r.project || p.Batch.ProjectID != r.project || p.Batch.ID != b.ID || p.Batch.UserID != r.user || !clip.SameSourceManifest(p.Batch.Sources, b.Sources) {
		return clip.ErrInvalid
	}
	if b.State != "consuming" || b.ProjectID != r.project {
		return clip.ErrSourceState
	}
	pricing := p.Approval.Pricing
	if !pricing.Valid() || pricing.Observe.Ref != modelRef(p.Observe) || pricing.Plan.Ref != modelRef(p.Write) || p.Approval.MaxCredits != pricing.MaxCredits {
		return clip.ErrPricingUnavailable
	}
	quoteStore, ok := s.store.(clip.QuoteStore)
	if !ok {
		return clip.ErrQuoteRequired
	}
	q, err := quoteStore.GetQuote(ctx, r.user, p.Approval.QuoteID)
	if err != nil {
		return err
	}
	// Expiry is enforced when the quote is consumed, not after a long, already
	// accepted preparation. Reconfirm the durable ownership and exact approval.
	if q.ConsumedJobID != r.job || q.ProjectID != r.project || q.BatchID != b.ID || q.Pricing != pricing {
		return clip.ErrQuoteChanged
	}
	recovery := s.selectRecovery(p.Recovery, b, pricing.Observe.Ref, p.Language)
	recovery.JobID = r.job
	if p.Recovery != nil && len(recovery.Chunks) != pricing.ReusedChunks {
		return clip.ErrQuoteChanged
	}
	if pricing.SkipNarration && (!recovery.PlanReady || recovery.Plan == "" || recovery.PlanDigest != planRecoveryDigest(p)) {
		return clip.ErrQuoteChanged
	}
	// A narration-only resume still needs the flow it will write over.
	if pricing.SkipFlow && (recovery.Plan == "" || recovery.PlanDigest != planRecoveryDigest(p) || !recovery.FlowReady && !recovery.PlanReady) {
		return clip.ErrQuoteChanged
	}
	if !pricing.SkipFlow {
		recovery.Plan, recovery.PlanDigest, recovery.PlanReady, recovery.FlowReady = "", "", false, false
	}
	r.p, r.b, r.pricing, r.recovery = p, b, pricing, recovery
	if !pricing.RenderOnly() {
		declared := make([]clip.AnalysisSource, 0, len(b.Sources))
		for _, v := range b.Sources {
			declared = append(declared, clip.AnalysisSource{RenderSource: clip.RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: clip.MediaInfo{DurationMS: v.DurationMS, Width: v.Width, Height: v.Height}}, Filename: v.Filename})
		}
		if err := r.validatePreparation(declared); err != nil {
			return err
		}
	}
	if validator, ok := s.renderer.(interface {
		ValidateAuthoredInput(context.Context, clip.PlanningInput) error
	}); ok {
		if err := validator.ValidateAuthoredInput(ctx, clip.PlanningInput{Language: p.Language, Composition: p.Composition, Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Design: p.Design()}); err != nil {
			return err
		}
	}
	return nil
}

// observeMedia times every media command and attributes it to the stage running when
// it started, so a stage total is explained by the operations inside it (CLIP-88).
func (r *generationRun) observeMedia(ctx context.Context) context.Context {
	return clip.WithMediaStageObserver(ctx, func(rec clip.MediaRecord) {
		if rec.Operation != "" {
			slog.Info("clip media operation", "job", r.job, "stage", r.stage, "operation", rec.Operation, "outcome", rec.Outcome, "elapsed_ms", rec.Elapsed.Milliseconds())
			return
		}
		slog.Info("clip render substage", "job", r.job, "stage", r.stage, "substage", rec.Substage, "elapsed_ms", rec.Elapsed.Milliseconds())
	})
}

// prepare proxies every source (reusing what the recovery holds), re-qualifies the
// models on the current documents and takes the one credit reservation. Every source,
// proxy, price and profile check precedes that reservation; later calls can consume
// only this exact allowance.
func (r *generationRun) prepare(ws clip.MediaWorkspace) error {
	s, ctx := r.s, r.ctx
	r.set("prepare", 0, len(r.b.Sources))
	sources, prepared, err := s.prepareRecoveredBatch(ctx, ws, r.b, r.recovery, func(n int) { r.set("prepare", n, len(r.b.Sources)) })
	if err != nil {
		return err
	}
	r.sources, r.prepared = sources, prepared
	return r.admitPrepared()
}

// Every source/copy verdict precedes the shared quote and reservation checks.
func (r *generationRun) admitPrepared() error {
	s, ctx, pricing := r.s, r.ctx, r.pricing
	sources, prepared := r.sources, r.prepared
	count := 0
	for _, v := range prepared {
		if v.reused == nil {
			count++
		}
	}
	r.checkpoint.TotalChunks = len(prepared)
	r.recovery.Sources, r.recovery.Pricing = sources, pricing
	if err := s.saveRecovery(ctx, r.user, r.project, r.recovery); err != nil {
		return err
	}
	r.checkpoint.Observations = make([]clip.SourceAnalysis, len(sources))
	for i, source := range sources {
		r.checkpoint.Observations[i].Source = source
	}
	if count > pricing.ObservationCalls {
		return clip.ErrQuoteChanged
	}
	if !pricing.RenderOnly() {
		if err := r.validatePreparation(sources); err != nil {
			return err
		}
		if s.pricing == nil {
			return clip.ErrPricingUnavailable
		}
		// The same qualification the quote ran, on the current document: a leaf
		// that drifted refuses here, before the reservation and any model call.
		current, err := s.freezeWork(ctx, pricing.Observe.Ref, pricing.Plan.Ref, count, pricing.SkipFlow, pricing.SkipNarration, pricing.Plan.ResponseRetries)
		if err != nil {
			return admissionRefusal(pricing.Observe.Ref, err)
		}
		if !current.Valid() || current.ObservationCalls != count || current.Observe != pricing.Observe || current.Plan != pricing.Plan || current.Narration != pricing.Narration || current.MaxCredits > r.p.Approval.MaxCredits {
			return clip.ErrQuoteChanged
		}
	}
	admitted, err := s.jobs.ReserveApproved(ctx, r.user, r.job, *r.p.Approval, count)
	if err != nil {
		return err
	}
	r.ctx = admitted
	return nil
}

// analyze observes every prepared chunk that the recovery did not already hold, checks
// each observation against the chunk it was asked about, and merges them per source.
func (r *generationRun) analyze() error {
	s, ctx := r.s, r.ctx
	r.set("analyze", 0, len(r.prepared))
	chunks := make([]clip.ChunkAnalysis, 0, len(r.prepared))
	for index, v := range r.prepared {
		r.checkpoint.Diagnostic.Values = map[string]int{"source": v.source + 1, "chunk": index + 1}
		if err := s.checkpoint(ctx, r.user, r.project, r.checkpoint); err != nil {
			return err
		}
		slog.Info("clip analysis started", "job", r.job, "source", v.source+1, "chunk", index+1, "total", len(r.prepared), "reused", v.reused != nil)
		var observation clip.ChunkAnalysis
		var err error
		if v.reused != nil {
			observation = *v.reused
		} else {
			observation, err = s.observePrepared(r.correcting("analyze"), v, r.sources[v.source], r.pricing, r.p.Language)
		}
		if err != nil {
			return admissionRefusal(r.pricing.Observe.Ref, err)
		}
		// The observer validates its chunk; check the identity and range again
		// before making it durable or allowing the next paid call.
		source := r.sources[v.source]
		if observation.SourceID != source.ID || observation.Fingerprint != source.Fingerprint || observation.Index != v.chunk.Index || observation.OffsetMS != v.chunk.OffsetMS || observation.DurationMS != v.chunk.DurationMS {
			return clip.ErrInvalid
		}
		if err := clip.ValidateSegments(s.cfg.Analysis, observation.Segments, observation.OffsetMS, observation.OffsetMS+observation.DurationMS); err != nil {
			return err
		}
		if v.reused == nil {
			r.recovery.Chunks = append(r.recovery.Chunks, observation)
			if err := s.saveRecovery(ctx, r.user, r.project, r.recovery); err != nil {
				return err
			}
		}
		chunks = append(chunks, observation)
		r.checkpoint.Observations[v.source].Segments = append(r.checkpoint.Observations[v.source].Segments, observation.Segments...)
		r.checkpoint.CompletedChunks = len(chunks)
		if index+1 == len(r.prepared) || r.prepared[index+1].source != v.source {
			r.checkpoint.CompletedSources++
		}
		r.set("analyze", len(chunks), len(r.prepared))
	}
	analyses, err := clip.MergeAnalyses(s.cfg.Analysis, r.sources, chunks)
	if err != nil {
		return err
	}
	r.analyses = analyses
	r.checkpoint.Diagnostic = clip.AttemptDiagnostic{}
	return nil
}

// correcting is a model call's own context: one retry observer per stage, so a
// correction says which call is being corrected.
func (r *generationRun) correcting(stage string) context.Context {
	return clip.WithResponseCorrectionObserver(r.ctx, func(retry, limit int, d clip.AttemptDiagnostic) error {
		r.checkpoint.Diagnostic = d
		r.checkpoint.Diagnostic.Values["retry"] = retry
		logAttemptDiagnostic(r.job, stage, d)
		if r.progress != nil {
			r.progress(stage+"_retry", retry, limit)
		}
		return r.s.checkpoint(r.ctx, r.user, r.project, r.checkpoint)
	})
}

// keep saves the plan as written so far, so a failure or a cancellation resumes on it
// and pays for one writing call rather than two (CLIP-87, CLIP-93).
func (r *generationRun) keep(flowReady, planReady bool) error {
	raw, err := clip.EncodeEditPlan(r.edit)
	if err != nil {
		return err
	}
	r.recovery.Plan, r.recovery.PlanDigest = raw, planRecoveryDigest(r.p)
	r.recovery.FlowReady, r.recovery.PlanReady = flowReady, planReady
	return r.s.saveRecovery(r.ctx, r.user, r.project, r.recovery)
}

// write asks the writer for the flow (or resumes the kept one), then the narration over
// it (CLIP-135), and lays the composition out; the plan is kept after every call.
func (r *generationRun) write() error {
	s, ctx, p, pricing := r.s, r.ctx, r.p, r.pricing
	in := r.planningInput()
	in.Analyses, in.SourceAudio = r.analyses, batchSourceAudio(r.b)
	r.set("flow", 0, 1)
	if err := s.checkpoint(ctx, r.user, r.project, r.checkpoint); err != nil {
		return err
	}
	var err error
	if pricing.SkipFlow {
		r.edit, err = clip.DecodeEditPlan(r.recovery.Plan)
	} else if p.Composition != nil {
		// A composition is written by the flow call and then the narration
		// over it (CLIP-135); the single writer is what a payload with no
		// composition snapshot still uses.
		r.edit, _, err = s.planner.Flow(r.correcting("flow"), pricing.Plan.Ref, in)
	} else {
		r.edit, _, err = s.planner.Plan(r.correcting("flow"), pricing.Plan.Ref, in)
	}
	if err != nil {
		return err
	}
	if r.edit.Portable == nil {
		r.edit = r.edit.WithFacts(p.Disclosure, p.Answers, p.Template.Preset, p.CTA, p.Template.Accent, p.HideDisclosure)
	}
	// The project's caption pace and accent are render inputs too: a plan
	// carries what was written, the project says how it is shown (CLIP-139).
	r.edit = r.edit.WithDesign(p.Design())
	// The owner's source-sound choice is the SERVER's to state, and it is
	// stated only here — after the model's own output has been validated, so
	// no prompt, response or plan digest ever carried it, and a resumed plan
	// takes the setting as it stands now (CLIP-100, CDS-6).
	r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
	if !pricing.SkipFlow {
		// The flow is written either way: a payload with no composition gets
		// its whole plan from the one writer, and is ready to render at once.
		if err := r.keep(true, r.edit.Portable == nil); err != nil {
			return err
		}
	}
	if !pricing.SkipNarration && r.edit.Portable != nil {
		r.set("narrate", 0, 1)
		if err := s.checkpoint(ctx, r.user, r.project, r.checkpoint); err != nil {
			return err
		}
		r.edit, _, err = s.planner.Narrate(r.correcting("narrate"), pricing.Narration.Ref, clip.NarrationInput{PlanningInput: in, Flow: r.edit})
		if err != nil {
			return err
		}
		r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
		if err := r.keep(true, false); err != nil {
			return err
		}
	}
	r.checkpoint.Diagnostic = clip.AttemptDiagnostic{Ranges: clip.AttemptRangeDiagnostics(r.edit, r.analyses), Values: map[string]int{"cut_count": len(r.edit.Cuts), "target_ms": p.TargetDurationMS, "after_ms": r.edit.DurationMS, "transition_ms": r.edit.TransitionTotal()}}
	return r.layout()
}

// layout lets the renderer place the composition's elements when it can, and keeps
// the laid-out plan as the one ready to render.
func (r *generationRun) layout() error {
	if r.edit.Portable == nil {
		return nil
	}
	layout, ok := r.s.renderer.(interface {
		LayoutComposition(context.Context, clip.EditPlan, []clip.RenderSource) (clip.EditPlan, []clip.CompositionElement, error)
	})
	if !ok {
		return nil
	}
	renderSources := make([]clip.RenderSource, len(r.sources))
	for i, v := range r.sources {
		renderSources[i] = v.RenderSource
	}
	r.set("narrate", 0, 1)
	var err error
	r.edit, _, err = layout.LayoutComposition(r.ctx, r.edit, renderSources)
	if err != nil {
		return err
	}
	r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
	if r.recovery.Plan, err = clip.EncodeEditPlan(r.edit); err != nil {
		return err
	}
	r.recovery.PlanReady = true
	return r.s.saveRecovery(r.ctx, r.user, r.project, r.recovery)
}

// save encodes the validated plan and the merged analysis for the commit. The attempt
// ends HERE, on the plan: no media execution runs and no result file is produced; the
// owner reads this plan in ②'s draft preview and starts the render they want (CLIP-151).
func (r *generationRun) save() error {
	s, ctx, p := r.s, r.ctx, r.p
	r.set("save", 0, 1)
	logAttemptDiagnostic(r.job, "save", r.checkpoint.Diagnostic)
	r.recovery.PlanReady = true
	var err error
	if r.recovery.Plan, err = clip.EncodeEditPlan(r.edit); err != nil {
		return err
	}
	if err := s.saveRecovery(ctx, r.user, r.project, r.recovery); err != nil {
		return err
	}
	if r.analysisJSON, err = json.Marshal(r.analyses); err != nil {
		return err
	}
	if r.edit.Portable == nil && p.Composition != nil && p.Composition.Snapshot.Legacy {
		projectSnapshot := clip.Project{VideoTemplateID: p.Composition.Snapshot.TemplateID, Answers: p.Answers, Disclosure: p.Disclosure, HideDisclosure: p.HideDisclosure, CTA: p.CTA}
		if r.edit.Portable, err = clip.FreezeLegacyPlan(projectSnapshot, r.edit, p.Template, s.projects.limits.Composition); err != nil {
			return err
		}
	}
	// The owner owns source sound, not the writer: the snapshot is taken
	// from the live leases, so a reselected source keeps the choice already
	// made about it and a new one stays silent (CLIP-18, CLIP-100).
	r.edit.SourceAudio = clip.FreezeSourceAudio(r.b, r.edit.Cuts)
	r.planJSON, err = clip.EncodeEditPlan(r.edit)
	return err
}

// finish commits the plan with the job. A failed attempt never replaces the prior plan,
// and a successful one never replaces the previous result either: it advances the plan
// the render is asked for, which is what leaves that result standing as a stale one
// (CLIP-26, CLIP-152).
func (r *generationRun) finish(currentProject clip.Project) error {
	if err := r.s.finisher.Complete(r.ctx, clip.AttemptResult{JobID: r.job, UserID: r.user, ProjectID: r.project, ExpectedRevision: currentProject.EditPlanRevision, Analysis: string(r.analysisJSON), EditPlan: r.planJSON}); err != nil {
		return err
	}
	r.set("cleanup", 0, 1)
	return nil
}

type preparedChunk struct {
	video  *llm.InlineVideo
	reused *clip.ChunkAnalysis
	source int
	chunk  clip.AnalysisChunk
}

func (s *GenerationService) prepareBatch(ctx context.Context, ws clip.MediaWorkspace, b clip.SourceBatch, progress func(int)) ([]clip.AnalysisSource, []preparedChunk, error) {
	return s.prepareRecoveredBatch(ctx, ws, b, clip.RecoveryState{}, progress)
}

func (s *GenerationService) prepareRecoveredBatch(ctx context.Context, ws clip.MediaWorkspace, b clip.SourceBatch, recovery clip.RecoveryState, progress func(int)) ([]clip.AnalysisSource, []preparedChunk, error) {
	if len(b.Sources) < 1 || len(b.Sources) > s.cfg.Media.Sources.MaxCount {
		return nil, nil, clip.ErrInvalidMedia
	}
	var sources []clip.AnalysisSource
	var probed []clip.ProbedSource
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
				probed = append(probed, clip.ProbedSource{Metadata: v.SourceMetadata, Info: cached.Info})
				var err error
				count, err = clip.ValidateProbedSources(s.cfg.Media, probed)
				if err != nil {
					return nil, nil, err
				}
				sources = append(sources, cached)
				for index := 0; index < n; index++ {
					c := recoveryChunk(recovery, v.ID, index)
					prepared = append(prepared, preparedChunk{source: i, chunk: clip.AnalysisChunk{SourceID: c.SourceID, Fingerprint: c.Fingerprint, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}, reused: c})
				}
				progress(i + 1)
				continue
			}
		}
		err := s.withSource(ctx, ws, v, clip.MediaInfo{}, func(source clip.MediaSource) error {
			// The container read is the cheap one and decides how many analysis
			// copies this source needs; producing those copies decodes it once and
			// settles its real length below (CLIP-33, CLIP-124).
			info, err := s.probeSource(ctx, ws, source.Path)
			if err != nil {
				return err
			}
			claimed := info.DurationMS
			probed = append(probed, clip.ProbedSource{Metadata: v.SourceMetadata, Info: info})
			count, err = clip.ValidateProbedSources(s.cfg.Media, probed)
			if err != nil {
				return err
			}
			source.Info = info
			input := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: v.ID, Fingerprint: v.Fingerprint, Info: info}, Filename: v.Filename}
			sources = append(sources, input)
			next := 0
			consume := func(c clip.AnalysisChunk) error {
				if old := recoveryChunk(recovery, v.ID, c.Index); old != nil && old.OffsetMS == c.OffsetMS && old.DurationMS == c.DurationMS {
					if c.Index != next {
						return clip.ErrInvalidMedia
					}
					next++
					prepared = append(prepared, preparedChunk{source: i, chunk: c, reused: old})
					return nil
				}
				if c.SourceID != v.ID || c.Fingerprint != v.Fingerprint || c.Index != next || clip.ValidateChunkInput(s.cfg.Analysis, clip.ChunkInput{Source: input, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS}) != nil || filepath.Dir(c.Path) != ws.Path || filepath.Clean(c.Path) != c.Path || seen[c.Path] {
					return clip.ErrInvalidMedia
				}
				if c.Bytes <= 0 || c.Bytes > s.cfg.Media.AnalysisMaxBytes {
					return clip.ErrAnalysisTooLarge
				}
				if c.Bytes > s.cfg.Media.PreparedMaxBytes-totalBytes {
					return clip.ErrWorkspaceLimit
				}
				if c.Info.ContainerDurationMS <= 0 || c.Info.ContainerDurationMS > 60000 || c.Info.Width <= 0 || c.Info.Height <= 0 || max(c.Info.Width, c.Info.Height) > 720 {
					return clip.ErrInvalidMedia
				}
				file, err := os.Lstat(c.Path)
				if err != nil || !file.Mode().IsRegular() || file.Size() != c.Bytes {
					return clip.ErrInvalidMedia
				}
				seen[c.Path] = true
				totalBytes += c.Bytes
				prepared = append(prepared, preparedChunk{source: i, chunk: c})
				next++
				return nil
			}
			verified := clip.MediaInfo{}
			if media, ok := s.media.(interface {
				PrepareAnalysisChunksExcept(context.Context, clip.MediaWorkspace, clip.MediaSource, func(int) bool, func(clip.AnalysisChunk) error) (clip.MediaInfo, error)
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
				return clip.ErrInvalidMedia
			}
			probed[len(probed)-1].Info = verified
			if _, err = clip.ValidateProbedSources(s.cfg.Media, probed); err != nil {
				return err
			}
			sources[len(sources)-1].Info = verified
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
		if len(prepared) != count || count > 49 {
			return nil, nil, clip.ErrInvalidMedia
		}
		progress(i + 1)
	}
	// Before charging, ensure the remaining entire BoundedText workspace can fit,
	// including a reloaded original and render intermediates. This checks disk
	// availability, not an allocation. Other filesystem users can still consume
	// space later, so actual writes retain the live guard too.
	for _, p := range prepared {
		if p.reused != nil {
			continue
		}
		file, err := os.Lstat(p.chunk.Path)
		if err != nil || !file.Mode().IsRegular() || file.Size() != p.chunk.Bytes {
			return nil, nil, clip.ErrInvalidMedia
		}
	}
	if err := ws.CheckCapacity(s.cfg.Media.WorkspaceMaxBytes - totalBytes); err != nil {
		return nil, nil, err
	}
	return sources, prepared, nil
}

// probeSource reads only what a container claims when the adapter can, so the
// decode that produces the analysis copies is the source's only decode.
func (s *GenerationService) probeSource(ctx context.Context, ws clip.MediaWorkspace, path string) (clip.MediaInfo, error) {
	if media, ok := s.media.(interface {
		ProbeContainer(context.Context, clip.MediaWorkspace, string) (clip.MediaInfo, error)
	}); ok {
		return media.ProbeContainer(ctx, ws, path)
	}
	return s.media.Probe(ctx, ws, path)
}

// chunkCount is how many analysis copies a length of footage needs — the same
// walk PrepareAnalysisChunks takes.
func chunkCount(cfg clip.MediaConfig, durationMS int) int {
	if cfg.ChunkDurationMS <= 0 {
		return 0
	}
	return (durationMS + cfg.ChunkDurationMS - 1) / cfg.ChunkDurationMS
}
