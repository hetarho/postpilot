package ai_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/platform/config"
)

// A declared maximum is stated to the writer and measured again on the answer,
// and an over-long one takes the same ladder a region slot takes: the grounded
// shorter alternative, then omission with its notice (CLIP-118, CDS-55).
func TestGeneratedTextObeysItsDeclaredMaximum(t *testing.T) {
	for _, tc := range []struct{ name, full, short, want, notice string }{
		{"shorter alternative", strings.Repeat("가", 12), "영상 속 모습", "영상 속 모습", "composition_text_shortened"},
		{"no alternative", strings.Repeat("가", 12), "", "", "composition_text_omitted"},
		{"within its maximum", "영상 속 모습", "영상 속", "영상 속 모습", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, p := nativeInput(), nativePlan()
			setNativeBody(&in, `<clip version="1" intro="b" caption="bold" outro="e"><guide>긴 장면으로 보여 준다</guide><text id="line" kind="ai" role="caption" basis="whole" chars="6">관찰한 모습</text><text id="opening" kind="fixed" role="hook" basis="output-start"/><text id="closing" kind="fixed" role="ending" basis="output-end"/></clip>`)
			in.Composition.Inputs = clip.CompositionInputs{}
			for _, c := range p["cuts"].([]any) {
				c.(map[string]any)["template_section_id"] = ""
			}
			g := nativeGenerated(p, 0)
			g["element_id"], g["cut_id"], g["text"], g["short_text"] = "line", "", tc.full, tc.short
			g["rows"], g["short_rows"], g["fact_refs"] = []string{}, []string{}, []any{}
			p["generated"] = []any{g}
			s, _, _ := newService(t, raw(p), true)
			plan, _, err := s.Plan(t.Context(), testRef(), in)
			if err != nil {
				t.Fatal(err)
			}
			notices := clip.ActivePlanNotices(plan)
			if tc.notice == "" {
				if len(notices) != 0 {
					t.Fatal("a text inside its maximum was repaired", notices)
				}
			} else if len(notices) != 1 || notices[0].Reason != tc.notice || notices[0].ElementID != "line" {
				t.Fatal(notices)
			}
			if tc.want == "" {
				if len(plan.Portable.Elements) != 0 {
					t.Fatal("an omitted text still reached the plan", plan.Portable.Elements)
				}
				return
			}
			if len(plan.Portable.Elements) != 1 || plan.Portable.Elements[0].Resolved.Text != tc.want {
				t.Fatal(plan.Portable.Elements)
			}
		})
	}
}

func TestWriterIsToldEachDeclaredTextMaximum(t *testing.T) {
	in := nativeInput()
	setNativeBody(&in, `<clip version="1" intro="a" caption="bold" outro="e"><text id="line" kind="ai" role="caption" basis="whole" chars="7">관찰</text><text id="pair" kind="ai" role="info" position="bottom" basis="whole"><row role="label">위치</row><row role="caption" chars="5">관찰</row></text><text id="free" kind="ai" role="caption" basis="whole">관찰</text><text id="opening" kind="fixed" role="hook" basis="output-start"><row>주제</row><row>부제</row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row>라벨</row><row>점수</row><row>마무리</row></text></clip>`)
	_, user := ai.BuildPlanPrompt(in, 200, config.ClipCompositionLimits())
	var payload struct {
		Limits []struct {
			Element string `json:"element_id"`
			Chars   int    `json:"chars"`
			Row     *int   `json:"row_index"`
		} `json:"generated_text_limits"`
	}
	if err := json.Unmarshal([]byte(user), &payload); err != nil {
		t.Fatal(err)
	}
	// Only declared maxima, and never a region row — those carry theirs in
	// generated_region_slots with the single-line rule beside them.
	if len(payload.Limits) != 2 {
		t.Fatal(payload.Limits)
	}
	if payload.Limits[0].Element != "line" || payload.Limits[0].Chars != 7 || payload.Limits[0].Row != nil {
		t.Fatal(payload.Limits[0])
	}
	if payload.Limits[1].Element != "pair" || payload.Limits[1].Chars != 5 || payload.Limits[1].Row == nil || *payload.Limits[1].Row != 1 {
		t.Fatal(payload.Limits[1])
	}
	if !strings.Contains(user, "generated_text_limits") {
		t.Fatal("the request never states the bound")
	}
}
