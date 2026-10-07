package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/usage"
)

const WritingTestJobKind = "writing_test"

type TestSettledCharge interface {
	SettledJobCharge(context.Context, string, string) (int, error)
}
type WritingTestJobRuntime interface {
	experiment.WritingTestRuntimeStore
	experiment.WritingTestExecutionTerminalStore
	experiment.WritingTestSettlementStore
}
type WritingTestJobs struct {
	queue   *job.Queue
	runner  *experiment.WritingTestRunner
	runtime WritingTestJobRuntime
	charges TestSettledCharge
}

func NewWritingTestJobs(queue *job.Queue, runner *experiment.WritingTestRunner, runtime WritingTestJobRuntime, charges TestSettledCharge) *WritingTestJobs {
	if queue == nil || runner == nil || runtime == nil || charges == nil {
		panic("experiment/app: writing test jobs requires queue, runner, runtime and actual charge reader")
	}
	queue.ProtectDetails(WritingTestJobKind)
	return &WritingTestJobs{queue: queue, runner: runner, runtime: runtime, charges: charges}
}

type testJobWire struct {
	Version int                           `json:"version"`
	Fence   experiment.TestExecutionFence `json:"fence"`
}

func encodeTestJob(fence experiment.TestExecutionFence) ([]byte, error) {
	return json.Marshal(testJobWire{Version: 1, Fence: fence})
}
func decodeTestJob(raw []byte) (experiment.TestExecutionFence, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return experiment.TestExecutionFence{}, experiment.ErrTestMaterialInvalid
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var w testJobWire
	if err := d.Decode(&w); err != nil {
		return experiment.TestExecutionFence{}, experiment.ErrTestMaterialInvalid
	}
	if w.Version != 1 || d.Decode(new(any)) != io.EOF || w.Fence.UserID == "" || w.Fence.TestID == "" || w.Fence.JobID == "" || w.Fence.RequestKey == "" {
		return experiment.TestExecutionFence{}, experiment.ErrTestMaterialInvalid
	}
	return w.Fence, nil
}
func (a *WritingTestJobs) StartWritingTestJob(ctx context.Context, work experiment.TestExecutionWork) (string, error) {
	if work.Fence.JobID == "" || work.Fence.UserID != work.Test.UserID || work.Fence.TestID != work.Test.ID {
		return "", experiment.ErrTestMaterialInvalid
	}
	if work.Fence.NonMetered != (len(work.Plan.Calls) == 0) {
		return "", experiment.ErrTestMaterialInvalid
	}
	payload, err := encodeTestJob(work.Fence)
	if err != nil {
		return "", err
	}
	calls := make([]job.PlannedCall, len(work.Plan.Calls))
	for i, c := range work.Plan.Calls {
		calls[i] = job.PlannedCall{Ref: c.Ref.String(), Stage: string(c.Stage), Count: c.Count, PromptTokens: c.PromptTokens, CompletionTokens: c.CompletionTokens}
	}
	return a.queue.EnqueueWithID(ctx, work.Fence.JobID, job.NewJob{Kind: WritingTestJobKind, UserID: work.Fence.UserID, TargetLanguage: work.Test.Input.TargetLanguage,
		CancellationPolicyVersion: 1, NonMetered: work.Fence.NonMetered, Payload: payload, PricingCalls: calls})
}
func (a *WritingTestJobs) ActivateWritingTestJob(ctx context.Context, user, id string) error {
	if err := a.queue.Activate(ctx, user, id); err != nil {
		// A lost activation response can leave an already queued/running job.
		found, readErr := a.queue.Result(ctx, user, id)
		if readErr == nil && found.Kind == WritingTestJobKind && found.DispatchReady {
			return nil
		}
		return errors.Join(err, readErr)
	}
	return nil
}
func (a *WritingTestJobs) CancelWritingTestJob(ctx context.Context, user, id string) error {
	_, err := a.queue.CancelOwned(ctx, user, id)
	return err
}
func (a *WritingTestJobs) Handler(ctx context.Context, found job.Job, progress job.Progress) error {
	fence, err := decodeTestJob(found.Payload)
	if err != nil {
		return err
	}
	if found.Kind != WritingTestJobKind || fence.UserID != found.UserID || fence.JobID != found.ID {
		return experiment.ErrTestMaterialInvalid
	}
	return a.runner.Run(usage.WithWork(ctx, usage.Work{UserID: found.UserID, Kind: found.Kind, JobID: found.ID}), fence, experiment.Progress(progress))
}
func (a *WritingTestJobs) Terminal(ctx context.Context, found job.Job, _ time.Time) error {
	persisted, err := a.queue.Result(ctx, found.UserID, found.ID)
	if err != nil {
		return err
	}
	found = persisted
	fence, err := decodeTestJob(found.Payload)
	if err != nil {
		pending, listErr := a.runtime.PendingTestSettlements(ctx)
		if listErr != nil {
			return listErr
		}
		matched := false
		for _, f := range pending {
			if f.JobID == found.ID && f.UserID == found.UserID {
				fence = f
				matched = true
				break
			}
		}
		if !matched {
			return err
		}
	}
	if found.Kind != WritingTestJobKind || fence.UserID != found.UserID || fence.JobID != found.ID || !job.Terminal(found.Status) {
		return experiment.ErrTestMaterialInvalid
	}
	return a.finishTerminal(ctx, fence, found)
}

func (a *WritingTestJobs) finishTerminal(ctx context.Context, fence experiment.TestExecutionFence, found job.Job) error {
	if found.Kind != WritingTestJobKind || found.UserID != fence.UserID || found.ID != fence.JobID || !job.Terminal(found.Status) {
		return experiment.ErrTestMaterialInvalid
	}
	outcome := experiment.TestExecutionSucceeded
	if found.Status == job.StatusCancelled {
		outcome = experiment.TestExecutionCancelled
	} else if found.Status == job.StatusFailed {
		outcome = experiment.TestExecutionFailed
	}
	var failure *experiment.Failure
	if found.Failure != nil {
		failure = &experiment.Failure{Reason: found.Failure.Reason}
	}
	if err := a.runtime.EndTestExecution(ctx, fence, outcome, failure); err != nil {
		return err
	}
	if fence.NonMetered {
		return a.runtime.ConfirmTestSettlement(ctx, fence, 0)
	}
	charge, err := a.charges.SettledJobCharge(ctx, found.UserID, found.ID)
	if err != nil {
		return err
	}
	return a.runtime.ConfirmTestSettlement(ctx, fence, charge)
}

// RecoverSettlements runs at boot after queue interruption/unactivated/hold
// recovery and before HTTP admission or workers. Only aggregate metadata is read
// after payload purge; completed output is never reconstructed here.
func (a *WritingTestJobs) RecoverSettlements(ctx context.Context) error {
	pending, err := a.runtime.PendingTestSettlements(ctx)
	if err != nil {
		return err
	}
	var failures []error
	for _, fence := range pending {
		found, err := a.queue.Result(ctx, fence.UserID, fence.JobID)
		if errors.Is(err, job.ErrNotFound) {
			// No durable job means no handler could issue work. The boot hold
			// sweep has abandoned/released any pre-insert admission already.
			if err := a.runtime.EndTestExecution(ctx, fence, experiment.TestExecutionFailed, &experiment.Failure{Reason: experiment.FailureReasonInterrupted}); err != nil {
				failures = append(failures, err)
				continue
			}
			if err := a.runtime.ConfirmTestSettlement(ctx, fence, 0); err != nil {
				failures = append(failures, err)
			}
			continue
		}
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if !job.Terminal(found.Status) {
			continue
		}
		if err := a.finishTerminal(ctx, fence, found); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
