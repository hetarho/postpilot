package job_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
)

type harness struct {
	queue  *job.Queue
	store  *jobstore.Store
	handle *db.DB
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", id, now); err != nil {
			t.Fatalf("insert user %s: %v", id, err)
		}
	}
	for _, id := range []string{"alice", "bob"} {
		for _, suffix := range []string{"", "-2"} {
			if _, err := handle.Writer.ExecContext(ctx,
				"INSERT INTO voices (id, user_id, name, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
				"voice-"+id+suffix, id, "voice"+suffix, suffix == "", now, now); err != nil {
				t.Fatalf("insert voice %s: %v", id, err)
			}
		}
	}
	for _, row := range []struct{ slug, user string }{{"post-a", "alice"}, {"post-b", "alice"}, {"post-bob", "bob"}} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO posts (slug, user_id, voice_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			row.slug, row.user, "voice-"+row.user, now, now); err != nil {
			t.Fatalf("insert post %s: %v", row.slug, err)
		}
	}
	store := jobstore.New(handle.Writer, handle.Reader)
	return &harness{queue: job.New(store, 10*time.Millisecond), store: store, handle: handle}
}

func waitFor(t *testing.T, queue *job.Queue, id, userID string, match func(*job.JobSummary) bool) *job.JobSummary {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		found, err := queue.Get(context.Background(), id, userID)
		if err == nil && match(found) {
			return found
		}
		time.Sleep(5 * time.Millisecond)
	}
	found, err := queue.Get(context.Background(), id, userID)
	t.Fatalf("job %s did not reach expected state: found=%+v err=%v", id, found, err)
	return nil
}

func postSlug(value string) *string { return &value }

func TestEnqueueReturnsBeforeHandlerAndWorkerPublishesProgress(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	release := make(chan struct{})
	h.queue.Register("fake", func(_ context.Context, _ job.Job, progress job.Progress) error {
		progress("observe", 3, 8)
		close(started)
		<-release
		progress("write", 1, 1)
		return nil
	})
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.queue.Run(workerCtx)

	type result struct {
		id  string
		err error
	}
	enqueued := make(chan result, 1)
	go func() {
		id, err := h.queue.Enqueue(context.Background(), job.NewJob{
			Kind: "fake", UserID: "alice", PostSlug: postSlug("post-a"),
		})
		enqueued <- result{id: id, err: err}
	}()

	var queued result
	select {
	case queued = <-enqueued:
		if queued.err != nil {
			t.Fatalf("Enqueue: %v", queued.err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Enqueue blocked on the handler")
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	running := waitFor(t, h.queue, queued.id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusRunning && found.Stage == "observe" && found.ProgressDone == 3
	})
	if running.ProgressTotal != 8 {
		t.Errorf("progress total = %d, want 8", running.ProgressTotal)
	}

	// A provider call may be in flight here; a separate write must not be waiting on a
	// transaction held by the job layer.
	if _, err := h.handle.Writer.Exec(
		"INSERT INTO users (id, password_hash, created_at) VALUES ('writer-proof', 'hash', ?)",
		time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatalf("concurrent writer while handler blocked: %v", err)
	}

	close(release)
	done := waitFor(t, h.queue, queued.id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusDone
	})
	if done.Stage != "write" || done.ProgressDone != 1 || done.ProgressTotal != 1 {
		t.Errorf("final progress = %+v", done)
	}
}

func TestEnqueueGuardsPostAndUserKind(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	first, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindGenerate, UserID: "alice", PostSlug: postSlug("post-a")})
	if err != nil {
		t.Fatalf("first post job: %v", err)
	}
	_, err = h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindRevise, UserID: "alice", PostSlug: postSlug("post-a")})
	var active *job.ErrAlreadyInProgress
	if !errors.As(err, &active) || active.ActiveID != first {
		t.Fatalf("second post job = %v, want active %s", err, first)
	}

	analysis, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: "alice"})
	if err != nil {
		t.Fatalf("first analysis: %v", err)
	}
	_, err = h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: "alice"})
	if !errors.As(err, &active) || active.ActiveID != analysis {
		t.Fatalf("second analysis = %v, want active %s", err, analysis)
	}
}

// Plan 10 A14: voice-owned work is guarded per (voice, kind), so two voices of one account
// analyze concurrently while one voice still attaches to its active job; the frozen voice
// is projected back, and any job frozen to a voice makes that voice busy for deletion.
func TestVoiceOwnedJobsAreGuardedPerVoice(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	first, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: "alice", VoiceID: "voice-alice"})
	if err != nil {
		t.Fatalf("first voice analysis: %v", err)
	}
	second, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: "alice", VoiceID: "voice-alice-2"})
	if err != nil {
		t.Fatalf("second voice analysis: %v", err)
	}
	var active *job.ErrAlreadyInProgress
	if _, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: "alice", VoiceID: "voice-alice"}); !errors.As(err, &active) || active.ActiveID != first {
		t.Fatalf("same voice analysis = %v, want active %s", err, first)
	}
	if _, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindLearnVoice, UserID: "alice", VoiceID: "voice-alice"}); err != nil {
		t.Fatalf("another kind for the same voice: %v", err)
	}
	found, err := h.queue.Get(ctx, second, "alice")
	if err != nil || found.VoiceID != "voice-alice-2" {
		t.Fatalf("job projection = %+v err=%v", found, err)
	}
	if summary, err := h.queue.ActiveForVoiceKind(ctx, "voice-alice-2", job.KindAnalyzeVoice); err != nil || summary == nil || summary.ID != second {
		t.Fatalf("active for voice = %+v err=%v", summary, err)
	}
	for voiceID, want := range map[string]bool{"voice-alice": true, "voice-alice-2": true, "voice-bob": false} {
		if busy, err := h.queue.HasActiveForVoice(ctx, voiceID); err != nil || busy != want {
			t.Fatalf("HasActiveForVoice(%s) = %v err=%v, want %v", voiceID, busy, err, want)
		}
	}
	// A post-backed job frozen to a voice also holds that voice.
	if _, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindGenerate, UserID: "bob", PostSlug: postSlug("post-bob"), VoiceID: "voice-bob"}); err != nil {
		t.Fatal(err)
	}
	if busy, _ := h.queue.HasActiveForVoice(ctx, "voice-bob"); !busy {
		t.Fatal("post-backed job did not hold its voice")
	}
	// The (voice, kind) index closes the race the precheck cannot: a direct insert races.
	if _, err := h.handle.Writer.ExecContext(ctx, "INSERT INTO generation_jobs(id,user_id,voice_id,kind,status,progress_done,progress_total,payload,created_at,updated_at) VALUES('dup','alice','voice-alice','analyze_voice','queued',0,0,'',?,?)", "2026-08-30T00:00:00Z", "2026-08-30T00:00:00Z"); err == nil {
		t.Fatal("the database accepted a second active analysis for one voice")
	}
}

// Learning and comparison jobs carry the triggering post for post-level lifecycle guards,
// but that must not weaken their per-voice serialization.
func TestPostBackedVoiceOwnedJobsAreAlsoGuardedPerVoice(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.handle.Writer.ExecContext(ctx,
		"INSERT INTO posts (slug, user_id, voice_id, created_at, updated_at) VALUES ('post-c', 'alice', 'voice-alice-2', ?, ?)",
		"2026-08-30T00:00:00Z", "2026-08-30T00:00:00Z"); err != nil {
		t.Fatalf("insert second-voice post: %v", err)
	}
	first, err := h.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindLearnVoice, UserID: "alice", VoiceID: "voice-alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatalf("first learning job: %v", err)
	}
	var active *job.ErrAlreadyInProgress
	if _, err := h.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindLearnVoice, UserID: "alice", VoiceID: "voice-alice", PostSlug: postSlug("post-b"),
	}); !errors.As(err, &active) || active.ActiveID != first {
		t.Fatalf("same voice learning on another post = %v, want active %s", err, first)
	}
	if _, err := h.queue.Enqueue(ctx, job.NewJob{
		Kind: job.KindLearnVoice, UserID: "alice", VoiceID: "voice-alice-2", PostSlug: postSlug("post-c"),
	}); err != nil {
		t.Fatalf("another voice learning on another post: %v", err)
	}
	if _, err := h.handle.Writer.ExecContext(ctx,
		"INSERT INTO generation_jobs(id,post_slug,user_id,voice_id,kind,status,progress_done,progress_total,payload,created_at,updated_at) VALUES('dup-post','post-b','alice','voice-alice','learn_voice','queued',0,0,'',?,?)",
		"2026-08-30T00:00:00Z", "2026-08-30T00:00:00Z"); err == nil {
		t.Fatal("the database accepted a second post-backed learning job for one voice")
	}
	if err := h.store.Insert(ctx, job.Job{
		ID: "store-dup", Kind: job.KindLearnVoice, UserID: "alice", VoiceID: "voice-alice",
		PostSlug: postSlug("post-b"), CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}); !errors.Is(err, job.ErrActiveConflict) {
		t.Fatalf("store duplicate error = %v, want ErrActiveConflict", err)
	}
}

func TestEnqueueRejectsPostOwnedByAnotherUser(t *testing.T) {
	h := newHarness(t)
	_, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: job.KindGenerate, UserID: "alice", PostSlug: postSlug("post-bob"),
	})
	if !errors.Is(err, job.ErrInvalidTarget) {
		t.Fatalf("foreign post enqueue = %v, want ErrInvalidTarget", err)
	}
	var count int
	if scanErr := h.handle.Reader.QueryRow(
		"SELECT COUNT(*) FROM generation_jobs WHERE post_slug = 'post-bob'",
	).Scan(&count); scanErr != nil || count != 0 {
		t.Fatalf("foreign post job count = %d, err = %v", count, scanErr)
	}
}

// The active-voice check in a service can race with deletion. The insert trigger is the
// durable arbiter, and the store must preserve that lifecycle meaning instead of leaking a
// driver error that the RPC edge would turn into Internal.
func TestStoreMapsInactiveVoiceTrigger(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	if _, err := h.handle.Writer.ExecContext(ctx,
		"UPDATE voices SET deleted_at = ? WHERE id = 'voice-alice-2'",
		"2026-08-30T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	err := h.store.Insert(ctx, job.Job{
		ID: "deleted-voice-job", Kind: job.KindAnalyzeVoice, UserID: "alice", VoiceID: "voice-alice-2",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	if !errors.Is(err, job.ErrVoiceUnavailable) {
		t.Fatalf("inactive voice insert = %v, want ErrVoiceUnavailable", err)
	}
}

func TestPanicFailsOneJobAndWorkerContinues(t *testing.T) {
	h := newHarness(t)
	h.queue.Register("sometimes-panic", func(_ context.Context, found job.Job, _ job.Progress) error {
		if string(found.Payload) == "panic" {
			panic("boom")
		}
		return nil
	})
	first, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "sometimes-panic", UserID: "alice", PostSlug: postSlug("post-a"), Payload: []byte("panic"),
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "sometimes-panic", UserID: "alice", PostSlug: postSlug("post-b"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.queue.Run(ctx)

	failed := waitFor(t, h.queue, first, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusFailed
	})
	if failed.Error != job.PanicMessage {
		t.Errorf("panic error = %q", failed.Error)
	}
	waitFor(t, h.queue, second, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusDone
	})
}

func TestProviderMessageIsPreserved(t *testing.T) {
	h := newHarness(t)
	h.queue.Register("quota", func(context.Context, job.Job, job.Progress) error {
		return &llm.ProviderError{Provider: "stub", Status: 429, Message: "daily free quota exhausted", Kind: llm.ErrRateLimited}
	})
	id, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "quota", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.queue.Run(ctx)
	failed := waitFor(t, h.queue, id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusFailed
	})
	if failed.Error != "daily free quota exhausted" {
		t.Errorf("error = %q", failed.Error)
	}
}

func TestOutputTruncationGetsActionableMessage(t *testing.T) {
	h := newHarness(t)
	h.queue.Register("truncated", func(context.Context, job.Job, job.Progress) error {
		return fmt.Errorf("write: %w", llm.ErrOutputTruncated)
	})
	id, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "truncated", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.queue.Run(ctx)
	failed := waitFor(t, h.queue, id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusFailed
	})
	for _, required := range []string{"출력 예산", "목표 길이", "다른 모델"} {
		if !strings.Contains(failed.Error, required) {
			t.Errorf("error = %q, want %q", failed.Error, required)
		}
	}
}

func TestSweepAndOwnership(t *testing.T) {
	h := newHarness(t)
	id, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "fake", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.PickNextQueued(context.Background(), time.Now()); err != nil {
		t.Fatalf("pick: %v", err)
	}
	if n, err := h.queue.SweepRunning(context.Background()); err != nil || n != 1 {
		t.Fatalf("SweepRunning = %d, %v", n, err)
	}
	found, err := h.queue.Get(context.Background(), id, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if found.Status != job.StatusFailed || found.Error != job.RestartMessage {
		t.Errorf("swept = %+v", found)
	}
	if _, err := h.queue.Get(context.Background(), id, "bob"); !errors.Is(err, job.ErrForbidden) {
		t.Errorf("foreign Get = %v", err)
	}
	if _, err := h.queue.Get(context.Background(), "missing", "alice"); !errors.Is(err, job.ErrNotFound) {
		t.Errorf("missing Get = %v", err)
	}
}

func TestBootSweepHoldsQueuedPersonalizationOnly(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	ids := make([]string, 0, 3)
	for _, kind := range []string{job.KindLearnVoice, job.KindCompareVoiceRule, job.KindValidateVoiceProfile} {
		id, err := h.queue.Enqueue(ctx, job.NewJob{Kind: kind, UserID: "alice"})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	ordinary, err := h.queue.Enqueue(ctx, job.NewJob{Kind: "ordinary", UserID: "alice", PostSlug: postSlug("post-a")})
	if err != nil {
		t.Fatal(err)
	}
	if n, err := h.queue.SweepQueuedPersonalization(ctx); err != nil || n != 3 {
		t.Fatalf("sweep queued personalization = %d, %v", n, err)
	}
	for _, id := range ids {
		found, err := h.queue.Get(ctx, id, "alice")
		if err != nil || found.Status != job.StatusFailed || found.Error != job.PersonalizationRestartMessage {
			t.Fatalf("personalization job = %+v err=%v", found, err)
		}
	}
	found, err := h.queue.Get(ctx, ordinary, "alice")
	if err != nil || found.Status != job.StatusQueued {
		t.Fatalf("ordinary queued job changed: %+v err=%v", found, err)
	}
}

func TestFailQueuedIsOwnerScopedAndCannotStopRunningWork(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	id, err := h.queue.Enqueue(ctx, job.NewJob{Kind: job.KindValidateVoiceProfile, UserID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if failed, err := h.queue.FailQueued(ctx, id, "bob", "link failed"); err != nil || failed {
		t.Fatalf("foreign fail queued = %v, %v", failed, err)
	}
	if failed, err := h.queue.FailQueued(ctx, id, "alice", "link failed"); err != nil || !failed {
		t.Fatalf("owner fail queued = %v, %v", failed, err)
	}
	found, err := h.queue.Get(ctx, id, "alice")
	if err != nil || found.Status != job.StatusFailed || found.Error != "link failed" {
		t.Fatalf("failed queued job = %+v, %v", found, err)
	}
	if failed, err := h.queue.FailQueued(ctx, id, "alice", "again"); err != nil || failed {
		t.Fatalf("terminal fail queued = %v, %v", failed, err)
	}
}

func TestShutdownLeavesRunningForNextSweep(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	h.queue.Register("shutdown", func(ctx context.Context, _ job.Job, _ job.Progress) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	id, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "shutdown", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go h.queue.Run(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()

	found := waitFor(t, h.queue, id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusRunning
	})
	if found.Status != job.StatusRunning {
		t.Fatalf("status = %s", found.Status)
	}
	if n, err := h.queue.SweepRunning(context.Background()); err != nil || n != 1 {
		t.Fatalf("next boot sweep = %d, %v", n, err)
	}
}

func TestShutdownAfterSuccessfulHandlerPersistsDone(t *testing.T) {
	h := newHarness(t)
	started := make(chan struct{})
	h.queue.Register("shutdown-after-success", func(ctx context.Context, _ job.Job, _ job.Progress) error {
		close(started)
		<-ctx.Done()
		return nil
	})
	id, err := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "shutdown-after-success", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go h.queue.Run(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancel()

	waitFor(t, h.queue, id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusDone
	})
}

func TestOrdinaryHandlerErrorIsNonEmpty(t *testing.T) {
	h := newHarness(t)
	h.queue.Register("error", func(context.Context, job.Job, job.Progress) error {
		return fmt.Errorf("사진을 읽지 못했어요")
	})
	id, _ := h.queue.Enqueue(context.Background(), job.NewJob{
		Kind: "error", UserID: "alice", PostSlug: postSlug("post-a"),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.queue.Run(ctx)
	found := waitFor(t, h.queue, id, "alice", func(found *job.JobSummary) bool {
		return found.Status == job.StatusFailed
	})
	if found.Error == "" {
		t.Fatal("failed job has an empty error")
	}
}
