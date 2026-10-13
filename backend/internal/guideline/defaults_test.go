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
	// GEN-73: the memories default reads the 취향: label memory retrieval puts on a taste and
	// calls what it adds AI-added meaning, in both languages; it names no other 기본 지침, so a
	// rename elsewhere cannot leave it pointing at a title that is gone.
	memories, _ := DefaultFor(KindPost, "memory_impressions")
	if memories.Ko.Name != "기억을 통한 감상 추가" || !strings.Contains(memories.Ko.Text, "AI가 더한 의미") || !strings.Contains(memories.En.Text, "AI-added meaning") ||
		!strings.Contains(memories.Ko.Text, "'취향:'") || !strings.Contains(memories.En.Text, "'취향:'") || strings.Contains(memories.Ko.Text, "AI 제안 구분") {
		t.Errorf("memory_impressions = %+v", memories)
	}
	// GEN-83: the impressions default keeps the owner's own impressions and infers taste from
	// the inputs as experience; 맛있었다 alone or a photo alone settles no flavor.
	impressions, _ := DefaultFor(KindPost, "impressions")
	if impressions.Ko.Name != "내 감상은 지키고 맛은 재료로 살리기" || impressions.En.Name != "Keep my impressions, draw taste from the material" {
		t.Errorf("impressions names = %q, %q", impressions.Ko.Name, impressions.En.Name)
	}
	for _, copy := range []struct {
		text             string
		needles, absents []string
	}{
		{impressions.Ko.Text, []string{"그 뜻을 바꾸지", "메뉴·재료·조리법·사진에 보이는 모습", "경험으로 자연스럽게", "'맛있었다'만으로 특정한 맛을 정하거나", "주지 않은 평가는 만들지"}, []string{"AI가 더한 의미로 분명히 구분", "실제로 느끼거나 경험한 일처럼 쓰지"}},
		{impressions.En.Text, []string{"do not change their meaning", "cooking method", "as what the author experienced", "from 'it was good' alone", "verdict the author did not give"}, []string{"never as something the author actually felt"}},
	} {
		for _, needle := range copy.needles {
			if !strings.Contains(copy.text, needle) {
				t.Errorf("impressions text lacks %q: %q", needle, copy.text)
			}
		}
		for _, absent := range copy.absents {
			if strings.Contains(copy.text, absent) {
				t.Errorf("impressions text still forbids inferred taste with %q", absent)
			}
		}
	}
	// GUIDE-41: a photo stands alone by default, and a group is the exception whose caption names
	// one subject; a caption joining different photos' subjects splits them, whatever the template
	// place or storyline paragraph put together, in both languages.
	groups, _ := DefaultFor(KindPost, "photo_groups")
	if groups.Ko.Name != "사진은 한 장씩" || groups.En.Name != "Photos stand alone" {
		t.Errorf("photo_groups names = %q, %q", groups.Ko.Name, groups.En.Name)
	}
	// `placed` frees a group from photo-order adjacency: a late photo of one subject can join
	// its kin wherever it sits in the photo order (GEN-77).
	for _, copy := range []struct{ text, single, condition, joined, override, placed string }{
		{groups.Ko.Text, "IMAGE 블록으로 한 장씩", "대상 하나를 말할 때만 묶으세요", "'가게 입구와 첫 상차림'처럼", "'n장 묶음' 사진 자리나 스토리라인의 한 문단", "사진 순서에서 떨어져 있더라도 한자리에 함께 놓을 사진"},
		{groups.En.Text, "one at a time as IMAGE blocks", "group only when it names one subject", "\"the entrance and the first dishes served\"", "suggested group place ('n장 묶음') or one storyline paragraph", "photos you place together into one GALLERY block, wherever they sit in the photo order"},
	} {
		single, gallery := strings.Index(copy.text, copy.single), strings.Index(copy.text, "GALLERY")
		if single < 0 || gallery < single {
			t.Errorf("photo_groups text does not default to single photos: %q", copy.text)
		}
		for _, clause := range []string{copy.condition, copy.joined, copy.override, copy.placed} {
			if !strings.Contains(copy.text, clause) {
				t.Errorf("photo_groups text lacks %q: %q", clause, copy.text)
			}
		}
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
	// The title opens with the search words the material supports and closes with a short
	// implicative phrase, invents no concrete fact, and counts rewording the owner's own
	// experience as wording (GEN-49); a form's write roles are where it applies.
	for _, copy := range []struct {
		text             string
		needles, absents []string
	}{
		{title.Ko.Text, []string{"검색할 말로 시작", "업종이나 대표 메뉴를 앞에", "함축한 짧은 구절", "재료에 없는 구체적 사실은 제목에서도 만들지", "새 사실이 아니라 표현", "write·answer_write 역할"}, []string{"정보를 늘어놓지", "꼭 필요한 것만"}},
		{title.En.Text, []string{"words someone would search", "category or key menu", "short implicative phrase", "never invent a concrete fact", "wording, not a new fact", "the write and answer_write roles"}, []string{"Do not list facts", "Include only the names"}},
	} {
		for _, needle := range copy.needles {
			if !strings.Contains(strings.ToLower(copy.text), strings.ToLower(needle)) {
				t.Errorf("titles text lacks %q: %q", needle, copy.text)
			}
		}
		for _, absent := range copy.absents {
			if strings.Contains(copy.text, absent) {
				t.Errorf("titles text still carries %q", absent)
			}
		}
	}
	// GEN-50: names lead, broad category labels fill the rest of the count, a purchase or
	// sponsorship label needs the material; the revise stage line, not this text, owns revision.
	tags, _ := DefaultFor(KindPost, "tags")
	for _, copy := range []struct {
		text             string
		needles, absents []string
	}{
		{tags.Ko.Text, []string{"재료로 확인되는", "남은 개수는 '맛집', '카페', '디저트'처럼", "다른 글에 그대로 쓰지는", "구매나 협찬을 나타내는 태그는 재료에 그렇게 적혀 있을 때만"}, []string{"후순위", "채우려고 덧붙이지", "요청받았을 때만"}},
		{tags.En.Text, []string{"most specific first", "Fill the rest of the count with broad labels", "only when the material says so"}, []string{"upper bound", "rank after", "For a revision"}},
	} {
		for _, needle := range copy.needles {
			if !strings.Contains(copy.text, needle) {
				t.Errorf("tags text lacks %q", needle)
			}
		}
		for _, absent := range copy.absents {
			if strings.Contains(copy.text, absent) {
				t.Errorf("tags text still carries %q", absent)
			}
		}
	}
}
