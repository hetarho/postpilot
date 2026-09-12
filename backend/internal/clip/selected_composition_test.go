package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestUnassignedSelectedFootagePreservesLiteralTextAndOptionalBindings(t *testing.T) {
	limits := config.ClipCompositionLimits()
	doc, problem := composition.Parse(`<clip version="1"><group id="menu"><field id="name" label="이름" required="true"/></group><repeat for="menu"><scene id="dish" scope="item"><text id="literal" kind="fixed" role="caption" basis="cut">  그대로  </text><text id="bound" kind="fixed" role="info" basis="cut"><value field="menu.name"/></text><text id="copy" kind="ai" role="caption" basis="cut">메뉴 설명</text></scene></repeat></clip>`, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	cuts := []composition.Cut{{ID: "first", SectionID: "dish", SourceID: "source", StartMS: 0, EndMS: 5000}, {ID: "second", SectionID: "dish", SourceID: "source", StartMS: 5000, EndMS: 10000}}
	result, fallbacks, err := clip.ResolveSelectedComposition(doc, clip.CompositionInputs{}, cuts, limits, 30000)
	if err != nil || len(result.Elements) != 2 || len(fallbacks) != 4 {
		t.Fatalf("%+v %+v %v", result, fallbacks, err)
	}
	if result.Elements[0].Text != "  그대로  " || result.Elements[1].CutID != "second" || len(doc.Sections) != 1 || cuts[0].SectionID != "dish" {
		t.Fatal("source or authored literal mutated")
	}
}
