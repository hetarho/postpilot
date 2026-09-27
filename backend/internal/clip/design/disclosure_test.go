package design_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// Five disclosure phrases, code-owned.
func TestDisclosurePhrases(t *testing.T) {
	if !reflect.DeepEqual(design.Disclosure, map[string]string{
		"ad": "광고", "sponsored": "협찬", "provided": "제품 제공",
		"paid": "소정의 원고료 지급", "self": "내돈내산",
	}) {
		t.Fatalf("disclosure phrases %+v", design.Disclosure)
	}
}

// No category preset, closing call to action, fact chip vocabulary or fact
// minimum is left in the design system for anything to read (CLIP-68, CDS-37,
// CDS-51).
func TestDesignCarriesNoCategoryPresets(t *testing.T) {
	var keys, timing map[string]json.RawMessage
	if err := json.Unmarshal(design.JSON(), &keys); err != nil {
		t.Fatal(err)
	}
	for _, retired := range []string{"presets", "cta", "facts"} {
		if _, ok := keys[retired]; ok {
			t.Fatalf("design.json still carries %q", retired)
		}
	}
	if err := json.Unmarshal(keys["timing"], &timing); err != nil {
		t.Fatal(err)
	}
	if _, ok := timing["chip_min_s"]; ok {
		t.Fatal("design.json still carries a chip exposure floor")
	}
}
