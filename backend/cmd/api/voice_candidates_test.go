package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type candidateCancellationTest struct{ jobCancellation }

func (c candidateCancellationTest) Kind(kind string) bool {
	return kind == job.KindWritingVoiceCandidates || c.jobCancellation.Kind(kind)
}
func (c candidateCancellationTest) Allowed(kind string, version int) bool {
	if kind == job.KindWritingVoiceCandidates {
		return version == 1
	}
	return c.jobCancellation.Allowed(kind, version)
}

type candidateEstimateTest struct{}

func (candidateEstimateTest) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return 5, true
}

func writingStyleFixture() string {
	values := make([]voice.WritingCandidate, 8)
	for i := range values {
		values[i] = voice.WritingCandidate{Name: fmt.Sprintf("편안한 말투 %d", i+1), Description: "소소한 기분을 편하게 이야기하는 말투예요.", Sample: strings.Repeat(fmt.Sprintf("산책하다 작은 가게에 들러 따뜻한 차를 마셨어요 %d. 창가에서 잠깐 쉬니 마음이 편안했어요. ", i+1), 5)}
	}
	raw, _ := json.Marshal(map[string]any{"candidates": values})
	return string(raw)
}
func candidateHarness(t *testing.T) (*requestHarness, *voice.CandidateService, *jobstore.Store) {
	t.Helper()
	h := newRequestHarness(t)
	kinds := jobKindsForTest()
	kinds.Cancellable = append(kinds.Cancellable, job.KindWritingVoiceCandidates)
	kinds.Authorized = append(kinds.Authorized, job.KindWritingVoiceCandidates)
	if kinds.FirstStages == nil {
		kinds.FirstStages = map[string]string{}
	}
	kinds.FirstStages[job.KindWritingVoiceCandidates] = "write"
	store := jobstore.New(h.d.Writer, h.d.Reader, kinds)
	h.queue = job.New(store, time.Millisecond, jobReportingForTest())
	h.queue.Admit(h.admit)
	h.queue.AllowCancellation(candidateCancellationTest{})
	h.models.answers = []string{writingStyleFixture()}
	service := voice.NewCandidateService(h.models, voiceCandidateJobs{queue: h.queue}, voicestore.New(h.d.Writer, h.d.Reader), testCompletionBudget(), candidateEstimateTest{})
	h.queue.Register(job.KindWritingVoiceCandidates, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return service.Run(ctx, voice.CandidateRun{ID: found.ID, UserID: found.UserID, WriteModel: found.WriteModel, Payload: found.Payload}, voice.Progress(progress))
	})
	return h, service, store
}
func runCandidateWorker(t *testing.T, h *requestHarness) {
	t.Helper()
	ctx, stop := context.WithCancel(context.Background())
	finished := make(chan struct{})
	go func() { defer close(finished); h.queue.Run(ctx) }()
	t.Cleanup(func() { stop(); <-finished })
}

func TestWritingStyleBatchOwnsOnePlannedBoundedCallAndDurableOwnerResult(t *testing.T) {
	h, service, _ := candidateHarness(t)
	ctx := context.Background()
	ref := llm.ModelRef{ProviderID: "p", ModelID: "m"}
	id, err := service.Start(ctx, "alice", ref)
	if err != nil {
		t.Fatal(err)
	}
	var running *voice.CandidatesRunningError
	if _, err := service.Start(ctx, "alice", ref); !errors.As(err, &running) || running.ActiveID != id {
		t.Fatalf("duplicate=%v", err)
	}
	if len(h.admit.holds) != 1 || len(h.admit.holds[0].Calls) != 1 {
		t.Fatalf("hold=%+v", h.admit.holds)
	}
	call := h.admit.holds[0].Calls[0]
	if call.Count != 1 || call.Stage != llm.StageNameWrite || call.CompletionTokens != 8192 || call.PromptTokens <= 0 {
		t.Fatalf("planned call=%+v", call)
	}
	if _, err := service.Get(ctx, "alice", id); !errors.Is(err, voice.ErrCandidatesNotReady) {
		t.Fatalf("premature=%v", err)
	}
	if _, err := service.Get(ctx, "bob", id); !errors.Is(err, voice.ErrCandidateNotFound) {
		t.Fatalf("foreign=%v", err)
	}
	runCandidateWorker(t, h)
	if terminal := h.waitTerminal(t, id); terminal != job.StatusDone {
		t.Fatalf("terminal=%s", terminal)
	}
	batch, err := service.Get(ctx, "alice", id)
	if err != nil || len(batch.Candidates) != 8 || h.models.calls != 1 {
		t.Fatalf("batch=%+v calls=%d err=%v", batch, h.models.calls, err)
	}
	handler := voicerpc.NewCandidateHandler(service)
	if _, err := handler.GetWritingVoiceCandidates(context.Background(), connect.NewRequest(&postpilotv1.GetWritingVoiceCandidatesRequest{JobId: id})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous=%v", err)
	}
	if _, err := handler.GetWritingVoiceCandidates(auth.WithUser(ctx, "bob"), connect.NewRequest(&postpilotv1.GetWritingVoiceCandidatesRequest{JobId: id})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign RPC=%v", err)
	}
	adopted, err := handler.AdoptWritingVoiceCandidate(auth.WithUser(ctx, "alice"), connect.NewRequest(&postpilotv1.AdoptWritingVoiceCandidateRequest{JobId: id, CandidateId: batch.Candidates[0].ID, MakeDefault: true}))
	if err != nil || !adopted.Msg.Voice.Made || !adopted.Msg.Voice.IsDefault || adopted.Msg.Voice.Origin != postpilotv1.VoiceOrigin_VOICE_ORIGIN_SYNTHETIC || h.models.calls != 1 {
		t.Fatalf("adopted=%v err=%v calls=%d", adopted, err, h.models.calls)
	}
	// A failed explicit replacement preserves the previous successful set.
	h.models.answers = []string{`{"candidates":[]}`}
	replacement, err := service.Start(ctx, "alice", ref)
	if err != nil {
		t.Fatal(err)
	}
	if terminal := h.waitTerminal(t, replacement); terminal != job.StatusFailed {
		t.Fatalf("invalid replacement=%s", terminal)
	}
	latest, err := service.Latest(ctx, "alice")
	if err != nil || latest.JobID != replacement || latest.ResultJobID != id || len(latest.Candidates) != 8 || h.models.calls != 2 {
		t.Fatalf("latest=%+v err=%v calls=%d", latest, err, h.models.calls)
	}
	if err := service.Cancel(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Adopt(ctx, "alice", id, batch.Candidates[0].ID, false); err != nil || h.models.calls != 2 {
		t.Fatalf("repeat adoption=%v calls=%d", err, h.models.calls)
	}
}

func TestWritingStyleCancellationAndRestartNeverRepeatProviderWork(t *testing.T) {
	t.Run("queued", func(t *testing.T) {
		h, service, store := candidateHarness(t)
		ctx := context.Background()
		id, err := service.Start(ctx, "alice", llm.ModelRef{ProviderID: "p", ModelID: "m"})
		if err != nil {
			t.Fatal(err)
		}
		if err := service.Cancel(ctx, "bob", id); !errors.Is(err, voice.ErrCandidateNotFound) {
			t.Fatalf("foreign cancel=%v", err)
		}
		if err := service.Cancel(ctx, "alice", id); err != nil {
			t.Fatal(err)
		}
		if err := service.Cancel(ctx, "alice", id); err != nil {
			t.Fatal(err)
		}
		if _, err := store.PickNextQueued(ctx, time.Now()); !errors.Is(err, job.ErrNotFound) {
			t.Fatalf("cancelled job dispatches=%v", err)
		}
		if _, err := service.Adopt(ctx, "alice", id, "style-1", true); !errors.Is(err, voice.ErrCandidatesNotReady) || h.models.calls != 0 {
			t.Fatalf("cancelled adoption=%v calls=%d", err, h.models.calls)
		}
	})
	t.Run("in flight", func(t *testing.T) {
		h, service, _ := candidateHarness(t)
		ctx := context.Background()
		h.models.entered, h.models.release = make(chan struct{}), make(chan struct{})
		id, err := service.Start(ctx, "alice", llm.ModelRef{ProviderID: "p", ModelID: "m"})
		if err != nil {
			t.Fatal(err)
		}
		runCandidateWorker(t, h)
		<-h.models.entered
		if err := service.Cancel(ctx, "alice", id); err != nil {
			close(h.models.release)
			t.Fatal(err)
		}
		close(h.models.release)
		if terminal := h.waitTerminal(t, id); terminal != job.StatusCancelled || h.models.calls != 1 {
			t.Fatalf("terminal=%s calls=%d", terminal, h.models.calls)
		}
		if _, err := service.Get(ctx, "alice", id); !errors.Is(err, voice.ErrCandidatesNotReady) {
			t.Fatalf("cancelled result=%v", err)
		}
	})
	t.Run("interrupted", func(t *testing.T) {
		h, service, store := candidateHarness(t)
		ctx := context.Background()
		id, err := service.Start(ctx, "alice", llm.ModelRef{ProviderID: "p", ModelID: "m"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := store.PickNextQueued(ctx, time.Now()); err != nil {
			t.Fatal(err)
		}
		if count, err := h.queue.SweepRunning(ctx); err != nil || count != 1 {
			t.Fatalf("sweep=%d err=%v", count, err)
		}
		if _, err := store.PickNextQueued(ctx, time.Now()); !errors.Is(err, job.ErrNotFound) {
			t.Fatalf("interrupted dispatch=%v", err)
		}
		summary, err := h.queue.Get(ctx, id, "alice")
		if err != nil || summary.Status != job.StatusFailed || summary.Failure.Reason != "JOB_INTERRUPTED" || h.models.calls != 0 {
			t.Fatalf("summary=%+v err=%v", summary, err)
		}
	})
}

func TestWritingStyleEstimateUsesConservativeAdmissionPromptBound(t *testing.T) {
	rate := plan.RateSnapshot{Source: "korea-eximbank", PublicationDate: "2026-09-29", ReferenceE4: 13600000, AppliedE4: 13600000}
	info := llm.ModelInfo{InputUSDPerMillion: "1", OutputUSDPerMillion: "4"}
	credits, ok := (voiceCandidateEstimates{rates: stubRates{rate: rate}}).CallCredits(context.Background(), info, 2000, 8192)
	want, wantOK := plan.CallCreditsAt(catalogPricer(info), rate, 30000, 8192)
	if !ok || !wantOK || credits != want {
		t.Fatalf("estimate=%d %v want=%d %v", credits, ok, want, wantOK)
	}
}

func TestWritingStyleDispatchRefusesMissingWorkAndDurableCancellationBeforeProvider(t *testing.T) {
	ref := llm.ModelRef{ProviderID: "p", ModelID: "m"}
	// Nil registry proves refusal happens before provider resolution or usage writes.
	if _, err := (voiceCandidateModels{}).Complete(context.Background(), ref, llm.Request{}); !errors.Is(err, job.ErrDispatchRefused) {
		t.Fatalf("unmetered work=%v", err)
	}
	h, service, store := candidateHarness(t)
	ctx := context.Background()
	id, err := service.Start(ctx, "alice", ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.PickNextQueued(ctx, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := service.Cancel(ctx, "alice", id); err != nil {
		t.Fatal(err)
	}
	callCtx := usage.WithWork(ctx, usage.Work{UserID: "alice", JobID: id, Kind: job.KindWritingVoiceCandidates})
	if _, err := (voiceCandidateModels{dispatch: store}).Complete(callCtx, ref, llm.Request{Stage: llm.StageNameWrite, MaxTokens: 8192}); !errors.Is(err, job.ErrDispatchRefused) || h.models.calls != 0 {
		t.Fatalf("cancelled dispatch=%v calls=%d", err, h.models.calls)
	}
}
