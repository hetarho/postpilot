package rpc

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/experiment"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

type retainedRecordsFake struct {
	found experiment.Experiment
	reads int
}

func (f *retainedRecordsFake) Get(_ context.Context, user, id string) (experiment.Experiment, error) {
	f.reads++
	if user != f.found.UserID || id != f.found.ID {
		return experiment.Experiment{}, experiment.ErrNotFound
	}
	return f.found, nil
}
func (f *retainedRecordsFake) List(_ context.Context, user string, _ experiment.Stage, _ experiment.Source) ([]experiment.Experiment, error) {
	f.reads++
	if user != f.found.UserID {
		return nil, nil
	}
	return []experiment.Experiment{f.found}, nil
}
func (f *retainedRecordsFake) ReflectionDetail(context.Context, experiment.Experiment) (experiment.ReflectionDetail, error) {
	return experiment.ReflectionDetail{}, nil
}
func (f *retainedRecordsFake) ReflectionPromptText(experiment.Experiment) string { return "" }
func TestLegacyNewModesAreAuthenticatedFriendlyRefusalsWithoutMutationDependencies(t *testing.T) {
	f := &retainedRecordsFake{}
	h := NewHandler(f)
	calls := []func(context.Context) error{func(ctx context.Context) error {
		_, e := h.DecideWriteExperiment(ctx, connect.NewRequest(&v1.DecideWriteExperimentRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.StartObserveExperiment(ctx, connect.NewRequest(&v1.StartObserveExperimentRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.StartWriteExperiment(ctx, connect.NewRequest(&v1.StartWriteExperimentRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.StartVoiceReflectionExperiment(ctx, connect.NewRequest(&v1.StartVoiceReflectionExperimentRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.RetryCandidate(ctx, connect.NewRequest(&v1.RetryCandidateRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.ChooseWinner(ctx, connect.NewRequest(&v1.ChooseWinnerRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.UseSingleCandidate(ctx, connect.NewRequest(&v1.UseSingleCandidateRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.DismissExperiment(ctx, connect.NewRequest(&v1.DismissExperimentRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.CompleteExperimentReview(ctx, connect.NewRequest(&v1.CompleteExperimentReviewRequest{}))
		return e
	}, func(ctx context.Context) error {
		_, e := h.GetLeaderboard(ctx, connect.NewRequest(&v1.GetLeaderboardRequest{}))
		return e
	}}
	for i, call := range calls {
		if e := call(context.Background()); connect.CodeOf(e) != connect.CodeUnauthenticated {
			t.Fatalf("unauthenticated endpoint%d=%v", i, e)
		}
		e := call(auth.WithUser(context.Background(), "alice"))
		if connect.CodeOf(e) != connect.CodeFailedPrecondition || experimentAppErrorDetail(t, e).GetReason() != experiment.FailureReasonTestLegacyReadOnly {
			t.Fatalf("retired endpoint%d=%v", i, e)
		}
	}
	if f.reads != 0 {
		t.Fatal("retired mutation read or created private work")
	}
}
func TestLegacyPaidRankedHistoryRemainsReadableWithoutInventedChampion(t *testing.T) {
	for _, count := range []int{3, 5} {
		f := &retainedRecordsFake{found: experiment.Experiment{ID: "old", UserID: "alice", Stage: experiment.StageWrite, ReviewMode: experiment.ReviewCandidateRanking, Status: experiment.StatusCompleted}}
		for i := 0; i < count; i++ {
			f.found.Candidates = append(f.found.Candidates, experiment.Candidate{ID: string(rune('a' + i)), Rank: i + 1, Status: experiment.CandidateSucceeded, Output: []byte(`{"title":"paid history","blocks":[{"type":"TEXT","content":"retained full post"}]}`)})
		}
		h := NewHandler(f)
		ctx := auth.WithUser(context.Background(), "alice")
		found, err := h.GetExperiment(ctx, connect.NewRequest(&v1.GetExperimentRequest{Id: "old"}))
		if err != nil || len(found.Msg.Experiment.Candidates) != count || found.Msg.Experiment.GetWinnerCandidateId() != "" {
			t.Fatalf("retained %d-history=%+v err=%v", count, found, err)
		}
		listed, err := h.ListExperiments(ctx, connect.NewRequest(&v1.ListExperimentsRequest{}))
		if err != nil || len(listed.Msg.Experiments) != 1 || len(listed.Msg.Experiments[0].Candidates) != count {
			t.Fatalf("retained list=%+v err=%v", listed, err)
		}
	}
}
func TestLegacyPublicationReturnsOnlyAuthoritativeCompletedReceiptsAndNeverReplaysPendingFlags(t *testing.T) {
	f := &retainedRecordsFake{found: experiment.Experiment{ID: "old", UserID: "alice", Stage: experiment.StageWrite, Status: experiment.StatusCompleted, WinnerCandidateID: "winner", ApplyRequested: true, AdoptionRequested: true, AppliedCandidateID: "winner", AdoptedCandidateID: "winner", Candidates: []experiment.Candidate{{ID: "winner", Status: experiment.CandidateSucceeded, Model: experiment.ModelRef{ProviderID: "p", ModelID: "winner"}}}}}
	h := NewHandler(f)
	ctx := auth.WithUser(context.Background(), "alice")
	apply := func() error {
		_, e := h.ApplyCandidateOutput(ctx, connect.NewRequest(&v1.ApplyCandidateOutputRequest{ExperimentId: "old", CandidateId: "winner", AdoptModel: true}))
		return e
	}
	adopt := func() error {
		_, e := h.AdoptCandidateModel(ctx, connect.NewRequest(&v1.AdoptCandidateModelRequest{ExperimentId: "old", CandidateId: "winner"}))
		return e
	}
	for _, call := range []func() error{apply, adopt} {
		if e := call(); experimentAppErrorDetail(t, e).GetReason() != experiment.FailureReasonTestLegacyReadOnly {
			t.Fatalf("pending intent replayed=%v", e)
		}
	}
	now := time.Now()
	f.found.AppliedAt = &now
	f.found.AdoptedAt = &now
	for _, call := range []func() error{apply, adopt} {
		if e := call(); e != nil {
			t.Fatalf("committed receipt unavailable=%v", e)
		}
	}
	_, e := h.ApplyCandidateOutput(ctx, connect.NewRequest(&v1.ApplyCandidateOutputRequest{ExperimentId: "old", CandidateId: "other"}))
	if experimentAppErrorDetail(t, e).GetReason() != experiment.FailureReasonTestLegacyReadOnly {
		t.Fatalf("different receipt replayed=%v", e)
	}
	_, e = h.AdoptCandidateModel(auth.WithUser(context.Background(), "bob"), connect.NewRequest(&v1.AdoptCandidateModelRequest{ExperimentId: "old", CandidateId: "winner"}))
	if connect.CodeOf(e) != connect.CodeNotFound {
		t.Fatalf("foreign receipt=%v", e)
	}
}
