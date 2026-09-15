package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestGeneratedRegionSlotsRepairIndividuallyAndPreserveFixedNeighbours(t *testing.T) {
	for _, tc := range []struct{ name, full, short, want, notice string }{
		{"shorter", strings.Repeat("가", 12), "영상 속 모습", "영상 속 모습", "intro_slot_shortened"},
		{"omit", strings.Repeat("가", 12), "", "", "intro_slot_omitted"},
		{"newline", "영상 속\n모습", "영상 속 모습", "영상 속 모습", "intro_slot_shortened"},
		{"ungrounded alternative", strings.Repeat("가", 12), "999원", "", "intro_slot_omitted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			body := `<clip version="1" intro="b" caption="bold" outro="e"><field id="name" label="이름"/><guide>긴 장면으로 보여 준다</guide><text id="opening" kind="fixed" role="hook" basis="output-start"><row kind="ai">관찰한 모습</row><row><value field="name"/></row></text><text id="closing" kind="fixed" role="ending" basis="output-end"/></clip>`
			setNativeBody(&in, body)
			in.Composition.Inputs = clip.CompositionInputs{Values: map[string]string{"name": "직접 쓴 제목"}}
			for _, c := range p["cuts"].([]any) {
				c.(map[string]any)["template_section_id"] = ""
			}
			g := nativeGenerated(p, 0)
			g["element_id"], g["cut_id"], g["text"], g["short_text"] = "opening", "", "", ""
			g["rows"], g["short_rows"] = []string{tc.full, "고치면 안 됨"}, []string{tc.short, "다른 제목"}
			g["fact_refs"] = []any{}
			p["generated"] = []any{g}
			s, models, _ := newService(t, raw(p), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if err != nil {
				t.Fatal(err)
			}
			if len(models.calls) != 1 || len(plan.Portable.Elements) != 1 {
				t.Fatal(plan)
			}
			rows := plan.Portable.Elements[0].Resolved.Rows
			if len(rows) != 2 || rows[0].Text != tc.want || rows[1].Text != "직접 쓴 제목" {
				t.Fatal(rows)
			}
			notices := clip.ActivePlanNotices(plan)
			if len(notices) != 1 || notices[0].Reason != tc.notice || notices[0].ElementID != "opening" {
				t.Fatal(notices)
			}
			encoded, err := clip.EncodeEditPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			restored, err := clip.DecodeEditPlan(encoded)
			if err != nil || restored.Portable.Elements[0].Resolved.Rows[1].Text != "직접 쓴 제목" {
				t.Fatal(err)
			}
		})
	}
}

func TestFixedRegionOverflowNeverEntersGeneratedLadder(t *testing.T) {
	in, p := nativeInput(), nativePlan()
	literal := strings.Repeat("가", 12)
	setNativeBody(&in, `<clip version="1" intro="b" caption="bold" outro="e"><guide>긴 장면으로 보여 준다</guide><text id="opening" kind="ai" role="hook" basis="output-start"><row kind="fixed">`+literal+`</row><row kind="ai">관찰</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"/></clip>`)
	in.Composition.Inputs = clip.CompositionInputs{}
	for _, c := range p["cuts"].([]any) {
		c.(map[string]any)["template_section_id"] = ""
	}
	g := nativeGenerated(p, 0)
	g["element_id"], g["cut_id"], g["text"] = "opening", "", ""
	g["rows"] = []string{"바꾸지 마세요", "영상 속 모습"}
	g["fact_refs"] = []any{}
	p["generated"] = []any{g}
	s, _, _ := newService(t, raw(p), true)
	plan, _, err := s.Plan(t.Context(), testRef(), in)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Portable.Elements[0].Resolved.Rows[0].Text != literal || len(plan.Notices) != 0 {
		t.Fatal(plan)
	}
}

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
