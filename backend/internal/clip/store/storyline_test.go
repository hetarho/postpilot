package store_test

import (
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func writtenStoryline() *clip.Storyline {
	return &clip.Storyline{Paragraphs: []clip.StorylineParagraph{{Text: "가게 앞에서 시작해요.", ObservationIDs: []string{"s/0"}}, {Text: "마무리"}}}
}

func analysedSources(t *testing.T, p clip.Project) []string {
	t.Helper()
	analyses, err := clip.RetainedObservations(p)
	if err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, a := range analyses {
		out = append(out, a.Source.ID)
	}
	return out
}

// CLIP-178: the storyline the flow opened with reaches the narration and is saved with the plan
// in the same write — not edited by hand, made with the sources this generation analysed.
func TestTheStorylineIsSavedWithThePlan(t *testing.T) {
	h := generationSetup(t)
	h.planner.storyline = writtenStoryline()
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if len(h.planner.narrated) != 1 || !reflect.DeepEqual(h.planner.narrated[0], writtenStoryline()) {
		t.Fatal("the narration was not written along the storyline", h.planner.narrated)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.Storyline == nil || !reflect.DeepEqual(p.Storyline.Paragraphs, writtenStoryline().Paragraphs) || p.Storyline.EditedByHand {
		t.Fatalf("the storyline was not saved as written: %+v", p.Storyline)
	}
	if want := analysedSources(t, p); len(want) == 0 || !reflect.DeepEqual(p.Storyline.MadeWithSources, want) {
		t.Fatalf("made with %v, want the analysed %v", p.Storyline.MadeWithSources, want)
	}
}

// A flow that wrote no storyline leaves the stored one where it is.
func TestAGenerationWithoutAStorylineKeepsTheStoredOne(t *testing.T) {
	h := generationSetup(t)
	h.planner.storyline = writtenStoryline()
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.planner.storyline = nil
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || p.Storyline == nil || !reflect.DeepEqual(p.Storyline.Paragraphs, writtenStoryline().Paragraphs) {
		t.Fatal("the stored storyline was lost", p.Storyline, err)
	}
}

// A continuation that resumes on the kept flow keeps the storyline it opened with: the
// recovery carries it, so the narration still writes along it and the save still stores it.
func TestAContinuationKeepsTheStorylineOfTheKeptFlow(t *testing.T) {
	h := generationSetup(t)
	h.planner.storyline = writtenStoryline()
	h.planner.narrateErr = llm.ErrBadOutput
	h.start(t)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected narration failure")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state == nil || !state.FlowReady || !reflect.DeepEqual(state.Storyline, writtenStoryline()) {
		t.Fatal("the recovery did not carry the storyline", state, err)
	}
	h.planner.narrateErr, h.planner.storyline = nil, nil
	flows := h.planner.flows
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.flows != flows {
		t.Fatal("the continuation rewrote the flow")
	}
	if last := h.planner.narrated[len(h.planner.narrated)-1]; !reflect.DeepEqual(last, writtenStoryline()) {
		t.Fatal("the resumed narration lost the storyline", last)
	}
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || p.Storyline == nil || !reflect.DeepEqual(p.Storyline.Paragraphs, writtenStoryline().Paragraphs) {
		t.Fatal("the continuation did not save the storyline", p.Storyline, err)
	}
}

// CLIP-131: a revision's rewrite writes no storyline and leaves the stored one exactly as it is.
func TestARevisionLeavesTheStoredStoryline(t *testing.T) {
	h := generationSetup(t)
	h.planner.storyline = writtenStoryline()
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	h.render(t)
	before, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	startRevision(t, h, "고기 장면을 먼저", clip.RevisionFlow)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.EditPlanRevision == before.EditPlanRevision || !reflect.DeepEqual(after.Storyline, before.Storyline) {
		t.Fatal("a revision changed the stored storyline", before.Storyline, after.Storyline)
	}
}
