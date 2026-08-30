package publishing

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"testing"
	"time"
)

type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

type fakeTokens struct {
	id      string
	idErr   error
	secrets []string
}

func (f *fakeTokens) NewID() (string, error) { return f.id, f.idErr }
func (f *fakeTokens) NewSecret(int) (string, error) {
	value := f.secrets[0]
	f.secrets = f.secrets[1:]
	return value, nil
}

type fakePosts struct {
	snapshot    PostSnapshot
	err         error
	identityErr error
	calls       int
}

func (f *fakePosts) PostIdentity(context.Context, string, string) (time.Time, error) {
	return f.snapshot.CreatedAt, f.identityErr
}

func (f *fakePosts) PublishingSnapshot(context.Context, string, string) (PostSnapshot, error) {
	f.calls++
	return f.snapshot, f.err
}

type copyCall struct{ source, target string }
type fakeStaging struct {
	copies     []copyCall
	deleted    []string
	sizes      map[string]int64
	copyErrAt  int
	deleteErr  error
	deleteErrs map[string]error
}

func (f *fakeStaging) Copy(_ context.Context, source, target string) (int64, error) {
	f.copies = append(f.copies, copyCall{source, target})
	if f.copyErrAt > 0 && len(f.copies) == f.copyErrAt {
		return 0, errors.New("copy failed")
	}
	return f.sizes[source], nil
}
func (f *fakeStaging) SignGet(context.Context, string, time.Duration) (string, error) {
	return "https://storage.test/signed", nil
}
func (f *fakeStaging) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	if err := f.deleteErrs[key]; err != nil {
		return err
	}
	return f.deleteErr
}
func (f *fakeStaging) ListStaged(context.Context, string) ([]StagedObject, error) { return nil, nil }

type fakeStore struct {
	agent            Agent
	latest           Job
	latestErr        error
	deletedLatest    Job
	deletedLatestErr error
	created          Job
	createdAssets    []Asset
	retried          string
	pairingHash      string
	pairingErr       error
	touchedAt        time.Time
	terminalCalls    int
	createGuard      func()
	reserveErr       error
	reserved         string
	released         string
	result           Job
	assets           []Asset
	assetsByJob      map[string][]Asset
	deletedAssets    int
	deletedAssetJobs []string
	terminalJobIDs   []string
}

func (f *fakeStore) CreatePairing(_ context.Context, hash, _, _ string, _, _ time.Time, _ int) error {
	f.pairingHash = hash
	return f.pairingErr
}
func (f *fakeStore) Enroll(context.Context, string, string, string, string, time.Time) (Agent, error) {
	return Agent{}, nil
}
func (f *fakeStore) AgentByTokenHash(context.Context, string) (Agent, error) { return f.agent, nil }
func (f *fakeStore) TouchAgent(_ context.Context, _, _ string, now time.Time) error {
	f.touchedAt = now
	return nil
}
func (f *fakeStore) OwnedAgent(context.Context, string, string) (Agent, error) { return f.agent, nil }
func (f *fakeStore) ListAgents(context.Context, string) ([]Agent, error) {
	return []Agent{f.agent}, nil
}
func (f *fakeStore) UpdateAgent(context.Context, string, string, string, string, Visibility, time.Time) (Agent, error) {
	return f.agent, nil
}
func (f *fakeStore) SyncAgent(context.Context, string, string, ProfileUpdate, time.Time) (Agent, error) {
	return f.agent, nil
}
func (f *fakeStore) RevokeAgent(context.Context, string, string, time.Time) error { return nil }
func (f *fakeStore) ReserveJobID(_ context.Context, _, jobID string, _ time.Time) error {
	if f.reserveErr != nil {
		return f.reserveErr
	}
	f.reserved = jobID
	return nil
}
func (f *fakeStore) ReleaseJobID(_ context.Context, _, jobID string) error {
	f.released = jobID
	return nil
}
func (f *fakeStore) CreateJob(ctx context.Context, job Job, assets []Asset, guard func(context.Context) error) error {
	if f.createGuard != nil {
		f.createGuard()
	}
	if err := guard(ctx); err != nil {
		return err
	}
	f.created, f.createdAssets = job, append([]Asset(nil), assets...)
	return nil
}
func (f *fakeStore) RetryAttentionJob(_ context.Context, _, jobID string, _ time.Time) (Job, error) {
	f.retried = jobID
	job := f.latest
	if job.ID == "" {
		job = f.deletedLatest
	}
	job.Status, job.Stage, job.ProgressSeq = StatusQueued, StageQueued, 0
	return job, nil
}
func (f *fakeStore) ListRetryableJobs(context.Context, string) ([]Job, error) {
	if f.latest.ID == "" {
		return nil, nil
	}
	return []Job{f.latest}, nil
}
func (f *fakeStore) OwnedJob(context.Context, string, string) (Job, error) {
	return f.latest, f.latestErr
}
func (f *fakeStore) LatestJobForPost(context.Context, string, string, time.Time) (Job, error) {
	return f.latest, f.latestErr
}
func (f *fakeStore) LatestJobForDeletedPost(context.Context, string, string) (Job, error) {
	return f.deletedLatest, f.deletedLatestErr
}
func (f *fakeStore) ClaimJob(context.Context, Agent, string, time.Time, time.Time) (Job, error) {
	return f.latest, f.latestErr
}
func (f *fakeStore) RenewLease(context.Context, Agent, string, string, time.Time, time.Time) error {
	return nil
}
func (f *fakeStore) UpdateProgress(context.Context, Agent, string, string, Stage, int64, Stage, int64, time.Time) (Job, error) {
	return f.latest, nil
}
func (f *fakeStore) Complete(context.Context, Agent, string, string, int64, string, time.Time) (Job, error) {
	if f.result.ID != "" {
		return f.result, nil
	}
	return f.latest, nil
}
func (f *fakeStore) Fail(context.Context, Agent, string, string, int64, Status, string, string, time.Time) (Job, error) {
	if f.result.ID != "" {
		return f.result, nil
	}
	return f.latest, nil
}
func (f *fakeStore) Cancel(context.Context, string, string, time.Time) (Job, error) {
	if f.result.ID != "" {
		return f.result, nil
	}
	return f.latest, nil
}
func (f *fakeStore) RequeueExpired(context.Context, time.Time) (int64, int64, error) {
	return 0, 0, nil
}
func (f *fakeStore) Assets(_ context.Context, jobID string) ([]Asset, error) {
	if assets, ok := f.assetsByJob[jobID]; ok {
		return append([]Asset(nil), assets...), nil
	}
	return append([]Asset(nil), f.assets...), nil
}
func (f *fakeStore) DeleteAssets(_ context.Context, jobID string) error {
	f.deletedAssets++
	f.deletedAssetJobs = append(f.deletedAssetJobs, jobID)
	return nil
}
func (f *fakeStore) LiveStagedKeys(context.Context) (map[string]struct{}, error) {
	return map[string]struct{}{}, nil
}
func (f *fakeStore) TerminalJobsWithAssets(context.Context) ([]string, error) {
	f.terminalCalls++
	return append([]string(nil), f.terminalJobIDs...), nil
}

func readyAgent() Agent {
	return Agent{ID: "agent", UserID: "alice", Platform: PlatformNaverBlog, PlatformAccountID: "my-blog", BrowserLabel: "Chrome", Categories: []Category{{ID: "daily", Name: "일상"}}, DefaultCategoryID: "daily", DefaultVisibility: VisibilityPublic, CompatibilityReady: true}
}

func startFixture() PostSnapshot {
	return PostSnapshot{PostSlug: "post", UserID: "alice", CreatedAt: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC), ContentRevision: 7, FinalizedRevision: 7,
		Content: Content{Title: "title", Tags: []string{"tag"}, Blocks: []Block{{Type: BlockText, Content: "first"}, {Type: BlockImage, File: "b.jpg", Caption: "B"}, {Type: BlockImage, File: "a.jpg", Caption: "A"}}},
		Images:  []SnapshotImage{{Filename: "a.jpg", Key: "posts/a.jpg", Bytes: 10}, {Filename: "b.jpg", Key: "posts/b.jpg", Bytes: 20}}}
}

func newStartService(store *fakeStore, posts *fakePosts, staging *fakeStaging) *Service {
	service := NewService(store, posts, staging, Config{PairingTTL: 10 * time.Minute, MaxPendingPairings: 8, LeaseTTL: 45 * time.Second, AssetURLTTL: 10 * time.Minute})
	service.clock = fakeClock{now: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)}
	service.tokens = &fakeTokens{id: "job", secrets: []string{"secret"}}
	return service
}

func TestStartFreezesExactContentAndStagesImagesInBlockOrder(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	posts := &fakePosts{snapshot: startFixture()}
	staging := &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10, "posts/b.jpg": 20}}
	service := newStartService(store, posts, staging)
	job, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPrivate})
	if err != nil {
		t.Fatal(err)
	}
	wantCopies := []copyCall{{"posts/b.jpg", "publishing/job/0000.jpg"}, {"posts/a.jpg", "publishing/job/0001.jpg"}}
	if !reflect.DeepEqual(staging.copies, wantCopies) {
		t.Fatalf("copies = %#v", staging.copies)
	}
	if job.Manifest == nil || job.Manifest.Content.Title != "title" || job.Manifest.CategoryName != "일상" || job.Manifest.Visibility != VisibilityPrivate || !reflect.DeepEqual(job.Manifest.Tags, []string{"tag"}) {
		t.Fatalf("manifest = %#v", job.Manifest)
	}
	if got := []Asset(job.Manifest.Assets); len(got) != 2 || got[0].Filename != "0000.jpg" || got[0].SourceFilename != "b.jpg" || got[1].Filename != "0001.jpg" || got[1].SourceFilename != "a.jpg" {
		t.Fatalf("manifest assets = %#v", got)
	}
	posts.snapshot.Content.Title = "changed later"
	posts.snapshot.Content.Tags[0] = "changed"
	posts.snapshot.Content.Blocks[0].Content = "changed"
	if store.created.Manifest.Content.Title != "title" || store.created.Manifest.Content.Tags[0] != "tag" || store.created.Manifest.Content.Blocks[0].Content != "first" {
		t.Fatal("stored manifest aliases later post mutation")
	}
}

func TestStartStagesRepeatedImageReferencesUnderUniqueLocalNames(t *testing.T) {
	snapshot := startFixture()
	snapshot.Content.Blocks = []Block{
		{Type: BlockImage, File: "a.jpg", Caption: "first"},
		{Type: BlockText, Content: "between"},
		{Type: BlockImage, File: "a.jpg", Caption: "second"},
	}
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	service := newStartService(store, &fakePosts{snapshot: snapshot}, &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10}})

	job, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Manifest.Assets) != 2 || job.Manifest.Assets[0].Filename != "0000.jpg" || job.Manifest.Assets[1].Filename != "0001.jpg" || job.Manifest.Assets[0].SourceFilename != "a.jpg" || job.Manifest.Assets[1].SourceFilename != "a.jpg" {
		t.Fatalf("repeated assets = %#v", job.Manifest.Assets)
	}
	if job.Manifest.Content.Blocks[0].File != "a.jpg" || job.Manifest.Content.Blocks[2].File != "a.jpg" {
		t.Fatal("canonical content was rewritten")
	}
}

func TestStartRejectsStaleRevisionBeforeCopy(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	posts := &fakePosts{snapshot: startFixture()}
	staging := &fakeStaging{sizes: map[string]int64{}}
	service := newStartService(store, posts, staging)
	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 6, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if !errors.Is(err, ErrStaleRevision) || len(staging.copies) != 0 {
		t.Fatalf("error=%v copies=%v", err, staging.copies)
	}
}

func TestStartRevalidatesPostAndCompensatesCopiesAtAtomicInsertGate(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	posts := &fakePosts{snapshot: startFixture()}
	staging := &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10, "posts/b.jpg": 20}}
	store.createGuard = func() {
		posts.snapshot.ContentRevision++
		posts.snapshot.FinalizedRevision = 0
	}
	service := newStartService(store, posts, staging)

	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if !errors.Is(err, ErrStaleRevision) || store.created.ID != "" || len(staging.deleted) != 2 {
		t.Fatalf("error=%v created=%#v deleted=%v", err, store.created, staging.deleted)
	}
}

func TestStartRevalidatesAgentAtAtomicInsertGate(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	posts := &fakePosts{snapshot: startFixture()}
	staging := &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10, "posts/b.jpg": 20}}
	store.createGuard = func() {
		now := time.Now()
		store.agent.RevokedAt = &now
	}
	service := newStartService(store, posts, staging)

	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if !errors.Is(err, ErrAgentRevoked) || store.created.ID != "" || len(staging.deleted) != 2 {
		t.Fatalf("error=%v created=%#v deleted=%v", err, store.created, staging.deleted)
	}
}

func TestStartRevalidatesFrozenCategoryNameAtAtomicInsertGate(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	posts := &fakePosts{snapshot: startFixture()}
	staging := &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10, "posts/b.jpg": 20}}
	store.createGuard = func() {
		store.agent.Categories[0].Name = "이름이 바뀐 카테고리"
	}
	service := newStartService(store, posts, staging)

	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if !errors.Is(err, ErrCategoryNotFound) || store.created.ID != "" || len(staging.deleted) != 2 {
		t.Fatalf("error=%v created=%#v deleted=%v", err, store.created, staging.deleted)
	}
}

func TestStartRejectsExistingLiveOrPublishedJobBeforeSnapshotAndCopy(t *testing.T) {
	for _, status := range []Status{StatusQueued, StatusRunning, StatusPublished, StatusOutcomeUnknown} {
		t.Run(string(status), func(t *testing.T) {
			store := &fakeStore{agent: readyAgent(), latest: Job{Status: status}}
			posts := &fakePosts{snapshot: startFixture()}
			staging := &fakeStaging{}
			service := newStartService(store, posts, staging)
			_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
			if !errors.Is(err, ErrAlreadyPublishing) || posts.calls != 0 || len(staging.copies) != 0 {
				t.Fatalf("error=%v postCalls=%d copies=%v", err, posts.calls, staging.copies)
			}
		})
	}
}

func TestStartCompensatesEveryCompletedCopy(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	staging := &fakeStaging{sizes: map[string]int64{"posts/b.jpg": 20, "posts/a.jpg": 10}, copyErrAt: 2}
	service := newStartService(store, &fakePosts{snapshot: startFixture()}, staging)
	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	wantDeleted := []string{"publishing/job/0001.jpg", "publishing/job/0000.jpg"}
	if err == nil || !reflect.DeepEqual(staging.deleted, wantDeleted) {
		t.Fatalf("error=%v deleted=%v", err, staging.deleted)
	}
}

func TestStartDeletesCurrentCopyWhenSizeVerificationFails(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	staging := &fakeStaging{sizes: map[string]int64{"posts/b.jpg": 19, "posts/a.jpg": 10}}
	service := newStartService(store, &fakePosts{snapshot: startFixture()}, staging)
	_, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if err == nil || !reflect.DeepEqual(staging.deleted, []string{"publishing/job/0000.jpg"}) {
		t.Fatalf("error=%v deleted=%v", err, staging.deleted)
	}
}

func TestStartStopsBeforeR2WhenIDGenerationFailsOrCollides(t *testing.T) {
	for name, configure := range map[string]func(*Service, *fakeStore){
		"rng failure": func(service *Service, _ *fakeStore) {
			service.tokens = &fakeTokens{idErr: errors.New("entropy unavailable")}
		},
		"reserved collision": func(_ *Service, store *fakeStore) {
			store.reserveErr = ErrAlreadyPublishing
		},
	} {
		t.Run(name, func(t *testing.T) {
			store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
			staging := &fakeStaging{sizes: map[string]int64{"posts/a.jpg": 10, "posts/b.jpg": 20}}
			service := newStartService(store, &fakePosts{snapshot: startFixture()}, staging)
			configure(service, store)
			_, err := service.Start(context.Background(), StartRequest{
				UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7,
				AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic,
			})
			if err == nil || len(staging.copies) != 0 {
				t.Fatalf("error=%v copies=%v", err, staging.copies)
			}
		})
	}
}

func TestFailedStagingReleasesReservedIDOnlyAfterObjectCleanup(t *testing.T) {
	store := &fakeStore{agent: readyAgent(), latestErr: ErrNotFound}
	staging := &fakeStaging{
		sizes: map[string]int64{"posts/b.jpg": 20, "posts/a.jpg": 10}, copyErrAt: 2,
	}
	service := newStartService(store, &fakePosts{snapshot: startFixture()}, staging)
	_, err := service.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7,
		AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic,
	})
	if err == nil || store.reserved != "job" || store.released != "job" || len(staging.deleted) != 2 {
		t.Fatalf("error=%v reserved=%q released=%q deleted=%v", err, store.reserved, store.released, staging.deleted)
	}
}

func TestExplicitRetryRequeuesSameFrozenAttentionJob(t *testing.T) {
	manifest := &Manifest{JobID: "same", ContentRevision: 7}
	store := &fakeStore{agent: readyAgent(), latest: Job{ID: "same", AgentID: "agent", Status: StatusNeedsAttention, Stage: StageOpeningEditor, ContentRevision: 7, CategoryID: "daily", Visibility: VisibilityPublic, Manifest: manifest}}
	posts := &fakePosts{err: errors.New("must not read a new snapshot")}
	service := newStartService(store, posts, &fakeStaging{})
	job, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if err != nil || job.ID != "same" || store.retried != "same" || posts.calls != 0 {
		t.Fatalf("job=%#v err=%v retried=%q postCalls=%d", job, err, store.retried, posts.calls)
	}
}

func TestExplicitRetryUsesFrozenAttentionJobAfterSourcePostDeletion(t *testing.T) {
	manifest := &Manifest{JobID: "same", PostSlug: "post", ContentRevision: 7}
	retained := Job{ID: "same", PostSlug: "post", AgentID: "agent", Status: StatusNeedsAttention, Stage: StageOpeningEditor, ContentRevision: 7, CategoryID: "daily", Visibility: VisibilityPublic, Manifest: manifest}
	store := &fakeStore{agent: readyAgent(), deletedLatest: retained}
	posts := &fakePosts{identityErr: ErrNotFound, err: errors.New("deleted post snapshot must not be read")}
	service := newStartService(store, posts, &fakeStaging{})

	job, err := service.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", ExpectedContentRevision: 7, AgentID: "agent", CategoryID: "daily", Visibility: VisibilityPublic})
	if err != nil || job.ID != "same" || store.retried != "same" || posts.calls != 0 {
		t.Fatalf("job=%#v err=%v retried=%q postCalls=%d", job, err, store.retried, posts.calls)
	}
}

func TestRetryByJobIDDoesNotRequireDeletedSourcePost(t *testing.T) {
	manifest := &Manifest{
		JobID: "same", PostSlug: "deleted", ContentRevision: 7,
		CategoryID: "daily", CategoryName: "일상", Visibility: VisibilityPublic,
		ExpectedPlatformAccountID: "my-blog",
	}
	retained := Job{
		ID: "same", UserID: "alice", PostSlug: "deleted", AgentID: "agent",
		Status: StatusNeedsAttention, Stage: StageOpeningEditor, ContentRevision: 7,
		CategoryID: "daily", Visibility: VisibilityPublic, Manifest: manifest,
	}
	store := &fakeStore{agent: readyAgent(), latest: retained}
	posts := &fakePosts{err: errors.New("retry must not inspect the source post")}
	service := newStartService(store, posts, &fakeStaging{})

	jobs, err := service.ListRetryable(context.Background(), "alice")
	if err != nil || len(jobs) != 1 || jobs[0].ID != "same" {
		t.Fatalf("retryable=%+v err=%v", jobs, err)
	}
	job, err := service.Retry(context.Background(), "alice", "same")
	if err != nil || job.Status != StatusQueued || store.retried != "same" || posts.calls != 0 {
		t.Fatalf("job=%+v err=%v retried=%q postCalls=%d", job, err, store.retried, posts.calls)
	}
}

func TestRetryByJobIDRejectsChangedAgentIdentityOrCategory(t *testing.T) {
	manifest := &Manifest{
		JobID: "same", CategoryID: "daily", CategoryName: "일상",
		ExpectedPlatformAccountID: "my-blog",
	}
	retained := Job{
		ID: "same", UserID: "alice", AgentID: "agent", Status: StatusNeedsAttention,
		CategoryID: "daily", Manifest: manifest,
	}
	for name, mutate := range map[string]func(*Agent){
		"identity": func(agent *Agent) { agent.PlatformAccountID = "other-blog" },
		"category": func(agent *Agent) { agent.Categories[0].Name = "renamed" },
	} {
		t.Run(name, func(t *testing.T) {
			agent := readyAgent()
			mutate(&agent)
			service := newStartService(&fakeStore{agent: agent, latest: retained}, &fakePosts{}, &fakeStaging{})
			if _, err := service.Retry(context.Background(), "alice", "same"); err == nil {
				t.Fatal("unsafe retained job was retried")
			}
		})
	}
}

func TestCancelRetainedDeletedPostDoesNotDependOnAgentState(t *testing.T) {
	for name, agent := range map[string]Agent{
		"revoked": func() Agent {
			value := readyAgent()
			now := time.Now()
			value.RevokedAt = &now
			return value
		}(),
		"identity changed": func() Agent {
			value := readyAgent()
			value.PlatformAccountID = "different-blog"
			return value
		}(),
	} {
		t.Run(name, func(t *testing.T) {
			current := Job{ID: "retained", UserID: "alice", AgentID: agent.ID, Status: StatusNeedsAttention, Stage: StageOpeningEditor}
			canceled := current
			canceled.Status = StatusCanceled
			store := &fakeStore{
				agent: agent, latest: current, result: canceled,
				assets: []Asset{{JobID: current.ID, StagedKey: "publishing/retained/0000.jpg"}},
			}
			posts := &fakePosts{identityErr: ErrNotFound, err: ErrNotFound}
			staging := &fakeStaging{}
			service := NewService(store, posts, staging, Config{})

			job, err := service.Cancel(context.Background(), "alice", current.ID)
			if err != nil || job.Status != StatusCanceled {
				t.Fatalf("job=%+v err=%v", job, err)
			}
			if posts.calls != 0 || store.deletedAssets != 1 || !reflect.DeepEqual(staging.deleted, []string{"publishing/retained/0000.jpg"}) {
				t.Fatalf("cancel depended on deleted source or skipped cleanup: postCalls=%d deletedAssets=%d deletes=%v", posts.calls, store.deletedAssets, staging.deleted)
			}
		})
	}
}

func TestPairingCodeHasUnambiguousAlphabetAndOnlyHashIsStored(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store, &fakePosts{}, &fakeStaging{}, Config{PairingTTL: 10 * time.Minute, MaxPendingPairings: 8})
	service.clock = fakeClock{now: time.Now()}
	service.tokens = &fakeTokens{secrets: []string{"random-device-secret"}}
	pairing, err := service.CreatePairing(context.Background(), "alice", "내 Mac")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}$`).MatchString(pairing.DeviceCode) {
		t.Fatalf("code = %q", pairing.DeviceCode)
	}
	if store.pairingHash == "" || store.pairingHash == pairing.DeviceCode {
		t.Fatal("raw pairing code reached persistence")
	}
}

func TestPairingLimitFromAtomicStoreOperationIsPreserved(t *testing.T) {
	store := &fakeStore{pairingErr: ErrPairingLimit}
	service := NewService(store, &fakePosts{}, &fakeStaging{}, Config{PairingTTL: 10 * time.Minute, MaxPendingPairings: 8})
	service.clock = fakeClock{now: time.Now()}
	service.tokens = &fakeTokens{secrets: []string{"random-device-secret"}}
	if _, err := service.CreatePairing(context.Background(), "alice", "내 Mac"); !errors.Is(err, ErrPairingLimit) {
		t.Fatalf("pairing limit error = %v", err)
	}
}

func TestAuthenticateAgentTouchesLastSeen(t *testing.T) {
	now := time.Date(2026, 8, 30, 13, 0, 0, 0, time.UTC)
	store := &fakeStore{agent: readyAgent()}
	service := NewService(store, &fakePosts{}, &fakeStaging{}, Config{})
	service.clock = fakeClock{now: now}

	agent, err := service.AuthenticateAgent(context.Background(), "raw-token")
	if err != nil {
		t.Fatal(err)
	}
	if store.touchedAt != now || agent.LastSeenAt == nil || !agent.LastSeenAt.Equal(now) {
		t.Fatalf("touch=%v agent.last_seen=%v", store.touchedAt, agent.LastSeenAt)
	}
}

func TestLeaseRecoveryDoesNotDependOnObjectCleanup(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store, &fakePosts{}, &fakeStaging{}, Config{})
	if _, _, err := service.RecoverExpired(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.terminalCalls != 0 {
		t.Fatal("lease recovery attempted terminal object cleanup")
	}
}

func TestTerminalTransitionsStaySuccessfulWhenObjectCleanupNeedsRetry(t *testing.T) {
	asset := Asset{JobID: "job", StagedKey: "publishing/job/0000.jpg"}
	agent := readyAgent()
	manifest := &Manifest{ExpectedPlatformAccountID: agent.PlatformAccountID}

	tests := []struct {
		name    string
		current Job
		result  Job
		act     func(*Service) (Job, error)
	}{
		{
			name:    "complete",
			current: Job{ID: "job", UserID: "alice", AgentID: agent.ID, Stage: StageVerifying, ProgressSeq: 1, Manifest: manifest},
			result:  Job{ID: "job", UserID: "alice", AgentID: agent.ID, Status: StatusPublished, Stage: StagePublished, Manifest: manifest},
			act: func(service *Service) (Job, error) {
				return service.Complete(context.Background(), agent, "job", "lease", 2, "https://blog.naver.com/my-blog/123")
			},
		},
		{
			name:    "fail",
			current: Job{ID: "job", UserID: "alice", AgentID: agent.ID, Stage: StageOpeningEditor, ProgressSeq: 1},
			result:  Job{ID: "job", UserID: "alice", AgentID: agent.ID, Status: StatusFailed, Stage: StageOpeningEditor},
			act: func(service *Service) (Job, error) {
				return service.Fail(context.Background(), agent, "job", "lease", 2, FailureEditorChanged, "local detail")
			},
		},
		{
			name:    "cancel",
			current: Job{ID: "job", UserID: "alice", AgentID: agent.ID, Status: StatusQueued, Stage: StageQueued},
			result:  Job{ID: "job", UserID: "alice", AgentID: agent.ID, Status: StatusCanceled, Stage: StageQueued},
			act: func(service *Service) (Job, error) {
				return service.Cancel(context.Background(), "alice", "job")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{
				latest: test.current, result: test.result, assets: []Asset{asset},
				terminalJobIDs: []string{"job"},
			}
			staging := &fakeStaging{deleteErr: errors.New("object store unavailable")}
			service := NewService(store, &fakePosts{}, staging, Config{})

			job, err := test.act(service)
			if err != nil || job.Status != test.result.Status {
				t.Fatalf("job=%+v err=%v", job, err)
			}
			if store.deletedAssets != 0 || !reflect.DeepEqual(staging.deleted, []string{asset.StagedKey}) {
				t.Fatalf("failed cleanup was treated as complete: deletedAssets=%d deletes=%v", store.deletedAssets, staging.deleted)
			}

			staging.deleteErr = nil
			if err := service.CleanupTerminals(context.Background()); err != nil {
				t.Fatal(err)
			}
			if store.deletedAssets != 1 || !reflect.DeepEqual(staging.deleted, []string{asset.StagedKey, asset.StagedKey}) {
				t.Fatalf("sweeper did not retry cleanup: deletedAssets=%d deletes=%v", store.deletedAssets, staging.deleted)
			}
		})
	}
}

func TestCleanupTerminalsContinuesAfterObjectFailures(t *testing.T) {
	blockedFirst := Asset{JobID: "blocked", StagedKey: "publishing/blocked/0000.jpg"}
	blockedSecond := Asset{JobID: "blocked", StagedKey: "publishing/blocked/0001.jpg"}
	healthy := Asset{JobID: "healthy", StagedKey: "publishing/healthy/0000.jpg"}
	store := &fakeStore{
		terminalJobIDs: []string{"blocked", "healthy"},
		assetsByJob: map[string][]Asset{
			"blocked": {blockedFirst, blockedSecond},
			"healthy": {healthy},
		},
	}
	staging := &fakeStaging{deleteErrs: map[string]error{
		blockedFirst.StagedKey: errors.New("object cannot be deleted"),
	}}
	service := NewService(store, &fakePosts{}, staging, Config{})

	if err := service.CleanupTerminals(context.Background()); err == nil {
		t.Fatal("cleanup failure was hidden")
	}
	wantDeletes := []string{blockedFirst.StagedKey, blockedSecond.StagedKey, healthy.StagedKey}
	if !reflect.DeepEqual(staging.deleted, wantDeletes) {
		t.Fatalf("cleanup stopped early: deletes=%v want=%v", staging.deleted, wantDeletes)
	}
	if !reflect.DeepEqual(store.deletedAssetJobs, []string{"healthy"}) {
		t.Fatalf("asset rows deleted for incomplete jobs or healthy job starved: %v", store.deletedAssetJobs)
	}
}
