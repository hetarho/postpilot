package composition_test

import (
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestLegacyRestaurantTemplateConvertsIntoFixedRegionsAndOneGuide(t *testing.T) {
	l := config.ClipCompositionLimits()
	body, err := os.ReadFile("testdata/legacy-restaurant.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, problem := composition.ParseTemplate(string(body), l); problem == nil || problem.Reason != "unsupported_section" || problem.ElementID != "exterior" {
		t.Fatalf("legacy body accepted as a template: %+v", problem)
	}
	converted, changed, problem := composition.ConvertLegacyTemplate(string(body), l)
	if problem != nil || !changed {
		t.Fatalf("conversion failed: %+v changed=%v", problem, changed)
	}
	d, problem := composition.ParseTemplate(converted, l)
	if problem != nil {
		t.Fatalf("converted body refused: %+v", problem)
	}
	if len(d.Sections) != 0 || len(d.Fields) != 8 || len(d.Groups) != 1 || d.Design != (composition.DesignSelection{Intro: "b", Caption: "bold", Outro: "e"}) || d.Accent != "coral" || d.Pace != "rapid" {
		t.Fatalf("fixed regions lost: %+v", d)
	}
	ids := []string{}
	for _, e := range d.Elements {
		ids = append(ids, e.ID)
	}
	if strings.Join(ids, " ") != "disclosure_badge intro closing_card" {
		t.Fatalf("elements %v", ids)
	}
	// The root guide stays first; the carried prose is one new guide holding every
	// scene guide and every scene-bound element, nothing dropped.
	if len(d.Guidance) != 3 || !strings.HasPrefix(d.Guidance[0], "\n    장면 배열이") {
		t.Fatalf("guides %d: %q", len(d.Guidance), d.Guidance)
	}
	carried := d.Guidance[1]
	for _, want := range []string{"가장 이른 시점의 클립으로 시작하세요", "메뉴판이나 주문표가 찍힌 클립이 있을 때만", "현재 메뉴에 연결된 클립만", "dish_caption [ai]: 이 컷에서 실제로 보이는 것 하나를 골라", "dish_menu [fixed]: {menu.name}", "dish_price [fixed]: {menu.price}", "info_line [ai]: {extra}에서 방문 결정에 필요한 정보 하나만", "로고, 간판, 나오면서 본 외관처럼"} {
		if !strings.Contains(carried, want) {
			t.Fatalf("carried guide lacks %q:\n%s", want, carried)
		}
	}
	again, changedAgain, problem := composition.ConvertLegacyTemplate(converted, l)
	if problem != nil || changedAgain || again != converted {
		t.Fatalf("conforming body did not convert to itself: %+v %v", problem, changedAgain)
	}
	t.Log(converted)
}

func TestConversionRefusesOverlongCarriedGuideInsteadOfTruncating(t *testing.T) {
	l := config.ClipCompositionLimits()
	body := `<clip version="1" intro="b" caption="bold" outro="e"><scene id="s" scope="scene"><guide>` + strings.Repeat("가", l.GuideChars) + `</guide><text id="c" kind="ai" role="caption" basis="cut">설명</text></scene><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`
	out, changed, problem := composition.ConvertLegacyTemplate(body, l)
	if problem == nil || problem.Reason != "guide_limit" || problem.ElementID != "clip" || problem.Line != 1 || changed || out != body {
		t.Fatalf("overlong prose was not refused: %+v %v", problem, changed)
	}
}

func TestTemplateGrammarNamesTheRefusedConstruct(t *testing.T) {
	l := config.ClipCompositionLimits()
	skeleton := `<text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/>`
	for _, c := range []struct{ body, reason, id string }{
		{`<clip version="1" intro="b" caption="bold" outro="e"><scene id="footage" scope="scene"/>` + skeleton + `</clip>`, "unsupported_section", "footage"},
		{`<clip version="1" intro="b" caption="bold" outro="e"><group id="menu"><field id="name" label="메뉴"/></group><repeat for="menu"><scene id="dish" scope="item"/></repeat>` + skeleton + `</clip>`, "unsupported_section", "repeat"},
		{`<clip version="1" intro="b" caption="bold" outro="e"><text id="c" kind="fixed" role="caption" basis="whole">x</text>` + skeleton + `</clip>`, "unsupported_role", "c"},
		{`<clip version="1" intro="b" caption="bold" outro="e"><field id="place" label="장소"/><text id="i" kind="fixed" role="info" basis="whole"><value field="place"/></text>` + skeleton + `</clip>`, "unsupported_role", "i"},
		{`<clip version="1" intro="b" caption="bold" outro="e"><text id="b" kind="fixed" role="badge" basis="cut">x</text>` + skeleton + `</clip>`, "unsupported_basis", "b"},
	} {
		_, problem := composition.ParseTemplate(c.body, l)
		if problem == nil || problem.Reason != c.reason || problem.ElementID != c.id {
			t.Fatalf("%s: got %+v", c.reason, problem)
		}
		// The snapshot grammar keeps reading what the template grammar refuses.
		if _, problem := composition.Parse(c.body, l); problem != nil && problem.Reason == c.reason {
			t.Fatalf("Parse changed: %+v", problem)
		}
	}
	ok := `<clip version="1" intro="b" caption="bold" outro="e" accent="teal" pace="rapid"><field id="place" label="장소" required="true">촬영한 장소</field><group id="menu" label="메뉴" min="1"><field id="name" label="메뉴 이름" required="true"/><field id="price" label="가격"/></group><guide>친구에게 말하듯 쓰세요.</guide><text id="disclosure" kind="fixed" role="badge" position="header" basis="whole">협찬</text><text id="intro" kind="fixed" role="hook" basis="output-start"><row kind="fixed"><value field="place"/></row></text><text id="closing" kind="fixed" role="ending" basis="output-end"><row>다음에</row></text></clip>`
	d, problem := composition.ParseTemplate(ok, l)
	if problem != nil || len(d.Fields) != 3 || len(d.Groups) != 1 || len(d.Guidance) != 1 || len(d.Elements) != 3 || d.Accent != "teal" || d.Pace != "rapid" {
		t.Fatalf("conforming template refused: %+v", problem)
	}
}
