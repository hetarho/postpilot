package rpc

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// A finalized project is read in ① and ② as well as played in ③ (CLIP-160). Both
// readings are projections of the stored plan and evidence, so they survive the
// finalization that deleted the originals — while every write stays refused.
func TestFinalizedProjectStillCarriesItsPlanAndObservations(t *testing.T) {
	analysis, err := json.Marshal([]clip.SourceAnalysis{{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fingerprint",
			Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}, Filename: "travel.mp4"},
		Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "음식을 촬영", Focal: clip.Point{X: .5, Y: .5}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.EncodeEditPlan(clip.EditPlan{Ratio: "vertical", DurationMS: 15000,
		Cuts: []clip.EditCut{{ID: "cut", SourceID: "source", Fingerprint: "fingerprint", EndMS: 15000,
			Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: 1000,
			Copies: []clip.Caption{{Text: "정확한 한글", Anchor: "upper_mid", Align: "center", Style: "bold", StartMS: 200, EndMS: 3000}}}}})
	if err != nil {
		t.Fatal(err)
	}
	project := clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", Analysis: string(analysis), EditPlan: plan,
		EditPlanRevision: 3, RenderedPlanRevision: 3, Result: &clip.Result{ID: "result", Key: "private-result-key"},
		Finalized: &clip.Finalization{At: time.Now(), PlanRevision: 3, ResultID: "result"}}
	store := &observationStore{project: project}
	service := testProjects(store)
	generation := clipapp.NewGenerationService(nil, service, nil, neutralProcessing{}, nil, nil, nil, neutralJobs{},
		clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Minute, OrphanMinAge: time.Minute}, neutralGenerationDeps())
	h := NewHandler(service).WithGeneration(generation, nil)
	ctx := auth.WithUser(context.Background(), "alice")

	got, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: "owned"}))
	if err != nil {
		t.Fatal(err)
	}
	editing := got.Msg.Project.GetEditing()
	if editing == nil || len(editing.GetPlan().GetCuts()) != 1 || editing.GetPlan().GetCuts()[0].GetId() != "cut" {
		t.Fatalf("② has nothing to read: %v", editing)
	}
	if copies := editing.GetPlan().GetCuts()[0].GetCopies(); len(copies) != 1 || copies[0].GetText() != "정확한 한글" {
		t.Fatalf("the captions the clip was made with are gone: %v", copies)
	}
	observations := got.Msg.Project.GetObservations()
	if observations.GetStatus() != "available" || len(observations.GetSources()) != 1 {
		t.Fatalf("the recorded observations are unreachable: %v", observations)
	}
	if got.Msg.Project.GetFinalizedAt() == "" || got.Msg.Project.GetFinalizedResultId() != "result" {
		t.Fatal("the project stopped reading as finalized")
	}

	// Reading is not editing: the writes a finalized project refuses are refused
	// the same way they were before it could be read (CLIP-76).
	_, err = h.SaveClipEditPlan(ctx, connect.NewRequest(&v1.SaveClipEditPlanRequest{
		ProjectId: "owned", ExpectedRevision: 3, Plan: editing.GetPlan()}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("a finalized plan accepted a save: %v", err)
	}
}
