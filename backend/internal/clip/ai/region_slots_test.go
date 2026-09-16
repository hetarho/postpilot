package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestWriterListsEachGeneratedRegionSlotLimit(t *testing.T) {
	in := nativeInput()
	setNativeBody(&in, `<clip version="1" intro="a" caption="bold" outro="e"><text id="opening" kind="ai" role="hook" basis="output-start"><row>주제</row><row kind="fixed">고정 제목</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row kind="ai">라벨</row><row kind="ai">점수</row><row kind="ai">마무리</row></text></clip>`)
	system, user := ai.BuildPlanPrompt(in, 200, config.ClipCompositionLimits())
	var payload struct {
		Slots []struct {
			Element             string `json:"element_id"`
			Index, Chars, Lines int
			Type                string
		} `json:"generated_region_slots"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 4 {
		t.Fatal(user)
	}
	for i, want := range []int{8, 22, 6, 18} {
		if payload.Slots[i].Chars != want || payload.Slots[i].Lines != 1 {
			t.Fatal(payload.Slots)
		}
	}
	if !strings.Contains(system, "single-line") || !strings.Contains(system, "row kind overrides") || strings.Contains(system, "opening card") {
		t.Fatal(system)
	}
	legacy, _ := ai.BuildPlanPrompt(clip.PlanningInput{}, 200, config.ClipCompositionLimits())
	if strings.Contains(legacy, `"hook"`) || strings.Contains(legacy, "opening card") || strings.Contains(string(ai.PlanSchema()), `"hook"`) {
		t.Fatal("legacy plan still requests hook")
	}
}
