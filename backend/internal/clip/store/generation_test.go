package store_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/store"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type processingObjects struct {
	*sourceObjects
	downloads  map[string]int
	results    []clip.StoredObject
	listErr    error
	short      bool
	failUpload bool
}

func (o *processingObjects) Download(ctx context.Context, key string, w io.Writer, limit int64) (int64, error) {
	o.downloads[key]++
	n := o.info[key].Bytes
	if o.short {
		n--
	}
	return io.CopyN(w, bytes.NewReader(make([]byte, n)), n)
}
func (o *processingObjects) Upload(_ context.Context, key string, r io.ReadSeeker, n int64, mime string) error {
	o.info[key] = clip.SourceObjectInfo{Bytes: n, ContentType: mime}
	if strings.HasPrefix(key, clip.ResultPrefix) {
		o.results = append(o.results, clip.StoredObject{Key: key, Modified: time.Now()})
	}
	if o.failUpload {
		return errors.New("upload failed after partial write")
	}
	read, err := io.Copy(io.Discard, r)
	if err != nil {
		return err
	}
	if read != n {
		return errors.New("short upload")
	}
	return nil
}
func (o *processingObjects) PresignRead(_ context.Context, key, filename string, attachment bool, _ time.Duration) (string, error) {
	suffix := "inline"
	if attachment {
		suffix = "attachment"
	}
	return "https://private.example/" + key + "?disposition=" + suffix, nil
}
func (o *processingObjects) ListResults(context.Context) ([]clip.StoredObject, error) {
	var out []clip.StoredObject
	for _, v := range o.results {
		if _, ok := o.info[v.Key]; ok {
			out = append(out, v)
		}
	}
	return out, o.listErr
}

type mediaFake struct {
	root        string
	probes      int
	durations   []int
	maxSources  int
	panicChunks bool
	cleanupErr  error
}

func (m *mediaFake) WithWorkspace(ctx context.Context, _ string, fn func(clip.MediaWorkspace) error) error {
	dir, err := os.MkdirTemp(m.root, "work-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	if err := fn(clip.MediaWorkspace{Path: dir}); err != nil {
		return err
	}
	return m.cleanupErr
}
func (m *mediaFake) Probe(_ context.Context, ws clip.MediaWorkspace, _ string) (clip.MediaInfo, error) {
	files, _ := filepath.Glob(filepath.Join(ws.Path, "source-*"))
	m.maxSources = max(m.maxSources, len(files))
	n := m.durations[m.probes%len(m.durations)]
	m.probes++
	return clip.MediaInfo{DurationMS: n, Width: 1920, Height: 1080}, nil
}
func (m *mediaFake) PrepareAnalysisChunks(_ context.Context, ws clip.MediaWorkspace, s clip.MediaSource, fn func(clip.AnalysisChunk) error) error {
	if m.panicChunks {
		panic("media panic")
	}
	for index, offset := 0, 0; offset < s.Info.DurationMS; index, offset = index+1, offset+60000 {
		p := filepath.Join(ws.Path, "proxy.mp4")
		if err := os.WriteFile(p, []byte("proxy"), 0600); err != nil {
			return err
		}
		err := fn(clip.AnalysisChunk{Path: p, SourceID: s.SourceID, Fingerprint: s.Fingerprint, Index: index, OffsetMS: offset, DurationMS: min(60000, s.Info.DurationMS-offset)})
		_ = os.Remove(p)
		if err != nil {
			return err
		}
	}
	return nil
}
func (m *mediaFake) CleanupStale(context.Context, time.Time) error { return nil }

type plannerFake struct {
	id                          string
	observe, plans              int
	input                       clip.PlanningInput
	gate, observeErr, errorPlan error
}

func (p *plannerFake) ValidateModels(o, w llm.ModelRef) error {
	if p.gate != nil {
		return p.gate
	}
	if o.String() != "p/o" || w.String() != "p/w" {
		return llm.ErrModelUnavailable
	}
	return nil
}
func (*plannerFake) Budgets() clip.CompletionBudgets {
	return clip.CompletionBudgets{Observe: 8192, Plan: 32768}
}
func (p *plannerFake) ObserveChunk(ctx context.Context, r llm.ModelRef, c clip.ChunkInput) (clip.ChunkAnalysis, llm.Usage, error) {
	if err := job.ConsumeClipCall(ctx, "alice", p.id, r.String(), 8192); err != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, err
	}
	p.observe++
	if p.observeErr != nil {
		return clip.ChunkAnalysis{}, llm.Usage{}, p.observeErr
	}
	return clip.ChunkAnalysis{SourceID: c.Source.ID, Fingerprint: c.Source.Fingerprint, Index: c.Index, OffsetMS: c.OffsetMS, DurationMS: c.DurationMS, Segments: []clip.Segment{{StartMS: c.OffsetMS, EndMS: c.OffsetMS + c.DurationMS, Event: "scene", Quality: "usable", Focal: clip.Point{X: .5, Y: .5}}}}, llm.Usage{}, nil
}
func (p *plannerFake) Plan(ctx context.Context, r llm.ModelRef, in clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	if err := job.ConsumeClipCall(ctx, "alice", p.id, r.String(), 32768); err != nil {
		return clip.EditPlan{}, llm.Usage{}, err
	}
	p.plans++
	p.input = in
	if p.errorPlan != nil {
		return clip.EditPlan{}, llm.Usage{}, p.errorPlan
	}
	s := in.Analyses[0].Source
	return clip.EditPlan{Ratio: in.Ratio, DurationMS: in.TargetDurationMS, Cuts: []clip.Cut{{ID: "cut", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: in.TargetDurationMS, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "서울", Style: "clean", Position: "bottom"}}}}, llm.Usage{}, nil
}

type rendererFake struct {
	fail  error
	calls int
}

func (r *rendererFake) Render(ctx context.Context, ws clip.MediaWorkspace, p clip.EditPlan, _ []clip.RenderSource, loader clip.RenderSourceLoader) (clip.RenderedVideo, error) {
	r.calls++
	if r.fail != nil {
		return clip.RenderedVideo{}, r.fail
	}
	for _, c := range p.Cuts {
		if err := loader(ctx, c.SourceID, func(clip.MediaSource) error { return nil }); err != nil {
			return clip.RenderedVideo{}, err
		}
	}
	path := filepath.Join(ws.Path, "result.mp4")
	if err := os.WriteFile(path, []byte("video"), 0600); err != nil {
		return clip.RenderedVideo{}, err
	}
	return clip.RenderedVideo{Path: path, Bytes: 5, Info: clip.MediaInfo{DurationMS: p.DurationMS}}, nil
}

type clipAdmitter struct {
	calls  []job.Start
	refuse error
	media  *mediaFake
}

func (a *clipAdmitter) Hold(_ context.Context, s job.Start) error {
	if a.media.probes != 2 {
		return errors.New("reservation preceded full manifest probe")
	}
	if a.refuse != nil {
		return a.refuse
	}
	a.calls = append(a.calls, s)
	return nil
}
func (*clipAdmitter) Release(context.Context, string)             {}
func (*clipAdmitter) Settle(context.Context, string)              {}
func (*clipAdmitter) OpenHolds(context.Context) ([]string, error) { return nil, nil }

type generationJobs struct{ q *job.Queue }

func (j generationJobs) Enqueue(ctx context.Context, s clip.GenerationStart) (string, error) {
	return j.q.Enqueue(ctx, job.NewJob{Kind: job.KindGenerateClip, UserID: s.UserID, ClipProjectID: s.ProjectID, ObserveModel: s.Observe, WriteModel: s.Write, Payload: s.Payload})
}
func (j generationJobs) Activate(ctx context.Context, user, id string) error {
	return j.q.ActivateClip(ctx, user, id)
}
func (j generationJobs) FailQueued(ctx context.Context, user, id string) (bool, error) {
	return j.q.FailQueued(ctx, id, user, job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
}
func (j generationJobs) Reserve(ctx context.Context, user, id, o, w string, n int, b clip.CompletionBudgets) (context.Context, error) {
	return j.q.ReserveClip(ctx, user, id, []job.PlannedCall{{Ref: o, Count: n, CompletionTokens: b.Observe}, {Ref: w, Count: 1, CompletionTokens: b.Plan}})
}
func (j generationJobs) Active(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	found, err := j.q.ActiveForClip(ctx, user, id)
	if found == nil || err != nil {
		return nil, err
	}
	return &clip.ClipJob{ID: found.ID, Status: found.Status, Stage: found.Stage}, nil
}
func (j generationJobs) Get(ctx context.Context, user, id string) (*clip.ClipJob, error) {
	found, err := j.q.Get(ctx, id, user)
	if errors.Is(err, job.ErrNotFound) {
		return nil, nil
	}
	if found == nil || err != nil {
		return nil, err
	}
	return &clip.ClipJob{ID: found.ID, Status: found.Status, Stage: found.Stage}, nil
}

type generationHarness struct {
	service  *clip.GenerationService
	projects *clip.Service
	store    *store.Store
	db       *db.DB
	sources  *clip.SourceService
	objects  *processingObjects
	media    *mediaFake
	planner  *plannerFake
	renderer *rendererFake
	admitter *clipAdmitter
	queue    *job.Queue
	jobs     *jobstore.Store
	template clip.VideoTemplate
	project  clip.Project
	batch    clip.SourceBatch
	cfg      clip.GenerationConfig
}

func generationSetup(t *testing.T) *generationHarness {
	t.Helper()
	projects, st, d := setup(t)
	template, project := create(t, projects)
	objects := &processingObjects{sourceObjects: fakeSources(), downloads: map[string]int{}}
	sources := clip.NewSourceService(st, objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute))
	projects.SetSources(sources)
	manifest := manifest(2)
	manifest[0].DurationMS = 60000
	manifest[1].DurationMS = 15000
	upload, err := sources.Create(context.Background(), "alice", project.ID, manifest)
	if err != nil {
		t.Fatal(err)
	}
	objects.upload(upload.Batch)
	var batch clip.SourceBatch
	for _, v := range upload.Batch.Sources {
		batch, err = sources.Confirm(context.Background(), "alice", upload.Batch.ID, v.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	media := &mediaFake{root: t.TempDir(), durations: []int{60001, 15000}}
	planner := &plannerFake{}
	renderer := &rendererFake{}
	jobs := jobstore.New(d.Writer, d.Reader)
	queue := job.New(jobs, time.Millisecond)
	admitter := &clipAdmitter{media: media}
	queue.Admit(admitter)
	cfg := clip.GenerationConfig{Media: clip.MediaConfig{Sources: config.ClipSourceLimits(6*time.Hour, 10*time.Minute), ChunkDurationMS: 60000, DurationToleranceMS: 1000}, Analysis: clip.AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}, ReadTTL: time.Minute, CleanupTimeout: time.Second, OrphanMinAge: time.Hour}
	service := clip.NewGenerationService(st, projects, sources, objects, media, planner, renderer, generationJobs{queue}, cfg)
	projects.SetGeneration(service)
	return &generationHarness{service, projects, st, d, sources, objects, media, planner, renderer, admitter, queue, jobs, template, project, batch, cfg}
}
func (h *generationHarness) start(t *testing.T) string {
	t.Helper()
	id, err := h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	return id
}
func (h *generationHarness) run(t *testing.T) error {
	t.Helper()
	ctx := context.Background()
	j, err := h.jobs.PickNextQueued(ctx, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	err = h.service.Run(ctx, j.UserID, j.ID, j.ClipProjectID, j.Payload, func(stage string, done, total int) {
		if e := h.jobs.UpdateProgress(ctx, j.ID, stage, done, total, time.Now()); e != nil {
			t.Fatal(e)
		}
	})
	status := job.StatusDone
	var failure *job.Failure
	if err != nil {
		status = job.StatusFailed
		var stage *clip.StageFailure
		if !errors.As(err, &stage) {
			t.Fatal("unstructured failure", err)
		}
		f := stage.Failure()
		failure = &job.Failure{Reason: f.Reason, Params: f.Params, TechnicalDetail: f.TechnicalDetail}
	}
	if e := h.jobs.Finish(ctx, j.ID, status, failure, time.Now()); e != nil {
		t.Fatal(e)
	}
	return err
}
func (h *generationHarness) assertClean(t *testing.T) {
	t.Helper()
	if _, err := h.store.GetSourceBatch(context.Background(), "alice", h.batch.ID); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal("batch retained", err)
	}
	files, err := os.ReadDir(h.media.root)
	if err != nil || len(files) != 0 {
		t.Fatal("workspace retained", files, err)
	}
	for key := range h.objects.info {
		if strings.HasPrefix(key, clip.SourcePrefix) {
			t.Fatal("source/proxy retained", key)
		}
	}
}
func TestGenerationProbesBeforeExactReservationAndFreezesInputs(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	if len(h.admitter.calls) != 0 {
		t.Fatal("held at enqueue")
	}
	guidance := "changed after enqueue"
	if _, err := h.projects.UpdateTemplate(context.Background(), "alice", h.template.ID, clip.TemplatePatch{CutGuidance: &guidance}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.projects.DeleteTemplate(context.Background(), "alice", h.template.ID); err != nil {
		t.Fatal("template detach must not invalidate frozen work", err)
	}
	if err := h.projects.DeleteProject(context.Background(), "alice", h.project.ID); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("busy delete", err)
	}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.assertClean(t)
	if len(h.admitter.calls) != 1 || len(h.admitter.calls[0].Calls) != 2 || h.admitter.calls[0].Calls[0].Count != 3 || h.admitter.calls[0].Calls[1].CompletionTokens != 32768 {
		t.Fatal("wrong actual call reservation", h.admitter.calls)
	}
	if h.planner.observe != 3 || h.planner.plans != 1 || h.planner.input.Template.CutGuidance == guidance || len(h.planner.input.Answers) != 1 || h.planner.input.Answers[0].Text != "서울" {
		t.Fatal("snapshot changed", h.planner)
	}
	if h.media.maxSources != 1 || h.objects.downloads[h.batch.Sources[0].Key] != 3 || h.objects.downloads[h.batch.Sources[1].Key] != 2 {
		t.Fatal("sources not staged sequentially or unused source rendered", h.objects.downloads)
	}
	p, err := h.projects.GetProject(context.Background(), "alice", h.project.ID)
	if err != nil || p.Result == nil || !strings.Contains(p.Result.ViewURL, "inline") || !strings.Contains(p.Result.DownloadURL, "attachment") || p.Analysis == "" || p.EditPlan == "" || p.EditPlanRevision != 1 || p.RenderedPlanRevision != 1 {
		t.Fatal(p, err)
	}
	stored, _ := h.store.GetProject(context.Background(), "alice", p.ID)
	if stored.Result.ViewURL != "" || strings.Contains(stored.Analysis, "https://") {
		t.Fatal("signed URL retained")
	}
	if err = h.projects.DeleteProject(context.Background(), "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	keys, _ := h.store.DeletionKeys(context.Background())
	if len(keys) != 1 || keys[0] != p.Result.Key {
		t.Fatal(keys)
	}
	if err = h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.info[p.Result.Key]; ok {
		t.Fatal("retained deleted result")
	}
}
func TestGenerationFailurePreservesOldResultAndCleansInputs(t *testing.T) {
	for _, mode := range []string{"hold", "probe", "download", "observe", "plan", "render", "save", "partial-upload", "workspace-cleanup"} {
		t.Run(mode, func(t *testing.T) {
			h := generationSetup(t)
			old := clip.Result{Key: clip.ResultPrefix + "alice/old/old.mp4", ContentType: "video/mp4", Bytes: 5, DurationMS: 30000, CreatedAt: time.Now()}
			if err := h.store.SaveGeneration(context.Background(), "alice", h.project.ID, "old analysis", "old plan", old); err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "hold":
				h.admitter.refuse = &plan.InsufficientCreditsError{Required: 100, Balance: 1, RenewsAt: time.Now()}
			case "probe":
				h.media.durations[1] = 20000
			case "download":
				h.objects.short = true
			case "observe":
				h.planner.observeErr = llm.ErrRateLimited
			case "plan":
				h.planner.errorPlan = llm.ErrBadOutput
			case "render":
				h.renderer.fail = errors.New("render failed")
			case "save":
				_, err := h.db.Writer.Exec("CREATE TRIGGER fail_clip_swap BEFORE UPDATE OF result_key ON clip_projects BEGIN SELECT RAISE(ABORT,'swap failed'); END;")
				if err != nil {
					t.Fatal(err)
				}
			case "partial-upload":
				h.objects.failUpload = true
			case "workspace-cleanup":
				h.media.cleanupErr = errors.New("workspace cleanup failed")
			}
			id := h.start(t)
			if err := h.run(t); err == nil {
				t.Fatal("wanted stage failure")
			}
			h.assertClean(t)
			p, err := h.store.GetProject(context.Background(), "alice", h.project.ID)
			if err != nil || p.Analysis != "old analysis" || p.EditPlan != "old plan" || p.Result.Key != old.Key || p.EditPlanRevision != 1 {
				t.Fatal("old result changed", p, err)
			}
			deletes, _ := h.store.DeletionKeys(context.Background())
			if len(deletes) != 0 {
				t.Fatal("failed transaction enqueued old result deletion", deletes)
			}
			j, _ := h.queue.Get(context.Background(), id, "alice")
			if j.Status != "failed" || j.Failure == nil || j.Failure.Params["stage"] == "" {
				t.Fatal(j)
			}
			if mode == "hold" || mode == "probe" || mode == "download" {
				if len(h.admitter.calls) != 0 || h.planner.observe != 0 || h.planner.plans != 0 {
					t.Fatal("unreserved provider call")
				}
			}
		})
	}
}
func TestGenerationCrashRecoveryNeverTouchesActiveBatch(t *testing.T) {
	h := generationSetup(t)
	id := h.start(t)
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.GetSourceBatch(context.Background(), "alice", h.batch.ID); err != nil {
		t.Fatal("active batch reaped", err)
	}
	if _, err := h.jobs.PickNextQueued(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.queue.SweepRunning(context.Background()); err != nil {
		t.Fatal(err)
	}
	j, _ := h.queue.Get(context.Background(), id, "alice")
	if j.Status != "failed" {
		t.Fatal(j)
	}
	if err := h.service.Sweep(context.Background()); err != nil {
		t.Fatal(err)
	}
	h.assertClean(t)
}
func TestGenerationPanicStillMarksSourceCleanup(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	h.media.panicChunks = true
	j, err := h.jobs.PickNextQueued(context.Background(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected panic")
			}
		}()
		_ = h.service.Run(context.Background(), "alice", j.ID, j.ClipProjectID, j.Payload, func(string, int, int) {})
	}()
	h.assertClean(t)
}
func TestGenerationOwnerAndModelGatesBeforeEnqueue(t *testing.T) {
	h := generationSetup(t)
	for _, user := range []string{"bob", "missing"} {
		if _, err := h.service.Start(context.Background(), user, h.project.ID, h.batch.ID, "p/o", "p/w"); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal(err)
		}
	}
	h.planner.gate = llm.ErrUnsupported
	if _, err := h.service.Start(context.Background(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w"); !errors.Is(err, llm.ErrUnsupported) {
		t.Fatal(err)
	}
	if n, err := h.queue.ActiveForClip(context.Background(), "alice", h.project.ID); err != nil || n != nil {
		t.Fatal(n, err)
	}
}
