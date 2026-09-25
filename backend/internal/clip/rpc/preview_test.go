package rpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/proto"
)

type previewRPCStore struct {
	clip.Store
	clip.SourceStore
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
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := json.Marshal([]clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}}})
	store := previewRPCStore{project: clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", EditPlan: raw, EditPlanRevision: 1, Analysis: string(analysis)}}
	projects := testProjects(store)
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	generation := clipapp.NewGenerationService(nil, projects, nil, neutralProcessing{}, nil, nil, previewRPCRenderer{}, neutralJobs{}, cfg, neutralGenerationDeps())
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
func (previewRPCRenderer) PrepareCaptionFrames(_ context.Context, _ clip.EditPlan, _ []clip.RenderSource, instance string, offset int, _ clip.PreviewConfig) (clip.CaptionFrames, error) {
	return clip.CaptionFrames{Sheet: []byte{1, 2, 3}, CellWidth: 900, CellHeight: 250, Columns: 4, Cells: 30,
		X: 90, Y: 640, FirstFrame: 30, FrameOffset: offset, NextOffset: -1}, nil
}

// The frames a browser render draws from are admitted exactly as a draft preview
// is: the owner's own project, at the revision they were looking at, pinned to
// the plan they hold (CLIP-159, CLIP-154).
func TestCaptionFramesRPCIsOwnerScopedAndPinnedToTheDraft(t *testing.T) {
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "fp", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	analysis, _ := json.Marshal([]clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}}})
	store := previewRPCStore{project: clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", EditPlan: raw, EditPlanRevision: 1, Analysis: string(analysis)}}
	projects := testProjects(store)
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	generation := clipapp.NewGenerationService(nil, projects, nil, neutralProcessing{}, nil, nil, previewRPCRenderer{}, neutralJobs{}, cfg, neutralGenerationDeps())
	h := NewHandler(projects).WithGeneration(generation, nil)
	wire := editingProto(&clip.CorrectionState{Plan: clip.CorrectionFromPlan(plan)}).Plan
	encoded, _ := (proto.MarshalOptions{Deterministic: true}).Marshal(wire)
	hash := sha256.Sum256(encoded)
	body := &v1.PrepareClipCaptionFramesRequest{ProjectId: "owned", ExpectedRevision: 1, Plan: wire,
		DraftHash: hex.EncodeToString(hash[:]), InstanceId: "caption/cut"}
	if _, err := h.PrepareClipCaptionFrames(t.Context(), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	if _, err := h.PrepareClipCaptionFrames(auth.WithUser(t.Context(), "bob"), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	ctx := auth.WithUser(t.Context(), "alice")
	out, err := h.PrepareClipCaptionFrames(ctx, connect.NewRequest(body))
	if err != nil {
		t.Fatal(err)
	}
	if out.Msg.DraftHash != body.DraftHash || out.Msg.Cells != 30 || out.Msg.NextOffset != -1 || out.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(out.Msg)
	}
	stale := &v1.PrepareClipCaptionFramesRequest{ProjectId: body.ProjectId, ExpectedRevision: 2, Plan: body.Plan,
		DraftHash: body.DraftHash, InstanceId: body.InstanceId}
	if _, err := h.PrepareClipCaptionFrames(ctx, connect.NewRequest(stale)); connect.CodeOf(err) != connect.CodeAborted {
		t.Fatal("a stale revision was served", err)
	}
	body.Plan.Hook = "changed"
	if _, err := h.PrepareClipCaptionFrames(ctx, connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatal("stale body hash accepted", err)
	}
	// A finalized project has no draft to read frames of (CLIP-76).
	store.project.Finalized = &clip.Finalization{PlanRevision: 1, ResultID: "result"}
	finalized := NewHandler(testProjects(store)).WithGeneration(
		clipapp.NewGenerationService(nil, testProjects(store), nil, neutralProcessing{}, nil, nil, previewRPCRenderer{}, neutralJobs{}, cfg, neutralGenerationDeps()), nil)
	body.Plan.Hook = ""
	if _, err := finalized.PrepareClipCaptionFrames(ctx, connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("a finalized project served frames", err)
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

func (previewRPCRenderer) RegionPresetSamples(_ context.Context, ratio, label string) ([]clip.RegionPresetSample, []clip.RegionPresetSample, error) {
	sample := func(id string) clip.RegionPresetSample {
		return clip.RegionPresetSample{Preset: id, SVG: "<g>" + ratio + " " + label + "</g>", Box: clip.Region{X: 100, Y: 800, Width: 880, Height: 300}}
	}
	return []clip.RegionPresetSample{sample("a"), sample("cover")}, []clip.RegionPresetSample{sample("b")}, nil
}

// ① asks for the preset drawings with its own slot label; the project lends its
// ratio and owner only, and a label without a place for the number is refused
// (CLIP-165).
func TestRegionPresetSamplesRPCIsOwnerScopedAndTakesTheCallersLabel(t *testing.T) {
	store := previewRPCStore{project: clip.Project{ID: "owned", UserID: "alice", Ratio: "square"}}
	projects := testProjects(store)
	cfg := clip.DefaultGenerationConfig(clip.Environment{GetTTL: time.Minute, OrphanMinAge: time.Hour})
	h := NewHandler(projects).WithGeneration(clipapp.NewGenerationService(nil, projects, nil, neutralProcessing{}, nil, nil, previewRPCRenderer{}, neutralJobs{}, cfg, neutralGenerationDeps()), nil)
	body := &v1.GetClipRegionPresetSamplesRequest{ProjectId: "owned", SlotLabel: "슬롯 {n}"}
	if _, err := h.GetClipRegionPresetSamples(t.Context(), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatal(err)
	}
	if _, err := h.GetClipRegionPresetSamples(auth.WithUser(t.Context(), "bob"), connect.NewRequest(body)); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal(err)
	}
	ctx := auth.WithUser(t.Context(), "alice")
	for _, label := range []string{"슬롯", "", "아주 긴 슬롯 이름입니다 {n}", "슬롯\n{n}"} {
		bad := &v1.GetClipRegionPresetSamplesRequest{ProjectId: "owned", SlotLabel: label}
		if _, err := h.GetClipRegionPresetSamples(ctx, connect.NewRequest(bad)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Fatalf("%q: %v", label, err)
		}
	}
	out, err := h.GetClipRegionPresetSamples(ctx, connect.NewRequest(body))
	if err != nil {
		t.Fatal(err)
	}
	m := out.Msg
	if m.Ratio != "square" || m.Canvas.GetWidth() != 1080 || m.Canvas.GetHeight() != 1080 || out.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal(m, out.Header())
	}
	if len(m.Intro) != 2 || m.Intro[1].Preset != "cover" || m.Intro[0].Svg != "<g>square 슬롯 {n}</g>" || m.Intro[0].Box.GetWidth() != 880 || len(m.Outro) != 1 || m.Outro[0].Preset != "b" {
		t.Fatal(m.Intro, m.Outro)
	}
}
