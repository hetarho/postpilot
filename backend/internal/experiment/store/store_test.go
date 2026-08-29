package store_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentstore "github.com/postpilot/backend/internal/experiment/store"
	"github.com/postpilot/backend/internal/platform/db"
)

func testStore(t *testing.T) (*experimentstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "experiment.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := "2026-08-29T00:00:00Z"
	for _, user := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES(?,?,?)`, user, "hash", now); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ slug, user string }{{"post-a", "alice"}, {"post-b", "bob"}} {
		if _, err := handle.Writer.Exec(`INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES(?,?,?,?)`, row.slug, row.user, now, now); err != nil {
			t.Fatal(err)
		}
	}
	return experimentstore.New(handle.Writer, handle.Reader), handle
}

func sample(id, user, slug string, at time.Time) experiment.Experiment {
	return experiment.Experiment{
		ID: id, UserID: user, PostSlug: slug, Stage: experiment.StageWrite, Status: experiment.StatusQueued,
		InputSnapshot: []byte(`{"private":true}`), InputHash: "hash", PromptVersion: "v1", CreatedAt: at,
		Candidates: []experiment.Candidate{
			{ID: id + "-left", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "a"}, ModelLabel: "A snapshot", DisplaySide: experiment.SideLeft, Status: experiment.CandidatePending},
			{ID: id + "-right", ExperimentID: id, Model: experiment.ModelRef{ProviderID: "p", ModelID: "b"}, ModelLabel: "B snapshot", DisplaySide: experiment.SideRight, Status: experiment.CandidatePending},
		},
	}
}

func TestStoreOwnershipStableSidesAndUnresolvedWriteGuard(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-1", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, found.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Candidates[0].DisplaySide != experiment.SideLeft || reloaded.Candidates[0].ModelLabel != "A snapshot" {
		t.Fatalf("reloaded = %+v", reloaded.Candidates)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	if pending, err := store.PendingForPost(ctx, "bob", "post-a"); err != nil || pending != nil {
		t.Fatalf("foreign pending = %+v, %v", pending, err)
	}
	if err := store.Create(ctx, sample("exp-2", "alice", "post-a", now.Add(time.Second))); !errors.Is(err, experiment.ErrInvalidState) {
		t.Fatalf("duplicate unresolved = %v", err)
	}
}

func TestStorePreservesSiblingOutputAndPurgesPrivateContent(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-1", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Second)
	left := found.Candidates[0]
	left.Status = experiment.CandidateSucceeded
	left.Output = []byte(`{"title":"paid result"}`)
	left.Usage = experiment.Usage{PromptTokens: 12, CostMicrousd: 7, CostSource: experiment.CostReported, LatencyMS: 90}
	left.FinishedAt = &finished
	right := found.Candidates[1]
	right.Status = experiment.CandidateFailed
	right.Error = "failed"
	right.Usage = experiment.Usage{CostSource: experiment.CostUnavailable, LatencyMS: 110}
	right.FinishedAt = &finished
	if err := store.CompleteCandidate(ctx, left); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteCandidate(ctx, right); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusPartial, &finished); err != nil {
		t.Fatal(err)
	}
	reloaded, _ := store.Get(ctx, found.ID)
	if string(reloaded.Candidates[0].Output) != string(left.Output) || reloaded.Candidates[1].Status != experiment.CandidateFailed {
		t.Fatalf("partial lost sibling output: %+v", reloaded.Candidates)
	}
	decided := now.Add(2 * time.Second)
	changed, err := store.Decide(ctx, found.ID, "alice", left.ID, experiment.StatusDecided, experiment.OutcomeUnpaired, decided, decided)
	if err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if n, err := store.PurgeExpired(ctx, decided.Add(time.Second)); err != nil || n != 1 {
		t.Fatalf("purge = %d, %v", n, err)
	}
	reloaded, _ = store.Get(ctx, found.ID)
	if len(reloaded.InputSnapshot) != 0 || len(reloaded.Candidates[0].Output) != 0 || reloaded.Candidates[0].ModelLabel != "A snapshot" || reloaded.Candidates[0].Usage.PromptTokens != 12 {
		t.Fatalf("purge removed durable metadata or retained payload: %+v", reloaded)
	}
}

func TestStorePostHookAndAccountCascade(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	postExperiment := sample("exp-post", "alice", "post-a", now)
	if err := store.Create(ctx, postExperiment); err != nil {
		t.Fatal(err)
	}
	candidate := postExperiment.Candidates[0]
	candidate.Status = experiment.CandidateSucceeded
	candidate.Output = []byte(`{"secret":true}`)
	if err := store.CompleteCandidate(ctx, candidate); err != nil {
		t.Fatal(err)
	}
	if err := store.PurgePost(ctx, "alice", "post-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM posts WHERE slug='post-a'`); err != nil {
		t.Fatal(err)
	}
	reloaded, err := store.Get(ctx, postExperiment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.PostSlug != "" || len(reloaded.InputSnapshot) != 0 || len(reloaded.Candidates[0].Output) != 0 {
		t.Fatalf("post deletion retained content: %+v", reloaded)
	}

	accountExperiment := sample("exp-account", "bob", "post-b", now)
	if err := store.Create(ctx, accountExperiment); err != nil {
		t.Fatal(err)
	}
	if _, err := handle.Writer.Exec(`DELETE FROM users WHERE id='bob'`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, accountExperiment.ID); !errors.Is(err, experiment.ErrNotFound) {
		t.Fatalf("account cascade get = %v", err)
	}
}

func TestStoreRecoversInterruptedCandidatesAtomically(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-running", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusRunning, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.StartCandidate(ctx, found.ID, found.Candidates[0].ID, now); err != nil {
		t.Fatal(err)
	}
	left := found.Candidates[0]
	left.Status = experiment.CandidateSucceeded
	left.Output = []byte(`{"ok":true}`)
	left.FinishedAt = &now
	if err := store.CompleteCandidate(ctx, left); err != nil {
		t.Fatal(err)
	}

	finished := now.Add(time.Minute)
	count, err := store.RecoverInterrupted(ctx, "restart", finished)
	if err != nil || count != 1 {
		t.Fatalf("recover = %d, %v", count, err)
	}
	reloaded, _ := store.Get(ctx, found.ID)
	if reloaded.Status != experiment.StatusPartial || reloaded.FinishedAt == nil {
		t.Fatalf("reloaded = %+v", reloaded)
	}
	if reloaded.Candidates[0].Status != experiment.CandidateSucceeded || reloaded.Candidates[1].Status != experiment.CandidateFailed || reloaded.Candidates[1].Error != "restart" {
		t.Fatalf("candidates = %+v", reloaded.Candidates)
	}
	if count, err := store.RecoverInterrupted(ctx, "restart", finished); err != nil || count != 0 {
		t.Fatalf("second recovery = %d, %v", count, err)
	}
}

func TestPendingWriteKeepsUnappliedVerdictRecoverable(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)
	found := sample("exp-apply", "alice", "post-a", now)
	if err := store.Create(ctx, found); err != nil {
		t.Fatal(err)
	}
	finished := now.Add(time.Second)
	for _, candidate := range found.Candidates {
		candidate.Status = experiment.CandidateSucceeded
		candidate.Output = []byte(`{"title":"ok"}`)
		candidate.FinishedAt = &finished
		if err := store.CompleteCandidate(ctx, candidate); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetStatus(ctx, found.ID, experiment.StatusReview, &finished); err != nil {
		t.Fatal(err)
	}
	decided := now.Add(2 * time.Second)
	if changed, err := store.Decide(ctx, found.ID, "alice", found.Candidates[0].ID, experiment.StatusDecided, experiment.OutcomeWinner, decided, decided.Add(30*24*time.Hour)); err != nil || !changed {
		t.Fatalf("decide = %v, %v", changed, err)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending == nil || pending.ID != found.ID {
		t.Fatalf("unapplied pending = %+v, %v", pending, err)
	}
	if err := store.SetApplied(ctx, found.ID, "alice", decided.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if pending, err := store.PendingForPost(ctx, "alice", "post-a"); err != nil || pending != nil {
		t.Fatalf("applied pending = %+v, %v", pending, err)
	}
}
