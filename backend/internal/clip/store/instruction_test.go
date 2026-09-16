package store_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

// The instruction belongs to the project (CLIP-1, CLIP-121): it round-trips
// through create, update and read, is bounded where it is stored, and an empty
// one is indistinguishable from never having written one.
func TestProjectInstructionRoundTripsAndIsBounded(t *testing.T) {
	service, _, _ := setup(t)
	template, _ := create(t, service)
	limit := config.ClipInstructionChars
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "instruction", Language: "ko", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
	if err != nil {
		t.Fatal(err)
	}
	if p.Instruction != "" {
		t.Fatal("a project written with no instruction carries one", p.Instruction)
	}
	written := "고기 굽는 소리를 살려 주세요."
	got, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Instruction: &written})
	if err != nil || got.Instruction != written {
		t.Fatalf("instruction lost on update: %+v %v", got, err)
	}
	if reread, e := service.GetProject(t.Context(), "alice", p.ID); e != nil || reread.Instruction != written {
		t.Fatalf("instruction lost on read: %+v %v", reread, e)
	}
	cleared := ""
	if got, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Instruction: &cleared}); err != nil || got.Instruction != "" {
		t.Fatalf("instruction could not be cleared: %+v %v", got, err)
	}
	// Exactly at the bound is stored; one character past it is refused, and the
	// refusal changes nothing.
	at := strings.Repeat("가", limit)
	if got, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Instruction: &at}); err != nil || got.Instruction != at {
		t.Fatalf("an instruction at its bound was refused: %v", err)
	}
	over := at + "가"
	if _, err = service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Instruction: &over}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatalf("an over-long instruction was stored: %v", err)
	}
	if reread, e := service.GetProject(t.Context(), "alice", p.ID); e != nil || reread.Instruction != at {
		t.Fatal("the refused instruction changed the stored one", e)
	}
	if _, err = service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "too long", Language: "ko", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000, Instruction: over}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatalf("an over-long instruction was created: %v", err)
	}
}

// The instruction is frozen with the answers and the composition, so editing it
// after the attempt is enqueued changes nothing in flight (CLIP-69).
func TestGenerationFreezesTheInstruction(t *testing.T) {
	h := generationSetup(t)
	frozen := "촬영한 장면만 설명해 주세요."
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Instruction: &frozen}); err != nil {
		t.Fatal(err)
	}
	id := h.start(t)
	j, err := h.jobs.GetByID(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot struct{ Instruction string }
	if json.Unmarshal(j.Payload, &snapshot) != nil || snapshot.Instruction != frozen {
		t.Fatalf("the instruction was not frozen: %s", string(j.Payload))
	}
	later := "완전히 다른 지시"
	if _, err = h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{Instruction: &later}); !errors.Is(err, clip.ErrBusy) {
		t.Fatalf("a project was edited mid-flight: %v", err)
	}
	if snapshot.Instruction != frozen {
		t.Fatal("the frozen payload followed a later edit")
	}
}

// A payload written before the field existed decodes as no instruction, which
// is exactly today's behaviour.
func TestPayloadWrittenBeforeTheInstructionDecodesAsNone(t *testing.T) {
	var snapshot struct{ Instruction string }
	if json.Unmarshal([]byte(`{"Version":5,"ProjectID":"project","Answers":[]}`), &snapshot) != nil || snapshot.Instruction != "" {
		t.Fatal("an older payload gained an instruction")
	}
}
