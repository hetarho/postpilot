package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
)

// insertPostJob writes one post generation's admission and its calls the way the ledger leaves
// them after settlement. appliedE4 of 0 is a job admitted before rates were frozen.
func insertPostJob(t *testing.T, handle *db.DB, jobID, userID, kind, reason string, settled time.Time, appliedE4 int64, calls ...[3]any) {
	t.Helper()
	ctx := context.Background()
	var source, date, reference, applied any
	if appliedE4 > 0 {
		source, date, reference, applied = "korea-eximbank", "2026-09-29", appliedE4, appliedE4
	}
	if _, err := handle.Writer.ExecContext(ctx,
		`INSERT INTO usage_admissions (user_id, kind, job_id, created_at, settled_at, settlement_reason,
		   fx_source, fx_publication_date, fx_reference_e4, fx_applied_e4)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		userID, kind, jobID, settled.Add(-time.Minute).UTC().Format(time.RFC3339Nano),
		settled.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"), reason, source, date, reference, applied); err != nil {
		t.Fatalf("insert admission %s: %v", jobID, err)
	}
	for _, call := range calls {
		if _, err := handle.Writer.ExecContext(ctx,
			`INSERT INTO usage_events (user_id, kind, job_id, stage, model, prompt_tokens, completion_tokens,
			   cost_microusd, cost_source, created_at)
			 VALUES (?, ?, ?, ?, ?, 10, 10, ?, ?, ?)`,
			userID, kind, jobID, call[0], "openrouter/vendor/model", call[1], call[2],
			settled.Add(-30*time.Second).UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatalf("insert event %s: %v", jobID, err)
		}
	}
}

// QUOTA-64: only succeeded `generate` settlements inside the window, admitted at a frozen rate
// and with every call of the stage priced, feed a stage's figure; each converts at its own rate.
func TestRecentPostFiguresReadOnlyEligibleSettlements(t *testing.T) {
	svc, handle := newServiceWithDB(t)
	ctx := context.Background()
	for _, id := range []string{"carol", "dave"} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO users (id, password_hash, plan, created_at) VALUES (?,'hash','free',?)",
			id, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	day := now.Add(-24 * time.Hour)
	// 1,000,000 micro-USD at 1,400 KRW/USD is 1,400 credits; 500,000 is 700.
	insertPostJob(t, handle, "job-a", "alice", "generate", "succeeded", day, 14_000_000,
		[3]any{"observe", 1_000_000, "reported"}, [3]any{"write", 500_000, "reported"})
	insertPostJob(t, handle, "job-b", "bob", "generate", "succeeded", day, 15_000_000,
		[3]any{"write", 600_000, "estimated"})
	insertPostJob(t, handle, "job-c", "carol", "generate", "succeeded", day, 14_000_000,
		[3]any{"write", 400_000, "reported"}, [3]any{"write", 1, "unavailable"})
	insertPostJob(t, handle, "job-failed", "dave", "generate", "failed", day, 14_000_000,
		[3]any{"write", 900_000, "reported"})
	insertPostJob(t, handle, "job-old", "dave", "generate", "succeeded", now.Add(-31*24*time.Hour), 14_000_000,
		[3]any{"write", 900_000, "reported"})
	insertPostJob(t, handle, "job-revise", "dave", "revise", "succeeded", day, 14_000_000,
		[3]any{"write", 900_000, "reported"})
	insertPostJob(t, handle, "job-unfrozen", "dave", "generate", "succeeded", day, 0,
		[3]any{"write", 900_000, "reported"})

	figures, err := svc.RecentPostFigures(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ref := llm.ModelRef{ProviderID: "openrouter", ModelID: "vendor/model"}
	write := figures[usage.StageModel{Stage: "write", Model: ref}]
	// job-a 700 and job-b 900 (600,000 at 1,500); job-c has an unpriced write call.
	if write.Posts != 2 || write.Accounts != 2 || write.Credits != 800 {
		t.Fatalf("write figure = %+v, want 2 posts from 2 accounts at 800", write)
	}
	observe := figures[usage.StageModel{Stage: "observe", Model: ref}]
	if observe.Posts != 1 || observe.Credits != 1_400 {
		t.Fatalf("observe figure = %+v", observe)
	}
	if write.Eligible() || observe.Eligible() {
		t.Fatal("a sample below the floor reads as eligible")
	}
	if len(figures) != 2 {
		t.Fatalf("figures = %+v, want write and observe only", figures)
	}
}
