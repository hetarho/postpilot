package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/plan"
)

func currentExportRights(t *testing.T, h *generationHarness) {
	t.Helper()
	rights := authstore.New(h.db.Writer, h.db.Reader)
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.Exports = h.store
	deps.PrepareExport = func(ctx context.Context, user string) (bool, error) {
		tier, err := rights.GetUserPlan(ctx, user)
		if err != nil {
			return false, err
		}
		if !plan.AllowsServerExport(tier) {
			return false, clip.ErrServerExportPlan
		}
		return tier == plan.Master, nil
	}
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, deps)
}

func TestNewServerExportsRequireCurrentRightsDespiteOldPositiveCounts(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	ctx := t.Context()
	now := time.Now().UTC()
	window := clip.ExportWindow{UserID: "alice", CoverageID: "legacy", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 6}
	if err := h.store.OpenExportWindow(ctx, window, "legacy-window"); err != nil {
		t.Fatal(err)
	}
	currentExportRights(t, h)
	b := rerenderBatch(t, h, true)
	for _, tier := range []plan.Plan{plan.Free, plan.Light, plan.Basic, plan.Pro} {
		if _, err := h.db.Writer.ExecContext(ctx, "UPDATE users SET plan=? WHERE id='alice'", tier); err != nil {
			t.Fatal(err)
		}
		if _, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer); !errors.Is(err, clip.ErrServerExportPlan) {
			t.Fatalf("%s: %v", tier, err)
		}
		w, ok, err := h.store.CurrentExportWindow(ctx, "alice", now)
		if err != nil || !ok || w.Reserved != 0 || w.Used != 0 || w.Allowance != 6 {
			t.Fatalf("%s mutated old window: %+v %v", tier, w, err)
		}
		if h.renderer.calls != 0 || len(h.admitter.calls) != 0 || len(h.objects.downloads) != 0 {
			t.Fatal("refusal started media or charged AI")
		}
	}
	if _, err := h.db.Writer.ExecContext(ctx, "UPDATE users SET plan='max' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer)
	if err != nil || id == "" {
		t.Fatal(id, err)
	}
	w, _, _ := h.store.CurrentExportWindow(ctx, "alice", now)
	if w.Reserved != 1 || w.Used != 0 {
		t.Fatalf("Max did not secure one old-window slot: %+v", w)
	}
}

func TestAcceptedServerExportSurvivesDowngradeResetAndCompletesOnce(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	ctx := t.Context()
	now := time.Now().UTC()
	first := clip.ExportWindow{UserID: "alice", CoverageID: "old", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 2}
	if err := h.store.OpenExportWindow(ctx, first, "old"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.ExecContext(ctx, "UPDATE users SET plan='max' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	currentExportRights(t, h)
	b := rerenderBatch(t, h, true)
	id, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer)
	if err != nil {
		t.Fatal(err)
	}
	// The secured origin ends, and the account's next period no longer includes exports.
	if _, err = h.db.Writer.ExecContext(ctx, "UPDATE users SET plan='pro' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if _, err = h.db.Writer.ExecContext(ctx, "UPDATE server_export_windows SET window_end=? WHERE coverage_id='old'", now.Add(-time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	next := clip.ExportWindow{UserID: "alice", CoverageID: "next", Start: now.Add(-time.Minute), End: now.Add(time.Hour), Allowance: 0}
	if err = h.store.OpenExportWindow(ctx, next, "next"); err != nil {
		t.Fatal(err)
	}
	if err = runRender(t, h); err != nil {
		t.Fatal("accepted render was revoked", err)
	}
	got, err := h.projects.GetProject(ctx, "alice", p.ID)
	if err != nil || got.Result == nil || got.Result.Key == "" {
		t.Fatal(got, err)
	}
	candidate := clip.AttemptResult{JobID: id, UserID: "alice", ProjectID: p.ID, ExpectedRevision: 1, Result: *got.Result}
	// The fixture finisher isolates rendering; execute the production export commit port explicitly.
	for range 2 {
		if err = h.store.CommitExport(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	var used, reserved int
	if err = h.db.Reader.QueryRowContext(ctx, "SELECT used,reserved FROM server_export_windows WHERE coverage_id='old'").Scan(&used, &reserved); err != nil || used != 1 || reserved != 0 {
		t.Fatal(used, reserved, err)
	}
	w, ok, err := h.store.CurrentExportWindow(ctx, "alice", now)
	if err != nil || !ok || w.Used != 0 || w.Reserved != 0 || w.Allowance != 0 {
		t.Fatal(w, err)
	}
	if _, err = h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer); !errors.Is(err, clip.ErrServerExportPlan) {
		t.Fatal("new start reused old rights", err)
	}
	if result, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer, true); err != nil || result != "" {
		t.Fatal("existing-file reuse was revoked", result, err)
	}
}

func TestExhaustedMaxRefusesAndMasterCreatesNoNumericReservation(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	ctx := t.Context()
	now := time.Now().UTC()
	w := clip.ExportWindow{UserID: "alice", CoverageID: "max", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 0}
	if err := h.store.OpenExportWindow(ctx, w, "max"); err != nil {
		t.Fatal(err)
	}
	currentExportRights(t, h)
	b := rerenderBatch(t, h, true)
	if _, err := h.db.Writer.ExecContext(ctx, "UPDATE users SET plan='max' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer); !errors.Is(err, clip.ErrExportAllowance) {
		t.Fatal(err)
	}
	if len(h.admitter.calls) != 0 || h.renderer.calls != 0 {
		t.Fatal("exhaustion charged or rendered")
	}
	if _, err := h.db.Writer.ExecContext(ctx, "UPDATE users SET plan='master' WHERE id='alice'"); err != nil {
		t.Fatal(err)
	}
	if id, err := h.service.StartRender(ctx, "alice", p.ID, b.ID, 1, clip.RenderServer); err != nil || id == "" {
		t.Fatal(id, err)
	}
	var reservations int
	if err := h.db.Reader.QueryRowContext(ctx, "SELECT COUNT(*) FROM server_export_reservations").Scan(&reservations); err != nil || reservations != 0 {
		t.Fatal(reservations, err)
	}
}

func TestServerStartFailsClosedWithoutAPlanAuthority(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	deps := generationDeps(generationFinisher{h.store}, &quotePricing{}, nil)
	deps.PrepareExport = nil
	deps.Exports = h.store
	service := clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, h.planner, h.renderer, h.clipJobs(), h.cfg, deps)
	if _, err := service.StartRender(t.Context(), "alice", p.ID, "untrusted-batch", 1, clip.RenderServer); !errors.Is(err, clip.ErrCompositionUnavailable) {
		t.Fatal(err)
	}
	if h.renderer.calls != 0 || len(h.admitter.calls) != 0 || len(h.objects.downloads) != 0 {
		t.Fatal("missing authority started media or charges")
	}
}
