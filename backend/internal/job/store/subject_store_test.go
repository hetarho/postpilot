package store_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/platform/db"
)

const (
	postSubject        = "post"
	voiceSubject       = "voice"
	clipProjectSubject = "clip_project"
	experimentSubject  = "model_experiment"
)

func testKinds() jobstore.Kinds {
	return jobstore.Kinds{
		Deferred:    []string{"generate_clip", "render_clip", "revise_clip"},
		Cancellable: []string{"generate_clip", "render_clip", "revise_clip"},
		Authorized:  []string{"generate_clip", "revise_clip"},
	}
}

// subjectHarness is a migrated temp database with the rows every subject dimension needs.
func subjectHarness(t *testing.T) (*jobstore.Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "subjects.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// The schema this task adds a generated column to carries thirteen triggers and two
	// partial unique indexes; a rebuild would be visible here.
	var integrity string
	if err := handle.Reader.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		t.Fatalf("integrity check: %v", err)
	}
	if integrity != "ok" {
		t.Fatalf("integrity check = %q, want ok", integrity)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, user := range []string{"alice", "bob"} {
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", user, now); err != nil {
			t.Fatalf("insert user %s: %v", user, err)
		}
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO voices (id, user_id, name, is_default, created_at, updated_at) VALUES (?, ?, 'voice', 1, ?, ?)",
			"voice-"+user, user, now, now); err != nil {
			t.Fatalf("insert voice %s: %v", user, err)
		}
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO posts (slug, user_id, voice_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
			"post-"+user, user, "voice-"+user, now, now); err != nil {
			t.Fatalf("insert post %s: %v", user, err)
		}
		if _, err := handle.Writer.ExecContext(ctx,
			"INSERT INTO clip_projects (id, user_id, title, ratio, target_duration_ms, created_at, updated_at) VALUES (?, ?, 'clip', 'square', 15000, ?, ?)",
			"clip-"+user, user, now, now); err != nil {
			t.Fatalf("insert clip project %s: %v", user, err)
		}
	}
	return jobstore.New(handle.Writer, handle.Reader, testKinds()), handle
}

func insert(t *testing.T, store *jobstore.Store, found job.Job) job.Job {
	t.Helper()
	found.Status = job.StatusQueued
	found.CreatedAt, found.UpdatedAt = time.Now(), time.Now()
	if err := store.Insert(context.Background(), found); err != nil {
		t.Fatalf("insert %s: %v", found.ID, err)
	}
	return found
}

// A voice-owned job may carry the post that caused it. One derived subject could only
// name one of the two, which is why the store resolves the dimension the caller asks for.
func TestActiveForFindsBothAttachmentsOfOneJob(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "learn", Kind: job.KindLearnVoice, UserID: "alice", Subjects: []job.Subject{
		{Dimension: postSubject, ID: "post-alice"}, {Dimension: voiceSubject, ID: "voice-alice"},
	}})

	byPost, err := store.ActiveFor(ctx, job.Subject{Dimension: postSubject, ID: "post-alice"}, job.Filter{})
	if err != nil || byPost == nil || byPost.ID != "learn" {
		t.Fatalf("by post = %v, %v; want the learning job", byPost, err)
	}
	byVoice, err := store.ActiveFor(ctx, job.Subject{Dimension: voiceSubject, ID: "voice-alice"}, job.Filter{Kind: job.KindLearnVoice})
	if err != nil || byVoice == nil || byVoice.ID != "learn" {
		t.Fatalf("by voice = %v, %v; want the learning job", byVoice, err)
	}
	if got := byPost.Subject(voiceSubject); got != "voice-alice" {
		t.Fatalf("voice subject of the read job = %q", got)
	}
}

func TestActiveForClipProjectIsOwnerScoped(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "clip-job", Kind: "generate_clip", UserID: "alice",
		ObserveModel: "p/o", WriteModel: "p/w",
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})

	mine, err := store.ActiveFor(ctx, job.Subject{Dimension: clipProjectSubject, ID: "clip-alice"}, job.Filter{UserID: "alice"})
	if err != nil || mine == nil || mine.ID != "clip-job" {
		t.Fatalf("owner lookup = %v, %v", mine, err)
	}
	if mine.DispatchReady {
		t.Fatal("a deferred kind was inserted ready to dispatch")
	}
	foreign, err := store.ActiveFor(ctx, job.Subject{Dimension: clipProjectSubject, ID: "clip-alice"}, job.Filter{UserID: "bob"})
	if err != nil || foreign != nil {
		t.Fatalf("foreign lookup = %v, %v; want nothing", foreign, err)
	}
	latest, err := store.LatestFor(ctx, job.Subject{Dimension: clipProjectSubject, ID: "clip-alice"}, job.Filter{UserID: "alice"})
	if err != nil || latest == nil || latest.ID != "clip-job" {
		t.Fatalf("latest = %v, %v", latest, err)
	}
}

// An experiment keeps its id in the payload; the generated column is what makes that a
// lookup. It is read-only: a job cannot be attached to it.
func TestExperimentSubjectIsDerivedAndUnattachable(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "exp-job", Kind: job.KindModelExperiment, UserID: "alice", Payload: []byte("exp-7")})

	found, err := store.ActiveFor(ctx, job.Subject{Dimension: experimentSubject, ID: "exp-7"}, job.Filter{})
	if err != nil || found == nil || found.ID != "exp-job" {
		t.Fatalf("experiment lookup = %v, %v", found, err)
	}
	if other, err := store.ActiveFor(ctx, job.Subject{Dimension: experimentSubject, ID: "exp-8"}, job.Filter{}); err != nil || other != nil {
		t.Fatalf("unknown experiment = %v, %v; want nothing", other, err)
	}
	err = store.Insert(ctx, job.Job{ID: "attached", Kind: job.KindModelExperiment, UserID: "alice", Status: job.StatusQueued,
		Subjects: []job.Subject{{Dimension: experimentSubject, ID: "exp-9"}}, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	if err == nil {
		t.Fatal("the store attached a job to a derived dimension")
	}
}

func TestUnknownDimensionIsRefused(t *testing.T) {
	store, _ := subjectHarness(t)
	if _, err := store.ActiveFor(context.Background(), job.Subject{Dimension: "widget", ID: "w1"}, job.Filter{}); err == nil {
		t.Fatal("an unknown dimension read as 'not busy'")
	}
}

// Unattached work is guarded per user and kind, and voice-only work has always been
// visible to that guard.
func TestActiveUnattachedIgnoresPostAndProjectWork(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "with-post", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})

	if found, err := store.ActiveUnattached(ctx, "alice", job.KindGenerate); err != nil || found != nil {
		t.Fatalf("unattached = %v, %v; want nothing while only post work is active", found, err)
	}
	insert(t, store, job.Job{ID: "voice-only", Kind: job.KindAnalyzeVoice, UserID: "alice",
		Subjects: []job.Subject{{Dimension: voiceSubject, ID: "voice-alice"}}})
	found, err := store.ActiveUnattached(ctx, "alice", job.KindAnalyzeVoice)
	if err != nil || found == nil || found.ID != "voice-only" {
		t.Fatalf("unattached = %v, %v; want the voice-only job", found, err)
	}
}

func TestActivateAndSweepOnlyTouchDeferredKinds(t *testing.T) {
	store, _ := subjectHarness(t)
	ctx := context.Background()
	insert(t, store, job.Job{ID: "clip-job", Kind: "generate_clip", UserID: "alice",
		ObserveModel: "p/o", WriteModel: "p/w",
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-alice"}}})
	insert(t, store, job.Job{ID: "plain", Kind: job.KindGenerate, UserID: "alice", TargetLanguage: "ko",
		Subjects: []job.Subject{{Dimension: postSubject, ID: "post-alice"}}})

	if released, err := store.Activate(ctx, "bob", "clip-job"); err != nil || released {
		t.Fatalf("foreign activation = %v, %v", released, err)
	}
	if released, err := store.Activate(ctx, "alice", "plain"); err != nil || released {
		t.Fatalf("activating an immediate kind = %v, %v; want no row", released, err)
	}
	released, err := store.Activate(ctx, "alice", "clip-job")
	if err != nil || !released {
		t.Fatalf("activation = %v, %v", released, err)
	}
	insert(t, store, job.Job{ID: "clip-waiting", Kind: "render_clip", UserID: "bob",
		Subjects: []job.Subject{{Dimension: clipProjectSubject, ID: "clip-bob"}}})
	swept, err := store.SweepUnactivated(ctx, job.Failure{Reason: "CLIP_PROCESSING_FAILED"})
	if err != nil || swept != 1 {
		t.Fatalf("sweep = %d, %v; want only the unactivated clip job", swept, err)
	}
	after, err := store.GetByID(ctx, "plain")
	if err != nil || after.Status != job.StatusQueued {
		t.Fatalf("the sweep touched an immediate kind: %v, %v", after.Status, err)
	}
}
