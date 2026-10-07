package app

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/usage"
)

type bridgeRuntime struct {
	mu             sync.Mutex
	work           experiment.TestExecutionWork
	outputs        map[string][]byte
	settled        map[string]int
	settlementSeen bool
	ended          bool
	outcome        experiment.TestExecutionOutcome
}

func (s *bridgeRuntime) BeginTestExecution(_ context.Context, f experiment.TestExecutionFence) (experiment.TestExecutionWork, error) {
	if f != s.work.Fence {
		return experiment.TestExecutionWork{}, experiment.ErrTestStateInvalid
	}
	return s.work, nil
}
func (s *bridgeRuntime) SaveTestCheckpoint(context.Context, experiment.TestExecutionFence, string, []byte) error {
	return nil
}
func (s *bridgeRuntime) CompleteTestCandidate(_ context.Context, _ experiment.TestExecutionFence, id string, output, _ []byte, failure *experiment.Failure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return experiment.ErrTestStateInvalid
	}
	if failure != nil {
		return errors.New("unexpected pipeline failure")
	}
	s.outputs[id] = output
	return nil
}
func (s *bridgeRuntime) FinishTestExecution(_ context.Context, _ experiment.TestExecutionFence, charge int, _ *experiment.Failure) (experiment.WritingTest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if charge != 0 || len(s.outputs) != 16 {
		return experiment.WritingTest{}, experiment.ErrTestStateInvalid
	}
	return s.work.Test, nil
}
func (s *bridgeRuntime) ConfirmTestSettlement(_ context.Context, f experiment.TestExecutionFence, charge int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.settlementSeen && charge != 0 {
		return errors.New("experiment confirmation preceded durable settlement")
	}
	s.settled[f.JobID] = charge
	return nil
}
func (s *bridgeRuntime) RecoverInterruptedTests(context.Context) error { return nil }

type bridgePipeline struct {
	t     *testing.T
	mu    sync.Mutex
	calls int
}

func (p *bridgePipeline) PrepareTestInput(ctx context.Context, _ experiment.TestExecutionWork, save func(context.Context, []byte) error, _ experiment.Progress) ([]byte, error) {
	return []byte("shared"), save(ctx, []byte("shared"))
}
func (p *bridgePipeline) RunTestCandidate(ctx context.Context, w experiment.TestExecutionWork, id string, _ []byte, save func(context.Context, []byte) error, _ experiment.Progress) (experiment.TestExecutionResult, error) {
	if work, ok := usage.WorkFromContext(ctx); !ok || work.UserID != w.Fence.UserID || work.JobID != w.Fence.JobID || work.Kind != WritingTestJobKind {
		p.t.Error("provider call lost accounting scope", work, ok)
	}
	if err := save(ctx, []byte("issued")); err != nil {
		return experiment.TestExecutionResult{}, err
	}
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()
	output, err := experiment.EncodeTestOutput(experiment.TestOutput{ContentLanguage: "ko", Content: experiment.TestOutputContent{Title: "title", Blocks: []experiment.TestOutputBlock{{Type: "TEXT", Content: id}}}, Storyline: &experiment.TestOutputStoryline{Paragraphs: []experiment.TestOutputParagraph{{Text: "complete plan"}}}})
	return experiment.TestExecutionResult{Output: output}, err
}

type bridgeGate struct {
	runtime *bridgeRuntime
	holds   []job.Start
}

func (g *bridgeGate) Hold(_ context.Context, s job.Start) error {
	g.holds = append(g.holds, s)
	return nil
}
func (g *bridgeGate) Release(context.Context, string)             {}
func (g *bridgeGate) OpenHolds(context.Context) ([]string, error) { return nil, nil }
func (g *bridgeGate) Settle(context.Context, string, string) {
	g.runtime.mu.Lock()
	defer g.runtime.mu.Unlock()
	g.runtime.settlementSeen = true
}
func (g *bridgeGate) SettledJobCharge(context.Context, string, string) (int, error) {
	g.runtime.mu.Lock()
	defer g.runtime.mu.Unlock()
	if !g.runtime.settlementSeen {
		return 0, usage.ErrChargeNotSettled
	}
	return 7, nil
}

type bridgeCancel struct{}

func (bridgeCancel) Kind(kind string) bool           { return kind == WritingTestJobKind }
func (bridgeCancel) Allowed(kind string, v int) bool { return kind == WritingTestJobKind && v == 1 }

func TestWritingTestJobsAdmitExactPlanWaitForBindingAndConfirmActualSettlement(t *testing.T) {
	h, err := db.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	if err := db.Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{WritingTestJobKind}, Cancellable: []string{WritingTestJobKind}})
	queue := job.New(store, 5*time.Millisecond, nil)
	queue.AllowCancellation(bridgeCancel{})
	fence := experiment.TestExecutionFence{UserID: "alice", TestID: "test", JobID: "stable-job", RequestKey: "start", Revision: 1}
	runtime := &bridgeRuntime{work: experiment.TestExecutionWork{Fence: fence, Test: experiment.WritingTest{ID: "test", UserID: "alice", Input: experiment.TestInput{TargetLanguage: "ko"}}, Plan: experiment.TestPlan{EstimateCredits: 99, Calls: []experiment.TestCall{{Ref: experiment.ModelRef{ProviderID: "p", ModelID: "writer"}, Stage: experiment.StageWrite, Count: 16, PromptTokens: 30000, CompletionTokens: 9000}}}}, outputs: map[string][]byte{}, settled: map[string]int{}}
	for _, id := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"} {
		runtime.work.CandidateIDs = append(runtime.work.CandidateIDs, id)
	}
	pipeline := &bridgePipeline{t: t}
	gate := &bridgeGate{runtime: runtime}
	queue.Admit(gate)
	bridge := NewWritingTestJobs(queue, experiment.NewWritingTestRunner(runtime, pipeline), runtime, gate)
	queue.Register(WritingTestJobKind, bridge.Handler)
	queue.OnTerminal(WritingTestJobKind, bridge.Terminal)
	for range 2 {
		if id, err := bridge.StartWritingTestJob(t.Context(), runtime.work); err != nil || id != fence.JobID {
			t.Fatal(id, err)
		}
	}
	if len(gate.holds) != 1 || len(gate.holds[0].Calls) != 1 || gate.holds[0].Calls[0].Count != 16 || gate.holds[0].Calls[0].CompletionTokens != 9000 {
		t.Fatal("exact plan not admitted", gate.holds)
	}
	if _, err := store.PickNextQueued(t.Context(), time.Now()); !errors.Is(err, job.ErrNotFound) {
		t.Fatal("unbound work dispatched", err)
	}
	if err := bridge.ActivateWritingTestJob(t.Context(), "alice", fence.JobID); err != nil {
		t.Fatal(err)
	}
	if err := bridge.ActivateWritingTestJob(t.Context(), "alice", fence.JobID); err != nil {
		t.Fatal("lost activation retry", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { queue.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(10 * time.Second)
	for {
		runtime.mu.Lock()
		charge, finished := runtime.settled[fence.JobID]
		runtime.mu.Unlock()
		if finished {
			if charge != 7 {
				t.Fatal("quote used as charge", charge)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("actual settlement did not arrive")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if pipeline.calls != 16 || len(runtime.outputs) != 16 || len(gate.holds) != 1 {
		t.Fatal("not exactly sixteen calls", pipeline.calls, len(runtime.outputs), gate.holds)
	}
	found, err := queue.Result(t.Context(), "alice", fence.JobID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bridge.Terminal(t.Context(), found, time.Now()); err != nil {
		t.Fatal("receipt replay", err)
	}
	if _, err := bridge.StartWritingTestJob(t.Context(), runtime.work); err != nil || len(gate.holds) != 1 {
		t.Fatal("review repeated admission", err, gate.holds)
	}
}

func TestWritingTestPrivateCheckpointCodecPreservesIssuedUncertaintyAndRejectsInjection(t *testing.T) {
	v := generation.WritingTestCheckpoint{SnapshotHash: "hash", Index: 1, WriteModel: llm.ModelRef{ProviderID: "p", ModelID: "writer"}, InFlightStage: "write", Prepared: true}
	raw, err := encodeTestCheckpoint(v)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeTestCheckpoint(raw)
	if err != nil || decoded.InFlightStage != "write" || decoded.WriteModel != v.WriteModel {
		t.Fatal(decoded, err)
	}
	for _, raw := range [][]byte{[]byte(`{"version":2,"checkpoint":{}}`), []byte(`{"version":1,"checkpoint":{},"provider_override":"other"}`), []byte(`{"version":1,"checkpoint":{}} {}`)} {
		if _, err := decodeTestCheckpoint(raw); !errors.Is(err, generation.ErrWritingTestCheckpointInvalid) {
			t.Fatal("injected state accepted", string(raw), err)
		}
	}
}

func (s *bridgeRuntime) EndTestExecution(_ context.Context, _ experiment.TestExecutionFence, outcome experiment.TestExecutionOutcome, _ *experiment.Failure) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ended = true
	s.outcome = outcome
	return nil
}
func (s *bridgeRuntime) PendingTestSettlements(context.Context) ([]experiment.TestExecutionFence, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.settled[s.work.Fence.JobID]; ok {
		return nil, nil
	}
	return []experiment.TestExecutionFence{s.work.Fence}, nil
}

func recoveryBridge(t *testing.T) (*WritingTestJobs, *bridgeRuntime, *job.Queue, *db.DB, *bridgeGate) {
	t.Helper()
	h, err := db.Open(filepath.Join(t.TempDir(), "recover.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close() })
	if err := db.Migrate(t.Context(), h.Writer); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES('alice','hash',?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	store := jobstore.New(h.Writer, h.Reader, jobstore.Kinds{Deferred: []string{WritingTestJobKind}})
	queue := job.New(store, time.Second, nil)
	fence := experiment.TestExecutionFence{UserID: "alice", TestID: "test", JobID: "recovery-job", RequestKey: "start", Revision: 1}
	runtime := &bridgeRuntime{work: experiment.TestExecutionWork{Fence: fence, Test: experiment.WritingTest{ID: "test", UserID: "alice", Input: experiment.TestInput{TargetLanguage: "ko"}}, Plan: experiment.TestPlan{Calls: []experiment.TestCall{{Ref: experiment.ModelRef{ProviderID: "p", ModelID: "writer"}, Stage: experiment.StageWrite, Count: 1, PromptTokens: 30000, CompletionTokens: 9000}}}}, outputs: map[string][]byte{"successful-neighbor": []byte("original exact output")}, settled: map[string]int{}}
	gate := &bridgeGate{runtime: runtime}
	queue.Admit(gate)
	bridge := NewWritingTestJobs(queue, experiment.NewWritingTestRunner(runtime, &bridgePipeline{t: t}), runtime, gate)
	return bridge, runtime, queue, h, gate
}

func TestWritingTestTerminalUsesPersistedFailureAndFencesLateCallbacksBeforePendingSettlement(t *testing.T) {
	bridge, runtime, queue, h, _ := recoveryBridge(t)
	if _, err := bridge.StartWritingTestJob(t.Context(), runtime.work); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Writer.Exec("UPDATE generation_jobs SET status='failed',error_reason=?,error_params=?,technical_detail='private provider detail',observe_model='p/private-observer',write_model='p/private-writer' WHERE id=?", job.FailureReasonInterrupted, `{"model":"private"}`, runtime.work.Fence.JobID); err != nil {
		t.Fatal(err)
	}
	found, err := queue.Result(t.Context(), "alice", runtime.work.Fence.JobID)
	if err != nil {
		t.Fatal(err)
	}
	public, err := queue.Get(t.Context(), runtime.work.Fence.JobID, "alice")
	if err != nil || public.ObserveModel != "" || public.WriteModel != "" || public.Failure == nil || public.Failure.TechnicalDetail != "" || len(public.Failure.Params) != 0 {
		t.Fatal("writing test constructor did not protect job diagnostics", public, err)
	}
	if found.Failure == nil || found.Failure.TechnicalDetail == "" {
		t.Fatal("private recovery evidence lost", found)
	}
	found.Status = job.StatusDone
	if err := bridge.Terminal(t.Context(), found, time.Now()); !errors.Is(err, usage.ErrChargeNotSettled) {
		t.Fatal("pending receipt became settled value", err)
	}
	if !runtime.ended || runtime.outcome != experiment.TestExecutionFailed || len(runtime.settled) != 0 {
		t.Fatal("actual failure did not fence epoch before settlement", runtime.outcome, runtime.settled)
	}
	if err := runtime.CompleteTestCandidate(t.Context(), runtime.work.Fence, "late", []byte("resurrection"), nil, nil); !errors.Is(err, experiment.ErrTestStateInvalid) {
		t.Fatal("late callback survived terminal failure", err)
	}
	if string(runtime.outputs["successful-neighbor"]) != "original exact output" || len(runtime.outputs) != 1 {
		t.Fatal("successful neighbor changed", runtime.outputs)
	}
}

func TestWritingTestRecoveryReadsPurgedTerminalMetadataAndActualOnceOnlyCharge(t *testing.T) {
	for _, status := range []string{job.StatusFailed, job.StatusCancelled} {
		t.Run(status, func(t *testing.T) {
			bridge, runtime, queue, h, gate := recoveryBridge(t)
			if _, err := bridge.StartWritingTestJob(t.Context(), runtime.work); err != nil {
				t.Fatal(err)
			}
			if _, err := h.Writer.Exec("UPDATE generation_jobs SET status=?,payload='',cancel_requested_at=CASE WHEN ?='cancelled' THEN ? ELSE NULL END WHERE id=?", status, status, time.Now().UTC().Format(time.RFC3339Nano), runtime.work.Fence.JobID); err != nil {
				t.Fatal(err)
			}
			gate.Settle(t.Context(), runtime.work.Fence.JobID, status)
			if err := bridge.RecoverSettlements(t.Context()); err != nil {
				t.Fatal(err)
			}
			want := experiment.TestExecutionFailed
			if status == job.StatusCancelled {
				want = experiment.TestExecutionCancelled
			}
			if runtime.outcome != want || runtime.settled[runtime.work.Fence.JobID] != 7 {
				t.Fatal("terminal fact or actual receipt lost", runtime.outcome, runtime.settled)
			}
			if err := bridge.RecoverSettlements(t.Context()); err != nil {
				t.Fatal("recovery replay", err)
			}
			found, err := queue.Result(t.Context(), "alice", runtime.work.Fence.JobID)
			if err != nil || len(found.Payload) != 0 {
				t.Fatal("private payload restored", found, err)
			}
		})
	}
}

func TestWritingTestRecoveryClosesNeverCreatedAttemptAtZeroAndLeavesActiveWork(t *testing.T) {
	bridge, runtime, _, _, _ := recoveryBridge(t)
	if err := bridge.RecoverSettlements(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !runtime.ended || runtime.outcome != experiment.TestExecutionFailed || runtime.settled[runtime.work.Fence.JobID] != 0 {
		t.Fatal("never-admitted attempt not closed at zero", runtime.outcome, runtime.settled)
	}
	active, activeRuntime, _, _, _ := recoveryBridge(t)
	if _, err := active.StartWritingTestJob(t.Context(), activeRuntime.work); err != nil {
		t.Fatal(err)
	}
	if err := active.RecoverSettlements(t.Context()); err != nil {
		t.Fatal(err)
	}
	if activeRuntime.ended || len(activeRuntime.settled) != 0 {
		t.Fatal("queued work incorrectly terminalized", activeRuntime.settled)
	}
}

type replayOnlyPipeline struct{ output []byte }

func (p replayOnlyPipeline) PrepareTestInput(ctx context.Context, _ experiment.TestExecutionWork, save func(context.Context, []byte) error, _ experiment.Progress) ([]byte, error) {
	return []byte("already prepared"), save(ctx, []byte("already prepared"))
}
func (p replayOnlyPipeline) RunTestCandidate(ctx context.Context, _ experiment.TestExecutionWork, _ string, _ []byte, save func(context.Context, []byte) error, _ experiment.Progress) (experiment.TestExecutionResult, error) {
	return experiment.TestExecutionResult{Output: p.output}, save(ctx, []byte("paid completed checkpoint"))
}

func TestWritingTestPaidCheckpointRecoveryCreatesNoNewHoldOrInventedLedgerCharge(t *testing.T) {
	_, runtime, queue, _, gate := recoveryBridge(t)
	runtime.work.Fence.NonMetered = true
	runtime.work.Plan.Calls = nil
	runtime.outputs = map[string][]byte{}
	for _, id := range []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"} {
		runtime.work.CandidateIDs = append(runtime.work.CandidateIDs, id)
	}
	paid, err := experiment.EncodeTestOutput(experiment.TestOutput{ContentLanguage: "ko", Content: experiment.TestOutputContent{Title: "paid exact result", Blocks: []experiment.TestOutputBlock{{Type: "TEXT", Content: "complete recovered content"}}}})
	if err != nil {
		t.Fatal(err)
	}
	bridge := NewWritingTestJobs(queue, experiment.NewWritingTestRunner(runtime, replayOnlyPipeline{output: paid}), runtime, gate)
	queue.Register(WritingTestJobKind, bridge.Handler)
	queue.OnTerminal(WritingTestJobKind, bridge.Terminal)
	if _, err := bridge.StartWritingTestJob(t.Context(), runtime.work); err != nil {
		t.Fatal(err)
	}
	if len(gate.holds) != 0 {
		t.Fatal("completed checkpoint took new reservation", gate.holds)
	}
	if err := bridge.ActivateWritingTestJob(t.Context(), "alice", runtime.work.Fence.JobID); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { queue.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	deadline := time.Now().Add(10 * time.Second)
	for {
		runtime.mu.Lock()
		charge, finished := runtime.settled[runtime.work.Fence.JobID]
		runtime.mu.Unlock()
		if finished {
			if charge != 0 {
				t.Fatal("replay fabricated charge", charge)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("replay did not close")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(runtime.outputs) != 16 || string(runtime.outputs["0"]) != string(paid) || len(gate.holds) != 0 {
		t.Fatal("paid output changed or replay admitted newwork", runtime.outputs, gate.holds)
	}
}
