package quality

import (
	"reflect"
	"testing"
)

// QUAL-23 is the contract: nine 분야 in this order, each with a stable ASCII id and a query
// that is its name with · replaced by a space.
func TestFieldsAreTheV1CatalogueInOrder(t *testing.T) {
	want := []Field{
		{ID: "restaurant", Name: "맛집", Query: "맛집"},
		{ID: "cafe", Name: "카페", Query: "카페"},
		{ID: "domestic_travel", Name: "국내여행", Query: "국내여행"},
		{ID: "fashion_beauty", Name: "패션·미용", Query: "패션 미용"},
		{ID: "product_review", Name: "상품리뷰", Query: "상품리뷰"},
		{ID: "parenting_marriage", Name: "육아·결혼", Query: "육아 결혼"},
		{ID: "pets", Name: "반려동물", Query: "반려동물"},
		{ID: "interior_diy", Name: "인테리어·DIY", Query: "인테리어 DIY"},
		{ID: "daily_life", Name: "일상·생각", Query: "일상 생각"},
	}
	got := Fields()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Fields() = %+v, want %+v", got, want)
	}

	// A caller holding the slice cannot rewrite the catalogue for everyone else.
	got[0] = Field{ID: "mutated"}
	if Fields()[0].ID != "restaurant" {
		t.Fatal("Fields() handed out the catalogue itself rather than a copy")
	}
}

func TestFieldByIDAndKnown(t *testing.T) {
	for _, field := range Fields() {
		found, ok := FieldByID(field.ID)
		if !ok || found != field {
			t.Errorf("FieldByID(%q) = %+v, %v; want %+v, true", field.ID, found, ok, field)
		}
		if !Known(field.ID) {
			t.Errorf("Known(%q) = false", field.ID)
		}
	}
	// 없음 is not a 분야, and neither is a Korean name, a query or a near miss of an id.
	for _, id := range []string{"", "맛집", "fashion beauty", "Restaurant", " cafe", "unknown"} {
		if found, ok := FieldByID(id); ok || found != (Field{}) {
			t.Errorf("FieldByID(%q) = %+v, %v; want nothing", id, found, ok)
		}
		if Known(id) {
			t.Errorf("Known(%q) = true", id)
		}
	}
}
