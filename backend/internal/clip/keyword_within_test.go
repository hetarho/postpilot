package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestKeywordWithinKeepsOnlyAWordTheTextCarries(t *testing.T) {
	for _, c := range []struct{ text, keyword, want string }{
		{"진하게 끓여낸 찌개", "찌개", "찌개"},
		{"푸짐한 고기와 채소", "든든한 한 끼", ""},
		{"푸짐한 고기와 채소", "", ""},
	} {
		if got := clip.KeywordWithin(c.text, c.keyword); got != c.want {
			t.Errorf("KeywordWithin(%q, %q) = %q, want %q", c.text, c.keyword, got, c.want)
		}
	}
}

// A narration caption stored after its copy fell back to a grounded shorter sentence that
// lacks the accent word (CDS-41, CDS-55) decodes without it, so the plan saves and
// 확정하기 confirms it (CLIP-75); a keyword its text holds is untouched (CDS-25).
func TestStoredOrphanKeywordIsDroppedOnDecode(t *testing.T) {
	cfg := clip.DefaultRenderConfig(clip.Environment{})
	p, plan := creationFixture(t)
	orphan := clip.NarrationCaption("narration-1", "푸짐한 고기와 채소", 2000, 5000)
	orphan.Keyword, orphan.FallbackReason = "든든한 한 끼", "shorter_copy"
	held := clip.NarrationCaption("narration-2", "진하게 끓여낸 찌개", 6000, 9000)
	held.Keyword = "찌개"
	plan.Portable.Elements = append(plan.Portable.Elements, orphan, held)
	raw, err := clip.EncodeEditPlan(plan)
	if err != nil {
		t.Fatal(err)
	}
	p.EditPlan = raw
	saved, err := clip.DecodeEditPlan(raw)
	if err != nil {
		t.Fatal(err)
	}
	keywords := map[string]string{}
	for _, text := range saved.Portable.Elements {
		keywords[text.Resolved.InstanceID] = text.Keyword
	}
	if keywords["narration-1"] != "" || keywords["narration-2"] != "찌개" {
		t.Fatal("the decode kept an orphan keyword or dropped a held one", keywords)
	}
	if _, err := clip.ApplyCorrection(cfg, p, clip.CorrectionFromPlan(saved)); err != nil {
		t.Fatal("an unchanged save of the healed plan was refused", err)
	}
	if refusal := clip.ConfirmationRefusal(p, false, cfg); refusal != "" {
		t.Fatal("확정하기 still refuses the stored plan", refusal)
	}
}
