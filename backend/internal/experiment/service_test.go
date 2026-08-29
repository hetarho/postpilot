package experiment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu   sync.Mutex
	rows map[string]Experiment
}

func newMemoryStore() *memoryStore { return &memoryStore{rows: map[string]Experiment{}} }
func (s *memoryStore) Create(_ context.Context, found Experiment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.rows {
		if found.Stage == StageWrite && row.UserID == found.UserID && row.PostSlug == found.PostSlug &&
			(row.Status == StatusQueued || row.Status == StatusRunning || row.Status == StatusReview || row.Status == StatusPartial || row.Status == StatusFailed) {
			return ErrInvalidState
		}
	}
	s.rows[found.ID] = cloneExperiment(found)
	return nil
}
func (s *memoryStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, id)
	return nil
}
func (s *memoryStore) Get(_ context.Context, id string) (Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.rows[id]
	if !ok {
		return Experiment{}, ErrNotFound
	}
	return cloneExperiment(row), nil
}
func (s *memoryStore) List(_ context.Context, userID string, stage Stage) ([]Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Experiment
	for _, row := range s.rows {
		if row.UserID == userID && (stage == "" || row.Stage == stage) {
			out = append(out, cloneExperiment(row))
		}
	}
	return out, nil
}
func (s *memoryStore) PendingForPost(_ context.Context, userID, slug string) (*Experiment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range s.rows {
		if row.UserID == userID && row.PostSlug == slug && row.Stage == StageWrite && !row.Revealed() {
			copy := cloneExperiment(row)
			return &copy, nil
		}
	}
	return nil, nil
}
func (s *memoryStore) SetJob(_ context.Context, id, userID, jobID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID {
		return ErrForbidden
	}
	row.JobID = jobID
	s.rows[id] = row
	return nil
}
func (s *memoryStore) SetSnapshot(_ context.Context, id string, snapshot Snapshot, hash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	row.InputSnapshot = append([]byte(nil), snapshot.Content...)
	row.InputHash = hash
	row.PromptVersion = snapshot.PromptVersion
	s.rows[id] = row
	return nil
}
func (s *memoryStore) SetStatus(_ context.Context, id string, status Status, finished *time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	row.Status = status
	row.FinishedAt = finished
	s.rows[id] = row
	return nil
}
func (s *memoryStore) StartCandidate(_ context.Context, experimentID, candidateID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[experimentID]
	for i := range row.Candidates {
		if row.Candidates[i].ID == candidateID {
			row.Candidates[i].Status = CandidateRunning
			row.Candidates[i].StartedAt = &now
		}
	}
	s.rows[experimentID] = row
	return nil
}
func (s *memoryStore) CompleteCandidate(_ context.Context, candidate Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[candidate.ExperimentID]
	for i := range row.Candidates {
		if row.Candidates[i].ID == candidate.ID {
			row.Candidates[i] = candidate
		}
	}
	s.rows[row.ID] = row
	return nil
}
func (s *memoryStore) FailUnfinished(_ context.Context, id, reason string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failUnfinished(id, reason, now)
}
func (s *memoryStore) RecoverInterrupted(_ context.Context, reason string, now time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	for id, row := range s.rows {
		if row.Status != StatusRunning {
			continue
		}
		if err := s.failUnfinished(id, reason, now); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}
func (s *memoryStore) ListQueued(context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for id, row := range s.rows {
		if row.Status == StatusQueued {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (s *memoryStore) failUnfinished(id, reason string, now time.Time) error {
	row, ok := s.rows[id]
	if !ok || (row.Status != StatusQueued && row.Status != StatusRunning) {
		return ErrInvalidState
	}
	hasSuccess := false
	for i := range row.Candidates {
		if row.Candidates[i].Status == CandidateSucceeded {
			hasSuccess = true
		}
		if row.Candidates[i].Status == CandidatePending || row.Candidates[i].Status == CandidateRunning {
			row.Candidates[i].Status = CandidateFailed
			row.Candidates[i].Error = reason
			row.Candidates[i].FinishedAt = &now
		}
	}
	row.Status = StatusFailed
	if hasSuccess {
		row.Status = StatusPartial
	}
	row.FinishedAt = &now
	s.rows[id] = row
	return nil
}
func (s *memoryStore) ResetFailedCandidates(_ context.Context, id string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	var n int64
	for i := range row.Candidates {
		if row.Candidates[i].Status == CandidateFailed {
			row.Candidates[i].Status = CandidatePending
			row.Candidates[i].Error = ""
			n++
		}
	}
	s.rows[id] = row
	return n, nil
}
func (s *memoryStore) RestoreFailedCandidates(_ context.Context, id string, candidates []Candidate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	for _, original := range candidates {
		if original.Status != CandidateFailed {
			continue
		}
		for i := range row.Candidates {
			if row.Candidates[i].ID == original.ID {
				row.Candidates[i] = original
			}
		}
	}
	s.rows[id] = row
	return nil
}
func (s *memoryStore) Decide(_ context.Context, id, userID, candidateID string, status Status, outcome Outcome, adoptionRequested bool, decidedAt, expiresAt time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID || (row.Status != StatusReview && row.Status != StatusPartial && row.Status != StatusFailed) {
		return false, nil
	}
	row.Status, row.WinnerCandidateID, row.Outcome = status, candidateID, outcome
	row.AdoptionRequested = adoptionRequested
	row.DecidedAt, row.ContentExpiresAt = &decidedAt, &expiresAt
	s.rows[id] = row
	return true, nil
}
func (s *memoryStore) SetApplyError(_ context.Context, id, userID, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID {
		return ErrForbidden
	}
	row.ApplyError = message
	s.rows[id] = row
	return nil
}
func (s *memoryStore) SetApplied(_ context.Context, id, userID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID || row.Status != StatusDecided || row.AppliedAt != nil {
		return ErrInvalidState
	}
	row.ApplyError = ""
	row.AppliedAt = &now
	s.rows[id] = row
	return nil
}
func (s *memoryStore) SetAdoptionError(_ context.Context, id, userID, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID {
		return ErrForbidden
	}
	row.AdoptionError = message
	s.rows[id] = row
	return nil
}
func (s *memoryStore) SetAdopted(_ context.Context, id, userID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.rows[id]
	if row.UserID != userID || row.Status != StatusDecided || row.AdoptedAt != nil {
		return ErrInvalidState
	}
	row.AdoptionError = ""
	row.AdoptedAt = &now
	s.rows[id] = row
	return nil
}
func (s *memoryStore) LeaderboardData(context.Context, string, Stage) ([]Experiment, []Candidate, error) {
	return nil, nil, nil
}
func (s *memoryStore) PurgeExpired(context.Context, time.Time) (int64, error) { return 0, nil }
func (s *memoryStore) PurgePost(context.Context, string, string) error        { return nil }

func cloneExperiment(found Experiment) Experiment {
	found.InputSnapshot = append([]byte(nil), found.InputSnapshot...)
	found.Candidates = append([]Candidate(nil), found.Candidates...)
	for i := range found.Candidates {
		found.Candidates[i].Output = append([]byte(nil), found.Candidates[i].Output...)
	}
	return found
}

type fakeCatalog struct {
	models    map[ModelRef]Model
	adopted   []ModelRef
	adoptErr  error
	active    ModelRef
	selected  bool
	activeErr error
}

func (c *fakeCatalog) Resolve(ref ModelRef) (Model, bool) {
	model, ok := c.models[ref]
	return model, ok
}
func (c *fakeCatalog) Adopt(_ context.Context, _ string, _ Stage, ref ModelRef) error {
	if c.adoptErr != nil {
		return c.adoptErr
	}
	c.adopted = append(c.adopted, ref)
	c.active, c.selected = ref, true
	return nil
}

func TestDecideWriteSeparatesApplyOnlyFromAdoptionAndRecoversPartialFailure(t *testing.T) {
	t.Run("apply only", func(t *testing.T) {
		svc, store, catalog, _, runner := newTestService()
		started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
		if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		ready, _ := store.Get(context.Background(), started.ExperimentID)
		decided, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, false)
		if err != nil || decided.AppliedAt == nil || decided.AdoptionRequested || decided.AdoptedAt != nil || len(catalog.adopted) != 0 || runner.applyCalls != 1 {
			t.Fatalf("apply only = %+v err=%v adopted=%v applies=%d", decided, err, catalog.adopted, runner.applyCalls)
		}
	})

	t.Run("application retry preserves adoption request", func(t *testing.T) {
		svc, store, catalog, _, runner := newTestService()
		started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
		if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		ready, _ := store.Get(context.Background(), started.ExperimentID)
		runner.applyErr = errors.New("post unavailable")
		partial, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, true)
		if err != nil || !partial.AdoptionRequested || partial.AppliedAt != nil || partial.ApplyError == "" || len(catalog.adopted) != 0 {
			t.Fatalf("partial = %+v err=%v adopted=%v", partial, err, catalog.adopted)
		}
		runner.applyErr = nil
		recovered, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, partial.AdoptionRequested)
		if err != nil || recovered.AppliedAt == nil || recovered.AdoptedAt == nil || len(catalog.adopted) != 1 || runner.applyCalls != 2 {
			t.Fatalf("recovered = %+v err=%v adopted=%v applies=%d", recovered, err, catalog.adopted, runner.applyCalls)
		}
	})

	t.Run("adoption-only retry", func(t *testing.T) {
		svc, store, catalog, _, runner := newTestService()
		started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
		if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		ready, _ := store.Get(context.Background(), started.ExperimentID)
		catalog.adoptErr = errors.New("selection unavailable")
		partial, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, true)
		if err != nil || partial.AppliedAt == nil || partial.AdoptionError != adoptFailedReason || partial.AdoptedAt != nil || runner.applyCalls != 1 {
			t.Fatalf("partial = %+v err=%v applies=%d", partial, err, runner.applyCalls)
		}
		catalog.adoptErr = nil
		recovered, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, true)
		if err != nil || recovered.AdoptedAt == nil || recovered.AdoptionError != "" || len(catalog.adopted) != 1 || runner.applyCalls != 1 {
			t.Fatalf("recovered = %+v err=%v adopted=%v applies=%d", recovered, err, catalog.adopted, runner.applyCalls)
		}
		if _, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, true); err != nil || len(catalog.adopted) != 1 || runner.applyCalls != 1 {
			t.Fatalf("idempotent retry err=%v adopted=%v applies=%d", err, catalog.adopted, runner.applyCalls)
		}
	})
}

func TestConcurrentWriteDecisionAppliesAndAdoptsExactlyOnce(t *testing.T) {
	svc, store, catalog, _, runner := newTestService()
	started, _ := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", Stage: StageWrite,
		ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"},
	})
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	ready, _ := store.Get(context.Background(), started.ExperimentID)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.DecideWrite(context.Background(), "alice", ready.ID, ready.Candidates[0].ID, true)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent decision failed: %v", err)
		}
	}
	if runner.applyCalls != 1 || len(catalog.adopted) != 1 {
		t.Fatalf("apply calls=%d adoption writes=%d", runner.applyCalls, len(catalog.adopted))
	}
}
func (c *fakeCatalog) Active(context.Context, string, Stage) (ModelRef, bool, error) {
	return c.active, c.selected, c.activeErr
}
func (c *fakeCatalog) Recommended(Stage, ModelRef) bool { return false }

type fakeJobs struct {
	ids      []string
	runnable map[string]bool
	err      error
}

func (j *fakeJobs) EnqueueExperiment(_ context.Context, request JobRequest) (string, error) {
	if j.err != nil {
		return "", j.err
	}
	id := fmt.Sprintf("job-%d", len(j.ids)+1)
	j.ids = append(j.ids, id)
	if j.runnable == nil {
		j.runnable = map[string]bool{}
	}
	j.runnable[request.ExperimentID] = true
	return id, nil
}
func (j *fakeJobs) HasRunnableExperiment(_ context.Context, id string) (bool, error) {
	return j.runnable[id], nil
}

type fakeRunner struct {
	snapshotCalls, runCalls, applyCalls int
	fail                                map[string]error
	applyErr                            error
}

func (r *fakeRunner) Snapshot(context.Context, StartRequest) (Snapshot, error) {
	r.snapshotCalls++
	return Snapshot{Content: []byte(`{"same":true}`), PromptVersion: "v1"}, nil
}
func (r *fakeRunner) PrepareWrite(_ context.Context, found Experiment, _ Progress) (Snapshot, error) {
	return Snapshot{Content: found.InputSnapshot, PromptVersion: found.PromptVersion}, nil
}
func (r *fakeRunner) RunCandidate(_ context.Context, _ Experiment, candidate Candidate, _ Progress) (CandidateResult, error) {
	r.runCalls++
	return CandidateResult{Output: []byte(`{"title":"ok"}`), Usage: UsageReport{PromptTokens: 10, CompletionTokens: 2}}, r.fail[candidate.Model.ModelID]
}
func (r *fakeRunner) ApplyWinner(context.Context, Experiment, Candidate, bool) error {
	r.applyCalls++
	return r.applyErr
}

func newTestService() (*Service, *memoryStore, *fakeCatalog, *fakeJobs, *fakeRunner) {
	a := ModelRef{ProviderID: "p", ModelID: "a"}
	b := ModelRef{ProviderID: "p", ModelID: "b"}
	store := newMemoryStore()
	catalog := &fakeCatalog{models: map[ModelRef]Model{a: {Ref: a, Label: "A", Enabled: true, Vision: true, InputUSDPerMillion: "1", OutputUSDPerMillion: "2"}, b: {Ref: b, Label: "B", Enabled: true, Vision: true, InputUSDPerMillion: "1", OutputUSDPerMillion: "2"}}}
	jobs := &fakeJobs{runnable: map[string]bool{}}
	runner := &fakeRunner{fail: map[string]error{}}
	svc := NewService(store, catalog, jobs, runner, 30*24*time.Hour)
	n := 0
	svc.newID = func() string { n++; return fmt.Sprintf("id-%d", n) }
	svc.now = func() time.Time { return time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC) }
	return svc, store, catalog, jobs, runner
}

func TestStartHandleChooseWriteExperiment(t *testing.T) {
	svc, store, _, jobs, runner := newTestService()
	started, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if runner.runCalls != 0 || runner.snapshotCalls != 1 || len(jobs.ids) != 1 || started.JobID == "" {
		t.Fatalf("start ran provider work or did not enqueue: runner=%+v jobs=%v", runner, jobs.ids)
	}
	found, _ := store.Get(context.Background(), started.ExperimentID)
	if len(found.Candidates) != 2 || found.Candidates[0].DisplaySide == found.Candidates[1].DisplaySide {
		t.Fatalf("candidate sides = %+v", found.Candidates)
	}
	if err := svc.Handle(context.Background(), found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ = store.Get(context.Background(), found.ID)
	if found.Status != StatusReview || runner.runCalls != 2 {
		t.Fatalf("after handle = %+v calls=%d", found, runner.runCalls)
	}
	chosen, err := svc.Choose(context.Background(), "alice", found.ID, found.Candidates[0].ID, false)
	if err != nil || chosen.Status != StatusDecided || chosen.Outcome != OutcomeWinner || runner.applyCalls != 1 {
		t.Fatalf("choose = %+v, %v applies=%d", chosen, err, runner.applyCalls)
	}
	if _, err := svc.Choose(context.Background(), "alice", found.ID, found.Candidates[0].ID, false); err != nil || runner.applyCalls != 1 {
		t.Fatalf("idempotent choose err=%v applies=%d", err, runner.applyCalls)
	}
	if _, err := svc.ApplyWinner(context.Background(), "alice", found.ID, false); err != nil || runner.applyCalls != 1 {
		t.Fatalf("idempotent apply err=%v applies=%d", err, runner.applyCalls)
	}
	if _, err := svc.Get(context.Background(), "mallory", found.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign get = %v", err)
	}
}

func TestStartRejectsNonPositiveTargetBeforeSnapshotOrEnqueue(t *testing.T) {
	svc, _, _, jobs, runner := newTestService()
	invalid := 0
	_, err := svc.Start(context.Background(), StartRequest{
		UserID: "alice", PostSlug: "post", Stage: StageWrite,
		ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}, TargetLength: &invalid,
	})
	if !errors.Is(err, ErrInvalidTargetLength) || runner.snapshotCalls != 0 || len(jobs.ids) != 0 {
		t.Fatalf("err=%v snapshots=%d jobs=%v", err, runner.snapshotCalls, jobs.ids)
	}
}

func TestPartialRetryRunsOnlyFailedCandidateAndUseSingleDoesNotRank(t *testing.T) {
	svc, store, _, jobs, runner := newTestService()
	runner.fail["b"] = errors.New("provider failed")
	started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(context.Background(), started.ExperimentID)
	if found.Status != StatusPartial {
		t.Fatalf("status = %s", found.Status)
	}
	runner.fail["b"] = nil
	if _, err := svc.Retry(context.Background(), "alice", found.ID); err != nil {
		t.Fatal(err)
	}
	if len(jobs.ids) != 2 {
		t.Fatalf("retry jobs = %v", jobs.ids)
	}
	if err := svc.Handle(context.Background(), found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if runner.runCalls != 3 {
		t.Fatalf("retry called %d candidates, want original 2 + failed 1", runner.runCalls)
	}

	// A separate partial experiment may use its survivor, but it is explicitly unpaired.
	svc2, store2, _, _, runner2 := newTestService()
	runner2.fail["b"] = errors.New("bad")
	second, _ := svc2.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "other", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	_ = svc2.Handle(context.Background(), second.ExperimentID, func(string, int, int) {})
	partial, _ := store2.Get(context.Background(), second.ExperimentID)
	var survivor string
	for _, candidate := range partial.Candidates {
		if candidate.Status == CandidateSucceeded {
			survivor = candidate.ID
		}
	}
	used, err := svc2.Choose(context.Background(), "alice", partial.ID, survivor, true)
	if err != nil || used.Outcome != OutcomeUnpaired {
		t.Fatalf("use single = %+v, %v", used, err)
	}
}

func TestRecoverInterruptedMakesRunningExperimentRetryable(t *testing.T) {
	svc, store, _, _, _ := newTestService()
	started, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(context.Background(), started.ExperimentID)
	if err := store.SetStatus(context.Background(), found.ID, StatusRunning, nil); err != nil {
		t.Fatal(err)
	}
	if err := store.StartCandidate(context.Background(), found.ID, found.Candidates[0].ID, svc.now()); err != nil {
		t.Fatal(err)
	}

	count, err := svc.RecoverInterrupted(context.Background())
	if err != nil || count != 1 {
		t.Fatalf("recover = %d, %v", count, err)
	}
	recovered, _ := store.Get(context.Background(), found.ID)
	if recovered.Status != StatusFailed || recovered.FinishedAt == nil {
		t.Fatalf("recovered experiment = %+v", recovered)
	}
	for _, candidate := range recovered.Candidates {
		if candidate.Status != CandidateFailed || candidate.Error != interruptedReason {
			t.Fatalf("recovered candidate = %+v", candidate)
		}
	}
	if count, err := svc.RecoverInterrupted(context.Background()); err != nil || count != 0 {
		t.Fatalf("idempotent recovery = %d, %v", count, err)
	}
}

func TestRecoverInterruptedFailsOnlyOrphanedQueuedExperiment(t *testing.T) {
	svc, store, _, jobs, _ := newTestService()
	started, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err != nil {
		t.Fatal(err)
	}
	if count, err := svc.RecoverInterrupted(context.Background()); err != nil || count != 0 {
		t.Fatalf("runnable queued recovery = %d, %v", count, err)
	}
	delete(jobs.runnable, started.ExperimentID)
	if count, err := svc.RecoverInterrupted(context.Background()); err != nil || count != 1 {
		t.Fatalf("orphan recovery = %d, %v", count, err)
	}
	recovered, _ := store.Get(context.Background(), started.ExperimentID)
	if recovered.Status != StatusFailed || recovered.Candidates[0].Status != CandidateFailed {
		t.Fatalf("orphan = %+v", recovered)
	}
}

func TestRetryEnqueueFailureRestoresCandidateState(t *testing.T) {
	svc, store, _, jobs, runner := newTestService()
	runner.fail["b"] = errors.New("provider failed")
	started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	before, _ := store.Get(context.Background(), started.ExperimentID)
	jobs.err = errors.New("queue unavailable")
	if _, err := svc.Retry(context.Background(), "alice", started.ExperimentID); err == nil {
		t.Fatal("retry enqueue unexpectedly succeeded")
	}
	after, _ := store.Get(context.Background(), started.ExperimentID)
	if after.Status != before.Status {
		t.Fatalf("status = %s, want %s", after.Status, before.Status)
	}
	for i := range before.Candidates {
		if after.Candidates[i].Status != before.Candidates[i].Status || after.Candidates[i].Error != before.Candidates[i].Error {
			t.Fatalf("candidate %d changed: before=%+v after=%+v", i, before.Candidates[i], after.Candidates[i])
		}
	}
}

func TestRetryDoesNotRevealRemovedCandidateModel(t *testing.T) {
	svc, store, catalog, _, runner := newTestService()
	runner.fail["b"] = errors.New("provider failed")
	started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	delete(catalog.models, ModelRef{"p", "b"})
	if _, err := svc.Retry(context.Background(), "alice", started.ExperimentID); !errors.Is(err, ErrRetryModelUnavailable) || strings.Contains(err.Error(), "p/b") {
		t.Fatalf("retry error leaked model identity: %v", err)
	}
	found, _ := store.Get(context.Background(), started.ExperimentID)
	if found.Status != StatusPartial {
		t.Fatalf("retry changed experiment: %+v", found)
	}
}

func TestApplyFailureStoresOnlyPublicMessage(t *testing.T) {
	svc, store, _, _, runner := newTestService()
	runner.applyErr = errors.New("SQLITE_CONSTRAINT posts.content internal-secret")
	started, _ := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", Stage: StageWrite, ModelA: ModelRef{"p", "a"}, ModelB: ModelRef{"p", "b"}})
	if err := svc.Handle(context.Background(), started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(context.Background(), started.ExperimentID)
	chosen, err := svc.Choose(context.Background(), "alice", found.ID, found.Candidates[0].ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if chosen.ApplyError != applyFailedReason {
		t.Fatalf("apply error = %q", chosen.ApplyError)
	}
}

var _ Store = (*memoryStore)(nil)
