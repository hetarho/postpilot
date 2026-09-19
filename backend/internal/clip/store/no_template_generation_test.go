package store_test

import (
	"encoding/json"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// noTemplateProject points the harness at a project minted from NO template: the
// same footage, and nothing a template would have supplied (CLIP-5).
func noTemplateProject(t *testing.T, h *generationHarness) clip.Project {
	t.Helper()
	p, err := h.projects.CreateProject(t.Context(), "alice", clip.ProjectInput{Language: "ko", Title: "템플릿 없이", Ratio: "vertical", TargetDurationMS: 30000, Disclosure: "sponsored"})
	if err != nil {
		t.Fatal(err)
	}
	h.project = p
	h.batch = rerenderBatch(t, h, false)
	return p
}

func TestAClipIsMintedGeneratedAndRerenderedWithNoTemplate(t *testing.T) {
	h := generationSetup(t)
	// A project with no template freezes a NON-legacy document, which only runs
	// where planning and rendering both understand the current plan.
	h.service = clip.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, layoutRenderer{h.renderer}, generationJobs{h.queue}, h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
	h.planner.portableFlow = true
	p := noTemplateProject(t, h)
	if p.VideoTemplateID != "" || p.Composition == nil || p.Composition.Snapshot.TemplateID != "" || p.Composition.Snapshot.Legacy {
		t.Fatal("minting without a template did not freeze the empty document", p.VideoTemplateID, p.Composition)
	}
	if p.Composition.Snapshot.Body != clip.EmptyCompositionBody() {
		t.Fatal("the frozen document is not the empty one", p.Composition.Snapshot.Body)
	}
	// Nothing is selected, and nothing selected IS the shared default (CLIP-139).
	if p.IntroPreset != "" || p.OutroPreset != "" || len(p.CaptionStyles) != 0 {
		t.Fatal("a project with no template invented a selection", p.IntroPreset, p.OutroPreset, p.CaptionStyles)
	}
	if presets := p.DesignSelection().RegionPresets(); presets.Intro != "b" || presets.Outro != "e" {
		t.Fatal("the unset selection did not resolve to the defaults", presets)
	}
	// Admission has no declared structure to satisfy and no field to require,
	// so the quote and the run go through untouched (CLIP-102).
	job := h.start(t)
	// What the job froze is what a restart resumes on: no recipe, and the empty
	// document in place of a template's.
	frozen, err := h.jobs.GetByID(t.Context(), job)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Version     int
		Template    clip.Recipe
		Composition *clip.ProjectComposition
	}
	if json.Unmarshal(frozen.Payload, &payload) != nil || payload.Composition == nil {
		t.Fatal("the frozen payload does not decode")
	}
	if payload.Template.Name != "" || payload.Template.CompositionBody != "" || payload.Composition.Snapshot.TemplateID != "" {
		t.Fatal("the payload froze a template that is not there", payload.Template, payload.Composition.Snapshot)
	}
	if err := h.run(t); err != nil {
		t.Fatal("a generation with no template failed", err)
	}
	planned, err := h.projects.GetProject(t.Context(), "alice", p.ID)
	if err != nil || planned.EditPlan == "" || planned.Result != nil {
		t.Fatal("the generation did not stop at a plan of its own", err)
	}
	if planned.VideoTemplateID != "" {
		t.Fatal("the generation attached a template", planned.VideoTemplateID)
	}
	// And the same project rerenders, still with none.
	batch := rerenderBatch(t, h, true)
	changed, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: stringOf("a")})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, batch.ID, changed.EditPlanRevision, clip.RenderServer); err != nil {
		t.Fatal("a rerender refused a project with no template", err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	latest, err := h.projects.GetProject(t.Context(), "alice", p.ID)
	if err != nil || latest.Result == nil || latest.EditPlanRevision != latest.RenderedPlanRevision || latest.VideoTemplateID != "" {
		t.Fatal("the rerender did not settle on a project with no template", latest.EditPlanRevision, latest.RenderedPlanRevision, err)
	}
}

func stringOf(s string) *string { return &s }

// Deleting a template detaches it and leaves the project everything it froze
// (CLIP-25): the revision rewrites that retained document, not a missing one.
func TestARevisionRunsAfterTheTemplateIsDeleted(t *testing.T) {
	h := revisionReady(t)
	if _, err := h.projects.DeleteTemplate(t.Context(), "alice", h.template.ID); err != nil {
		t.Fatal(err)
	}
	detached, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || detached.VideoTemplateID != "" {
		t.Fatal("the template was not detached", detached.VideoTemplateID, err)
	}
	if detached.Composition == nil || detached.Composition.Snapshot.TemplateID != h.template.ID {
		t.Fatal("the project lost the composition it froze", detached.Composition)
	}
	before := detached.EditPlanRevision
	startRevision(t, h, "자막을 더 짧게 써줘", clip.RevisionNarration)
	if err := h.run(t); err != nil {
		t.Fatal("a revision refused a project whose template is gone", err)
	}
	after, err := h.projects.GetProject(t.Context(), "alice", h.project.ID)
	if err != nil || after.EditPlanRevision != before+1 {
		t.Fatal("the revision did not advance the plan", after.EditPlanRevision, before, err)
	}
	if after.Composition == nil || after.Composition.Snapshot.TemplateID != h.template.ID {
		t.Fatal("the revision rewrote the retained document away", after.Composition)
	}
}
