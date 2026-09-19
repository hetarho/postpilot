package rpc

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	clipapp "github.com/postpilot/backend/internal/clip/app"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type observationStore struct {
	clip.Store
	clip.SourceStore
	project clip.Project
}

func (s *observationStore) GetProject(_ context.Context, user, id string) (clip.Project, error) {
	if user != s.project.UserID || id != s.project.ID {
		return clip.Project{}, clip.ErrNotFound
	}
	return s.project, nil
}
func (s *observationStore) ListProjectRequests(context.Context, string, string) ([]clip.ProjectRequest, error) {
	return nil, nil
}
func (s *observationStore) ListProjects(_ context.Context, user string) ([]clip.Project, error) {
	if user == s.project.UserID {
		return []clip.Project{s.project}, nil
	}
	return nil, nil
}

func TestRetainedObservationDetailIsOwnerScopedAndStructured(t *testing.T) {
	a := []clip.SourceAnalysis{{
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source-a", Fingerprint: "fingerprint-a",
			Info: clip.MediaInfo{DurationMS: 12000, Width: 1920, Height: 1080}}, Filename: "a.mp4"},
		Segments: []clip.Segment{{StartMS: 3500, EndMS: 10500, Event: "음식을 촬영", Action: "접시를 든다",
			Motion: "카메라가 다가간다", Subjects: []string{"접시"},
			Speech: "맛있어요", Quality: "선명함", Focal: clip.Point{X: .5, Y: .5}, Scene: "food", ReadableText: true,
			Certainty: clip.CertaintyUncertain, Usability: clip.UsabilityUsable},
			// A record stored before the v2 contract keeps its two empty status
			// fields all the way to the owner; nothing invents one for it.
			{StartMS: 10500, EndMS: 12000, Event: "테이블", Quality: "흔들림"}},
	}, {
		Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source-b", Fingerprint: "fingerprint-b",
			Info: clip.MediaInfo{DurationMS: 20000}}, Filename: "empty.mp4"},
	}}
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	s := &observationStore{project: clip.Project{ID: "owned", UserID: "alice", Analysis: string(raw),
		Ratio: "vertical", Result: &clip.Result{Key: "private-result-key", DownloadURL: "https://download.test/result"}}}
	h := NewHandler(testProjects(s))
	ctx := auth.WithUser(context.Background(), "alice")
	r, err := h.GetClipProject(ctx, connect.NewRequest(&v1.GetClipProjectRequest{Id: "owned"}))
	if err != nil {
		t.Fatal(err)
	}
	got := r.Msg.Project.Observations
	if got.GetStatus() != "available" || len(got.Sources) != 2 || got.Sources[0].Source.Filename != "a.mp4" || len(got.Sources[1].Segments) != 0 {
		t.Fatalf("sources: %v", got)
	}
	segment := got.Sources[0].Segments[0]
	if segment.StartMs != 3500 || segment.EndMs != 10500 || segment.Event != "음식을 촬영" || segment.Speech != "맛있어요" || segment.Quality != "선명함" || strings.Join(segment.Subjects, ",") != "접시" {
		t.Fatalf("evidence changed: %v", segment)
	}
	if segment.Action != "접시를 든다" || segment.Motion != "카메라가 다가간다" || segment.Certainty != "uncertain" || segment.Usability != "usable" {
		t.Fatalf("recorded action, motion or status lost: %v", segment)
	}
	if legacy := got.Sources[0].Segments[1]; legacy.Certainty != "" || legacy.Usability != "" || legacy.Action != "" || legacy.Motion != "" {
		t.Fatalf("a legacy record was given an invented status: %v", legacy)
	}
	wire, _ := protojson.Marshal(got)
	for _, forbidden := range []string{"private-result-key", "readableText", "https://", "analysisJson", "confidence"} {
		if strings.Contains(string(wire), forbidden) {
			t.Fatalf("private/unsupported detail %q", forbidden)
		}
	}
	for i, scene := range a[0].Segments {
		want := clip.ObservedCutFocal(scene)
		if got.Sources[0].Segments[i].GetFocal().GetX() != want.X || got.Sources[0].Segments[i].GetFocal().GetY() != want.Y {
			t.Fatal("owner add preview lost canonical focal", got.Sources[0].Segments[i])
		}
	}
	listed, err := h.ListClipProjects(ctx, connect.NewRequest(&v1.ListClipProjectsRequest{}))
	if err != nil || listed.Msg.Projects[0].Observations != nil {
		t.Fatalf("list acquired observations: %v %v", listed, err)
	}
	for _, tc := range []struct{ user, id string }{{"bob", "owned"}, {"alice", "foreign"}} {
		_, err := h.GetClipProject(auth.WithUser(context.Background(), tc.user), connect.NewRequest(&v1.GetClipProjectRequest{Id: tc.id}))
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Fatalf("foreign observation readable: %v", err)
		}
	}
}

func TestAbsentOrUnreadableObservationsPreserveTheResult(t *testing.T) {
	for _, tc := range []struct{ raw, status string }{
		{"", "empty"}, {"  ", "empty"}, {"null", "empty"}, {"[]", "empty"},
		{"broken JSON", "unavailable"}, {`[{"unexpected":"data"}]`, "unavailable"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			s := &observationStore{project: clip.Project{ID: "owned", UserID: "alice", Ratio: "vertical", Analysis: tc.raw,
				Result: &clip.Result{Key: "result"}}}
			service := testProjects(s)
			generation := clipapp.NewGenerationService(nil, service, nil, neutralProcessing{}, nil, nil, nil, neutralJobs{}, clip.GenerationConfig{ReadTTL: time.Minute, CleanupTimeout: time.Minute, OrphanMinAge: time.Minute}, neutralGenerationDeps())

			h := NewHandler(service).WithGeneration(generation, nil)
			if tc.status == "unavailable" {
				// Without the unavailable guard this attempts to decode the correction
				// and fails the whole detail read before it can return the video.
				s.project.EditPlan = "not a usable correction"
			}
			r, err := h.GetClipProject(auth.WithUser(context.Background(), "alice"), connect.NewRequest(&v1.GetClipProjectRequest{Id: "owned"}))
			if err != nil || r.Msg.Project.GetObservations().GetStatus() != tc.status || r.Msg.Project.GetResult().GetDownloadUrl() != "https://objects.test/result?download" {
				t.Fatalf("lost readable result: %v %v", r, err)
			}
		})
	}
}
