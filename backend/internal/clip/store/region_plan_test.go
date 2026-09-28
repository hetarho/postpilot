package store_test

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func planRegion(t *testing.T, p clip.Project, role string) []clip.PortableText {
	t.Helper()
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	var out []clip.PortableText
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.Role == role {
			out = append(out, text)
		}
	}
	return out
}

func regionRows(text clip.PortableText) []string {
	var out []string
	for _, row := range text.Resolved.Rows {
		out = append(out, row.Text)
	}
	return out
}

func regionEdit(p clip.Project, patch func(*clip.ProjectPatch)) clip.ProjectPatch {
	revision := p.Regions.Revision
	out := clip.ProjectPatch{ExpectedRegionRevision: &revision}
	patch(&out)
	return out
}

func slotText(id, text string) clip.RegionSlotPatch { return clip.RegionSlotPatch{ID: id, Text: &text} }

// A project whose template declares no region gets one in its existing plan
// the moment it is enabled with words, in that same write: the footage and the
// output stay, the plan revision moves so the render goes stale, and nothing
// else is rewritten (CLIP-66, CLIP-139, CLIP-188).
func TestRegionEditsReachTheExistingPlanInTheSameWrite(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	ctx := t.Context()
	if before.Result == nil || before.RenderedPlanRevision != before.EditPlanRevision || len(planRegion(t, before, "hook")) != 0 {
		t.Fatal("the fixture needs a rendered plan without regions")
	}
	on := true
	enabled, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(before, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{slotText("project-intro-1", "성수 골목")}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	intro := planRegion(t, enabled, "hook")
	if len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"성수 골목", ""}) || intro[0].Resolved.StartMS != 0 || intro[0].Resolved.EndMS != 2500 {
		t.Fatal("the enabled intro is not in the plan", intro)
	}
	if enabled.EditPlanRevision != before.EditPlanRevision+1 || enabled.RenderedPlanRevision != before.RenderedPlanRevision || enabled.Regions.Revision != before.Regions.Revision+1 {
		t.Fatal("the region edit did not stale the render in the same write", enabled.EditPlanRevision, enabled.RenderedPlanRevision, enabled.Regions.Revision)
	}
	old, _ := clip.DecodeEditPlan(before.EditPlan)
	now, _ := clip.DecodeEditPlan(enabled.EditPlan)
	if !reflect.DeepEqual(old.Cuts, now.Cuts) || old.DurationMS != now.DurationMS || !reflect.DeepEqual(old.Portable.Cuts, now.Portable.Cuts) || enabled.Analysis != before.Analysis {
		t.Fatal("a region edit rebuilt the footage")
	}

	// An instruction is for the next generation: the plan and its revision
	// stay, while the slot draft moves on.
	instruction := "동네 이름"
	instructed, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(enabled, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-2", Instruction: &instruction}}}
	}))
	if err != nil || instructed.EditPlan != enabled.EditPlan || instructed.EditPlanRevision != enabled.EditPlanRevision || instructed.Regions.Revision != enabled.Regions.Revision+1 {
		t.Fatal("an instruction rewrote the plan", err)
	}
	// The same words again change nothing at all.
	same, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(instructed, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Slots: []clip.RegionSlotPatch{slotText("project-intro-1", "성수 골목")}}
	}))
	if err != nil || same.EditPlan != instructed.EditPlan || same.EditPlanRevision != instructed.EditPlanRevision || same.Regions.Revision != instructed.Regions.Revision {
		t.Fatal("a semantic no-op moved a revision", err)
	}

	// A larger preset draws its extra slot empty and keeps the owner's words.
	cover := "cover"
	grown, err := h.projects.UpdateProject(ctx, "alice", before.ID, clip.ProjectPatch{IntroPreset: &cover})
	if err != nil || !slices.Equal(regionRows(planRegion(t, grown, "hook")[0]), []string{"성수 골목", "", ""}) || grown.EditPlanRevision <= same.EditPlanRevision {
		t.Fatal("the preset change did not reach the plan", err)
	}
	// Switching off draws nothing, keeps the draft and stales the render again.
	off := false
	switched, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(grown, func(p *clip.ProjectPatch) { p.IntroRegion = &clip.RegionPatch{Enabled: &off} }))
	if err != nil || len(planRegion(t, switched, "hook")) != 0 || switched.Regions.Intro.Slots[0].Text != "성수 골목" || switched.EditPlanRevision != grown.EditPlanRevision+1 {
		t.Fatal("switching the intro off", err)
	}
}

// A region edit made against another revision of the regions is refused, and
// an owner's words too wide for their slot refuse the whole write: nothing of
// it — the enablement, the words or the plan — is stored (CDS-64, CLIP-188).
func TestRegionWritesRefuseStaleAndUnfittingEditsWholesale(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	ctx := t.Context()
	on := true
	stale := before.Regions.Revision + 1
	if _, err := h.projects.UpdateProject(ctx, "alice", before.ID, clip.ProjectPatch{ExpectedRegionRevision: &stale, IntroRegion: &clip.RegionPatch{Enabled: &on}}); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("a stale region write was accepted", err)
	}
	wide := strings.Repeat("하나둘셋넷", 12)
	_, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(before, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{slotText("project-intro-1", wide)}}
	}))
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "copy_limit" {
		t.Fatal("the unfitting slot was not named", err)
	}
	after, err := h.projects.GetProject(ctx, "alice", before.ID)
	if err != nil || after.EditPlan != before.EditPlan || after.EditPlanRevision != before.EditPlanRevision || !reflect.DeepEqual(after.Regions, before.Regions) {
		t.Fatal("a refused region write left part of itself behind", err)
	}
	// A preset that cannot hold the owner's words is refused the same way, and
	// the preset stays where it was.
	cover := "cover"
	if _, err := h.projects.UpdateProject(ctx, "alice", before.ID, clip.ProjectPatch{IntroPreset: &cover}); err != nil {
		t.Fatal(err)
	}
	long, err := h.projects.GetProject(ctx, "alice", before.ID)
	if err != nil {
		t.Fatal(err)
	}
	fits := strings.Repeat("하나둘셋넷", 5)
	long, err = h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(long, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Slots: []clip.RegionSlotPatch{slotText("project-intro-1", fits)}}
	}))
	if err != nil {
		t.Fatal("the words the cover's first slot holds were refused", err)
	}
	narrow := "a"
	if _, err := h.projects.UpdateProject(ctx, "alice", before.ID, clip.ProjectPatch{IntroPreset: &narrow}); !errors.As(err, &problem) || problem.ElementID != "project-intro-1" || problem.Reason != "copy_limit" {
		t.Fatal("a preset too narrow for the owner's words was not refused by the slot", err)
	}
	if current, _ := h.projects.GetProject(ctx, "alice", before.ID); current.IntroPreset != "cover" || current.EditPlan != long.EditPlan || current.Regions.Intro.Slots[0].Text != fits {
		t.Fatal("the refused preset change left part of itself behind")
	}
}

// Correcting a region's line in ② is its slot's owner-fixed text in the same
// save, which the storyline surface then reads; a concurrent region edit makes
// that save a conflict instead of being overwritten (CLIP-188).
func TestCorrectedRegionTextIsTheSlotInTheSameSave(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	ctx := t.Context()
	on := true
	p, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(before, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{slotText("project-intro-1", "성수 골목")}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	state, err := h.service.EditingState(p)
	if err != nil {
		t.Fatal(err)
	}
	draft := state.Plan
	for i := range draft.Elements {
		if draft.Elements[i].InstanceID == "project-intro" {
			draft.Elements[i].Rows[1].Text = "저녁 영업"
		}
	}
	// A slot edit landing before the save is kept beside the corrected line.
	instruction := "영업 시간"
	raced, err := h.projects.UpdateProject(ctx, "alice", p.ID, regionEdit(p, func(patch *clip.ProjectPatch) {
		patch.IntroRegion = &clip.RegionPatch{Slots: []clip.RegionSlotPatch{{ID: "project-intro-2", Instruction: &instruction}}}
	}))
	if err != nil || raced.EditPlanRevision != p.EditPlanRevision {
		t.Fatal(err)
	}
	saved, err := h.service.SaveCorrection(ctx, "alice", p.ID, raced.EditPlanRevision, draft)
	if err != nil {
		t.Fatal(err)
	}
	if s := saved.Regions.Intro.Slots[1]; s.Text != "저녁 영업" || !s.OwnerFixed || s.Instruction != instruction || saved.Regions.Revision != raced.Regions.Revision+1 {
		t.Fatalf("the corrected line is not the slot's: %+v", saved.Regions.Intro)
	}
	if got := regionRows(planRegion(t, saved, "hook")[0]); !slices.Equal(got, []string{"성수 골목", "저녁 영업"}) || saved.EditPlanRevision != raced.EditPlanRevision+1 {
		t.Fatal("the plan does not draw the corrected slot", got)
	}
	// Region state computed from a revision a later edit replaced is refused
	// with the plan it came with, rather than written over that edit.
	stale := raced.Regions.Clone()
	stale.Intro.Slots[0].Text = "덮어쓰기"
	if _, err := h.store.SaveCorrection(ctx, "alice", p.ID, saved.EditPlanRevision, saved.EditPlan, &stale); !errors.Is(err, clip.ErrPlanConflict) {
		t.Fatal("a correction overwrote a newer slot edit", err)
	}
	reread, err := h.projects.GetProject(ctx, "alice", p.ID)
	if err != nil || !reflect.DeepEqual(reread.Regions, saved.Regions) || reread.EditPlan != saved.EditPlan {
		t.Fatal("reloading brought back another copy of the region words", err)
	}
}

// A writer's plan is saved drawing the project's regions: the words the calls
// drafted become the generated slots', an owner-fixed slot keeps its own, no
// template entry the plan carries is drawn, and a region that is off is not
// drawn whatever the entries held (CLIP-68, CLIP-187).
func TestAWrittenPlanIsSavedDrawingTheProjectRegions(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	ctx := t.Context()
	on := true
	p, err := h.projects.UpdateProject(ctx, "alice", before.ID, regionEdit(before, func(p *clip.ProjectPatch) {
		p.IntroRegion = &clip.RegionPatch{Enabled: &on, Slots: []clip.RegionSlotPatch{slotText("project-intro-1", "성수 골목")}}
	}))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	entry := func(id, role, basis string, a, b int, rows ...string) clip.PortableText {
		e := composition.Element{ID: id, Kind: "ai", Role: role, Style: "auto", Position: "auto", Align: "center", Basis: basis, StartMS: &a, EndMS: &b}
		text := clip.PortableText{Resolved: composition.ResolvedElement{InstanceID: id, Element: e}}
		text.Resolved.StartMS, text.Resolved.EndMS = a, b
		if basis == "output-end" {
			text.Resolved.StartMS, text.Resolved.EndMS = plan.DurationMS+a, plan.DurationMS+b
		}
		for _, row := range rows {
			text.Resolved.Rows = append(text.Resolved.Rows, composition.ResolvedRow{Text: row})
		}
		return text
	}
	written := plan
	portable := *plan.Portable
	portable.Elements = []clip.PortableText{entry("hello", "hook", "output-start", 0, 2500, "writer first", "writer second"), entry("bye", "ending", "output-end", -3000, 0, "writer bye")}
	written.Portable = &portable
	raw, err := clip.EncodeEditPlan(written)
	if err != nil {
		t.Fatal(err)
	}
	drafts := []clip.RegionDraft{{SlotID: "project-intro-1", Text: "덮어쓰기"}, {SlotID: "project-intro-2", Text: "writer second"}, {SlotID: "project-outro-1", Text: "꺼진 아웃트로"}}
	if err := h.store.SaveGeneratedPlan(ctx, "alice", p.ID, p.Analysis, raw, "", drafts, p.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	saved, err := h.projects.GetProject(ctx, "alice", p.ID)
	if err != nil {
		t.Fatal(err)
	}
	intro := planRegion(t, saved, "hook")
	if len(intro) != 1 || intro[0].Resolved.InstanceID != "project-intro" || !slices.Equal(regionRows(intro[0]), []string{"성수 골목", "writer second"}) {
		t.Fatal("the written intro does not draw the project's slots", intro)
	}
	if len(planRegion(t, saved, "ending")) != 0 {
		t.Fatal("an outro that is off was drawn from the writer's entry")
	}
	if s := saved.Regions.Intro.Slots; s[0].Text != "성수 골목" || !s[0].OwnerFixed || s[1].Text != "writer second" || s[1].OwnerFixed || saved.Regions.Outro.Slots[0].Text != "" {
		t.Fatalf("the slots do not hold the words the plan draws: %+v", saved.Regions)
	}
	// A revision request rewrites the flow or the narration and no region
	// word: whatever its writer put in template entries, the plan it saves
	// draws the slots as they stand (CLIP-131).
	portable.Elements = []clip.PortableText{entry("hello", "hook", "output-start", 0, 2500, "revised first", "revised second")}
	revised := written
	revised.Portable = &portable
	if raw, err = clip.EncodeEditPlan(revised); err != nil {
		t.Fatal(err)
	}
	after, err := h.store.SaveRevisedPlan(ctx, "alice", p.ID, "", saved.EditPlanRevision, raw)
	if err != nil {
		t.Fatal(err)
	}
	intro = planRegion(t, after, "hook")
	if len(intro) != 1 || !slices.Equal(regionRows(intro[0]), []string{"성수 골목", "writer second"}) || !reflect.DeepEqual(after.Regions, saved.Regions) {
		t.Fatal("a revision rewrote the region words", intro, after.Regions)
	}
}

// ①'s autosave carries the presets the regions already render in with every
// save. That chooses nothing (CLIP-111): no region switches on and the plan and
// its revision stay where they were.
func TestAnAutosaveCarryingTheCurrentPresetsChangesNoRegion(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	presets := before.DesignSelection().RegionPresets()
	title := "새 제목"
	after, err := h.projects.UpdateProject(t.Context(), "alice", before.ID, clip.ProjectPatch{Title: &title, IntroPreset: &presets.Intro, OutroPreset: &presets.Outro})
	if err != nil {
		t.Fatal(err)
	}
	if after.Regions.Intro.Enabled || after.Regions.Outro.Enabled || after.Regions.Revision != before.Regions.Revision {
		t.Fatal("an autosave switched a region on", after.Regions)
	}
	if after.EditPlan != before.EditPlan || after.EditPlanRevision != before.EditPlanRevision || after.RenderedPlanRevision != before.RenderedPlanRevision {
		t.Fatal("an autosave staled the render")
	}
}

// A project whose regions were only ever derived from its plan records them the
// moment its plan is projected from them: read again, the slots are the ones
// the plan was drawn from, not a derivation of the projected element (CLIP-190).
func TestProjectingAPlanRecordsTheRegionsItWasDrawnFrom(t *testing.T) {
	h, before, _ := completedNativeClip(t)
	ctx := t.Context()
	plan, err := clip.DecodeEditPlan(before.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	a, b := 0, 2500
	e := composition.Element{ID: "hello", Kind: "ai", Role: "hook", Style: "auto", Position: "auto", Align: "center", Basis: "output-start", StartMS: &a, EndMS: &b}
	portable := *plan.Portable
	portable.Elements = []clip.PortableText{{Resolved: composition.ResolvedElement{InstanceID: "hello", Element: e, StartMS: 0, EndMS: 2500, Rows: []composition.ResolvedRow{{Text: "쓴 말"}}}}}
	plan.Portable = &portable
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := h.store.SaveCorrection(ctx, "alice", before.ID, before.EditPlanRevision, raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET regions_json=NULL WHERE id=?`, before.ID); err != nil {
		t.Fatal(err)
	}
	derived, err := h.projects.GetProject(ctx, "alice", before.ID)
	if err != nil || !derived.Regions.Intro.Enabled || derived.Regions.Intro.Slots[0].Text != "쓴 말" || derived.Regions.Intro.Slots[0].OwnerFixed {
		t.Fatal("the fixture does not derive a generated intro", derived.Regions, err)
	}
	cover := "cover"
	grown, err := h.projects.UpdateProject(ctx, "alice", before.ID, clip.ProjectPatch{IntroPreset: &cover})
	if err != nil || grown.EditPlanRevision <= legacy.EditPlanRevision {
		t.Fatal(err)
	}
	if intro := planRegion(t, grown, "hook"); len(intro) != 1 || intro[0].Resolved.InstanceID != "project-intro" {
		t.Fatal("the preset change did not project the intro", intro)
	}
	var recorded bool
	if err := h.db.Reader.QueryRow(`SELECT regions_json IS NOT NULL FROM clip_projects WHERE id=?`, before.ID).Scan(&recorded); err != nil || !recorded {
		t.Fatal("the regions the plan was drawn from were not recorded", err)
	}
	if !reflect.DeepEqual(grown.Regions.Intro.Slots[0], derived.Regions.Intro.Slots[0]) {
		t.Fatalf("reading the projected plan changed the slot: %+v vs %+v", grown.Regions.Intro.Slots[0], derived.Regions.Intro.Slots[0])
	}
}
