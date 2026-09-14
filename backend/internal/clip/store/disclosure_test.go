package store_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestDisclosureVisibilityPersistenceAndRenderRevision(t *testing.T) {
	service, store, db := setup(t)
	template, p := create(t, service)
	if p.HideDisclosure {
		t.Fatal("legacy default hidden")
	}
	// An existing result remains available while its render is marked stale.
	if _, err := db.Writer.Exec(`UPDATE clip_projects SET edit_plan_json='{}', edit_plan_revision=1, rendered_plan_revision=1 WHERE id=?`, p.ID); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		hidden   bool
		revision int
	}{{true, 2}, {true, 2}, {false, 3}} {
		updated, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{HideDisclosure: &tc.hidden})
		if err != nil || updated.HideDisclosure != tc.hidden || updated.EditPlanRevision != tc.revision || updated.RenderedPlanRevision != 1 || updated.Disclosure != "sponsored" {
			t.Fatal(updated, err)
		}
		got, err := store.GetProject(t.Context(), "alice", p.ID)
		if err != nil || got.HideDisclosure != tc.hidden {
			t.Fatal(got, err)
		}
	}
	hidden := true
	_, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{HideDisclosure: &hidden})
	if err != nil {
		t.Fatal(err)
	}
	name := "renamed"
	got, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Title: &name})
	if err != nil || !got.HideDisclosure {
		t.Fatal("older update reset flag", got, err)
	}
	created, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "hidden", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000, Disclosure: "sponsored", HideDisclosure: true})
	if err != nil || !created.HideDisclosure {
		t.Fatal(created, err)
	}
	got, err = store.GetProject(t.Context(), "alice", created.ID)
	if err != nil || !got.HideDisclosure {
		t.Fatal(got, err)
	}
}

func TestDisclosureChoiceReachesRerenderAndCannotChangeWhileRunning(t *testing.T) {
	h, old, _ := completedClip(t)
	hidden := true
	changed, err := h.projects.UpdateProject(t.Context(), "alice", old.ID, clip.ProjectPatch{HideDisclosure: &hidden})
	if err != nil || changed.EditPlanRevision != old.EditPlanRevision+1 {
		t.Fatal(changed, err)
	}
	if changed.Result.Key != old.Result.Key {
		t.Fatal("previous result lost")
	}
	batch := rerenderBatch(t, h, true)
	if _, err = h.service.StartRender(t.Context(), "alice", old.ID, batch.ID, changed.EditPlanRevision); err != nil {
		t.Fatal(err)
	}
	hidden = false
	if _, err := h.store.UpdateProject(t.Context(), "alice", old.ID, clip.ProjectPatch{HideDisclosure: &hidden}, changed.UpdatedAt); !errors.Is(err, clip.ErrBusy) {
		t.Fatal("active visibility changed", err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	if !h.renderer.plan.HideDisclosure || h.renderer.plan.Disclosure != "sponsored" {
		t.Fatal(h.renderer.plan)
	}
	latest, err := h.projects.GetProject(t.Context(), "alice", old.ID)
	if err != nil || latest.EditPlanRevision != latest.RenderedPlanRevision {
		t.Fatal(latest, err)
	}
}

func TestDisclosureChoiceReachesApprovedGeneration(t *testing.T) {
	h := generationSetup(t)
	hidden := true
	if _, err := h.projects.UpdateProject(t.Context(), "alice", h.project.ID, clip.ProjectPatch{HideDisclosure: &hidden}); err != nil {
		t.Fatal(err)
	}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if !h.planner.input.HideDisclosure || !h.renderer.plan.HideDisclosure {
		t.Fatal("generation lost visibility")
	}
	h.assertClean(t)
}

// The browser sends the whole draft on every save, so a composition project's
// empty disclosure comes back on the patch that carries its inputs. Refusing it
// froze those projects at creation (review/clip-project-update-260914 F1).
func TestCompositionProjectSavesItsEmptyDisclosure(t *testing.T) {
	service, _, _ := setup(t)
	template, err := service.CreateTemplate(t.Context(), "alice", clip.Recipe{Name: "composition", CompositionBody: nativeBody})
	if err != nil {
		t.Fatal(err)
	}
	inputs := clip.CompositionInputs{Values: map[string]string{"b": "18,000원"}}
	p, err := service.CreateProject(t.Context(), "alice", clip.ProjectInput{Title: "draft", VideoTemplateID: template.ID, Ratio: "vertical", TargetDurationMS: 15000, CompositionInputs: &inputs})
	if err != nil || p.Disclosure != "" {
		t.Fatal(p.Disclosure, err)
	}
	empty := ""
	inputs.Items = map[string][]composition.Item{"menu": {{ID: "dish-a", Values: map[string]string{"price": "9,000원"}}}}
	saved, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Disclosure: &empty, CompositionInputs: &inputs})
	if err != nil || saved.Disclosure != "" || len(saved.Composition.Inputs.Items["menu"]) != 1 {
		t.Fatal(saved, err)
	}
	unsupported := "nope"
	if _, err := service.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{Disclosure: &unsupported}); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("unsupported disclosure admitted", err)
	}
}
