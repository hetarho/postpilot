package store_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// withWriterCaption writes one narration caption into the stored plan, as the
// writer's narration call does, and returns its id.
func withWriterCaption(t *testing.T, h *generationHarness, p clip.Project, edit func(*clip.PortableText)) string {
	t.Helper()
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil || plan.Portable == nil {
		t.Fatal(err)
	}
	text := clip.NarrationCaption("narration-9", "자막", 0, min(3000, plan.DurationMS))
	if edit != nil {
		edit(&text)
	}
	plan.Portable.Elements = append(plan.Portable.Elements, text)
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET edit_plan_json=? WHERE id=?`, raw, p.ID); err != nil {
		t.Fatal(err)
	}
	return text.Resolved.Element.ID
}

func refusing(id string, refused func(clip.PortableText) bool) func(clip.EditPlan) error {
	return func(plan clip.EditPlan) error {
		for _, text := range plan.Portable.Elements {
			if text.Resolved.Element.ID == id && refused(text) {
				return &composition.Problem{ElementID: id, Reason: "copy_limit"}
			}
		}
		return nil
	}
}

func savedPlan(t *testing.T, h *generationHarness, id string) (clip.Project, clip.EditPlan) {
	t.Helper()
	p, err := h.projects.GetProject(t.Context(), "alice", id)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := clip.DecodeEditPlan(p.EditPlan)
	if err != nil {
		t.Fatal(err)
	}
	return p, plan
}

func hasElement(plan clip.EditPlan, id string) bool {
	for _, text := range plan.Portable.Elements {
		if text.Resolved.Element.ID == id {
			return true
		}
	}
	return false
}

// CLIP-224: a writer caption the render start's layout refuses no longer stops
// the render. It is omitted with one notice, the plan is saved as the next
// revision, and the render starts on that revision.
func TestRenderStartOmitsARefusedWriterCaption(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	id := withWriterCaption(t, h, p, nil)
	h.renderer.refuse = refusing(id, func(clip.PortableText) bool { return true })
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderServer); err != nil {
		t.Fatal(err)
	}
	after, plan := savedPlan(t, h, p.ID)
	if after.EditPlanRevision != p.EditPlanRevision+1 || hasElement(plan, id) {
		t.Fatalf("revision %d→%d, caption kept=%v", p.EditPlanRevision, after.EditPlanRevision, hasElement(plan, id))
	}
	found := false
	for _, n := range plan.Notices {
		found = found || n.ElementID == id && n.Reason == "copy_limit" && n.Action == "omitted"
	}
	if !found {
		t.Fatalf("no omission notice: %+v", plan.Notices)
	}
}

// A caption that fits once it is freed from its frozen placement is kept, and
// only its placement changes.
func TestRenderStartFreesAPlacedWriterCaptionBeforeOmittingIt(t *testing.T) {
	h, p, _ := completedNativeClip(t)
	id := withWriterCaption(t, h, p, func(text *clip.PortableText) {
		text.Placement = &clip.CompositionPlacement{Style: "bold", Position: "bottom", StartMS: text.Resolved.StartMS, EndMS: text.Resolved.EndMS}
	})
	_, before := savedPlan(t, h, p.ID)
	h.renderer.refuse = refusing(id, func(text clip.PortableText) bool { return text.Placement != nil })
	if _, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderServer); err != nil {
		t.Fatal(err)
	}
	_, plan := savedPlan(t, h, p.ID)
	if !hasElement(plan, id) || len(plan.Notices) != len(before.Notices) {
		t.Fatalf("kept=%v notices=%+v", hasElement(plan, id), plan.Notices)
	}
}

// What the owner made is the owner's to fix: a refused owner-edited caption
// and any refusal that names no writer caption still refuse the start, and
// nothing is saved.
func TestRenderStartStillRefusesWhatTheWriterDidNotAuthor(t *testing.T) {
	for name, refusal := range map[string]func(string) error{
		"owner-edited caption": nil,
		"non-caption element":  func(string) error { return &composition.Problem{ElementID: "intro", Reason: "copy_limit"} },
		"untyped error":        func(string) error { return clip.ErrCopyTooLong },
	} {
		t.Run(name, func(t *testing.T) {
			h, p, _ := completedNativeClip(t)
			id := withWriterCaption(t, h, p, func(text *clip.PortableText) { text.OwnerEdited = refusal == nil })
			if refusal == nil {
				h.renderer.refuse = refusing(id, func(clip.PortableText) bool { return true })
			} else {
				h.renderer.refuse = func(clip.EditPlan) error { return refusal(id) }
			}
			_, err := h.service.StartRender(t.Context(), "alice", p.ID, h.batch.ID, p.EditPlanRevision, clip.RenderServer)
			var problem *composition.Problem
			if err == nil || !errors.As(err, &problem) && !errors.Is(err, clip.ErrCopyTooLong) {
				t.Fatal(err)
			}
			after, _ := savedPlan(t, h, p.ID)
			if after.EditPlanRevision != p.EditPlanRevision {
				t.Fatalf("saved a revision: %d→%d", p.EditPlanRevision, after.EditPlanRevision)
			}
		})
	}
}
