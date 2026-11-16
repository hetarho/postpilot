package rpc_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

func TestVoiceExperienceRPC(t *testing.T) {
	handle := openVoiceTestDB(t)
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	handler := voicerpc.NewHandler(service)
	alice := auth.WithUser(context.Background(), "alice")
	bob := auth.WithUser(context.Background(), "bob")
	created, err := service.CreateVoice(alice, "alice", "기본 말투")
	if err != nil {
		t.Fatal(err)
	}
	cafe := postpilotv1.VoiceExperienceKind_VOICE_EXPERIENCE_KIND_CAFE
	start := func(ctx context.Context, voiceID string, kind postpilotv1.VoiceExperienceKind, name string) (*postpilotv1.VoiceExperience, error) {
		response, err := handler.StartVoiceExperience(ctx, connect.NewRequest(&postpilotv1.StartVoiceExperienceRequest{VoiceId: voiceID, Kind: kind, Name: name}))
		if err != nil {
			return nil, err
		}
		return response.Msg.GetExperience(), nil
	}

	for name, call := range map[string]func() error{
		"unknown kind": func() error { _, err := start(alice, created.ID, 0, ""); return err },
		"long name":    func() error { _, err := start(alice, created.ID, cafe, strings.Repeat("가", 41)); return err },
		"foreign":      func() error { _, err := start(bob, created.ID, cafe, ""); return err },
	} {
		want := map[string]connect.Code{"unknown kind": connect.CodeInvalidArgument, "long name": connect.CodeInvalidArgument, "foreign": connect.CodeNotFound}[name]
		if got := connect.CodeOf(call()); got != want {
			t.Fatalf("%s code = %v, want %v", name, got, want)
		}
	}

	experience, err := start(alice, created.ID, cafe, " 성수 카페 ")
	if err != nil || experience.GetId() == "" || experience.GetName() != "성수 카페" || experience.GetKind() != cafe {
		t.Fatalf("start = %+v err=%v", experience, err)
	}
	listed, err := handler.ListVoiceExperiences(alice, connect.NewRequest(&postpilotv1.ListVoiceExperiencesRequest{VoiceId: created.ID}))
	if err != nil || len(listed.Msg.GetExperiences()) != 1 || listed.Msg.GetExperiences()[0].GetId() != experience.GetId() {
		t.Fatalf("list = %+v err=%v", listed, err)
	}
	if _, err := handler.ListVoiceExperiences(bob, connect.NewRequest(&postpilotv1.ListVoiceExperiencesRequest{VoiceId: created.ID})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign list code = %v", err)
	}

	// An answer naming its experience, and one without it.
	answer, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "thread_cafe_opening_1", Body: "성수동에서 친구랑 갔어요.", ExperienceId: experience.GetId()}))
	if err != nil || answer.Msg.GetSample().GetExperienceId() != experience.GetId() {
		t.Fatalf("thread answer = %+v err=%v", answer, err)
	}
	if _, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "thread_cafe_opening_1", Body: "성수동에서 갔어요."})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("thread answer without experience code = %v", err)
	}
	if _, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "thread_cafe_opening_1", Body: "성수동에서 갔어요.", ExperienceId: "nope"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown experience code = %v", err)
	}
	plain, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "opening_greeting", Body: "안녕하세요!"}))
	if err != nil || plain.Msg.GetSample().GetExperienceId() != "" {
		t.Fatalf("situation answer = %+v err=%v", plain, err)
	}

	// A deleted voice refuses a new experience but stays readable (VOICE-15).
	if _, err := handler.DeleteVoice(alice, connect.NewRequest(&postpilotv1.DeleteVoiceRequest{VoiceId: created.ID})); err != nil {
		t.Fatal(err)
	}
	if _, err := start(alice, created.ID, cafe, ""); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("deleted start code = %v", err)
	}
	if listed, err := handler.ListVoiceExperiences(alice, connect.NewRequest(&postpilotv1.ListVoiceExperiencesRequest{VoiceId: created.ID})); err != nil || len(listed.Msg.GetExperiences()) != 1 {
		t.Fatalf("deleted list = %+v err=%v", listed, err)
	}
}
