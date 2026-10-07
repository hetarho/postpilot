package guideline

import (
	"reflect"
	"strings"
	"testing"
)

// GUIDE-41, GUIDE-42: exactly the product's 기본 지침, in their order, each with a stable key and
// both languages, only 자연스러운 한국어 문체 kept to a Korean target and only 기억을 통한 감상
// 추가 kept to a run that carries memories (GEN-73).
func TestDefaultRegistryIsTheProductsOrder(t *testing.T) {
	keys := func(kind Kind) []string {
		var out []string
		for _, d := range Defaults(kind) {
			out = append(out, d.Key)
		}
		return out
	}
	if got, want := keys(KindPost), []string{"facts", "impressions", "memory_impressions", "naming", "order", "opening", "photo_moments", "photo_groups", "closing", "no_listing", "titles", "tags", "ending_run", "natural_korean"}; !reflect.DeepEqual(got, want) {
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
			if d.KoreanTargetOnly != (d.Key == "natural_korean" || d.Key == "ending_run") {
				t.Errorf("%q KoreanTargetOnly = %v", d.Key, d.KoreanTargetOnly)
			}
			if d.MemoriesOnly != (d.Key == "memory_impressions") {
				t.Errorf("%q MemoriesOnly = %v", d.Key, d.MemoriesOnly)
			}
		}
	}
	if _, ok := DefaultFor(KindPost, "clip_facts"); ok {
		t.Error("a clip key resolved as a post default")
	}
	natural, _ := DefaultFor(KindPost, "natural_korean")
	if text, ok := natural.Text(LanguageEnglish); ok || text != "" {
		t.Error("the Korean-target-only default reached an English target")
	}
	// VOICE-47: no run of identical endings is voice text; it is this 기본 지침, and its text
	// states EndingMaxConsecutive in words.
	endings, _ := DefaultFor(KindPost, "ending_run")
	if endings.Ko.Name != "같은 종결어미 세 번 잇지 않기" || !strings.Contains(endings.Ko.Text, "두 번까지는 괜찮습니다") || EndingMaxConsecutive != 2 {
		t.Errorf("ending_run = %+v, max %d", endings, EndingMaxConsecutive)
	}
	// GEN-73: the memories default names itself the exception to 감상은 내가 쓴 것만 and reads the
	// 취향: label memory retrieval puts on a taste, in both languages.
	memories, _ := DefaultFor(KindPost, "memory_impressions")
	if memories.Ko.Name != "기억을 통한 감상 추가" || !strings.Contains(memories.Ko.Text, "'내 감상을 지키고 AI 제안 구분'과 함께 적용") ||
		!strings.Contains(memories.Ko.Text, "'취향:'") || !strings.Contains(memories.En.Text, "'취향:'") {
		t.Errorf("memory_impressions = %+v", memories)
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

func TestPostRecommendationsKeepPlanClaimsSeparateAndAttributeTitleAdvice(t *testing.T) {
	facts, _ := DefaultFor(KindPost, "facts")
	for _, needle := range []string{"해당 템플릿 입력란", "근거가 있는 스토리라인", "배치를 승인한 것만으로"} {
		if !strings.Contains(facts.Ko.Text, needle) {
			t.Fatalf("grounding lost its source boundary: %q", needle)
		}
	}
	if !strings.Contains(facts.En.Text, "approval of its arrangement never makes its claims author-supplied facts") {
		t.Fatal("English grounding promotes plan approval to an owner fact")
	}
	title, _ := DefaultFor(KindPost, "titles")
	if !strings.Contains(title.Ko.Text, "제품의 추천") || !strings.Contains(title.Ko.Text, "노출 보장이 아닙니다") || !strings.Contains(title.En.Text, "product recommendations") || !strings.Contains(title.En.Text, "not a published Naver numeric penalty threshold") {
		t.Fatal("title recommendation claims unsupported platform authority")
	}
	tags, _ := DefaultFor(KindPost, "tags")
	for _, needle := range []string{"재료로 확인되는", "후순위", "태그 수를 채우려고 덧붙이지", "다른 글에 그대로 쓰지는", "요청받았을 때만"} {
		if !strings.Contains(tags.Ko.Text, needle) {
			t.Fatalf("switchable tag selection lost %q", needle)
		}
	}
	if !strings.Contains(tags.En.Text, "fill the upper bound") {
		t.Fatal("English tag advice still asks to fill a required count")
	}
}
