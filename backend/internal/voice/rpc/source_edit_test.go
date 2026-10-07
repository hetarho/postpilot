package rpc_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type analysisPricing struct{}

func (analysisPricing) CallCredits(context.Context, llm.ModelInfo, int64, int64) (int, bool) {
	return 5, true
}

func TestSourceEditingRPCPinsAuthenticationRevisionAndSafeEstimate(t *testing.T) {
	db := openVoiceTestDB(t)
	ctx := context.Background()
	service := voice.NewService(voicestore.New(db.Writer, db.Reader), models{}, jobs{}).WithAnalysisEstimates(analysisPricing{}, 8192)
	found, err := service.CreateVoice(ctx, "alice", "편집할 말투")
	if err != nil {
		t.Fatal(err)
	}
	sample, err := service.AddSample(ctx, "alice", found.ID, "원본", strings.Repeat("반갑고 즐거웠어요.\n", voice.ReadySentences))
	if err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)
	alice := auth.WithUser(ctx, "alice")
	body := strings.Repeat("다시 즐겁게 썼어요.\n", voice.ReadySentences)
	request := &v1.UpdateVoiceSampleRequest{VoiceId: found.ID, SampleId: sample.ID, ExpectedContentRevision: 1, OperationKey: "edit", Body: &body}
	if _, err := handler.UpdateVoiceSample(ctx, connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal("anonymous edit accepted", err)
	}
	updated, err := handler.UpdateVoiceSample(alice, connect.NewRequest(request))
	if err != nil || updated.Msg.Sample.ContentRevision != 2 {
		t.Fatal("RPC lost content revision", err)
	}
	if replay, err := handler.UpdateVoiceSample(alice, connect.NewRequest(request)); err != nil || replay.Msg.Sample.ContentRevision != 2 {
		t.Fatal("RPC receipt replay failed", err)
	}
	stale := &v1.UpdateVoiceSampleRequest{VoiceId: request.VoiceId, SampleId: request.SampleId, ExpectedContentRevision: request.ExpectedContentRevision, OperationKey: "stale", Body: request.Body}
	if _, err := handler.UpdateVoiceSample(alice, connect.NewRequest(stale)); connect.CodeOf(err) != connect.CodeAborted || voiceReason(t, err) != "VOICE_SAMPLE_REVISION_CONFLICT" {
		t.Fatal("stale RPC reason", err)
	}
	if _, err := handler.UpdateVoiceSample(auth.WithUser(ctx, "bob"), connect.NewRequest(request)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("foreign edit accepted", err)
	}
	quote, err := handler.EstimateVoiceAnalysis(alice, connect.NewRequest(&v1.EstimateVoiceAnalysisRequest{VoiceId: found.ID, Model: &v1.ModelRef{ProviderId: "stub", ModelId: "analyze"}}))
	if err != nil || quote.Msg.Credits == nil || *quote.Msg.Credits != 5 || quote.Msg.Free {
		t.Fatal("analysis quote not exposed", err)
	}
	profile, err := handler.GetVoiceProfile(alice, connect.NewRequest(&v1.GetVoiceProfileRequest{VoiceId: found.ID}))
	if err != nil || profile.Msg.Profile.Samples[0].ContentRevision != 2 {
		t.Fatal("profile sample revision was missing", err)
	}
}
