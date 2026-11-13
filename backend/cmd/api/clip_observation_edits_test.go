package main

import (
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
	"github.com/postpilot/backend/internal/clip"
	cliprpc "github.com/postpilot/backend/internal/clip/rpc"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// CLIP-235, CLIP-160: through the wired handler and store, the owner's correction is saved and
// read back marked by hand, another account cannot reach it, and finalization ends it while
// the finalized project still reads the corrected observations.
func TestClipObservationEditsThroughTheWiredHandler(t *testing.T) {
	h := newFinalizationHarness(t)
	rpc := cliprpc.NewHandler(h.service).WithSources(h.sources).WithGeneration(h.generation, h.queue)
	current, err := h.clips.GetProject(t.Context(), "alice", "clip")
	if err != nil {
		t.Fatal(err)
	}
	analyses, err := clip.RetainedObservations(current)
	if err != nil || len(analyses) == 0 {
		t.Fatal("the harness project holds no observations", err)
	}
	// The harness records its source without scenes; give it the one an observer would.
	analyses[0].Segments = []clip.Segment{{EndMS: 16000, Event: "scene", Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}
	raw, err := json.Marshal(analyses)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.clips.SaveAnalysis(t.Context(), "alice", "clip", string(raw), h.now); err != nil {
		t.Fatal(err)
	}
	event := "주인이 고친 장면"
	req := connect.NewRequest(&v1.SaveClipObservationEditsRequest{ClipId: "clip", Edits: []*v1.ClipObservationEdit{{ObservationId: clip.ObservationID(analyses[0].Source.ID, 0), Event: &event}}})
	if _, err := rpc.SaveClipObservationEdits(auth.WithUser(t.Context(), "bob"), req); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatal("another account reached the observations", err)
	}
	owner := auth.WithUser(t.Context(), "alice")
	saved, err := rpc.SaveClipObservationEdits(owner, req)
	if err != nil {
		t.Fatal(err)
	}
	if source := saved.Msg.Observations.GetSources()[0]; !source.OwnerEdited || !source.Segments[0].OwnerEdited || source.Segments[0].Event != event {
		t.Fatal("the saved correction is not projected", source)
	}
	read, err := rpc.GetClipProject(owner, connect.NewRequest(&v1.GetClipProjectRequest{Id: "clip"}))
	if err != nil {
		t.Fatal(err)
	}
	if source := read.Msg.Project.Observations.GetSources()[0]; !source.OwnerEdited || source.Segments[0].Event != event {
		t.Fatal("the project read lost the correction", source)
	}
	if _, err := h.service.FinalizeProject(t.Context(), h.request()); err != nil {
		t.Fatal(err)
	}
	if _, err := rpc.SaveClipObservationEdits(owner, req); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatal("a finalized project took a correction", err)
	}
	read, err = rpc.GetClipProject(owner, connect.NewRequest(&v1.GetClipProjectRequest{Id: "clip"}))
	if err != nil || !read.Msg.Project.Observations.GetSources()[0].OwnerEdited {
		t.Fatal("the finalized project does not read the corrected observations", err)
	}
}
