package experiment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type runnerRuntime struct {
	mu                           sync.Mutex
	work                         TestExecutionWork
	started, cancelled, finished bool
	checkpoints                  map[string][]byte
	results                      map[string]TestExecutionResult
	failures                     map[string]*Failure
}

func newRunnerRuntime(count int) *runnerRuntime {
	s := &runnerRuntime{work: TestExecutionWork{Fence: TestExecutionFence{UserID: "alice", TestID: "test", JobID: "job", RequestKey: "start"}}, checkpoints: map[string][]byte{}, results: map[string]TestExecutionResult{}, failures: map[string]*Failure{}}
	for i := range count {
		s.work.CandidateIDs = append(s.work.CandidateIDs, fmt.Sprint(i))
	}
	return s
}
func (s *runnerRuntime) BeginTestExecution(_ context.Context, f TestExecutionFence) (TestExecutionWork, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || f != s.work.Fence {
		return TestExecutionWork{}, ErrTestStateInvalid
	}
	s.started = true
	return s.work, nil
}
func (s *runnerRuntime) SaveTestCheckpoint(ctx context.Context, f TestExecutionFence, id string, p []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelled || f != s.work.Fence {
		return ErrTestStateInvalid
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.checkpoints[id] = append([]byte(nil), p...)
	return nil
}
func (s *runnerRuntime) CompleteTestCandidate(_ context.Context, f TestExecutionFence, id string, o, a []byte, failure *Failure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancelled || f != s.work.Fence {
		return ErrTestStateInvalid
	}
	s.results[id] = TestExecutionResult{Output: o, Accounting: a}
	s.failures[id] = failure
	return nil
}
func (s *runnerRuntime) FinishTestExecution(_ context.Context, _ TestExecutionFence, cost int, failure *Failure) (WritingTest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cost != 0 {
		return WritingTest{}, errors.New("runner invented confirmed charge")
	}
	if s.cancelled || len(s.results) != len(s.work.CandidateIDs) {
		return WritingTest{}, ErrTestStateInvalid
	}
	s.finished = true
	return WritingTest{}, nil
}
func (s *runnerRuntime) ConfirmTestSettlement(context.Context, TestExecutionFence, int) error {
	return nil
}
func (s *runnerRuntime) RecoverInterruptedTests(context.Context) error { return nil }

type runnerPipeline struct {
	active, max, calls atomic.Int64
	started            chan string
	release            chan struct{}
	sharedError        error
	panicID            string
	sharedPanic        bool
	failID             string
}

func (p *runnerPipeline) PrepareTestInput(ctx context.Context, _ TestExecutionWork, save func(context.Context, []byte) error, _ Progress) ([]byte, error) {
	if p.sharedPanic {
		panic("private shared diagnostics")
	}
	if p.sharedError != nil {
		return nil, p.sharedError
	}
	raw := []byte("shared checkpoint")
	return raw, save(ctx, raw)
}
func (p *runnerPipeline) RunTestCandidate(ctx context.Context, _ TestExecutionWork, id string, _ []byte, save func(context.Context, []byte) error, _ Progress) (TestExecutionResult, error) {
	if p.panicID != "" && id == p.panicID {
		panic("private supplier diagnostics")
	}
	if err := save(ctx, []byte("issued")); err != nil {
		return TestExecutionResult{}, err
	}
	p.calls.Add(1)
	active := p.active.Add(1)
	for old := p.max.Load(); active > old; old = p.max.Load() {
		if p.max.CompareAndSwap(old, active) {
			break
		}
	}
	defer p.active.Add(-1)
	if p.started != nil {
		p.started <- id
	}
	if p.release != nil {
		<-p.release
	}
	if err := save(ctx, []byte("settled")); err != nil {
		return TestExecutionResult{}, err
	}
	if id == p.failID {
		return TestExecutionResult{Accounting: []byte("confirmed partial usage")}, errors.New("candidate failed")
	}
	return TestExecutionResult{Output: []byte("complete post " + id), Accounting: []byte("actual tokens")}, nil
}

func TestWritingTestRunnerBoundsSixteenPipelinesToFiveAndWaitsForAllOutputs(t *testing.T) {
	store := newRunnerRuntime(16)
	pipeline := &runnerPipeline{started: make(chan string, 16), release: make(chan struct{}), failID: "none"}
	runner := NewWritingTestRunner(store, pipeline)
	finished := make(chan error, 1)
	last := 0
	go func() {
		finished <- runner.Run(t.Context(), store.work.Fence, func(_ string, done, total int) {
			if total != 16 || done != last+1 {
				t.Errorf("progress skipped/backtracked %d/%d after%d", done, total, last)
			}
			last = done
		})
	}()
	for range 5 {
		select {
		case <-pipeline.started:
		case <-time.After(5 * time.Second):
			t.Fatal("five pipelines did not start")
		}
	}
	if pipeline.active.Load() != 5 || pipeline.max.Load() != 5 {
		t.Fatal("concurrency", pipeline.active.Load(), pipeline.max.Load())
	}
	select {
	case <-finished:
		t.Fatal("run completed before outputs")
	default:
	}
	store.mu.Lock()
	if store.finished || len(store.results) != 0 {
		t.Fatal("barrier opened early")
	}
	store.mu.Unlock()
	close(pipeline.release)
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run did not finish")
	}
	if pipeline.calls.Load() != 16 || pipeline.max.Load() > 5 || !store.finished || len(store.results) != 16 || last != 16 {
		t.Fatal("full bounded work", pipeline.calls.Load(), pipeline.max.Load(), len(store.results), last)
	}
	if len(store.checkpoints["shared"]) == 0 {
		t.Fatal("shared preparation not durable")
	}
}
func TestWritingTestRunnerFailureHasNoAutomaticLossAndRetryRunsOnlySelectedFailures(t *testing.T) {
	store := newRunnerRuntime(4)
	pipeline := &runnerPipeline{failID: "1"}
	if err := NewWritingTestRunner(store, pipeline).Run(t.Context(), store.work.Fence, nil); err == nil {
		t.Fatal("failed result hidden")
	}
	if len(store.results) != 4 || pipeline.calls.Load() != 4 || store.failures["1"] == nil || len(store.results["1"].Output) != 0 {
		t.Fatal("successful neighbors or failure lost", store.results, store.failures)
	}
	retry := newRunnerRuntime(1)
	retry.work.CandidateIDs = []string{"1"}
	retry.work.Test.Candidates = []TestCandidate{{ID: "0", Status: string(TestCandidateSucceeded), Output: []byte("old exact output")}}
	retried := &runnerPipeline{failID: "none"}
	if err := NewWritingTestRunner(retry, retried).Run(t.Context(), retry.work.Fence, nil); err != nil {
		t.Fatal(err)
	}
	if retried.calls.Load() != 1 || len(retry.results) != 1 || retry.results["0"].Output != nil {
		t.Fatal("successful candidate rerun", retry.results, retried.calls.Load())
	}
}
func TestWritingTestRunnerSharedFailureNeverIssuesWriters(t *testing.T) {
	store := newRunnerRuntime(8)
	pipeline := &runnerPipeline{sharedError: errors.New("shared observation failed")}
	if err := NewWritingTestRunner(store, pipeline).Run(t.Context(), store.work.Fence, nil); err == nil {
		t.Fatal("shared failure hidden")
	}
	if pipeline.calls.Load() != 0 || len(store.results) != 8 || !store.finished {
		t.Fatal("writers issued or failure not durable")
	}
}
func TestWritingTestRunnerCancellationFencesLateProviderCallbacks(t *testing.T) {
	store := newRunnerRuntime(16)
	pipeline := &runnerPipeline{started: make(chan string, 16), release: make(chan struct{}), failID: "none"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	finished := make(chan error, 1)
	go func() { finished <- NewWritingTestRunner(store, pipeline).Run(ctx, store.work.Fence, nil) }()
	for range 5 {
		select {
		case <-pipeline.started:
		case <-time.After(5 * time.Second):
			t.Fatal("providers did not begin")
		}
	}
	store.mu.Lock()
	store.cancelled = true
	store.mu.Unlock()
	cancel()
	close(pipeline.release)
	select {
	case err := <-finished:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel cleanup did not return")
	}
	if len(store.results) != 0 || store.finished || pipeline.calls.Load() > 5 {
		t.Fatal("late result restored or new calls issued", store.results, pipeline.calls.Load())
	}
}

func TestWritingTestRunnerContainsPanicsOnSharedAndCandidateGoroutines(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			store := newRunnerRuntime(4)
			pipeline := &runnerPipeline{failID: "none", panicID: "1", sharedPanic: shared}
			if err := NewWritingTestRunner(store, pipeline).Run(t.Context(), store.work.Fence, nil); !errors.Is(err, errWritingTestPipelinePanic) {
				t.Fatal("panic not normalized", err)
			}
			if !store.finished || len(store.results) != 4 {
				t.Fatal("panic did not terminalize all candidates")
			}
			if store.failures["1"] == nil || store.failures["1"].TechnicalDetail == "private supplier diagnostics" {
				t.Fatal("panic detail leaked", store.failures)
			}
		})
	}
}
