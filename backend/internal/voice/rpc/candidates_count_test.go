package rpc

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

type countModels struct{}

func (countModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{Stages: []string{llm.StageNameWrite}, Levels: map[string]string{llm.StageNameWrite: "free"}}, ref.ProviderID == "p" && ref.ModelID == "w"
}
func (countModels) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	panic("estimate/start must not call provider")
}

type countBudget struct{}

func (countBudget) Short(bool) int { return 8192 }

type countEstimate struct{}

func (countEstimate) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return 10, true
}

type countStore struct{}

func (countStore) AdoptCandidate(context.Context, voice.CandidateAdoption) (voice.Voice, error) {
	panic("start must not adopt")
}

type countJobs struct{ requests []voice.CandidateJobRequest }

func (j *countJobs) EnqueueCandidates(_ context.Context, in voice.CandidateJobRequest) (string, error) {
	j.requests = append(j.requests, in)
	return "batch", nil
}
func (*countJobs) CandidateResult(context.Context, string, string) (voice.CandidateJob, error) {
	return voice.CandidateJob{}, voice.ErrCandidateNotFound
}
func (*countJobs) LatestCandidates(context.Context, string, string) (*voice.CandidateJob, error) {
	return nil, nil
}
func (*countJobs) SaveCandidateResult(context.Context, string, []byte) error { return nil }
func (*countJobs) CancelCandidates(context.Context, string, string) error    { return nil }
func TestCandidateRPCAdmitsBinaryFormatsAndKeepsAuthenticatedDefaultEight(t *testing.T) {
	jobs := &countJobs{}
	handler := NewCandidateHandler(voice.NewCandidateService(countModels{}, jobs, countStore{}, countBudget{}, countEstimate{}))
	ctx := auth.WithUser(context.Background(), "alice")
	model := &v1.ModelRef{ProviderId: "p", ModelId: "w"}
	for _, n := range []int32{0, 2, 4, 8, 16} {
		estimate, err := handler.EstimateWritingVoiceCandidates(ctx, connect.NewRequest(&v1.EstimateWritingVoiceCandidatesRequest{WriteModel: model, CandidateCount: n}))
		if err != nil || !estimate.Msg.Free {
			t.Fatalf("%d estimate %v", n, err)
		}
		result, err := handler.StartWritingVoiceCandidates(ctx, connect.NewRequest(&v1.StartWritingVoiceCandidatesRequest{WriteModel: model, CandidateCount: n}))
		if err != nil || result.Msg.JobId != "batch" {
			t.Fatalf("%d start %v", n, err)
		}
		var input struct {
			Count      int      `json:"count"`
			Directions []string `json:"directions"`
		}
		request := jobs.requests[len(jobs.requests)-1]
		if err := json.Unmarshal(request.Payload, &input); err != nil {
			t.Fatal(err)
		}
		expected := int(n)
		if expected == 0 {
			expected = 8
		}
		if input.Count != expected || len(input.Directions) != expected || request.UserID != "alice" || request.CompletionTokens != 8192*expected/8 {
			t.Fatalf("%d request=%+v", n, request)
		}
	}
	before := len(jobs.requests)
	for _, n := range []int32{-1, 1, 3, 5, 17} {
		if _, err := handler.StartWritingVoiceCandidates(ctx, connect.NewRequest(&v1.StartWritingVoiceCandidatesRequest{WriteModel: model, CandidateCount: n})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%d err=%v", n, err)
		}
	}
	if _, err := handler.StartWritingVoiceCandidates(context.Background(), connect.NewRequest(&v1.StartWritingVoiceCandidatesRequest{CandidateCount: 3})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("anonymous=%v", err)
	}
	if len(jobs.requests) != before {
		t.Fatal("invalid/foreign request admitted")
	}
}
