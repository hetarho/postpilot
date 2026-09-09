package clip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"io"
	"log/slog"
	"net/url"
	"reflect"
	"strings"
	"time"
)

var ErrBusy = errors.New("clip busy")

type GenerationService struct {
	store    GenerationStore
	projects *Service
	sources  *SourceService
	objects  ProcessingObjects
	media    Media
	planner  Planner
	renderer Renderer
	jobs     GenerationJobs
	cfg      GenerationConfig
}

func NewGenerationService(store GenerationStore, projects *Service, sources *SourceService, objects ProcessingObjects, media Media, planner Planner, renderer Renderer, jobs GenerationJobs, cfg GenerationConfig) *GenerationService {
	if cfg.ReadTTL <= 0 || cfg.CleanupTimeout <= 0 || cfg.OrphanMinAge <= 0 {
		panic("invalid clip generation configuration")
	}
	return &GenerationService{store, projects, sources, objects, media, planner, renderer, jobs, cfg}
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
}

func modelRef(s string) llm.ModelRef {
	p, m, _ := strings.Cut(s, "/")
	return llm.ModelRef{ProviderID: p, ModelID: m}
}
func (s *GenerationService) Start(ctx context.Context, user, id, batch, observe, write string) (string, error) {
	p, err := s.projects.store.GetProject(ctx, user, id)
	if err != nil {
		return "", err
	}
	active, err := s.jobs.Active(ctx, user, id)
	if err != nil {
		return "", err
	}
	if active != nil {
		return "", ErrBusy
	}
	t, err := s.projects.store.GetTemplate(ctx, user, p.VideoTemplateID)
	if err != nil {
		return "", err
	}
	if err = RequiredAnswers(t, p); err != nil {
		return "", err
	}
	if err = s.planner.ValidateModels(modelRef(observe), modelRef(write)); err != nil {
		return "", err
	}
	b, err := s.store.GetSourceBatch(ctx, user, batch)
	if err != nil {
		return "", err
	}
	if b.ProjectID != id || b.State != "ready" || !time.Now().Before(b.ExpiresAt) || len(b.Sources) == 0 {
		return "", ErrSourceState
	}
	for _, v := range b.Sources {
		if v.State != "ready" || v.ActualBytes != v.Bytes {
			return "", ErrSourceState
		}
	}
	answers := make([]Answer, 0, len(t.InformationFields))
	for _, f := range t.InformationFields {
		for _, a := range p.Answers {
			if a.Label == f.Label {
				answers = append(answers, a)
				break
			}
		}
	}
	payload, err := json.Marshal(generationPayload{1, id, p.Ratio, observe, write, p.TargetDurationMS, t.Recipe, answers, b})
	if err != nil {
		return "", err
	}
	job, err := s.jobs.Enqueue(ctx, GenerationStart{user, id, observe, write, payload})
	if err != nil {
		return "", err
	}
	// A queued row is invisible to the dispatcher until its source lease is linked.
	if err = s.store.LinkSourceJob(ctx, user, batch, job, time.Now()); err == nil {
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
	case errors.As(e.Cause, &credits):
		f = llm.Failure{Reason: "INSUFFICIENT_CREDITS", Params: map[string]string{"required": fmt.Sprint(credits.Required), "balance": fmt.Sprint(credits.Balance), "renews_at": credits.RenewsAt.Format(time.RFC3339)}}
	case errors.Is(e.Cause, ErrInvalidMedia):
		f = llm.Failure{Reason: "CLIP_INVALID_MEDIA", TechnicalDetail: "Source verification failed before analysis."}
	case errors.Is(e.Cause, ErrCopyTooLong):
		f = llm.Failure{Reason: "CLIP_COPY_TOO_LONG"}
	case errors.Is(e.Cause, ErrInvalid):
		f = llm.Failure{Reason: "CLIP_INVALID_INPUT"}
	case errors.Is(e.Cause, ErrSourceState), errors.Is(e.Cause, ErrNotFound):
		f = llm.Failure{Reason: "CLIP_SOURCE_UNAVAILABLE"}
	}
	if f.Reason == "UNKNOWN_FAILURE" {
		f = llm.Failure{Reason: "CLIP_PROCESSING_FAILED", TechnicalDetail: "Clip processing failed in stage " + e.Stage + "."}
	}
	if f.Params == nil {
		f.Params = map[string]string{}
	}
	f.Params["stage"] = e.Stage
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
	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&p); err != nil || p.Version != 1 || p.ProjectID != project || p.Batch.ID != b.ID || p.Batch.UserID != user || !reflect.DeepEqual(p.Batch.Sources, b.Sources) {
		return ErrInvalid
	}
	if dec.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	if b.State != "consuming" {
		return ErrSourceState
	}
	set := func(name string, done, total int) { stage = name; progress(name, done, total) }
	var analysisJSON, planJSON []byte
	var result Result
	err = s.media.WithWorkspace(ctx, job, func(ws MediaWorkspace) error {
		set("prepare", 0, len(b.Sources))
		sources, infos, count, err := s.probeBatch(ctx, ws, b, func(n int) { set("prepare", n, len(b.Sources)) })
		if err != nil {
			return err
		}
		if err = s.planner.ValidateModels(modelRef(p.Observe), modelRef(p.Write)); err != nil {
			return err
		}
		admitted, err := s.jobs.Reserve(ctx, user, job, p.Observe, p.Write, count, s.planner.Budgets())
		if err != nil {
			return err
		}
		ctx = admitted
		set("analyze", 0, count)
		var chunks []ChunkAnalysis
		for i, v := range b.Sources {
			err = s.withSource(ctx, ws, v, infos[i], func(source MediaSource) error {
				return s.media.PrepareAnalysisChunks(ctx, ws, source, func(chunk AnalysisChunk) error {
					observation, e := s.observe(ctx, b, chunk, sources[i], modelRef(p.Observe))
					if e != nil {
						return e
					}
					chunks = append(chunks, observation)
					set("analyze", len(chunks), count)
					return nil
				})
			})
			if err != nil {
				return err
			}
		}
		analyses, err := MergeAnalyses(s.cfg.Analysis, sources, chunks)
		if err != nil {
			return err
		}
		set("plan", 0, 1)
		edit, _, err := s.planner.Plan(ctx, modelRef(p.Write), PlanningInput{Template: p.Template, Answers: p.Answers, Ratio: p.Ratio, TargetDurationMS: p.TargetDurationMS, Analyses: analyses})
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
					return s.withSource(ctx, ws, v, infos[i], fn)
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
		planJSON, err = json.Marshal(edit)
		if err != nil {
			return err
		}
		result = Result{Key: key, ContentType: "video/mp4", Bytes: video.Bytes, DurationMS: video.Info.DurationMS, CreatedAt: time.Now()}
		return nil
	})
	if err != nil {
		return err
	}
	// Publish only after local cleanup has succeeded too. Any earlier failure
	// leaves the old result intact and the new object to the minimum-age sweep.
	if err = s.store.SaveGeneration(ctx, user, project, string(analysisJSON), string(planJSON), result); err != nil {
		return err
	}
	set("cleanup", 0, 1)
	return nil
}
