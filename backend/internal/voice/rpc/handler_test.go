package rpc_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type models struct{}

func (models) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{}, false, nil
}
func (models) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	return llm.Response{}, nil
}

type jobs struct{}

func (jobs) Enqueue(context.Context, voice.AnalysisJobRequest) (string, error) { return "job", nil }
func (jobs) ActiveForUserKind(context.Context, string, string) (*voice.ActiveJob, error) {
	return nil, nil
}

func TestVoiceRPCIsScopedOnlyByAuthenticatedContext(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, userID := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", userID, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	for _, row := range []struct{ user, style, rules string }{
		{"alice", "alice style", "alice rules"}, {"bob", "bob style", "bob rules"},
	} {
		if _, err := service.Update(context.Background(), row.user, row.style, row.rules); err != nil {
			t.Fatal(err)
		}
	}
	handler := voicerpc.NewHandler(service)
	for _, userID := range []string{"alice", "bob"} {
		response, err := handler.GetVoiceProfile(
			auth.WithUser(context.Background(), userID),
			connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{}),
		)
		if err != nil {
			t.Fatal(err)
		}
		if got := response.Msg.GetProfile(); got.GetStyleguide() != userID+" style" || got.GetRules() != userID+" rules" {
			t.Fatalf("%s received foreign profile: %+v", userID, got)
		}
	}

	_, err = handler.AddVoiceSample(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{Body: strings.Repeat("가", 199)}),
	)
	if connect.CodeOf(err) != connect.CodeInvalidArgument || !strings.Contains(err.Error(), "199") {
		t.Fatalf("short sample RPC error = %v", err)
	}
	_, err = handler.AddVoiceSample(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{
			Body:  strings.Repeat("가", 200),
			Model: &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "analyze"},
		}),
	)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("missing model RPC error = %v", err)
	}
	if count, err := voicestore.New(handle.Writer, handle.Reader).CountSamples(context.Background(), "alice"); err != nil || count != 0 {
		t.Fatalf("model failure stored %d samples: %v", count, err)
	}
}
