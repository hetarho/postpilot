package clip_test

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

const unassignedDoc = `<clip version="1" intro="b" caption="bold" outro="e"><group id="menu"><field id="name" label="이름" required="true"/></group><repeat for="menu"><scene id="dish" scope="item"><text id="literal" kind="fixed" role="caption" basis="cut">  그대로  </text><text id="bound" kind="fixed" role="info" basis="cut"><value field="menu.name"/></text><text id="copy" kind="ai" role="caption" basis="cut">메뉴 설명</text></scene></repeat><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`

func resolvedIDs(t *composition.Timeline) []string {
	ids := []string{}
	for _, e := range t.Elements {
		ids = append(ids, e.Element.ID+"/"+e.CutID)
	}
	return ids
}

// An item nobody could name costs the cut its item facts, never the scene
// sentence: the footage was filmed either way (CLIP-64).
func TestUnassignedSelectedFootageKeepsSceneCopyAndDropsItemBindings(t *testing.T) {
	limits := clip.DefaultCompositionLimits()
	doc, problem := composition.Parse(unassignedDoc, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	cuts := []composition.Cut{{ID: "first", SectionID: "dish", SourceID: "source", StartMS: 0, EndMS: 5000}, {ID: "second", SectionID: "dish", SourceID: "source", StartMS: 5000, EndMS: 10000}}
	result, fallbacks, err := clip.ResolveSelectedComposition(doc, clip.CompositionInputs{}, cuts, limits, 30000)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"literal/first", "copy/first", "literal/second", "copy/second"}
	if got := resolvedIDs(&result); !slices.Equal(got, want) {
		t.Fatalf("resolved %v, want %v", got, want)
	}
	if len(fallbacks) != 2 {
		t.Fatalf("dropped %d elements, want only the two item bindings: %+v", len(fallbacks), fallbacks)
	}
	for _, f := range fallbacks {
		if f.ElementID != "bound" || f.Reason != "item_unassigned" {
			t.Fatal("dropped something other than the item binding", f)
		}
	}
	if result.Elements[0].Text != "  그대로  " || len(doc.Sections) != 1 || cuts[0].SectionID != "dish" {
		t.Fatal("source or authored literal mutated")
	}
}

// A cut whose item is known takes the whole section, this path untouched.
func TestAssignedSelectedFootageKeepsEveryElement(t *testing.T) {
	limits := clip.DefaultCompositionLimits()
	doc, problem := composition.Parse(unassignedDoc, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	inputs := clip.CompositionInputs{Items: map[string][]composition.Item{"menu": {{ID: "one", Values: map[string]string{"name": "된장찌개"}}}}}
	cuts := []composition.Cut{{ID: "first", SectionID: "dish", SourceID: "source", GroupID: "menu", ItemID: "one", StartMS: 0, EndMS: 5000}}
	result, fallbacks, err := clip.ResolveSelectedComposition(doc, inputs, cuts, limits, 30000)
	if err != nil || len(fallbacks) != 0 {
		t.Fatalf("%+v %v", fallbacks, err)
	}
	want := []string{"literal/first", "bound/first", "copy/first"}
	if got := resolvedIDs(&result); !slices.Equal(got, want) {
		t.Fatalf("resolved %v, want %v", got, want)
	}
}
