package experiment

import (
	"context"
	"errors"
	"sync"
)

const MaxConcurrentTestCandidates = 5

var errWritingTestPipelinePanic = errors.New("writing test pipeline interrupted unexpectedly")

// TestExecutionPipeline is full-post behavior supplied at the application seam.
// Checkpoints stay opaque in this context and must commit before and after calls.
type TestExecutionPipeline interface {
	PrepareTestInput(context.Context, TestExecutionWork, func(context.Context, []byte) error, Progress) ([]byte, error)
	RunTestCandidate(context.Context, TestExecutionWork, string, []byte, func(context.Context, []byte) error, Progress) (TestExecutionResult, error)
}
type TestExecutionResult struct{ Output, Accounting []byte }
type WritingTestRunner struct {
	store    WritingTestRuntimeStore
	pipeline TestExecutionPipeline
}

func NewWritingTestRunner(store WritingTestRuntimeStore, pipeline TestExecutionPipeline) *WritingTestRunner {
	if store == nil || pipeline == nil {
		panic("experiment: writing test runner requires durable runtime and full-post pipeline")
	}
	return &WritingTestRunner{store: store, pipeline: pipeline}
}

// Run claims one durable attempt. Every callback is checked by its store fence,
// including successful checkpoint replay; cancellation cannot republish content.
func (r *WritingTestRunner) Run(ctx context.Context, fence TestExecutionFence, progress Progress) error {
	work, err := r.store.BeginTestExecution(ctx, fence)
	if err != nil {
		return err
	}
	if progress == nil {
		progress = func(string, int, int) {}
	}
	shared, err := r.prepare(ctx, work, func(ctx context.Context, raw []byte) error {
		return r.store.SaveTestCheckpoint(ctx, work.Fence, "shared", raw)
	}, progress)
	if err != nil {
		failure := normalizeFailure(err)
		for _, id := range work.CandidateIDs {
			if saveErr := r.store.CompleteTestCandidate(ctx, work.Fence, id, nil, nil, &failure); saveErr != nil {
				return errors.Join(err, saveErr)
			}
		}
		_, finishErr := r.store.FinishTestExecution(ctx, work.Fence, 0, &failure)
		return errors.Join(err, finishErr)
	}
	done := 0
	var mu sync.Mutex
	var failures []error
	tasks := make(chan string)
	var workers sync.WaitGroup
	for range min(MaxConcurrentTestCandidates, len(work.CandidateIDs)) {
		workers.Go(func() {
			for id := range tasks {
				result, runErr := r.candidate(ctx, work, id, shared, func(ctx context.Context, raw []byte) error {
					return r.store.SaveTestCheckpoint(ctx, work.Fence, id, raw)
				}, func(string, int, int) {})
				var failure *Failure
				if runErr != nil {
					normalized := normalizeFailure(runErr)
					failure = &normalized
				}
				if saveErr := r.store.CompleteTestCandidate(ctx, work.Fence, id, result.Output, result.Accounting, failure); saveErr != nil {
					runErr = errors.Join(runErr, saveErr)
				}
				if runErr != nil {
					mu.Lock()
					failures = append(failures, runErr)
					mu.Unlock()
				}
				mu.Lock()
				done++
				progress("writing_test", done, len(work.CandidateIDs))
				mu.Unlock()
			}
		})
	}
enqueue:
	for _, id := range work.CandidateIDs {
		select {
		case tasks <- id:
		case <-ctx.Done():
			break enqueue
		}
	}
	close(tasks)
	workers.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}
	var failure *Failure
	if len(failures) > 0 {
		normalized := normalizeFailure(failures[0])
		failure = &normalized
	}
	_, err = r.store.FinishTestExecution(ctx, work.Fence, 0, failure)
	return errors.Join(append(failures, err)...)
}

func (r *WritingTestRunner) prepare(ctx context.Context, work TestExecutionWork, save func(context.Context, []byte) error, progress Progress) (shared []byte, err error) {
	defer func() {
		if recover() != nil {
			shared = nil
			err = errWritingTestPipelinePanic
		}
	}()
	return r.pipeline.PrepareTestInput(ctx, work, save, progress)
}
func (r *WritingTestRunner) candidate(ctx context.Context, work TestExecutionWork, id string, shared []byte, save func(context.Context, []byte) error, progress Progress) (result TestExecutionResult, err error) {
	defer func() {
		if recover() != nil {
			result = TestExecutionResult{}
			err = errWritingTestPipelinePanic
		}
	}()
	return r.pipeline.RunTestCandidate(ctx, work, id, shared, save, progress)
}
