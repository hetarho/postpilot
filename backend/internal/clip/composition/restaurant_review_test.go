package composition_test

import (
	"os"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestRestaurantReviewListsMenusInOneExactCaption(t *testing.T) {
	body, err := os.ReadFile("testdata/restaurant-review.xml")
	if err != nil {
		t.Fatal(err)
	}
	doc, problem := composition.ParseTemplate(string(body), clip.DefaultCompositionLimits())
	if problem != nil {
		t.Fatal(problem)
	}
	if len(doc.Groups) != 0 {
		t.Fatalf("menus should use one field, got groups: %+v", doc.Groups)
	}
	if !slices.ContainsFunc(doc.Fields, func(field composition.Field) bool {
		return field.ID == "menu" && field.Required && doc.Maxima[field.ID] == 18
	}) {
		t.Fatal("menu field must accept the complete on-screen list")
	}
	if !slices.ContainsFunc(doc.Elements, func(element composition.Element) bool {
		return element.ID == "menu_callout" && element.Kind == "fixed" && element.Role == "caption" &&
			len(element.Parts) == 1 && element.Parts[0].Field == "menu"
	}) {
		t.Fatal("menu list must be shown verbatim, not selected by the writer")
	}
}
