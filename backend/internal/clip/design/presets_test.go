package design_test

import (
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-50: five presets, each fixing hook tone, style mix, chip priority, default
// CTA, cut rhythm and default accent. CDS-51 keeps the values in configuration.
func TestPresetsMatchCDS50(t *testing.T) {
	want := map[string]design.Preset{
		"restaurant": {Label: "음식점", Hook: "fact", Chips: []string{"위치", "가격", "메뉴"}, CTA: "place", Accent: "coral",
			Styles: map[string]int{"clean": 50, "mark": 25, "memo": 20, "bold": 5}, CutMinS: 2.5, CutMaxS: 4, Rhythm: "cut"},
		"cafe": {Label: "카페", Hook: "space", Chips: []string{"위치", "메뉴", "영업"}, CTA: "save", Accent: "teal",
			Styles: map[string]int{"clean": 45, "memo": 30, "bold": 15, "mark": 10}, CutMinS: 4, CutMaxS: 6, Rhythm: "fade"},
		"stay": {Label: "숙소·여행", Hook: "location", Chips: []string{"위치", "가격", "영업"}, CTA: "blog", Accent: "blue",
			Styles: map[string]int{"clean": 50, "memo": 35, "bold": 10, "mark": 5}, CutMinS: 4, CutMaxS: 6, Rhythm: "fade", PriceNote: "1박 기준"},
		"beauty": {Label: "뷰티", Hook: "usage", Chips: []string{"가격", "메뉴", "평점"}, CTA: "blog", Accent: "pink",
			Styles: map[string]int{"clean": 40, "mark": 30, "memo": 20, "bold": 10}, CutMinS: 2, CutMaxS: 3, Rhythm: "cut", Refuse: "efficacy"},
		"home": {Label: "생활용품·가전", Hook: "problem", Chips: []string{"가격", "메뉴", "평점"}, CTA: "blog", Accent: "amber",
			Styles: map[string]int{"clean": 40, "mark": 35, "memo": 20, "bold": 5}, CutMinS: 3, CutMaxS: 4, Rhythm: "cut", Refuse: "specs"},
	}
	if !reflect.DeepEqual(design.Presets, want) {
		t.Fatalf("presets\n got %+v\nwant %+v", design.Presets, want)
	}
	for id, p := range want {
		if p.Label == "" {
			t.Fatalf("%s has no category label for its hook card", id)
		}
		total := 0
		for style, share := range p.Styles {
			if _, ok := design.Styles[style]; !ok {
				t.Fatalf("%s mixes an unknown style %q", id, style)
			}
			total += share
		}
		if total != 100 {
			t.Fatalf("%s style mix sums to %d", id, total)
		}
		if _, ok := design.CTA[p.CTA]; !ok {
			t.Fatalf("%s names an unknown CTA %q", id, p.CTA)
		}
		if _, ok := design.Accent[p.Accent]; !ok {
			t.Fatalf("%s names an unknown accent %q", id, p.Accent)
		}
		// CDS-30 fixes the chip vocabulary, and CDS-37 the cut range.
		for _, chip := range p.Chips {
			if _, ok := design.Fact.Prompts[chip]; !ok {
				t.Fatalf("%s names an unknown chip %q", id, chip)
			}
		}
		if p.CutMinS < design.Timing.CutMinS || p.CutMaxS > design.Timing.CutMaxS || p.CutMinS >= p.CutMaxS {
			t.Fatalf("%s cut rhythm %v..%v", id, p.CutMinS, p.CutMaxS)
		}
	}
}

// CDS-31 and CDS-29: five disclosure phrases and three CTAs, code-owned.
func TestDisclosureAndCTAPhrases(t *testing.T) {
	if !reflect.DeepEqual(design.Disclosure, map[string]string{
		"ad": "광고", "sponsored": "협찬", "provided": "제품 제공",
		"paid": "소정의 원고료 지급", "self": "내돈내산",
	}) {
		t.Fatalf("disclosure phrases %+v", design.Disclosure)
	}
	if !reflect.DeepEqual(design.CTA, map[string]string{
		"blog": "자세한 후기는 블로그에", "place": "위치는 프로필 정보 태그에서", "save": "저장해두고 방문해보세요",
	}) {
		t.Fatalf("CTA phrases %+v", design.CTA)
	}
	// An empty CTA resolves to the preset's, and a preset-less template to the
	// shared default (CDS-51).
	for preset, want := range map[string]string{"restaurant": "place", "cafe": "save", "stay": "blog", "": "blog", "unknown": "blog"} {
		if got := design.DefaultCTA(preset, ""); got != want {
			t.Fatalf("%q default CTA = %s want %s", preset, got, want)
		}
	}
	if design.DefaultCTA("restaurant", "save") != "save" {
		t.Fatal("an explicit CTA was overridden by the preset")
	}
}

// CDS-1 and CDS-30: the reserved labels, the CDS-1 minimum and the seed a preset
// puts in front of the owner.
func TestReservedFactsAndPresetSeed(t *testing.T) {
	if !reflect.DeepEqual(design.Fact.Labels, []string{"상호", "위치", "가격", "메뉴", "영업", "평점"}) {
		t.Fatalf("reserved labels %v", design.Fact.Labels)
	}
	if !reflect.DeepEqual(design.Fact.Chips, []string{"위치", "가격", "메뉴", "영업", "평점"}) {
		t.Fatalf("chip vocabulary %v", design.Fact.Chips)
	}
	if !reflect.DeepEqual(design.Fact.Minimum.Labels, []string{"상호", "위치", "가격", "메뉴"}) || design.Fact.Minimum.Count != 2 {
		t.Fatalf("CDS-1 minimum %+v", design.Fact.Minimum)
	}
	// 상호 first — every card carries the name — then the preset's own chips.
	for preset, want := range map[string][]string{
		"restaurant": {"상호", "위치", "가격", "메뉴"},
		"beauty":     {"상호", "가격", "메뉴", "평점"},
		"cafe":       {"상호", "위치", "메뉴", "영업"},
		"":           {"상호", "위치", "가격", "메뉴"},
	} {
		got := []string{}
		for _, f := range design.PresetFields(preset) {
			got = append(got, f.Label)
			if f.Prompt == "" || f.Prompt != design.Fact.Prompts[f.Label] {
				t.Fatalf("%s field %s has no code-owned prompt", preset, f.Label)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%q seed = %v want %v", preset, got, want)
		}
	}
	// Every reserved label has a prompt, and no prompt names a label that is not
	// reserved.
	if len(design.Fact.Prompts) != len(design.Fact.Labels) {
		t.Fatalf("prompts %d labels %d", len(design.Fact.Prompts), len(design.Fact.Labels))
	}
}
