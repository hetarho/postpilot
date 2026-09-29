package store_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestServerExportReservationsAreAtomicAndBoundToOriginMonth(t *testing.T) {
	_, exports, _ := setup(t)
	ctx := t.Context()
	now := time.Now().UTC()
	first := clip.ExportWindow{UserID: "alice", CoverageID: "paid:alice", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 2}
	if err := exports.OpenExportWindow(ctx, first, "paid:alice:first"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan struct {
		id  string
		err error
	}, 12)
	for i := 0; i < cap(results); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("render-%d", i)
			results <- struct {
				id  string
				err error
			}{id, exports.ReserveExport(ctx, "alice", "project", 1, id, now)}
		}(i)
	}
	wg.Wait()
	close(results)
	accepted := []string{}
	refused := 0
	for result := range results {
		if result.err == nil {
			accepted = append(accepted, result.id)
			continue
		}
		if errors.Is(result.err, clip.ErrExportAllowance) {
			refused++
			continue
		}
		t.Fatal(result.err)
	}
	if len(accepted) != 2 || refused != 10 {
		t.Fatalf("accepted=%v refused=%d", accepted, refused)
	}
	w, ok, err := exports.CurrentExportWindow(ctx, "alice", now)
	if err != nil || !ok || w.Reserved != 2 || w.Used != 0 {
		t.Fatalf("window=%+v ok=%v err=%v", w, ok, err)
	}
	if err := exports.ReserveExport(ctx, "alice", "project", 1, accepted[0], now); err != nil {
		t.Fatal("duplicate reservation", err)
	}
	w, _, _ = exports.CurrentExportWindow(ctx, "alice", now)
	if w.Reserved != 2 {
		t.Fatalf("duplicate changed count: %+v", w)
	}
}

func TestServerExportRecoveryReturnsOnlyOpenOriginSlots(t *testing.T) {
	_, exports, handle := setup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	first := clip.ExportWindow{UserID: "alice", CoverageID: "paid:alice", Start: now.Add(-time.Hour), End: now.Add(time.Hour), Allowance: 2}
	if err := exports.OpenExportWindow(ctx, first, "first"); err != nil {
		t.Fatal(err)
	}
	if err := exports.ReserveExport(ctx, "alice", "project", 1, "render-1", now); err != nil {
		t.Fatal(err)
	}
	if err := exports.BindExport(ctx, "render-1", "job-1"); err != nil {
		t.Fatal(err)
	}
	if err := exports.CommitExport(ctx, clip.AttemptResult{JobID: "job-1", UserID: "alice", ProjectID: "project", ExpectedRevision: 1, Result: clip.Result{Kind: clip.RenderServer, Key: "result.mp4"}}); err != nil {
		t.Fatal(err)
	}
	if err := exports.CommitExport(ctx, clip.AttemptResult{JobID: "job-1", UserID: "alice", ProjectID: "project", ExpectedRevision: 1, Result: clip.Result{Kind: clip.RenderServer, Key: "result.mp4"}}); err != nil {
		t.Fatal(err)
	}
	if err := exports.ReserveExport(ctx, "alice", "project", 2, "render-2", now); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `INSERT INTO generation_jobs(id,user_id,kind,status,created_at,updated_at)
		VALUES ('job-2','alice','render_clip','running',?,?)`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := exports.BindExport(ctx, "render-2", "job-2"); err != nil {
		t.Fatal(err)
	}
	second := clip.ExportWindow{UserID: "alice", CoverageID: "paid:alice", Start: first.End, End: first.End.Add(30 * 24 * time.Hour), Allowance: 2}
	if err := exports.OpenExportWindow(ctx, second, "second"); err != nil {
		t.Fatal(err)
	}
	if err := exports.RecoverExports(ctx); err != nil {
		t.Fatal(err)
	}
	active, _, _ := exports.CurrentExportWindow(ctx, "alice", now)
	if active.Reserved != 1 || active.Used != 1 {
		t.Fatalf("running job lost its origin slot: %+v", active)
	}
	if err := exports.ReleaseExport(ctx, "job-2"); err != nil {
		t.Fatal(err)
	}
	active, _, _ = exports.CurrentExportWindow(ctx, "alice", now)
	if active.Reserved != 1 {
		t.Fatalf("active job was released: %+v", active)
	}
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE generation_jobs SET status='failed',finished_at=?,updated_at=? WHERE id='job-2'`, now.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := exports.RecoverExports(ctx); err != nil {
		t.Fatal(err)
	}
	if err := exports.ReserveExport(ctx, "alice", "project", 3, "render-3", now); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.ExecContext(ctx, `UPDATE server_export_reservations SET created_at=? WHERE id='render-3'`, now.Add(-3*time.Minute).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if err := exports.RecoverExports(ctx); err != nil {
		t.Fatal(err)
	}
	old, ok, err := exports.CurrentExportWindow(ctx, "alice", now)
	if err != nil || !ok || old.Used != 1 || old.Reserved != 0 {
		t.Fatalf("old=%+v ok=%v err=%v", old, ok, err)
	}
	newWindow, ok, err := exports.CurrentExportWindow(ctx, "alice", first.End.Add(time.Minute))
	if err != nil || !ok || newWindow.Used != 0 || newWindow.Reserved != 0 || newWindow.Allowance != 2 {
		t.Fatalf("new=%+v ok=%v err=%v", newWindow, ok, err)
	}
}
