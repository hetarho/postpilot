package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	experimentapp "github.com/postpilot/backend/internal/experiment/app"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/post"
)

func admitLifecycleFixture(t *testing.T, c *contexts, key, slug string) experiment.TestExecutionWork {
	t.Helper()
	start := experiment.TestStart{UserID: "alice", RequestKey: key, QuoteKey: "quote-" + key, Factor: experiment.FactorModel, ModelStage: experiment.StageWrite, Count: 2, Input: experiment.TestInput{SourcePostSlug: slug, Material: "explicit owned material", TargetLanguage: "ko", TagCount: 4}}
	privateSource := []byte(`{"material":"private full source","origins":{"version":1,"sources":[{"id":"frozen-memory","kind":"memory","text":"private approved origin evidence","available":true}]}}`)
	plan := experiment.TestPlan{Free: true, Snapshot: experiment.TestSnapshot{Common: privateSource, Hash: "frozen-hash", PromptVersion: "frozen-version"}, Calls: []experiment.TestCall{{Ref: experiment.ModelRef{ProviderID: "p", ModelID: "writer"}, Stage: experiment.StageWrite, Count: 2, PromptTokens: 30000, CompletionTokens: 8192}}}
	for _, model := range []string{"one", "two"} {
		ref := experiment.TestEntrantRef{SourceKind: "model", Model: experiment.ModelRef{ProviderID: "p", ModelID: model}}
		start.Entrants = append(start.Entrants, ref)
		plan.Snapshot.Variants = append(plan.Snapshot.Variants, experiment.FrozenTestVariant{Reference: ref, Content: []byte("private entrant " + model), Revision: model, SemanticKey: model})
	}
	if err := c.experimentStore.PutTestQuote(t.Context(), experiment.TestQuote{Key: start.QuoteKey, UserID: "alice", Fingerprint: experiment.WritingTestStartFingerprint(start), Start: start, Plan: plan, Free: true, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Retention: 30 * 24 * time.Hour}); err != nil {
		t.Fatal(err)
	}
	found, _, err := c.experimentStore.AdmitTest(t.Context(), start, plan)
	if err != nil {
		t.Fatal(err)
	}
	work, err := c.experimentStore.PreparedTestWork(t.Context(), "alice", found.ID)
	if err != nil {
		t.Fatal(err)
	}
	return work
}
func lifecycleContexts(t *testing.T) (*contexts, *platform) {
	t.Helper()
	t.Setenv("MAIL_DRIVER", "log")
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := wiringPlatform(t, cfg)
	c, err := buildContexts(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	for _, user := range []string{"alice", "bob"} {
		if _, err := p.db.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)", user, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			t.Fatal(err)
		}
	}
	return c, p
}
func TestWritingTestSourceDeleteFencesPrivatePayloadAndCancelsUnattachedJob(t *testing.T) {
	c, p := lifecycleContexts(t)
	if _, err := p.db.Writer.Exec("INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('source','alice',?,?)", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	work := admitLifecycleFixture(t, c, "source-run", "source")
	payload, err := json.Marshal(struct {
		Version int                           `json:"version"`
		Fence   experiment.TestExecutionFence `json:"fence"`
	}{Version: 1, Fence: work.Fence})
	if err != nil {
		t.Fatal(err)
	}
	jobs := jobstore.New(p.db.Writer, p.db.Reader, jobKinds())
	if err := jobs.Insert(t.Context(), job.Job{ID: work.Fence.JobID, UserID: "alice", Kind: experimentapp.WritingTestJobKind, CancellationPolicyVersion: 1, TargetLanguage: "ko", Payload: payload, CreatedAt: time.Now(), UpdatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.experimentStore.BindTestJob(t.Context(), work.Fence, work.Fence.JobID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.experimentStore.BeginTestExecution(t.Context(), work.Fence); err != nil {
		t.Fatal(err)
	}
	if err := c.experimentStore.SaveTestCheckpoint(t.Context(), work.Fence, "shared", []byte(`{"answer":{"origins":{"version":1,"sources":[{"id":"private-result","text":"private captured origin evidence"}]}}}`)); err != nil {
		t.Fatal(err)
	}
	registerJobs(c)
	if err := c.post.DeletePost(t.Context(), "alice", "source"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.post.AttachedImages(t.Context(), "alice", "source"); !errors.Is(err, post.ErrNotFound) {
		t.Fatal("source remained", err)
	}
	found, err := c.experimentStore.GetTest(t.Context(), "alice", work.Fence.TestID)
	if err != nil {
		t.Fatal(err)
	}
	if found.PurgeFence != 1 || found.Status != experiment.TestCancelled || found.SourcePostSlug != "" || len(found.CommonSnapshot) != 0 {
		t.Fatal("source private payload not fenced", found)
	}
	stopped, err := jobs.GetByID(t.Context(), work.Fence.JobID)
	if err != nil || stopped.Status != job.StatusCancelled || stopped.CancelRequestedAt == nil {
		t.Fatal("provider job not cancelled", stopped, err)
	}
	if err := c.experimentStore.SaveTestCheckpoint(t.Context(), work.Fence, "shared", []byte("late private result")); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatal("late callback restored source", err)
	}
	if err := c.writingTestLifecycle.PurgePost(t.Context(), "alice", "source"); err != nil {
		t.Fatal("purge recovery not idempotent", err)
	}
}
func TestWritingTestBootClosesNeverCreatedAttemptBeforeWorkersOrHTTP(t *testing.T) {
	c, p := lifecycleContexts(t)
	work := admitLifecycleFixture(t, c, "lost-before-job", "")
	restarted, err := buildContexts(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	found, err := restarted.experimentStore.GetTest(t.Context(), "alice", work.Fence.TestID)
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != experiment.TestFailed || found.ConfirmedCredits != 0 || found.ReservedCredits != 0 {
		t.Fatal("unissued boot attempt not reconciled", found)
	}
	if _, err := restarted.jobs.Result(t.Context(), "alice", work.Fence.JobID); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("boot recreated missing provider job", err)
	}
	if err := restarted.writingTestLifecycle.Recover(t.Context()); err != nil {
		t.Fatal("boot recovery replay", err)
	}
}

func TestWritingTestWiredExpiryPurgesPrivateInputOnceAndKeepsTerminalMetadata(t *testing.T) {
	c, p := lifecycleContexts(t)
	work := admitLifecycleFixture(t, c, "expiry", "")
	if err := c.writingTestLifecycle.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	before, err := c.experimentStore.GetTest(t.Context(), "alice", work.Fence.TestID)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.CommonSnapshot) == 0 || before.Status != experiment.TestFailed {
		t.Fatal("fixture not terminal and retained", before)
	}
	if _, err := p.db.Writer.Exec("UPDATE writing_tests SET content_expires_at=? WHERE id=?", time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano), before.ID); err != nil {
		t.Fatal(err)
	}
	if count, err := c.writingTestLifecycle.PurgeExpired(t.Context(), time.Now()); err != nil || count != 1 {
		t.Fatal("expiry did not run through current/legacy lifecycle", count, err)
	}
	after, err := c.experimentStore.GetTest(t.Context(), "alice", before.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.Factor != before.Factor || after.Count != before.Count || after.Status != before.Status || after.ConfirmedCredits != before.ConfirmedCredits || after.PurgeFence != 1 || len(after.CommonSnapshot) != 0 {
		t.Fatal("private expiry lost terminal metadata", before, after)
	}
	for _, candidate := range after.Candidates {
		if len(candidate.FrozenVariant) != 0 || len(candidate.Output) != 0 {
			t.Fatal("candidate private payload survived expiry", candidate)
		}
	}
	if count, err := c.writingTestLifecycle.PurgeExpired(t.Context(), time.Now()); err != nil || count != 0 {
		t.Fatal("expiry repeated", count, err)
	}
	if err := c.experimentStore.SaveTestCheckpoint(t.Context(), work.Fence, "shared", []byte("late restored payload")); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatal("expiry callback restored private material", err)
	}
	var admissions int
	if err := p.db.Reader.QueryRow("SELECT COUNT(*) FROM usage_admissions WHERE user_id='alice' AND kind=?", experimentapp.WritingTestJobKind).Scan(&admissions); err != nil || admissions != 0 {
		t.Fatal("retention admitted new work", admissions, err)
	}
}

func TestWritingTestProductionOrdinaryJobPortsFilterActiveAndLatestRows(t *testing.T) {
	c, p := lifecycleContexts(t)
	at := time.Now().UTC()
	if _, err := p.db.Writer.Exec("INSERT INTO posts(slug,user_id,created_at,updated_at) VALUES('source','alice',?,?)", at.Format(time.RFC3339Nano), at.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	store := jobstore.New(p.db.Writer, p.db.Reader, jobKinds())
	if err := store.Insert(t.Context(), job.Job{ID: "ordinary", UserID: "alice", Kind: job.KindGenerate, Subjects: []job.Subject{{Dimension: post.JobSubject, ID: "source"}}, TargetLanguage: "ko", CreatedAt: at, UpdatedAt: at}); err != nil {
		t.Fatal(err)
	}
	reader := experimentapp.NewPostJobs(c.jobs, ordinaryPostWriteKinds(), postContentKinds())
	active, err := reader.ActiveOrdinaryForPost(t.Context(), "alice", "source")
	if err != nil || active == nil || active.ID != "ordinary" {
		t.Fatal(active, err)
	}
	if foreign, err := reader.ActiveOrdinaryForPost(t.Context(), "bob", "source"); err != nil || foreign != nil {
		t.Fatal("foreign ordinary job exposed", foreign, err)
	}
	guard := experimentapp.NewPostTestJobGuard(func(tx *sql.Tx) experimentapp.OrdinaryWriteJobs { return jobstore.NewTx(tx, jobKinds()) }, ordinaryPostWriteKinds())
	tx, err := p.db.Writer.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	busy, err := guard.HasOrdinaryWrite(t.Context(), tx, "alice", "source")
	_ = tx.Rollback()
	if err != nil || !busy {
		t.Fatal("transactional source publication ignored ordinary writer", busy, err)
	}
	if _, err := p.db.Writer.Exec("UPDATE generation_jobs SET status='done' WHERE id='ordinary'"); err != nil {
		t.Fatal(err)
	}
	if err := store.Insert(t.Context(), job.Job{ID: "unrelated-newer", UserID: "alice", Kind: job.KindExtractMemory, Subjects: []job.Subject{{Dimension: post.JobSubject, ID: "source"}}, CreatedAt: at.Add(time.Second), UpdatedAt: at.Add(time.Second)}); err != nil {
		t.Fatal(err)
	}
	active, err = reader.ActiveOrdinaryForPost(t.Context(), "alice", "source")
	if err != nil || active != nil {
		t.Fatal("unrelated operation blocks ordinary writer", active, err)
	}
	latest, err := reader.LatestOrdinaryForPosts(t.Context(), "alice", []string{"source"})
	if err != nil || latest["source"].ID != "ordinary" || latest["source"].Status != job.StatusDone {
		t.Fatal("new unrelated job masked ordinary success", latest, err)
	}
	tx, err = p.db.Writer.BeginTx(t.Context(), nil)
	if err != nil {
		t.Fatal(err)
	}
	busy, err = guard.HasOrdinaryWrite(t.Context(), tx, "alice", "source")
	_ = tx.Rollback()
	if err != nil || busy {
		t.Fatal("publication rejected an unrelated operation", busy, err)
	}
}
