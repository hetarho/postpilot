package guideline

import (
	"reflect"
	"strings"
	"testing"
)

// GUIDE-41, GUIDE-42: exactly the product's 기본 지침, in their order, each with a stable key and
// both languages, and only 자연스러운 한국어 문체 kept to a Korean target.
func TestDefaultRegistryIsTheProductsOrder(t *testing.T) {
	keys := func(kind Kind) []string {
		var out []string
		for _, d := range Defaults(kind) {
			out = append(out, d.Key)
		}
		return out
	}
	if got, want := keys(KindPost), []string{"facts", "impressions", "naming", "order", "opening", "photo_moments", "closing", "no_listing", "titles", "tags", "natural_korean"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("post defaults = %v, want %v", got, want)
	}
	if got, want := keys(KindClip), []string{"clip_facts", "clip_impressions", "clip_hook", "clip_continuity", "clip_order", "clip_wrap_up", "clip_no_repeated_promotion"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("clip defaults = %v, want %v", got, want)
	}
	for _, kind := range []Kind{KindPost, KindClip} {
		for _, d := range Defaults(kind) {
			if d.Kind != kind || !isASCIIKey(d.Key) {
				t.Errorf("%q: kind %s, want %s and an ASCII key", d.Key, d.Kind, kind)
			}
			for _, copy := range []DefaultCopy{d.Ko, d.En} {
				if strings.TrimSpace(copy.Name) == "" || strings.TrimSpace(copy.Text) == "" {
					t.Errorf("%q lacks a name or a text in one language: %+v", d.Key, d)
				}
			}
			if d.KoreanTargetOnly != (d.Key == "natural_korean") {
				t.Errorf("%q KoreanTargetOnly = %v", d.Key, d.KoreanTargetOnly)
			}
		}
	}
	if _, ok := DefaultFor(KindPost, "clip_facts"); ok {
		t.Error("a clip key resolved as a post default")
	}
	if text, ok := Defaults(KindPost)[10].Text(LanguageEnglish); ok || text != "" {
		t.Error("the Korean-target-only default reached an English target")
	}
	if text, ok := Defaults(KindPost)[0].Text(LanguageEnglish); !ok || !strings.HasPrefix(text, "State no concrete fact") {
		t.Errorf("the English text of facts = %q", text)
	}
	// The slice is a copy: reordering it cannot reorder the registry.
	first := Defaults(KindPost)
	first[0], first[1] = first[1], first[0]
	if Defaults(KindPost)[0].Key != "facts" {
		t.Fatal("the registry is shared with its callers")
	}
}

func isASCIIKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if !(r >= 'a' && r <= 'z' || r == '_') {
			return false
		}
	}
	return true
}
