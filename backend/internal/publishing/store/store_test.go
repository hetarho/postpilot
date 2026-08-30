package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/publishing"
)

func TestPendingPairingCapIsAtomicUnderConcurrency(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	const attempts = 12
	const limit = 3
	var wait sync.WaitGroup
	errs := make(chan error, attempts)
	for index := 0; index < attempts; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			errs <- store.CreatePairing(ctx, "code-"+string(rune('a'+index)), "alice", "Mac", now.Add(time.Minute), now, limit)
		}(index)
	}
	wait.Wait()
	close(errs)
	inserted, limited := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			inserted++
		case errors.Is(err, publishing.ErrPairingLimit):
			limited++
		default:
			t.Fatalf("unexpected create pairing error: %v", err)
		}
	}
	if inserted != limit || limited != attempts-limit {
		t.Fatalf("inserted=%d limited=%d", inserted, limited)
	}
}

func testStore(t *testing.T) (*Store, *db.DB) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "publishing.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	return New(handle.Writer, handle.Reader), handle
}

func allowPublishStart(context.Context) error { return nil }

func reserveJobID(t *testing.T, store *Store, ctx context.Context, userID, jobID string, now time.Time) {
	t.Helper()
	if err := store.ReserveJobID(ctx, userID, jobID, now); err != nil {
		t.Fatal(err)
	}
}

func TestPairingExpiryAndAgentRevocationAreImmediate(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	if _, err := handle.Writer.Exec(`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)`, formatTime(now)); err != nil {
		t.Fatal(err)
	}
	if err := store.CreatePairing(ctx, "expired", "alice", "old", now.Add(-time.Minute), now.Add(-time.Hour), 8); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enroll(ctx, "expired", "token-old", "agent-old", "Chrome", now); !errors.Is(err, publishing.ErrPairingInvalid) {
		t.Fatalf("expired pairing error=%v", err)
	}

	if err := store.CreatePairing(ctx, "valid", "alice", "Mac", now.Add(time.Minute), now, 8); err != nil {
		t.Fatal(err)
	}
	agent, err := store.Enroll(ctx, "valid", "token-hash", "agent", "Chrome", now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AgentByTokenHash(ctx, "token-hash"); err != nil {
		t.Fatal(err)
	}
	if err := store.RevokeAgent(ctx, agent.UserID, agent.ID, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AgentByTokenHash(ctx, "token-hash"); !errors.Is(err, publishing.ErrNotFound) {
		t.Fatalf("revoked token lookup error=%v", err)
	}
}

func TestTwoAccountsClaimOnlyTheirOwnJobsAndRevocationLeavesTheOtherUsable(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	stamp := formatTime(now)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
		`INSERT INTO users(id,password_hash,created_at) VALUES('bob','hash','` + stamp + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,compatibility_ready,created_at,updated_at) VALUES('alice-agent','alice','alice-token','Alice Mac','naver_blog',1,'` + stamp + `','` + stamp + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,compatibility_ready,created_at,updated_at) VALUES('bob-agent','bob','bob-token','Bob Mac','naver_blog',1,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	newJob := func(id, userID, agentID string, createdAt time.Time) publishing.Job {
		return publishing.Job{
			ID: id, UserID: userID, PostSlug: id + "-post", PostCreatedAt: createdAt,
			AgentID: agentID, Platform: publishing.PlatformNaverBlog, Status: publishing.StatusQueued,
			Stage: publishing.StageQueued, ContentRevision: 1, CategoryID: "daily",
			Visibility: publishing.VisibilityPublic, CreatedAt: createdAt, UpdatedAt: createdAt,
			Manifest: &publishing.Manifest{JobID: id, PostSlug: id + "-post", ContentRevision: 1},
		}
	}
	reserveJobID(t, store, ctx, "alice", "alice-job", now)
	if err := store.CreateJob(ctx, newJob("alice-job", "alice", "alice-agent", now), nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}
	reserveJobID(t, store, ctx, "bob", "bob-job", now.Add(time.Second))
	if err := store.CreateJob(ctx, newJob("bob-job", "bob", "bob-agent", now.Add(time.Second)), nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}

	alice := publishing.Agent{ID: "alice-agent", UserID: "alice"}
	claimed, err := store.ClaimJob(ctx, alice, "alice-lease", now.Add(time.Minute), now)
	if err != nil || claimed.ID != "alice-job" || claimed.UserID != "alice" {
		t.Fatalf("alice claim = %+v, %v", claimed, err)
	}
	if err := store.RevokeAgent(ctx, "alice", "alice-agent", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.RenewLease(
		ctx, alice, "alice-job", "alice-lease", now.Add(2*time.Minute), now.Add(3*time.Second),
	); !errors.Is(err, publishing.ErrLeaseInvalid) {
		t.Fatalf("revoked agent renewed an authenticated lease: %v", err)
	}
	if _, err := store.UpdateProgress(
		ctx, alice, "alice-job", "alice-lease", publishing.StageClaimed, 1,
		publishing.StagePreparing, 2, now.Add(3*time.Second),
	); !errors.Is(err, publishing.ErrLeaseInvalid) {
		t.Fatalf("revoked agent crossed the progress/commit fence: %v", err)
	}
	if _, err := store.AgentByTokenHash(ctx, "alice-token"); !errors.Is(err, publishing.ErrNotFound) {
		t.Fatalf("revoked alice token error = %v", err)
	}
	bob, err := store.AgentByTokenHash(ctx, "bob-token")
	if err != nil {
		t.Fatal(err)
	}
	claimed, err = store.ClaimJob(ctx, bob, "bob-lease", now.Add(time.Minute), now)
	if err != nil || claimed.ID != "bob-job" || claimed.UserID != "bob" {
		t.Fatalf("bob claim = %+v, %v", claimed, err)
	}
}

func TestExpiredOrStaleLeaseCannotMutateJob(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	stamp := formatTime(now)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
		`INSERT INTO voices(id,user_id,name,is_default,created_at,updated_at) VALUES('voice','alice','기본',1,'` + stamp + `','` + stamp + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,status,content_revision,finalized_revision,created_at,updated_at) VALUES('post','alice','voice','finalized',1,1,'` + stamp + `','` + stamp + `')`,
		`INSERT INTO posts(slug,user_id,voice_id,status,content_revision,finalized_revision,created_at,updated_at) VALUES('post-attention','alice','voice','finalized',1,1,'` + stamp + `','` + stamp + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,platform_account_id,browser_label,categories_json,default_category_id,compatibility_ready,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog','alice-blog','Chrome','[{"ID":"daily","Name":"일상"}]','daily',1,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	manifest := &publishing.Manifest{JobID: "job", PostSlug: "post", ContentRevision: 1, Content: publishing.Content{Title: "title"}, CategoryID: "daily", Visibility: publishing.VisibilityPublic, ExpectedPlatformAccountID: "alice-blog"}
	job := publishing.Job{ID: "job", UserID: "alice", PostSlug: "post", PostCreatedAt: now, AgentID: "agent", Platform: publishing.PlatformNaverBlog, Status: publishing.StatusQueued, Stage: publishing.StageQueued, ContentRevision: 1, Manifest: manifest, CategoryID: "daily", Visibility: publishing.VisibilityPublic, CreatedAt: now, UpdatedAt: now}
	reserveJobID(t, store, ctx, "alice", job.ID, now)
	if err := store.CreateJob(ctx, job, nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}
	agent := publishing.Agent{ID: "agent", UserID: "alice"}
	if _, err := store.ClaimJob(ctx, agent, "lease-one", now.Add(45*time.Second), now); err != nil {
		t.Fatal(err)
	}
	failed, err := store.Fail(ctx, agent, "job", "lease-one", 2, publishing.StatusFailed, "safe", "safe", now.Add(time.Second))
	if err != nil || failed.Status != publishing.StatusFailed {
		t.Fatalf("valid lease failure status=%q err=%v", failed.Status, err)
	}

	second := job
	second.ID = "job-two"
	second.Manifest = &publishing.Manifest{JobID: "job-two", PostSlug: "post", ContentRevision: 1, Content: publishing.Content{Title: "title"}, CategoryID: "daily", Visibility: publishing.VisibilityPublic, ExpectedPlatformAccountID: "alice-blog"}
	reserveJobID(t, store, ctx, "alice", second.ID, now)
	if err := store.CreateJob(ctx, second, nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJob(ctx, agent, "lease-two", now.Add(45*time.Second), now); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Fail(ctx, agent, "job-two", "lease-two", 2, publishing.StatusFailed, "safe", "safe", now.Add(time.Minute)); !errors.Is(err, publishing.ErrLeaseInvalid) {
		t.Fatalf("expired lease failure error=%v", err)
	}
	if requeued, _, err := store.RequeueExpired(ctx, now.Add(time.Minute)); err != nil || requeued != 1 {
		t.Fatalf("requeued=%d err=%v", requeued, err)
	}
	if _, err := store.ClaimJob(ctx, agent, "lease-three", now.Add(2*time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProgress(ctx, agent, "job-two", "lease-two", publishing.StageClaimed, 1, publishing.StagePreparing, 2, now.Add(time.Minute)); !errors.Is(err, publishing.ErrLeaseInvalid) {
		t.Fatalf("stale lease progress error=%v", err)
	}
	if _, err := store.UpdateProgress(ctx, agent, "job-two", "lease-three", publishing.StageClaimed, 1, publishing.StagePreparing, 2, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.UpdateProgress(ctx, agent, "job-two", "lease-three", publishing.StageClaimed, 1, publishing.StageOpeningEditor, 3, now.Add(time.Minute)); !errors.Is(err, publishing.ErrLeaseInvalid) {
		t.Fatalf("stale state compare-and-swap error=%v", err)
	}
	steps := []publishing.Stage{publishing.StageOpeningEditor, publishing.StageFillingContent, publishing.StageUploadingPhotos, publishing.StageCommitting}
	current := publishing.StagePreparing
	sequence := int64(2)
	for _, next := range steps {
		if _, err := store.UpdateProgress(ctx, agent, "job-two", "lease-three", current, sequence, next, sequence+1, now.Add(time.Minute)); err != nil {
			t.Fatalf("progress %s -> %s: %v", current, next, err)
		}
		current, sequence = next, sequence+1
	}
	unknown, err := store.Fail(ctx, agent, "job-two", "lease-three", sequence+1, publishing.StatusFailed, "safe", "safe", now.Add(time.Minute))
	if err != nil || unknown.Status != publishing.StatusOutcomeUnknown || unknown.ErrorCode != "commit_outcome_unknown" {
		t.Fatalf("commit-fenced failure job=%+v err=%v", unknown, err)
	}

	attention := job
	attention.ID = "job-attention"
	attention.PostSlug = "post-attention"
	attention.Manifest = &publishing.Manifest{JobID: attention.ID, PostSlug: attention.PostSlug, ContentRevision: 1, Content: publishing.Content{Title: "title"}, CategoryID: "daily", Visibility: publishing.VisibilityPublic, ExpectedPlatformAccountID: "alice-blog"}
	reserveJobID(t, store, ctx, "alice", attention.ID, now)
	if err := store.CreateJob(ctx, attention, nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimJob(ctx, agent, "lease-attention", now.Add(3*time.Minute), now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Fail(ctx, agent, attention.ID, "lease-attention", 2, publishing.StatusNeedsAttention, "captcha", "repair", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if canceled, err := store.Cancel(ctx, "alice", attention.ID, now.Add(2*time.Minute)); err != nil || canceled.Status != publishing.StatusCanceled {
		t.Fatalf("cancel needs-attention job=%+v err=%v", canceled, err)
	}

	if _, err := handle.Writer.ExecContext(ctx, `DELETE FROM posts WHERE slug='post' AND user_id='alice'`); err != nil {
		t.Fatalf("delete source post with retained publication history: %v", err)
	}
	if retained, err := store.OwnedJob(ctx, "alice", "job-two"); err != nil || retained.PostSlug != "post" {
		t.Fatalf("retained publication history=%+v err=%v", retained, err)
	}
}

func TestRetryAttentionAtomicallyRequiresFrozenActiveProfile(t *testing.T) {
	for name, mutation := range map[string]string{
		"unchanged":         "",
		"revoked":           `UPDATE publishing_agents SET revoked_at=updated_at WHERE id='agent'`,
		"category renamed":  `UPDATE publishing_agents SET categories_json='[{"ID":"daily","Name":"새 이름"}]' WHERE id='agent'`,
		"identity switched": `UPDATE publishing_agents SET platform_account_id='other-blog' WHERE id='agent'`,
	} {
		t.Run(name, func(t *testing.T) {
			store, handle := testStore(t)
			ctx := context.Background()
			now := time.Date(2026, 8, 31, 1, 0, 0, 0, time.UTC)
			stamp := formatTime(now)
			for _, statement := range []string{
				`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
				`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,platform_account_id,browser_label,categories_json,default_category_id,compatibility_ready,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog','alice-blog','Chrome','[{"ID":"daily","Name":"일상"}]','daily',1,'` + stamp + `','` + stamp + `')`,
			} {
				if _, err := handle.Writer.ExecContext(ctx, statement); err != nil {
					t.Fatal(err)
				}
			}
			manifest := &publishing.Manifest{
				JobID: "attention", PostSlug: "deleted", ContentRevision: 1,
				Content: publishing.Content{Title: "title"}, CategoryID: "daily", CategoryName: "일상",
				Visibility: publishing.VisibilityPublic, ExpectedPlatformAccountID: "alice-blog",
			}
			job := publishing.Job{
				ID: "attention", UserID: "alice", PostSlug: "deleted", PostCreatedAt: now,
				AgentID: "agent", Platform: publishing.PlatformNaverBlog, Status: publishing.StatusQueued,
				Stage: publishing.StageQueued, ContentRevision: 1, Manifest: manifest, CategoryID: "daily",
				Visibility: publishing.VisibilityPublic, CreatedAt: now, UpdatedAt: now,
			}
			reserveJobID(t, store, ctx, "alice", job.ID, now)
			if err := store.CreateJob(ctx, job, nil, allowPublishStart); err != nil {
				t.Fatal(err)
			}
			agent := publishing.Agent{ID: "agent", UserID: "alice"}
			if _, err := store.ClaimJob(ctx, agent, "lease", now.Add(time.Minute), now); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Fail(ctx, agent, job.ID, "lease", 2, publishing.StatusNeedsAttention, "captcha", "repair", now.Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			if mutation != "" {
				if _, err := handle.Writer.ExecContext(ctx, mutation); err != nil {
					t.Fatal(err)
				}
			}
			retried, err := store.RetryAttentionJob(ctx, "alice", job.ID, now.Add(2*time.Second))
			if mutation == "" {
				if err != nil || retried.Status != publishing.StatusQueued {
					t.Fatalf("retry=%+v err=%v", retried, err)
				}
				return
			}
			if !errors.Is(err, publishing.ErrTransition) {
				t.Fatalf("unsafe retry error=%v", err)
			}
		})
	}
}

func TestDeletedSlugHistoryDoesNotAttachToNewPostIncarnation(t *testing.T) {
	store, handle := testStore(t)
	ctx := context.Background()
	firstCreated := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	secondCreated := firstCreated.Add(time.Second)
	stamp := formatTime(firstCreated)
	for _, statement := range []string{
		`INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash','` + stamp + `')`,
		`INSERT INTO publishing_agents(id,user_id,token_hash,label,platform,compatibility_ready,created_at,updated_at) VALUES('agent','alice','token','Mac','naver_blog',1,'` + stamp + `','` + stamp + `')`,
	} {
		if _, err := handle.Writer.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	newJob := func(id string, incarnation time.Time) publishing.Job {
		return publishing.Job{
			ID: id, UserID: "alice", PostSlug: "reused", PostCreatedAt: incarnation,
			AgentID: "agent", Platform: publishing.PlatformNaverBlog, Status: publishing.StatusQueued,
			Stage: publishing.StageQueued, ContentRevision: 1, CategoryID: "daily",
			Visibility: publishing.VisibilityPublic, CreatedAt: incarnation, UpdatedAt: incarnation,
			Manifest: &publishing.Manifest{JobID: id, PostSlug: "reused", ContentRevision: 1},
		}
	}
	reserveJobID(t, store, ctx, "alice", "old-job", firstCreated)
	if err := store.CreateJob(ctx, newJob("old-job", firstCreated), nil, allowPublishStart); err != nil {
		t.Fatal(err)
	}
	reserveJobID(t, store, ctx, "alice", "new-job", secondCreated)
	if err := store.CreateJob(ctx, newJob("new-job", secondCreated), nil, allowPublishStart); err != nil {
		t.Fatalf("new incarnation inherited old slot: %v", err)
	}
	old, err := store.LatestJobForPost(ctx, "alice", "reused", firstCreated)
	if err != nil || old.ID != "old-job" {
		t.Fatalf("old history=%+v err=%v", old, err)
	}
	current, err := store.LatestJobForPost(ctx, "alice", "reused", secondCreated)
	if err != nil || current.ID != "new-job" {
		t.Fatalf("current history=%+v err=%v", current, err)
	}
	latestDeleted, err := store.LatestJobForDeletedPost(ctx, "alice", "reused")
	if err != nil || latestDeleted.ID != "new-job" {
		t.Fatalf("latest retained history=%+v err=%v", latestDeleted, err)
	}
}
