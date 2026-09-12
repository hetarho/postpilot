package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/platform/config"
	"google.golang.org/protobuf/proto"
)

type previewRPCStore struct {
	clip.Store
	project clip.Project
}

func (s previewRPCStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	if user != "alice" || id != s.project.ID {
		return clip.Project{}, clip.ErrNotFound
	}
	return s.project, nil
}

type previewRPCRenderer struct{ clip.Renderer }

func (previewRPCRenderer) PreparePreview(context.Context, clip.EditPlan, []clip.RenderSource, []string, int, clip.PreviewConfig) (clip.PreparedPreview, error) {
	return clip.PreparedPreview{Canvas: clip.Canvas{Width: 1080, Height: 1920}, NextOffset: -1, Parity: []clip.PreviewParity{clip.PreviewSourceContrast, clip.PreviewAudioNormalization, clip.PreviewFrameTiming}}, nil
}
func TestPreviewRPCAuthenticatesHashOwnerAndReadOnlyResponse(t *testing.T) {
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "fp", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
	raw, err := clip.EncodeEditPlan(plan, []string{"clean"})
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := json.Marshal([]clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}}})
	store := previewRPCStore{project: clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", EditPlan: raw, EditPlanRevision: 1, Analysis: string(analysis)}}
	projects := clip.NewService(store, config.ClipLimits())
	cfg := config.ClipGeneration(&config.Config{PresignGetTTL: time.Minute, OrphanMinAge: time.Hour})
	generation := clip.NewGenerationService(nil, projects, nil, nil, nil, nil, previewRPCRenderer{}, nil, cfg)
	h := NewHandler(projects).WithGeneration(generation, nil)
	wire := editingProto(&clip.CorrectionState{Plan: clip.CorrectionFromPlan(plan)}).Plan
	encoded, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(wire)
	hash := sha256.Sum256(encoded)
	body := &v1.PrepareClipPreviewRequest{ProjectId: "owned", ExpectedRevision: 1, Plan: wire, DraftHash: hex.EncodeToString(hash[:])}
	if _, err := h.PrepareClipPreview(t.Context(), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	if _, err := h.PrepareClipPreview(auth.WithUser(t.Context(), "bob"), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	ctx := auth.WithUser(t.Context(), "alice")
	out, err := h.PrepareClipPreview(ctx, connect.NewRequest(body))
	if err != nil {
		t.Fatal(err)
	}
	if out.Msg.DraftHash != body.DraftHash || out.Header().Get("Cache-Control") != "private, no-store" || len(out.Msg.Parity) != 3 || out.Msg.NextOffset != -1 {
		t.Fatal(out.Msg)
	}
	body.Plan.Hook = "changed"
	if _, err := h.PrepareClipPreview(ctx, connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatal("stale body hash accepted", err)
	}
}
func TestNativeEditingWireKeepsOptionalTimingAndBoundRows(t *testing.T) {
	zero, end := 0, 1200
	value := clip.CorrectionText{InstanceID: "owned", ElementID: "label", Kind: "fixed", Role: "info", Text: "<정확한 문구>", Rows: []composition.ResolvedRow{{Role: "caption", Text: "9,900원"}}, Style: "auto", Position: "header", Align: "center", Basis: "output-start", StartMS: &zero, EndMS: &end, GroupID: "menu", ItemID: "one", ResolvedEndMS: end}
	if got := correctionText(correctionTextProto(value)); !reflect.DeepEqual(value, got) {
		t.Fatal(got)
	}
	for _, err := range []error{clip.ErrPreviewTooLarge, clip.ErrPreviewBusy} {
		if connect.CodeOf(toConnectError(err)) != connect.CodeResourceExhausted {
			t.Fatal(err)
		}
	}
}

func TestPreviewLimitIncludesJSONBase64AndManifest(t *testing.T) {
	out := &v1.PrepareClipPreviewResponse{CanvasWidth: 1080, CanvasHeight: 1920, NextOffset: -1}
	for range 8 {
		out.Assets = append(out.Assets, &v1.ClipPreviewAsset{Png: make([]byte, 450*1024)})
	}
	if proto.Size(out) >= 4<<20 {
		t.Fatal("fixture is not under the binary limit")
	}
	if previewResponseFits(out, 4<<20) {
		t.Fatal("base64 response exceeded its limit")
	}
	out.Assets = out.Assets[:1]
	if !previewResponseFits(out, 4<<20) {
		t.Fatal("bounded response refused")
	}
}
