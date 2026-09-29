package voice

import (
	"reflect"
	"strings"
	"testing"
)

// VOICE-61: a pasted Naver post keeps only the owner's prose — its place card, hours, hashtags,
// source line, address and a URL-only line count toward no sentence.
func TestProseLinesDropThePlaceCardAndTheHashtags(t *testing.T) {
	pasted := strings.Join([]string{
		"연남동 골목에 있는 작은 국숫집에 다녀왔어요.",
		"",
		"📍 연남국수",
		"📍️ 연남국수",
		"주소: 서울 마포구 연남로 12",
		"영업시간 11:00 - 21:00",
		"운영시간：매일",
		"전화 02-123-4567",
		"휴무 월요일",
		"주차 불가",
		"가격: 9,000원",
		"위치 연남동 끝",
		"⏰ 11시 오픈",
		"☎️ 02-000-0000",
		"서울 마포구 연남동 223-14",
		"서울특별시 마포구 동교로 1",
		"https://naver.me/xyz",
		"#연남동맛집 #국수",
		"[출처] 연남국수 후기|작성자 누구",
		"국물이 정말 진했어요! 다음에 또 갈 거예요ㅎㅎ",
		"ㅋㅋㅋ",
	}, "\n")
	got := ProseLines(pasted)
	want := []string{
		"연남동 골목에 있는 작은 국숫집에 다녀왔어요.",
		"국물이 정말 진했어요! 다음에 또 갈 거예요ㅎㅎ",
		"ㅋㅋㅋ",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("prose lines = %q, want %q", got, want)
	}
	if n := len(ProseSentences(pasted)); n != 4 {
		t.Fatalf("prose sentences = %d, want 4", n)
	}
}

// A line that only starts like a label or a 시·도 is still prose.
func TestProseLinesKeepSentencesThatMerelyLookLikeInfo(t *testing.T) {
	for _, line := range []string{
		"주소를 몰라서 한참 헤맸어요.",
		"가격이 착해서 좋았어요.",
		"서울에서 제일 맛있는 집이에요.",
		"부산 여행 둘째 날이었어요.",
		"#1 추천 메뉴는 비빔국수예요.",
	} {
		if got := ProseLines(line); len(got) != 1 {
			t.Errorf("%q was dropped", line)
		}
	}
}

// VOICE-32: 60 prose sentences and every part; N is the sentence share, held at 99 while a
// part is missing.
func TestReadinessCountsSentencesAndParts(t *testing.T) {
	sentences := func(n int) string { return strings.Repeat("정말 좋았어요.\n", n) }
	answer := func(key string, n int) Sample {
		return Sample{Kind: SampleKindAnswer, PromptKey: key, Body: sentences(n)}
	}
	post := func(n int) Sample { return Sample{Kind: SampleKindPost, Body: sentences(n)} }
	for _, test := range []struct {
		name    string
		samples []Sample
		percent int
		missing []PromptPart
	}{
		{"empty", nil, 0, []PromptPart{PartOpening, PartDescription, PartClosing}},
		{"half a post", []Sample{post(30)}, 50, []PromptPart{}},
		{"a whole post", []Sample{post(60)}, 100, []PromptPart{}},
		{"more than enough", []Sample{post(90)}, 100, []PromptPart{}},
		{"answers without a closing", []Sample{answer("opening_greeting", 30), answer("photo_food", 40)}, 99, []PromptPart{PartClosing}},
		{"answers covering every part", []Sample{answer("opening_greeting", 20), answer("situation_value", 20), answer("closing_reader", 20)}, 100, []PromptPart{}},
		{"a short answer set", []Sample{answer("opening_greeting", 3)}, 5, []PromptPart{PartDescription, PartClosing}},
		{"non-prose lines count nothing", []Sample{{Kind: SampleKindPost, Body: strings.Repeat("#태그\n", 80)}}, 0, []PromptPart{}},
	} {
		got := ReadinessOf(test.samples)
		if got.Percent != test.percent || !reflect.DeepEqual(got.MissingParts, test.missing) || got.Needed != ReadySentences {
			t.Errorf("%s: readiness = %+v, want %d%% missing %v", test.name, got, test.percent, test.missing)
		}
	}
}

// VOICE-60: 20 prompts, 4 openings, 12 descriptions (6 on a photo) and 4 closings, unique keys.
func TestThePromptSetHasItsShape(t *testing.T) {
	counts := map[PromptPart]int{}
	photos := 0
	keys := map[string]bool{}
	for _, prompt := range Prompts() {
		counts[prompt.Part]++
		if prompt.Photo {
			photos++
			if prompt.Part != PartDescription {
				t.Errorf("%s: a photo prompt outside the descriptions", prompt.Key)
			}
		}
		if keys[prompt.Key] || prompt.Text == "" {
			t.Errorf("%s: duplicate key or empty text", prompt.Key)
		}
		keys[prompt.Key] = true
	}
	if len(keys) != 20 || counts[PartOpening] != 4 || counts[PartDescription] != 12 || counts[PartClosing] != 4 || photos != 6 {
		t.Fatalf("prompt set = %v photos=%d total=%d", counts, photos, len(keys))
	}
}
