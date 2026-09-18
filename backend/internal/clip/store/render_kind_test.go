package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestRenderKindRejectsBeforeAccessingDependencies(t *testing.T) {
	s := &clip.GenerationService{}
	for _, kind := range []clip.RenderKind{"", "unknown"} {
		if _, err := s.StartRender(t.Context(), "alice", "project", "batch", 1, kind); !errors.Is(err, clip.ErrInvalid) {
			t.Fatal(kind, err)
		}
	}
}

func TestStoredRenderKindSurvivesStagingAndReplacement(t *testing.T) {
	h, p, _ := completedClip(t)
	id := h.startRender(t)
	candidate := clip.AttemptResult{JobID: id, UserID: "alice", ProjectID: p.ID, ExpectedRevision: p.EditPlanRevision,
		Result: clip.Result{Kind: clip.RenderBrowser, Key: "clip-results/alice/browser.mp4", ContentType: "video/mp4", Bytes: 123, DurationMS: 30000, CreatedAt: time.Now()}}
	if err := h.store.StageAttemptResult(t.Context(), candidate); err != nil {
		t.Fatal(err)
	}
	stored, err := h.store.GetAttemptResult(t.Context(), id)
	if err != nil || !clip.SameAttemptResult(stored, candidate) {
		t.Fatal(stored, err)
	}
	changed := candidate
	changed.Result.Kind = clip.RenderServer
	if err := h.store.StageAttemptResult(t.Context(), changed); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("changed kind accepted on retry", err)
	}
	if err := h.store.SaveRender(t.Context(), "alice", p.ID, p.EditPlanRevision, stored.Result); err != nil {
		t.Fatal(err)
	}
	p, err = h.store.GetProject(t.Context(), "alice", p.ID)
	if err != nil || p.Result.Kind != clip.RenderBrowser {
		t.Fatal(p, err)
	}
}

func TestLegacyStoredResultDefaultsToServer(t *testing.T) {
	s, _, db := setup(t)
	_, p := create(t, s)
	if p.Result != nil {
		t.Fatal("new project has a result", p)
	}
	// The pre-kind write shape omits the new column, as every existing row did.
	_, err := db.Writer.Exec("UPDATE clip_projects SET result_key='old.mp4',result_created_at=? WHERE id=?", time.Now().UTC().Format(time.RFC3339Nano), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	p, err = s.GetProject(t.Context(), "alice", p.ID)
	if err != nil || p.Result == nil || p.Result.Kind != clip.RenderServer {
		t.Fatal(p, err)
	}
}
