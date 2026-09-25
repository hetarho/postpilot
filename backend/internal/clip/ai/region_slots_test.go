package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/clip/design"
)

func TestWriterListsEachGeneratedRegionSlotLimit(t *testing.T) {
	in := nativeInput()
	// The body names no preset; the slot counts come from the PROJECT's own
	// selection (CLIP-14, CLIP-139, CLIP-147).
	in.Design = clip.ProjectDesign{IntroPreset: "a", OutroPreset: "e"}
	setNativeBody(&in, `<clip version="1"><text id="opening" kind="ai" role="hook"><row>주제</row><row kind="fixed">고정 제목</row></text><text id="closing" kind="fixed" role="ending"><row kind="ai">라벨</row><row kind="ai">점수</row><row kind="ai">마무리</row></text></clip>`)
	system, user := ai.BuildPlanPrompt(in, 200, clip.DefaultCompositionLimits())
	var payload struct {
		Slots []struct {
			Element      string  `json:"element_id"`
			Row          int     `json:"row_index"`
			Role         string  `json:"role"`
			Size         float64 `json:"size"`
			Floor        float64 `json:"floor"`
			Lines        int     `json:"lines"`
			MaxSyllables int     `json:"max_syllables"`
		} `json:"generated_region_slots"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 4 {
		t.Fatal(user)
	}
	// Each row is described by the slot it lands in (CLIP-147, CDS-86): the
	// intro's first slot and the outro's three, with what fits them at the floor.
	for i, want := range []struct {
		kind, id string
		slot     int
		role     string
		lines    int
	}{{"intro", "a", 0, "headline", 2}, {"outro", "e", 0, "label", 1}, {"outro", "e", 1, "display", 2}, {"outro", "e", 2, "caption", 1}} {
		spec, width, _ := design.RegionSlotAt(want.kind, want.id, in.Ratio, want.slot)
		got := payload.Slots[i]
		if got.Role != want.role || got.Lines != want.lines || got.Size != spec.Size || got.Floor != spec.Floor || got.MaxSyllables != design.RegionSlotBudget(spec, width) || got.MaxSyllables <= 0 {
			t.Fatal(i, payload.Slots)
		}
	}
	if !strings.Contains(system, "max_syllables") || strings.Contains(system, "single-line") || !strings.Contains(system, "row kind overrides") || strings.Contains(system, "opening card") {
		t.Fatal(system)
	}
	legacy, _ := ai.BuildPlanPrompt(clip.PlanningInput{}, 200, clip.DefaultCompositionLimits())
	if strings.Contains(legacy, `"hook"`) || strings.Contains(legacy, "opening card") || strings.Contains(string(ai.PlanSchema()), `"hook"`) {
		t.Fatal("legacy plan still requests hook")
	}
}

// CLIP-147: a second intro entry's generated row lands in the slot after the
// first entry's line, so it is described by that slot and not by slot one.
func TestAGeneratedRowIsDescribedByTheSlotItLandsIn(t *testing.T) {
	in := nativeInput()
	in.Design = clip.ProjectDesign{IntroPreset: "a", OutroPreset: "e"}
	setNativeBody(&in, `<clip version="1"><text id="title" kind="fixed" role="hook"><row>고정 제목</row></text><text id="subtitle" kind="ai" role="hook"><row>부제</row></text></clip>`)
	_, user := ai.BuildPlanPrompt(in, 200, clip.DefaultCompositionLimits())
	var payload struct {
		Slots []struct {
			Element string `json:"element_id"`
			Role    string `json:"role"`
		} `json:"generated_region_slots"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 1 || payload.Slots[0].Element != "subtitle" || payload.Slots[0].Role != "label" {
		t.Fatal(payload.Slots)
	}
}
