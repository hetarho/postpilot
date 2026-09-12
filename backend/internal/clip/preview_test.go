package clip_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

type previewProjectStore struct {
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
func previewSetup(t *testing.T) (*clip.GenerationService, *previewProjectStore, *previewRenderer, clip.CorrectionPlan) {
	p, draft := correctionFixture(t)
	p.ID, p.UserID = "owned", "alice"
	store := &previewProjectStore{project: p}
	render := &previewRenderer{}
	cfg := config.ClipGeneration(&config.Config{PresignGetTTL: time.Minute, OrphanMinAge: time.Hour})
	service := clip.NewGenerationService(nil, clip.NewService(store, config.ClipLimits()), nil, nil, nil, nil, render, nil, cfg)
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
	original, styles, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	original.Portable, err = clip.FreezeLegacyPlan(p, original, clip.Recipe{CopyStyles: styles}, config.ClipCompositionLimits())
	if err != nil {
		t.Fatal(err)
	}
	original.Portable.Snapshot.Legacy = false
	p.EditPlan, err = clip.EncodeEditPlan(original, styles)
	if err != nil {
		t.Fatal(err)
	}
	draft := clip.CorrectionFromPlan(original)
	draft.Elements[0].Text = "수정한 문구"
	draft.Cuts[0].Focal = &clip.Point{X: .1, Y: .8}
	next, _, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, draft)
	if err != nil {
		t.Fatal(err)
	}
	if next.Portable.Elements[0].Resolved.Text != "수정한 문구" || !next.Portable.Elements[0].OwnerEdited || next.Cuts[0].Focal.X != .1 || clip.AutomaticCompositionRepair(next.Portable.Elements[0]) {
		t.Fatal("lost authored correction")
	}
	raw, err := clip.EncodeEditPlan(next, styles)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := clip.DecodeEditPlan(raw)
	if err != nil || again.Portable.Elements[0].Resolved.Text != "수정한 문구" {
		t.Fatal(err)
	}
	draft.Cuts[0].StartMS = 100
	draft.DurationMS -= 100
	trimmed, _, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, draft)
	if err != nil {
		t.Fatal("trim", err)
	}
	if _, err := clip.EncodeEditPlan(trimmed, styles); err != nil {
		t.Fatal("trim lost frozen evidence", err)
	}
	draft.Elements[0].InstanceID = "forged"
	if _, _, err := clip.ApplyCorrection(config.ClipRender(&config.Config{}), p, draft); err == nil {
		t.Fatal("accepted foreign element")
	}
}

func TestLegacyPreviewUsesTheCurrentHookInsteadOfThePreviousSnapshot(t *testing.T) {
	service, store, render, draft := previewSetup(t)
	store.project.Answers = []clip.Answer{{Label: "상호", Text: "카페"}, {Label: "위치", Text: "서울"}}
	previous := clip.LegacyProjectComposition(store.project, clip.Recipe{CopyStyles: []string{"clean", "memo"}})
	store.project.Composition = &previous
	draft.Hook = "서울 카페"
	if _, err := service.PreparePreview(t.Context(), "alice", "owned", 1, "hook", draft, nil, 0); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, element := range render.last.Portable.Elements {
		if element.Resolved.Element.ID == "legacy-hook" {
			for _, row := range element.Resolved.Rows {
				if row.Text == draft.Hook {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("preview reused the saved opening text")
	}
}

func TestEditingProjectionDoesNotWidenLegacyStoredPlanJSON(t *testing.T) {
	p, _ := correctionFixture(t)
	var envelope struct{ Plan map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(p.EditPlan), &envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Plan) != 3 || envelope.Plan["NativeComposition"] != nil || envelope.Plan["Elements"] != nil {
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
