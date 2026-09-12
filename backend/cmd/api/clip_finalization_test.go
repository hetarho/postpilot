package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	clipstore "github.com/postpilot/backend/internal/clip/store"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
)

type finalizationObjects struct {
	clip.ProcessingObjects
	mu         sync.Mutex
	objects    map[string]clip.SourceObjectInfo
	failDelete bool
	signs      int
}

func (o *finalizationObjects) PresignSource(context.Context, string, string, time.Duration) (clip.SignedSourcePut, error) {
	return clip.SignedSourcePut{URL: "https://example.test/put"}, nil
}
func (o *finalizationObjects) PresignSourcePlayback(context.Context, string, string, time.Duration) (string, error) {
	o.signs++
	return "https://example.test/source", nil
}
func (o *finalizationObjects) PresignRead(context.Context, string, string, bool, time.Duration) (string, error) {
	return "https://example.test/result", nil
}
func (o *finalizationObjects) HeadSource(_ context.Context, key string) (clip.SourceObjectInfo, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	v, ok := o.objects[key]
	if !ok {
		return v, clip.ErrNotFound
	}
	return v, nil
}
func (o *finalizationObjects) Delete(_ context.Context, key string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.failDelete {
		return errors.New("storage temporarily unavailable")
	}
	delete(o.objects, key)
	return nil
}
func (o *finalizationObjects) ListSourceKeys(context.Context) ([]string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	var keys []string
	for key := range o.objects {
		if strings.HasPrefix(key, clip.SourcePrefix) {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

type finalizationHarness struct {
	*cancellationHarness
	service    *clip.Service
	generation *clip.GenerationService
	sources    *clip.SourceService
	objects    *finalizationObjects
	batch      clip.SourceBatch
	project    clip.Project
	now        time.Time
}

func newFinalizationHarness(t *testing.T) *finalizationHarness {
	t.Helper()
	h := &finalizationHarness{cancellationHarness: newCancellationHarness(t, nil), now: time.Now()}
	cfg := &config.Config{PresignGetTTL: 5 * time.Minute, OrphanMinAge: time.Hour, ClipSourceBatchTTL: 6 * time.Hour, PresignPutTTL: 10 * time.Minute}
	h.service = clip.NewService(h.clips, config.ClipLimits())
	h.objects = &finalizationObjects{objects: map[string]clip.SourceObjectInfo{}}
	h.sources = clip.NewSourceService(h.clips, h.objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return h.now })
	h.service.SetSources(h.sources)
	h.generation = clip.NewGenerationService(h.clips, h.service, h.sources, h.objects, nil, nil, nil, clipJobs{h.queue}, config.ClipGeneration(cfg))
	h.service.SetGeneration(h.generation)
	h.service.SetFinalizer(clipFinalizer{writer: h.db.Writer, clips: h.clips, cfg: config.ClipRender(cfg)})
	upload, err := h.sources.Create(t.Context(), "alice", "clip", []clip.SourceMetadata{{Filename: "source.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 16000, Width: 640, Height: 640, Fingerprint: strings.Repeat("a", 64)}})
	if err != nil {
		t.Fatal(err)
	}
	src := upload.Batch.Sources[0]
	h.objects.objects[src.Key] = clip.SourceObjectInfo{Bytes: 100, ContentType: "video/mp4"}
	h.batch, err = h.sources.Confirm(t.Context(), "alice", upload.Batch.ID, src.ID)
	if err != nil {
		t.Fatal(err)
	}
	plan := clip.EditPlan{Ratio: "square", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: src.ID, Fingerprint: src.Fingerprint, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
	raw, err := clip.EncodeEditPlan(plan, []string{"clean"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := json.Marshal([]clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: src.ID, Fingerprint: src.Fingerprint, Info: clip.MediaInfo{DurationMS: 16000, Width: 640, Height: 640}}}}})
	if err != nil {
		t.Fatal(err)
	}
	result := clip.Result{Key: "clip-results/alice/clip/valid.mp4", ContentType: "video/mp4", Bytes: 50, DurationMS: 15000, CreatedAt: h.now}
	if err := h.clips.SaveGeneration(t.Context(), "alice", "clip", string(analysis), raw, result); err != nil {
		t.Fatal(err)
	}
	h.objects.objects[result.Key] = clip.SourceObjectInfo{Bytes: 50, ContentType: "video/mp4"}
	h.project, err = h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil {
		t.Fatal(err)
	}
	return h
}
func (h *finalizationHarness) request() clip.FinalizationRequest {
	return clip.FinalizationRequest{UserID: "alice", ProjectID: "clip", ExpectedRevision: h.project.EditPlanRevision, ExpectedResultID: h.project.Result.ID}
}
func (h *finalizationHarness) assertFinalized(t *testing.T) {
	t.Helper()
	p, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil || p.Finalized == nil || p.Finalized.ResultID != h.project.Result.ID || p.Finalized.PlanRevision != h.project.EditPlanRevision || p.Result.Key != h.project.Result.Key {
		t.Fatal(p, err)
	}
	var n int
	if err := h.db.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions").Scan(&n); err != nil || n != 0 {
		t.Fatal("finalization debited credits", n, err)
	}
}

func TestClipFinalizationPreservesMatchingResultAndFencesEveryMutation(t *testing.T) {
	h := newFinalizationHarness(t)
	p, err := h.service.FinalizeProject(t.Context(), h.request())
	if err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
	if p.Finalized == nil {
		t.Fatal("missing confirmation")
	}
	src := h.batch.Sources[0]
	if _, ok := h.objects.objects[src.Key]; ok {
		t.Fatal("original not deleted")
	}
	if _, ok := h.objects.objects[h.project.Result.Key]; !ok {
		t.Fatal("result deleted")
	}
	if _, err := h.sources.Playback(t.Context(), "alice", "clip", src.ID, src.Fingerprint); err == nil {
		t.Fatal("finalized source signed")
	}
	if _, err := h.sources.Create(t.Context(), "alice", "clip", []clip.SourceMetadata{src.SourceMetadata}); err == nil {
		t.Fatal("new upload allowed")
	}
	title := "changed"
	if _, err := h.service.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{Title: &title}); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal(err)
	}
	if _, err := h.clips.SaveCorrection(t.Context(), "alice", "clip", h.project.EditPlanRevision, h.project.EditPlan); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal("no-op edit bypassed finalization", err)
	}
	if _, err := h.generation.StartRender(t.Context(), "alice", "clip", h.batch.ID, h.project.EditPlanRevision); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal(err)
	}
	if _, err := h.generation.Quote(t.Context(), "alice", "clip", h.batch.ID, "p/o", "p/w"); !errors.Is(err, clip.ErrFinalized) {
		t.Fatal(err)
	}
	if _, err := h.queue.Enqueue(t.Context(), job.NewJob{UserID: "alice", ClipProjectID: "clip", Kind: job.KindRenderClip, NonMetered: true}); !errors.Is(err, job.ErrInvalidTarget) {
		t.Fatal("enqueue bypassed finalization", err)
	}
	again, err := h.service.FinalizeProject(t.Context(), h.request())
	if err != nil || !again.Finalized.At.Equal(p.Finalized.At) {
		t.Fatal("confirmation was not idempotent", again, err)
	}
	got, err := h.service.GetProject(t.Context(), "alice", "clip")
	if err != nil || got.Result.ViewURL == "" || got.Result.DownloadURL == "" {
		t.Fatal("confirmed result not downloadable", got, err)
	}
	if err := h.service.DeleteProject(t.Context(), "alice", "clip"); err != nil {
		t.Fatal("finalized project could not be deleted", err)
	}
}

func TestClipFinalizationCleanupFailureAndLateUploadNeverReopenEditing(t *testing.T) {
	h := newFinalizationHarness(t)
	h.objects.failDelete = true
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal("storage failure undid confirmation", err)
	}
	h.assertFinalized(t)
	var seq int
	var name, path string
	if err := h.db.Reader.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := h.db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := db.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { reopened.Close() })
	h.db = reopened
	h.clips = clipstore.New(reopened.Writer, reopened.Reader)
	h.sources = clip.NewSourceService(h.clips, h.objects, config.ClipSourceLimits(6*time.Hour, 10*time.Minute), func() time.Time { return h.now })
	h.objects.failDelete = false
	if err := h.sources.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	key := h.batch.Sources[0].Key
	if _, ok := h.objects.objects[key]; ok {
		t.Fatal("retry did not delete original")
	}
	h.objects.objects[key] = clip.SourceObjectInfo{Bytes: 100, ContentType: "video/mp4"}
	h.now = h.now.Add(11 * time.Minute)
	if err := h.sources.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, ok := h.objects.objects[key]; ok {
		t.Fatal("late PUT survived its tombstone")
	}
	h.assertFinalized(t)
	// A new service only reads durable state; a retry cannot reset the timestamp.
	restarted := clip.NewService(h.clips, config.ClipLimits())
	restarted.SetFinalizer(clipFinalizer{writer: h.db.Writer, clips: h.clips, cfg: config.ClipRender(&config.Config{})})
	if _, err := restarted.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
}

func TestClipFinalizationRejectsForeignStaleInvalidAndActiveRequests(t *testing.T) {
	for _, mode := range []string{"foreign", "revision", "result", "missing result", "invalid plan", "queued", "running", "cancelling", "deleting"} {
		t.Run(mode, func(t *testing.T) {
			h := newFinalizationHarness(t)
			req := h.request()
			switch mode {
			case "foreign":
				req.UserID = "bob"
			case "revision":
				req.ExpectedRevision--
			case "result":
				req.ExpectedResultID = "different"
			case "missing result":
				if _, err := h.db.Writer.Exec("UPDATE clip_projects SET result_key=NULL WHERE id='clip'"); err != nil {
					t.Fatal(err)
				}
			case "invalid plan":
				if _, err := h.db.Writer.Exec("UPDATE clip_projects SET edit_plan_json='invalid' WHERE id='clip'"); err != nil {
					t.Fatal(err)
				}
			case "deleting":
				if _, err := h.clips.BeginProjectSourceCleanup(t.Context(), "alice", "clip"); err != nil {
					t.Fatal(err)
				}
			default:
				id := h.enqueue(t, job.KindRenderClip)
				if mode != "queued" {
					if err := h.queue.ActivateClip(t.Context(), "alice", id); err != nil {
						t.Fatal(err)
					}
					if _, err := h.jobs.PickNextQueued(t.Context(), time.Now()); err != nil {
						t.Fatal(err)
					}
				}
				if mode == "cancelling" {
					if _, err := h.queue.CancelClipJob(t.Context(), "alice", "clip", id); err != nil {
						t.Fatal(err)
					}
				}
			}
			if _, err := h.service.FinalizeProject(t.Context(), req); err == nil {
				t.Fatal("invalid finalization accepted")
			}
			var at *string
			if err := h.db.Reader.QueryRow("SELECT finalized_at FROM clip_projects WHERE id='clip'").Scan(&at); err != nil || at != nil {
				t.Fatal("failed request finalized", at, err)
			}
		})
	}
}

func TestClipExpiredOriginalsDoNotPreventFinalization(t *testing.T) {
	h := newFinalizationHarness(t)
	h.now = h.now.Add(24 * time.Hour)
	if err := h.sources.Sweep(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
}

func TestClipFinalizationRPCIsOwnedAndReturnsCurrentIdentityOnConflict(t *testing.T) {
	h := newFinalizationHarness(t)
	rpc := cliprpc.NewHandler(h.service).WithSources(h.sources).WithGeneration(h.generation, h.queue)
	req := connect.NewRequest(&v1.FinalizeClipProjectRequest{ProjectId: "clip", ExpectedRevision: int32(h.project.EditPlanRevision), ExpectedResultId: h.project.Result.ID})
	if _, err := rpc.FinalizeClipProject(t.Context(), req); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	if _, err := rpc.FinalizeClipProject(auth.WithUser(t.Context(), "bob"), req); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	owner := auth.WithUser(t.Context(), "alice")
	req.Msg.ExpectedResultId = "stale"
	_, err := rpc.FinalizeClipProject(owner, req)
	var app *connect.Error
	if !errors.As(err, &app) || app.Code() != connect.CodeFailedPrecondition {
		t.Fatal(err)
	}
	current := false
	for _, d := range app.Details() {
		v, e := d.Value()
		if p, ok := v.(*v1.ClipProject); e == nil && ok {
			current = p.Result.Id == h.project.Result.ID
		}
	}
	if !current {
		t.Fatal("conflict omitted refreshed owned project")
	}
	req.Msg.ExpectedResultId = h.project.Result.ID
	response, err := rpc.FinalizeClipProject(owner, req)
	if err != nil {
		t.Fatal(err)
	}
	p := response.Msg.Project
	if p.FinalizedResultId != h.project.Result.ID || p.GetCanEdit() || p.GetCanFinalize() || p.FinalizationRefusal != "finalized" || p.Editing != nil || p.Observations != nil || p.Result.DownloadUrl == "" {
		t.Fatal(p)
	}
}

func TestClipFinalizationCompetesWithEditStartAndDelete(t *testing.T) {
	for _, action := range []string{"edit", "start", "delete"} {
		t.Run(action, func(t *testing.T) {
			for range 6 {
				h := newFinalizationHarness(t)
				start := make(chan struct{})
				results := make(chan error, 2)
				go func() { <-start; _, err := h.service.FinalizeProject(t.Context(), h.request()); results <- err }()
				go func() {
					<-start
					var err error
					switch action {
					case "edit":
						plan, styles, e := clip.DecodeEditPlan(h.project.EditPlan)
						if e != nil {
							results <- e
							return
						}
						volume := .5
						plan.Cuts[0].Volume = &volume
						raw, e := clip.EncodeEditPlan(plan, styles)
						if e != nil {
							results <- e
							return
						}
						_, err = h.clips.SaveCorrection(t.Context(), "alice", "clip", h.project.EditPlanRevision, raw)
					case "start":
						_, err = h.queue.Enqueue(t.Context(), job.NewJob{UserID: "alice", ClipProjectID: "clip", Kind: job.KindRenderClip, NonMetered: true})
					case "delete":
						err = h.service.DeleteProject(t.Context(), "alice", "clip")
					}
					results <- err
				}()
				close(start)
				first, second := <-results, <-results
				if action != "delete" && first == nil && second == nil {
					t.Fatal("both incompatible transitions committed")
				}
				p, err := h.clips.GetProject(t.Context(), "alice", "clip")
				if errors.Is(err, clip.ErrNotFound) && action == "delete" {
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				if p.Finalized != nil {
					if p.EditPlanRevision != h.project.EditPlanRevision || p.Result.ID != h.project.Result.ID {
						t.Fatal("confirmed a changed result", p)
					}
					active, err := h.jobs.ActiveForClip(t.Context(), "alice", "clip")
					if err != nil || active != nil {
						t.Fatal("finalized with active work", active, err)
					}
				} else if first == nil && second == nil {
					t.Fatal("neither transition was authoritative")
				}
			}
		})
	}
}

func TestClipFinalizationResultIdentityDistinguishesSameRevisionRenders(t *testing.T) {
	h := newFinalizationHarness(t)
	old := h.request()
	r := *h.project.Result
	r.Key, r.CreatedAt = "clip-results/alice/clip/new-render.mp4", time.Now()
	if err := h.clips.SaveRender(t.Context(), "alice", "clip", h.project.EditPlanRevision, r); err != nil {
		t.Fatal(err)
	}
	next, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil || next.Result.ID == h.project.Result.ID || next.EditPlanRevision != h.project.EditPlanRevision {
		t.Fatal(next, err)
	}
	if _, err := h.service.FinalizeProject(t.Context(), old); !errors.Is(err, clip.ErrFinalizationConflict) {
		t.Fatal("old render confirmed", err)
	}
	h.project = next
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
}

func TestClipFinalizationSurvivesReusableTemplateDeletion(t *testing.T) {
	h := newFinalizationHarness(t)
	template, err := h.service.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "template", Preset: "restaurant", CopyStyles: []string{"clean"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.UpdateProject(t.Context(), "alice", "clip", clip.ProjectPatch{VideoTemplateID: &template.ID}); err != nil {
		t.Fatal(err)
	}
	p, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil {
		t.Fatal(err)
	}
	// A template association changes the draft revision; this fixture commits its matching render.
	r := *p.Result
	r.Key = "clip-results/alice/clip/template-render.mp4"
	if err = h.clips.SaveRender(t.Context(), "alice", "clip", p.EditPlanRevision, r); err != nil {
		t.Fatal(err)
	}
	h.project, err = h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	if _, err = h.service.DeleteTemplate(t.Context(), "alice", template.ID); err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
	got, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil || got.VideoTemplateID != "" || got.Composition == nil || got.Composition.Snapshot.TemplateID != template.ID {
		t.Fatal("template deletion changed frozen content", got, err)
	}
}

type lostFinalizationReply struct {
	clip.ProjectFinalizer
	lost bool
}

func (f *lostFinalizationReply) Finalize(ctx context.Context, req clip.FinalizationRequest) (clip.Project, error) {
	p, err := f.ProjectFinalizer.Finalize(ctx, req)
	if err == nil && !f.lost {
		f.lost = true
		return clip.Project{}, errors.New("reply lost after commit")
	}
	return p, err
}
func TestClipFinalizationAmbiguousResponseResolvesFromDurableIdentity(t *testing.T) {
	h := newFinalizationHarness(t)
	h.service.SetFinalizer(&lostFinalizationReply{ProjectFinalizer: clipFinalizer{writer: h.db.Writer, clips: h.clips, cfg: config.ClipRender(&config.Config{})}})
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err == nil {
		t.Fatal("fixture did not lose response")
	}
	h.assertFinalized(t)
	before, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil {
		t.Fatal(err)
	}
	after, err := h.service.FinalizeProject(t.Context(), h.request())
	if err != nil || !after.Finalized.At.Equal(before.Finalized.At) {
		t.Fatal(after, err)
	}
}

func TestClipFinalizationRollsBackWhenCleanupIntentCannotCommit(t *testing.T) {
	h := newFinalizationHarness(t)
	if _, err := h.db.Writer.Exec(`CREATE TRIGGER fail_finalization_cleanup BEFORE UPDATE OF state ON clip_source_batches WHEN NEW.state='cleanup_pending' BEGIN SELECT RAISE(ABORT,'fixture cleanup write failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err == nil {
		t.Fatal("partial transaction succeeded")
	}
	p, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil || p.Finalized != nil {
		t.Fatal("failed cleanup intent left a confirmed record", p, err)
	}
	src := h.batch.Sources[0]
	if _, err := h.sources.Playback(t.Context(), "alice", "clip", src.ID, src.Fingerprint); err != nil {
		t.Fatal("rollback left source access revoked", err)
	}
	if _, err := h.db.Writer.Exec("DROP TRIGGER fail_finalization_cleanup"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	h.assertFinalized(t)
}
