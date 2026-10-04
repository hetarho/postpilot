package rpc

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// countDecodes counts how many times a read decodes a project's retained
// analysis until the test ends. Each decode also answers one source more than
// the project holds, so a projection that decoded the analysis itself instead
// of taking the read's decode shows by lacking it.
func countDecodes(t *testing.T) *int {
	t.Helper()
	n, decode := 0, decodeObservations
	decodeObservations = func(p clip.Project) ([]clip.SourceAnalysis, error) {
		n++
		analyses, err := decode(p)
		return append(analyses, clip.SourceAnalysis{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "decoded", Fingerprint: "decoded",
			Info: clip.MediaInfo{DurationMS: 1000, Width: 1920, Height: 1080}}, Filename: "decoded.mp4"}}), err
	}
	t.Cleanup(func() { decodeObservations = decode })
	return &n
}

func readProject(t *testing.T) clip.Project {
	t.Helper()
	analysis, err := json.Marshal([]clip.SourceAnalysis{{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fingerprint",
			Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}, Filename: "travel.mp4"},
		Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "음식을 촬영", Focal: clip.Point{X: .5, Y: .5}}, {StartMS: 15000, EndMS: 30000, Event: "가게 앞"}},
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
	return clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", Analysis: string(analysis), EditPlan: plan, EditPlanRevision: 2,
		Storyline: &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "음식을 먼저", ObservationIDs: []string{"source/0"}}}, MadeWithSources: []string{"source"}}}
}

// A detail read decodes the retained analysis once and builds the storyline
// once: the storyline, the observations and the editing state are all
// projected from that one decode.
func TestAProjectReadDecodesItsAnalysisOnce(t *testing.T) {
	service := testProjects(&observationStore{project: readProject(t)})
	generation := clipapp.NewGenerationService(nil, service, nil, neutralProcessing{}, nil, nil, nil, neutralJobs{},
		clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Minute, OrphanMinAge: time.Minute}, neutralGenerationDeps())
	h := NewHandler(service).WithGeneration(generation, nil)
	decodes := countDecodes(t)
	got, err := h.GetClipProject(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.GetClipProjectRequest{Id: "owned"}))
	if err != nil {
		t.Fatal(err)
	}
	if *decodes != 1 {
		t.Fatalf("one read decoded the analysis %d times", *decodes)
	}
	// All three projections carry what that one decode answered.
	p := got.Msg.Project
	if s := p.GetStoryline(); len(s.GetParagraphs()) != 1 || !reflect.DeepEqual(s.GetTakenOutObservationIds(), []string{"source/1"}) || !reflect.DeepEqual(s.GetAddedSourceIds(), []string{"decoded"}) {
		t.Fatalf("the storyline was not built from the read's decode: %v", s)
	}
	if o := p.GetObservations(); o.GetStatus() != "available" || len(o.GetSources()) != 2 || len(o.GetSources()[0].GetSegments()) != 2 {
		t.Fatalf("the observations were not projected from the read's decode: %v", o)
	}
	if e := p.GetEditing(); len(e.GetPlan().GetCuts()) != 1 || len(e.GetSources()) != 2 || e.GetSources()[1].GetId() != "decoded" {
		t.Fatalf("the editing state was not built from the read's decode: %v", e)
	}
}

// The directory reads no plan and no analysis: a row says whether the clip has
// a storyline, which is all the list shows of it, and leaves what changed
// since, the observations and the finalization verdict to the detail.
func TestTheDirectoryReadsNoPlanOrAnalysis(t *testing.T) {
	project := readProject(t)
	project.Result = &clip.Result{ID: "result", Key: "private-result-key"}
	project.RenderedPlanRevision = project.EditPlanRevision
	service := testProjects(&observationStore{project: project})
	generation := clipapp.NewGenerationService(nil, service, nil, neutralProcessing{}, nil, nil, nil, neutralJobs{},
		clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Minute, OrphanMinAge: time.Minute}, neutralGenerationDeps())
	h := NewHandler(service).WithGeneration(generation, nil)
	decodes := countDecodes(t)
	listed, err := h.ListClipProjects(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.ListClipProjectsRequest{}))
	if err != nil || len(listed.Msg.Projects) != 1 {
		t.Fatal(listed, err)
	}
	if *decodes != 0 {
		t.Fatalf("the directory decoded the analysis %d times", *decodes)
	}
	row := listed.Msg.Projects[0]
	if s := row.GetStoryline(); len(s.GetParagraphs()) != 1 || s.GetParagraphs()[0].GetText() != "음식을 먼저" || len(s.GetTakenOutObservationIds()) != 0 || len(s.GetAddedSourceIds()) != 0 {
		t.Fatalf("the row's storyline: %v", s)
	}
	if row.Observations != nil || row.Editing != nil || row.GetEditPlanRevision() != 2 || row.GetResult().GetId() != "result" {
		t.Fatalf("the row: %v", row)
	}
	if row.GetCanFinalize() || row.GetFinalizationRefusal() != "unavailable" || !row.GetCanEdit() {
		t.Fatalf("a row without its plan claimed a finalization verdict: %v %q", row.GetCanFinalize(), row.GetFinalizationRefusal())
	}
}
