package clip_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"github.com/postpilot/backend/internal/clip"
)

type previewProjectStore struct {
	clip.SourceStore
	clip.Store
	mu      sync.Mutex
	project clip.Project
	reads   int
}

func (s *previewProjectStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if user != s.project.UserID || id != s.project.ID {
		return clip.Project{}, clip.ErrNotFound
	}
	return s.project, nil
}

type previewRenderer struct {
	last clip.EditPlan
	clip.Renderer
	enter    chan struct{}
	release  chan struct{}
	called   int
	deadline time.Time
}

func (r *previewRenderer) PreparePreview(ctx context.Context, p clip.EditPlan, sources []clip.RenderSource, ids []string, offset int, cfg clip.PreviewConfig) (clip.PreparedPreview, error) {
	r.called++
	r.last = p
	r.deadline, _ = ctx.Deadline()
	if len(sources) == 0 || p.Portable == nil {
		return clip.PreparedPreview{}, clip.ErrInvalid
	}
	if r.enter != nil {
		r.enter <- struct{}{}
		select {
		case <-r.release:
		case <-ctx.Done():
			return clip.PreparedPreview{}, ctx.Err()
		}
	}
	return clip.PreparedPreview{NextOffset: -1}, nil
}

// previewSetup is a native project: a draft is drawn from the composition it
// was written into.
func previewSetup(t *testing.T) (*clipapp.GenerationService, *previewProjectStore, *previewRenderer, clip.CorrectionPlan) {
	p, _ := correctionFixture(t)
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Portable = nativePortable(plan)
	if p.EditPlan, err = clip.EncodeEditPlan(plan); err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(plan)
	p.ID, p.UserID = "owned", "alice"
	store := &previewProjectStore{project: p}
	render := &previewRenderer{}
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	service := clipapp.NewGenerationService(nil, testProjects(store), nil, neutralProcessing{}, nil, nil, render, neutralJobs{}, cfg, neutralGenerationDeps())
	return service, store, render, draft
}
func TestPreviewIsOwnedReadOnlyAndRevisionScoped(t *testing.T) {
	service, store, render, draft := previewSetup(t)
	before := store.project
	if _, err := service.PreparePreview(t.Context(), "bob", "owned", 1, "hash", draft, nil, 0); !errors.Is(err, clip.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := service.PreparePreview(t.Context(), "alice", "owned", 2, "hash", draft, nil, 0); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
	out, err := service.PreparePreview(t.Context(), "alice", "owned", 1, "hash", draft, nil, 0)
	if err != nil || out.DraftHash != "hash" || !reflect.DeepEqual(before, store.project) || render.called != 1 {
		t.Fatal(out, err, render.called)
	}
	if time.Until(render.deadline) > 5*time.Second || time.Until(render.deadline) <= 0 {
		t.Fatal("missing bounded deadline")
	}
	if _, err := service.PreparePreview(t.Context(), "alice", "owned", 1, "hash", draft, make([]string, 9), 0); !errors.Is(err, clip.ErrPreviewTooLarge) {
		t.Fatal(err)
	}
}
func TestPreviewOwnerAdmissionCancellationAndConcurrentSave(t *testing.T) {
	service, store, render, draft := previewSetup(t)
	render.enter, render.release = make(chan struct{}, 1), make(chan struct{})
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		_, err := service.PreparePreview(ctx, "alice", "owned", 1, "old", draft, nil, 0)
		result <- err
	}()
	<-render.enter
	if _, err := service.PreparePreview(t.Context(), "alice", "owned", 1, "new", draft, nil, 0); !errors.Is(err, clip.ErrPreviewBusy) {
		t.Fatal(err)
	}
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	go func() {
		_, err := service.PreparePreview(t.Context(), "alice", "owned", 1, "new", draft, nil, 0)
		result <- err
	}()
	<-render.enter
	store.mu.Lock()
	store.project.EditPlanRevision++
	store.mu.Unlock()
	close(render.release)
	if err := <-result; !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal(err)
	}
}

func TestNativeCorrectionKeepsContentAndRejectsForgedIdentities(t *testing.T) {
	p, _ := correctionFixture(t)
	original, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	original.Portable = nativePortable(original)
	p.EditPlan, err = clip.EncodeEditPlan(original)
	if err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(original)
	draft.Elements[0].Text = "수정한 문구"
	draft.Cuts[0].Focal = &clip.Point{X: .1, Y: .8}
	next, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if next.Portable.Elements[0].Resolved.Text != "수정한 문구" || !next.Portable.Elements[0].OwnerEdited || next.Cuts[0].Focal.X != .1 || clip.AutomaticCompositionRepair(next.Portable.Elements[0]) {
		t.Fatal("lost authored correction")
	}
	raw, err := clip.EncodeEditPlan(next)
	if err != nil {
		t.Fatal(err)
	}
	again, err := clip.DecodeEditPlan(raw)
	if err != nil || again.Portable.Elements[0].Resolved.Text != "수정한 문구" {
		t.Fatal(err)
	}
	draft.Cuts[0].StartMS = 100
	draft.DurationMS -= 100
	trimmed, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft)
	if err != nil {
		t.Fatal("trim", err)
	}
	if _, err := clip.EncodeEditPlan(trimmed); err != nil {
		t.Fatal("trim lost frozen evidence", err)
	}
	draft.Elements[0].InstanceID = "forged"
	if _, err := clip.ApplyCorrection(clip.DefaultRenderConfig(clip.Environment{}), p, draft); err == nil {
		t.Fatal("accepted foreign element")
	}
}

func TestEditingProjectionDoesNotWidenLegacyStoredPlanJSON(t *testing.T) {
	p, _ := correctionFixture(t)
	var envelope struct{ Plan map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(p.EditPlan), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Plan) != 2 || envelope.Plan["NativeComposition"] != nil || envelope.Plan["Elements"] != nil {
		t.Fatal("legacy envelope widened", envelope.Plan)
	}
	var cuts []map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Plan["Cuts"], &cuts); err != nil {
		t.Fatal(err)
	}
	if cuts[0]["Focal"] != nil {
		t.Fatal("legacy focal duplicated")
	}
}
