package store_test

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	clipapp "github.com/postpilot/backend/internal/clip/app"
	"github.com/postpilot/backend/internal/clip/composition"
)

// T456, the whole edit-to-output path of a clip with no template (CLIP-5, CLIP-187, CLIP-188,
// CLIP-189, CLIP-191). The owner turns the intro on and writes one slot, 스토리라인 먼저 drafts
// the other and the build from the storyline draws both; the outro goes on with the owner's
// words; one caption takes an owner style outside the AI set and a correction retypes an intro
// row, which is the slot. Both render kinds start from exactly that plan. A preset that shrinks
// keeps the words past it as unused with their notice and growing it draws them again; off and
// on again keeps every word. Nothing past the approved calls asks a model.
func TestAnEditedClipWithNoTemplateReachesBothRenderKindsAsEdited(t *testing.T) {
	h := generationSetup(t)
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, layoutRenderer{h.renderer}, h.clipJobs(), h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
	noTemplateProject(t, h)
	withIntro(t, h)
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	runStoryline(t, h)
	q, err := h.service.QuoteFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.service.StartFromStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p := project(t, h)
	if intro := planRegion(t, p, "hook"); len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "성수동"}) {
		t.Fatal("the build did not draw the owner's word and the draft", intro)
	}
	calls := func() int {
		return h.planner.observe + h.planner.plans + h.planner.flows + h.planner.narrations + len(h.planner.storylines) + len(h.planner.revisions)
	}
	approved := calls()

	on, bye := true, "다시 만나요"
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.OutroRegion = &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{{ID: "project-outro-1", Text: &bye}}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	state, err := h.service.EditingState(p)
	if err != nil {
		t.Fatal(err)
	}
	draft := state.Plan
	styled := ""
	for i, text := range draft.Elements {
		switch {
		case text.Role == "caption" && styled == "":
			draft.Elements[i].Owner.Style, styled = "neon", text.InstanceID
			// This owner's caption must leave the enabled intro and outro to
			// their region words; the stored writer interval predates that rule.
			start, end := 3000, 6000
			draft.Elements[i].Basis = "output-start"
			draft.Elements[i].StartMS, draft.Elements[i].EndMS = &start, &end
		case text.InstanceID == "project-intro":
			draft.Elements[i].Rows[1].Text = "연남동"
		}
	}
	if styled == "" {
		t.Fatal("the fixture's plan has no caption to style")
	}
	p, err = h.service.SaveCorrection(t.Context(), "alice", p.ID, p.EditPlanRevision, draft)
	if err != nil {
		t.Fatal("the correction was refused:", err)
	}
	if slot := p.Regions.Intro.Slots[1]; slot.Text != "연남동" || !slot.OwnerFixed {
		t.Fatal("the corrected row is not the slot", slot)
	}

	// The server render is given exactly that plan.
	batch := rerenderBatch(t, h, false)
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, batch.ID, p.EditPlanRevision, clip.RenderServer); err != nil {
		t.Fatal(err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	drawn := func(plan clip.EditPlan, role string) []string {
		for _, text := range plan.Portable.Elements {
			if text.Resolved.Element.Role == role {
				return regionRows(text)
			}
		}
		return nil
	}
	rendered := h.renderer.plan
	if !slices.Equal(drawn(rendered, "hook"), []string{"골목 저녁", "연남동"}) || !slices.Equal(drawn(rendered, "ending"), []string{"다시 만나요", ""}) {
		t.Fatal("the server render did not draw the slots as edited", drawn(rendered, "hook"), drawn(rendered, "ending"))
	}
	owner := ""
	for _, text := range rendered.Portable.Elements {
		if text.Resolved.InstanceID == styled {
			owner = text.Owner.Style
		}
	}
	if owner != "neon" {
		t.Fatal("the server render lost the owner's style", owner)
	}
	// The browser render starts from the same saved revision.
	p = project(t, h)
	if p.RenderedPlanRevision != p.EditPlanRevision {
		t.Fatal("the server render did not settle on the edited revision")
	}
	browser := rerenderBatch(t, h, false)
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, browser.ID, p.EditPlanRevision, clip.RenderBrowser); err != nil {
		t.Fatal("a browser render of the edited plan was refused:", err)
	}

	// A preset with room for a third slot, a word in it, then back to two: the word stays as
	// unused with its notice, and the wider preset draws it again.
	cover, a, evening := "cover", "a", "저녁 영업"
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: &cover})
	if err != nil {
		t.Fatal(err)
	}
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.IntroRegion = &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-3", Text: &evening}}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: &a})
	if err != nil {
		t.Fatal(err)
	}
	if intro := planRegion(t, p, "hook"); len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "연남동"}) {
		t.Fatal("the narrower preset did not draw its own two slots", intro)
	}
	if p.Regions.Intro.Slots[2].Text != evening || !slices.ContainsFunc(clip.ProjectNotices(p), func(n clip.PlanNotice) bool {
		return n.Reason == clip.NoticeRegionLineSurplus && n.ElementID == "project-intro-3"
	}) {
		t.Fatal("the unused word was lost or not reported", p.Regions.Intro.Slots, clip.ProjectNotices(p))
	}
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroPreset: &cover})
	if err != nil {
		t.Fatal(err)
	}
	if intro := planRegion(t, p, "hook"); len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "연남동", evening}) {
		t.Fatal("growing the preset did not draw the kept word", intro)
	}

	// Off and on again: nothing drawn, then every word where it was.
	off := false
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.IntroRegion = &clip.RegionPatch{Enabled: &off}
	}))
	if err != nil || len(planRegion(t, p, "hook")) != 0 {
		t.Fatal("a region turned off still draws", err)
	}
	p, err = h.projects.UpdateProject(t.Context(), "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.IntroRegion = &clip.RegionPatch{Enabled: &on}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if intro := planRegion(t, p, "hook"); len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "연남동", evening}) {
		t.Fatal("turning the region back on lost its words", intro)
	}
	if calls() != approved {
		t.Fatal("an edit, a render or a preset asked a model", calls(), approved)
	}
}

// storedRow is what opening a project may never change: its plan, its revisions, its result and
// whether its regions were ever recorded.
func storedRow(t *testing.T, h *generationHarness, id string) [5]string {
	t.Helper()
	var plan, regions, result, edit, rendered string
	err := h.db.Reader.QueryRow(`SELECT coalesce(edit_plan_json,''), coalesce(regions_json,'NULL'), coalesce(result_key,''), edit_plan_revision, rendered_plan_revision FROM clip_projects WHERE id=?`, id).Scan(&plan, &regions, &result, &edit, &rendered)
	if err != nil {
		t.Fatal(err)
	}
	return [5]string{plan, regions, result, edit, rendered}
}

// T456, CLIP-190: opening a project saved before its slots were recorded changes nothing it
// confirmed and asks no model. Presets stored without region elements enable no region; the
// words a plan already draws for a region are the slots it is read with; a finalized clip whose
// originals are gone reads the same and stays unwritable.
func TestOpeningOlderProjectsChangesNothingTheyConfirmed(t *testing.T) {
	calls := func(h *generationHarness) int {
		return h.planner.observe + h.planner.plans + h.planner.flows + h.planner.narrations + len(h.planner.storylines) + len(h.planner.revisions)
	}
	open := func(t *testing.T, h *generationHarness, id string) clip.Project {
		t.Helper()
		before, asked := storedRow(t, h, id), calls(h)
		var read clip.Project
		for range 2 {
			p, err := h.projects.GetProject(t.Context(), "alice", id)
			if err != nil {
				t.Fatal(err)
			}
			read = p
		}
		if storedRow(t, h, id) != before || calls(h) != asked {
			t.Fatal("opening the project changed it or asked a model")
		}
		return read
	}
	t.Run("presets without region elements", func(t *testing.T) {
		h, p, _ := completedNativeClip(t)
		if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET regions_json=NULL, intro_preset='cover', outro_preset='e' WHERE id=?`, p.ID); err != nil {
			t.Fatal(err)
		}
		read := open(t, h, p.ID)
		if read.Regions.Intro.Enabled || read.Regions.Outro.Enabled || len(planRegion(t, read, "hook")) != 0 {
			t.Fatal("a stored preset enabled a region", read.Regions)
		}
	})
	t.Run("saved generated regions", func(t *testing.T) {
		h, p, _ := completedNativeClip(t)
		plan, err := clip.DecodeEditPlan(p.EditPlan)
		if err != nil {
			t.Fatal(err)
		}
		a, b := 0, 2500
		hook := composition.Element{ID: "hello", Kind: "ai", Role: "hook", Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &a, EndMS: &b}
		portable := *plan.Portable
		portable.Elements = append(slices.Clone(portable.Elements), clip.PortableText{Resolved: composition.ResolvedElement{InstanceID: "hello", Element: hook, StartMS: 0, EndMS: 2500, Rows: []composition.ResolvedRow{{Text: "쓴 말"}}}})
		plan.Portable = &portable
		raw, err := clip.EncodeEditPlan(plan)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=?, regions_json=NULL WHERE id=?`, raw, p.ID); err != nil {
			t.Fatal(err)
		}
		read := open(t, h, p.ID)
		if slot := read.Regions.Intro.Slots[0]; !read.Regions.Intro.Enabled || slot.Text != "쓴 말" || slot.OwnerFixed || read.Regions.Outro.Enabled {
			t.Fatal("the plan's own region words were not the slots it reads with", read.Regions)
		}
		if intro := planRegion(t, read, "hook"); len(intro) != 1 || intro[0].Resolved.InstanceID != "hello" {
			t.Fatal("opening re-projected the plan", intro)
		}
	})
	t.Run("finalized without originals", func(t *testing.T) {
		h, p, _ := completedNativeClip(t)
		const at = "2026-09-28T00:00:00Z"
		if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET regions_json=NULL WHERE id=?`, p.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET result_id='result',result_key='result-key',result_content_type='video/mp4',result_bytes=1,result_duration_ms=15000,result_created_at=?,rendered_plan_revision=edit_plan_revision,finalized_at=?,finalized_plan_revision=edit_plan_revision,finalized_result_key='result-key',source_access_revoked_at=? WHERE id=?`, at, at, at, p.ID); err != nil {
			t.Fatal(err)
		}
		read := open(t, h, p.ID)
		if read.Finalized == nil || read.Regions.Intro.Enabled || read.Regions.Outro.Enabled {
			t.Fatal("the finalized clip did not read as it was confirmed", read.Finalized, read.Regions)
		}
		on := true
		if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, clip.ProjectPatch{IntroRegion: &clip.RegionPatch{Enabled: &on}}); err == nil {
			t.Fatal("a finalized clip took a region edit")
		}
	})
}

// T456 on the other approved path (CLIP-5, CLIP-187): 바로 만들기 on a clip with no template
// drafts the generated slot in its flow call, draws it beside the owner's word, and leaves an
// outro that is on with nothing in it unresolved — drawn by nothing, still on — through to the
// server render.
func TestADirectGenerationWithNoTemplateDrawsWhatItDrafted(t *testing.T) {
	h := generationSetup(t)
	h.service = clipapp.NewGenerationService(h.store, h.projects, h.sources, h.objects, h.media, compositionPlanner{h.planner}, layoutRenderer{h.renderer}, h.clipJobs(), h.cfg, generationDeps(generationFinisher{h.store}, &quotePricing{}, nil))
	noTemplateProject(t, h)
	p := withIntro(t, h)
	on := true
	if _, err := h.projects.UpdateProject(t.Context(), "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.OutroRegion = &clip.RegionPatch{Enabled: &on}
	})); err != nil {
		t.Fatal(err)
	}
	h.planner.storyline = writtenStoryline()
	h.planner.regionDrafts = []clip.RegionDraft{{SlotID: "project-intro-2", Text: "성수동"}}
	h.start(t)
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	p = project(t, h)
	if intro := planRegion(t, p, "hook"); len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"골목 저녁", "성수동"}) {
		t.Fatal("the direct path did not draw the owner's word and its draft", intro)
	}
	if !p.Regions.Outro.Enabled || len(planRegion(t, p, "ending")) != 0 {
		t.Fatal("an outro with nothing in it was drawn or switched off", p.Regions.Outro, planRegion(t, p, "ending"))
	}
	batch := rerenderBatch(t, h, false)
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, batch.ID, p.EditPlanRevision, clip.RenderServer); err != nil {
		t.Fatal(err)
	}
	if err := runRender(t, h); err != nil {
		t.Fatal(err)
	}
	for _, text := range h.renderer.plan.Portable.Elements {
		if text.Resolved.Element.Role == "ending" {
			t.Fatal("the render drew an outro with nothing in it")
		}
		if text.Resolved.Element.Role == "hook" && !slices.Equal(regionRows(text), []string{"골목 저녁", "성수동"}) {
			t.Fatal("the render drew other intro words", regionRows(text))
		}
	}
}
