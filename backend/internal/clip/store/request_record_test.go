package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func requests(t *testing.T, h *generationHarness) []clip.ProjectRequest {
	t.Helper()
	p, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil {
		t.Fatal(err)
	}
	return p.Requests
}

// What the owner asked the AI for is kept with the project, verbatim and newest
// first (CLIP-133). The write point is ACCEPTANCE: a save writes nothing, so an
// instruction typed and never used leaves no trace of having been asked for.
func TestEveryAcceptedRequestIsKeptAndNoSaveWritesOne(t *testing.T) {
	h := generationSetup(t)
	h.planner.portableFlow = true
	written := "  고기 굽는 소리를 살려 주세요.  "
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Instruction: &written}); err != nil {
		t.Fatal(err)
	}
	if got := requests(t, h); len(got) != 0 {
		t.Fatal("a save recorded a request nothing asked for", got)
	}
	before := time.Now().Add(-time.Second)
	h.start(t)
	got := requests(t, h)
	if len(got) != 1 || got[0].Kind != clip.RequestInstruction {
		t.Fatal("an accepted generation did not record the instruction it froze", got)
	}
	// Verbatim: the surrounding spaces the owner typed are part of what was asked.
	if got[0].Body != written {
		t.Fatalf("the instruction was not kept verbatim: %q", got[0].Body)
	}
	if !got[0].CreatedAt.After(before) {
		t.Fatal("the entry carries no time", got[0].CreatedAt)
	}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if len(requests(t, h)) != 1 {
		t.Fatal("running the generation recorded a second entry")
	}
	startRevision(t, h, "자막을 줄여줘", clip.RevisionNarration)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	got = requests(t, h)
	if len(got) != 2 || got[0].Kind != clip.RevisionRequestKind(clip.RevisionNarration) || got[0].Body != "자막을 줄여줘" {
		t.Fatal("the revision's own words and target were not recorded newest first", got)
	}
	if got[1].Kind != clip.RequestInstruction {
		t.Fatal("the order is not newest first", got)
	}
	// A refused run keeps its entry: what was asked for is what happened.
	h.planner.narrateErr = errors.New("writer refused")
	startRevision(t, h, "다시 써줘", clip.RevisionNarration)
	if err := h.run(t); err == nil {
		t.Fatal("expected the injected writer failure")
	}
	if got = requests(t, h); len(got) != 3 || got[0].Body != "다시 써줘" {
		t.Fatal("a failed revision lost what it was asked for", got)
	}
	// And a save made afterwards still records nothing.
	cleared := ""
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Instruction: &cleared}); err != nil {
		t.Fatal(err)
	}
	if len(requests(t, h)) != 3 {
		t.Fatal("a save recorded a request")
	}
}

// A clip written without direction is a thing the record has to explain, so the
// entry exists and says the instruction was empty rather than being skipped.
func TestAGenerationWithNoInstructionRecordsThatItHadNone(t *testing.T) {
	h := generationSetup(t)
	h.start(t)
	got := requests(t, h)
	if len(got) != 1 || got[0].Kind != clip.RequestInstruction || got[0].Body != "" {
		t.Fatal("a generation with no instruction recorded nothing", got)
	}
}

// The record is the project's: it outlives the sources' retention and goes only
// when the project does (CLIP-24).
func TestDeletingTheProjectTakesItsRequestsWithIt(t *testing.T) {
	service, st, _ := setup(t)
	_, p := create(t, service)
	for _, r := range []clip.ProjectRequest{
		{Kind: clip.RequestInstruction, Body: "처음", CreatedAt: time.Now()},
		{Kind: clip.RevisionRequestKind(clip.RevisionBoth), Body: "다시", CreatedAt: time.Now()},
	} {
		if err := st.RecordProjectRequest(t.Context(), r.Body, "alice", p.ID, r); err != nil {
			t.Fatal(err)
		}
	}
	// A kind nobody declared is refused rather than stored.
	if err := st.RecordProjectRequest(t.Context(), "bad", "alice", p.ID, clip.ProjectRequest{Kind: "revision:everything", Body: "x", CreatedAt: time.Now()}); err == nil {
		t.Fatal("an undeclared kind was recorded")
	}
	if got, err := st.ListProjectRequests(t.Context(), "alice", p.ID); err != nil || len(got) != 2 {
		t.Fatal("the record did not read back", got, err)
	}
	if err := service.DeleteProject(t.Context(), "alice", p.ID); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListProjectRequests(t.Context(), "alice", p.ID)
	if err != nil || len(got) != 0 {
		t.Fatal("the record outlived the project", got, err)
	}
}
