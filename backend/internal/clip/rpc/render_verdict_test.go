package rpc

import (
	"context"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
)

type verdictStore struct {
	clip.GenerationStore
	clip.BrowserRenderStore
	user, id string
	verdict  clip.RenderVerdict
}

func (s *verdictStore) GetBrowserRender(_ context.Context, user, id string) (clip.BrowserRender, error) {
	s.user, s.id = user, id
	return clip.BrowserRender{UserID: user, ID: id, Ratio: "square", DurationMS: 15000}, nil
}
func (s *verdictStore) SaveBrowserRenderVerdict(_ context.Context, user, id string, v clip.RenderVerdict, _ time.Time) error {
	s.user, s.id, s.verdict = user, id, v
	return nil
}

func TestReportedVerdictUsesActorAndReturnsDeliveryNotices(t *testing.T) {
	store := &verdictStore{}
	h := NewHandler(nil)
	h.generation = clipapp.NewGenerationService(store, nil, nil, nil, nil, nil, nil, nil, clip.GenerationConfig{Render: config.ClipRender(&config.Config{}), ReadTTL: time.Minute, CleanupTimeout: time.Second, OrphanMinAge: time.Hour}, neutralGenerationDeps())
	ctx := auth.WithUser(t.Context(), "owner")
	m := &v1.ClipRenderMeasurements{Width: 1080, Height: 1080, FrameRateNumerator: 30, FrameRateDenominator: 1, VideoFrames: 450, VideoCodec: "h264", VideoProfile: "High"}
	response, err := h.ReportClipRenderVerdict(ctx, connect.NewRequest(&v1.ReportClipRenderVerdictRequest{RenderId: "render", Measurements: m, Passed: true}))
	if err != nil || !response.Msg.Passed || store.user != "owner" || store.id != "render" || !store.verdict.ReportedPassed {
		t.Fatal(response, store, err)
	}
	// A producer's own refusal remains a notice even when its reported numbers
	// fall inside the contract. No media adapter exists in this handler.
	response, err = h.ReportClipRenderVerdict(ctx, connect.NewRequest(&v1.ReportClipRenderVerdictRequest{RenderId: "render", Measurements: m, Passed: false}))
	if err != nil || response.Msg.Passed || len(response.Msg.Notices) != 1 || response.Msg.Notices[0].Code != "render_output_verdict" {
		t.Fatal(response, err)
	}
	if _, err := h.ReportClipRenderVerdict(ctx, connect.NewRequest(&v1.ReportClipRenderVerdictRequest{})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatal(err)
	}
}
