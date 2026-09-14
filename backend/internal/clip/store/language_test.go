package store_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestProjectLanguageIsExplicitAndPersists(t *testing.T) {
	service, _, _ := setup(t)
	template, _ := create(t, service)
	for _, language := range []string{"", "ja", "ko", "en"} {
		p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "language", Language: language, VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000})
		if language == "" || language == "ja" {
			if !errors.Is(err, clip.ErrInvalid) {
				t.Fatalf("accepted missing/invalid language: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		got, err := service.GetProject(t.Context(), "alice", p.ID)
		if err != nil || got.Language != language {
			t.Fatalf("language lost: %+v %v", got, err)
		}
	}
}

func TestGenerationFreezesLanguageAndRetainsOptionalSceneRegions(t *testing.T) {
	h := generationSetup(t)
	if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET language='en' WHERE id=?`, h.project.ID); err != nil {
		t.Fatal(err)
	}
	h.planner.captionSafe = []clip.Region{{X: .1, Y: .1, Width: .8, Height: .2}}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	for _, language := range h.planner.languages {
		if language != "en" {
			t.Fatalf("lost frozen observation language: %s", language)
		}
	}
	if len(h.planner.languages) != 3 || h.planner.input.Language != "en" {
		t.Fatal("language did not reach generation")
	}
	state, err := h.store.GetRecovery(t.Context(), "alice", h.project.ID)
	if err != nil || state.Language != "en" || !reflect.DeepEqual(state.Chunks[0].Segments[0].CaptionSafe, h.planner.captionSafe) {
		t.Fatal("recovery lost language or regions", err)
	}
	checkpoint, err := h.store.GetAttemptCheckpoint(t.Context(), "alice", h.project.ID, h.planner.id)
	if err != nil || !reflect.DeepEqual(checkpoint.Observations[0].Segments[0].CaptionSafe, h.planner.captionSafe) {
		t.Fatal("checkpoint lost regions", err)
	}
	project, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	var analyses []clip.SourceAnalysis
	if err != nil || json.Unmarshal([]byte(project.Analysis), &analyses) != nil || !reflect.DeepEqual(analyses[0].Segments[0].CaptionSafe, h.planner.captionSafe) {
		t.Fatal("successful analysis lost regions", err)
	}
}
