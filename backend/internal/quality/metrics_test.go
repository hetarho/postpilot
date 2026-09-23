package quality

import (
	"fmt"
	"math"
	"reflect"
	"testing"
)

func text(paragraphs ...string) []Block {
	var blocks []Block
	for _, paragraph := range paragraphs {
		blocks = append(blocks, Block{Type: BlockText, Content: paragraph})
	}
	return blocks
}

func sample(slug, title string, nouns []string, paragraphs ...string) Sample {
	return Sample{Slug: slug, Doc: Document{Title: title, Blocks: text(paragraphs...)}, Language: LanguageKorean, Nouns: nouns}
}

func near(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil || math.Abs(*got-want) > 1e-12 {
		t.Errorf("%s = %v, want %v", name, deref(got), want)
	}
}

func deref(value *float64) any {
	if value == nil {
		return nil
	}
	return *value
}

func TestMetricIdsAreTheCanonicalAsciiIds(t *testing.T) {
	want := []Metric{"title_saturation", "cross_post_phrases", "in_post_repetition", "composition"}
	if got := Metrics(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Metrics() = %v, want %v", got, want)
	}
	for _, m := range want {
		if got, ok := ParseMetric(string(m)); !ok || got != m {
			t.Errorf("ParseMetric(%q) = %q, %v", m, got, ok)
		}
	}
	for _, id := range []string{"", "M1", "TITLE_SATURATION", "score"} {
		if _, ok := ParseMetric(id); ok {
			t.Errorf("ParseMetric(%q) accepted", id)
		}
	}
}

func TestTitleSaturationCountsTitlesContainingTheMostFrequentNoun(t *testing.T) {
	english := sample("e", "English Latte Notes", []string{"latte"})
	english.Language = LanguageEnglish
	window := []Sample{
		sample("a", "을지로 감자탕 맛집 후기", []string{"감자탕", "을지로"}),
		sample("b", "성수 감자탕 솔직 후기", []string{" 감자탕 "}),
		sample("c", "성수동 카페 라떼", []string{"카페", "라떼"}),
		// No nouns of its own, but its title still counts, by the prefix rule.
		sample("d", "감자탕집에서 먹은 점심", nil),
		english,
	}
	got := MeasureTitleSaturation(window)
	near(t, "value", got.Value, 3.0/5)
	if got.Noun != "감자탕" {
		t.Fatalf("noun = %q, want 감자탕", got.Noun)
	}

	// Only the TitleWindow most recent titles count: the 101st is outside.
	var long []Sample
	for i := 0; i < TitleWindow; i++ {
		long = append(long, sample(fmt.Sprint(i), "무제", []string{"감자탕"}))
	}
	long = append(long, sample("old", "감자탕 맛집", nil))
	got = MeasureTitleSaturation(long)
	near(t, "value over a full window", got.Value, 0)
	if got.Noun != "" {
		t.Fatalf("a noun no title contains was named: %q", got.Noun)
	}
}

func TestTitleSaturationTiesGoToTheLongerNounThenLexicographic(t *testing.T) {
	longer := MeasureTitleSaturation([]Sample{
		sample("a", "감자탕 맛집", []string{"감자", "감자탕"}),
		sample("b", "감자탕 두 그릇", nil),
	})
	if longer.Noun != "감자탕" {
		t.Fatalf("an equal count went to %q, want the longer 감자탕", longer.Noun)
	}
	lexicographic := MeasureTitleSaturation([]Sample{
		sample("a", "라떼 카페", []string{"카페", "라떼"}),
		sample("b", "카페 라떼", nil),
	})
	if lexicographic.Noun != "라떼" {
		t.Fatalf("an equal count and length went to %q, want 라떼", lexicographic.Noun)
	}
}

func TestTitleSaturationIsAbsentWithoutReturnedNouns(t *testing.T) {
	for name, window := range map[string][]Sample{
		"nothing published": nil,
		"no nouns":          {sample("a", "감자탕 맛집", nil), sample("b", "감자탕 후기", []string{})},
		"only blank nouns":  {sample("a", "감자탕 맛집", []string{"  ", ""})},
	} {
		if got := MeasureTitleSaturation(window); got.Value != nil || got.Noun != "" {
			t.Errorf("%s: %+v, want absent", name, got)
		}
	}
}

const (
	sharedRun = "을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹고"
	draftRun  = "제주 협재 해변은 에메랄드빛 물이 얕고 넓게 펼쳐져서 좋았다"
)

func TestCrossPostShareIsMeasuredAgainstTheTwentyMostRecentOthers(t *testing.T) {
	var published []Sample
	for i := 0; i < 25; i++ {
		published = append(published, sample(fmt.Sprintf("p%02d", i), "후기", nil, fmt.Sprintf("오늘은 %d번째 날이다", i)))
	}
	// The 21st post shares one run with p05 and another with a draft.
	published[20] = sample("p20", "후기", nil, sharedRun+" 왔다", draftRun)
	published[5] = sample("p05", "후기", nil, "지난주 "+sharedRun+" 돌아왔다")

	others := OthersOf(published, "p05")
	if len(others) != PostWindow {
		t.Fatalf("others = %d, want %d", len(others), PostWindow)
	}
	for _, other := range others {
		if other.Slug == "p05" {
			t.Fatal("a post is compared with itself")
		}
	}
	if others[len(others)-1].Slug != "p20" {
		t.Fatalf("the window ends at %s, want p20 (the 20th other)", others[len(others)-1].Slug)
	}
	if overlap, ok := MeasureCrossPost(published[5], others); !ok || overlap.Share <= 0 {
		t.Fatalf("p05 against its others = %+v, %v", overlap, ok)
	}

	// A draft's 20 most recent others are p00..p19, so p20 is outside its window: the run the two
	// share does not count, and the share is a real zero rather than an absent one.
	draft := sample("draft", "초안", nil, "여름에 "+draftRun)
	if overlap, ok := MeasureCrossPost(draft, published[20:21]); !ok || overlap.Share <= 0 {
		t.Fatalf("the draft against p20 alone = %+v, %v; the fixture shares nothing", overlap, ok)
	}
	draftOthers := OthersOf(published, "draft")
	if len(draftOthers) != PostWindow || draftOthers[len(draftOthers)-1].Slug != "p19" {
		t.Fatalf("draft window = %d ending at %s", len(draftOthers), draftOthers[len(draftOthers)-1].Slug)
	}
	overlap, ok := MeasureCrossPost(draft, draftOthers)
	if !ok || overlap.Share != 0 {
		t.Fatalf("the draft against p00..p19 = %+v, %v; want a real zero", overlap, ok)
	}
}

func TestTheNamedRunStandsInTheMostWindowPosts(t *testing.T) {
	widely := "성수동 골목 카페에서 고소한 라떼를 천천히 마시며 쉬었다"
	longer := "제주 협재 해변은 에메랄드빛 물이 얕고 넓게 펼쳐져서 아이와 걷기에 좋았다"
	published := []Sample{
		sample("a", "후기", nil, "첫째 "+widely),
		sample("b", "후기", nil, "둘째 "+widely, longer),
		sample("c", "후기", nil, "셋째 "+widely, "또 "+longer),
		sample("d", "후기", nil, "넷째 날은 비가 왔다"),
	}
	// The run in three posts beats the longer one in two.
	if got := namedRun(published); got != widely {
		t.Fatalf("named run = %q, want %q", got, widely)
	}

	// Equal standing: the longer run wins.
	short := "을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹고"
	tied := []Sample{
		sample("a", "후기", nil, short, longer),
		sample("b", "후기", nil, "또 "+short, "다시 "+longer),
	}
	if got := namedRun(tied); got != longer {
		t.Fatalf("named run = %q, want the longer %q", got, longer)
	}

	// Equal standing and length: the run whose oldest post was published least recently wins.
	first := "하나 둘 셋 넷 다섯 여섯 일곱 여덟"
	second := "가 나 다 라 마 바 사 아"
	earliest := []Sample{
		sample("new", "후기", nil, first),
		sample("mid", "후기", nil, first, second),
		sample("old", "후기", nil, second),
	}
	if got := namedRun(earliest); got != second {
		t.Fatalf("named run = %q, want %q, whose oldest post is older", got, second)
	}

	if got := namedRun([]Sample{sample("a", "후기", nil, "혼자 쓴 글")}); got != "" {
		t.Fatalf("a lone post named %q", got)
	}
}

func TestRepetitionShareAndTitleRelevance(t *testing.T) {
	s := sample("a", "을지로 감자탕 노포 후기", []string{"감자탕", "을지로", "국물", "라면", "후기", "감자탕"},
		"감자탕을 먹었다. 감자탕은 뼈가 많았다.", "을지로 골목 끝 노포였다. 국물이 진했다.")
	got := MeasureRepetition(s)
	// Body occurrences: 감자탕 2, 을지로 1, 국물 1, 라면 0, 후기 0 — the top noun takes 2 of 4.
	near(t, "share", got.Share, 0.5)
	if got.TopNoun != "감자탕" {
		t.Fatalf("top noun = %q", got.TopNoun)
	}
	// The title holds 감자탕, 을지로 and 후기; the body covers the first two.
	near(t, "title relevance", got.TitleRelevance, 2.0/3)

	// A noun the title holds and the body never uses: relevance is a real zero, the share absent.
	absentBody := MeasureRepetition(sample("b", "라면 맛집", []string{"라면"}, "오늘은 쉬었다."))
	if absentBody.Share != nil || absentBody.TopNoun != "" {
		t.Fatalf("share with no body occurrence = %v", deref(absentBody.Share))
	}
	near(t, "relevance with no body occurrence", absentBody.TitleRelevance, 0)

	// A title naming none of the nouns has no relevance to measure.
	noTitle := MeasureRepetition(sample("c", "오늘의 기록", []string{"감자탕"}, "감자탕을 먹었다."))
	if noTitle.TitleRelevance != nil {
		t.Fatalf("relevance with no title noun = %v", deref(noTitle.TitleRelevance))
	}
	near(t, "share of a single noun", noTitle.Share, 1)
}

func TestRepetitionIsAbsentWithoutNouns(t *testing.T) {
	for name, nouns := range map[string][]string{"nil": nil, "empty": {}, "blank": {" ", ""}} {
		if got := MeasureRepetition(sample("a", "감자탕 후기", nouns, "감자탕을 먹었다.")); !reflect.DeepEqual(got, Repetition{}) {
			t.Errorf("%s nouns: %+v, want no M3", name, got)
		}
	}
}

func TestCompositionCountsCharactersPhotosTypesAndSentences(t *testing.T) {
	doc := Document{Title: "을지로 감자탕 후기", Blocks: []Block{
		{Type: BlockText, Content: "감자탕을 먹었다. 국물이 진했다!"},
		{Type: BlockHeading, Content: "메뉴"},
		{Type: BlockImage, File: "IMG_1.jpg"},
		{Type: BlockImage, File: "  "},
		{Type: BlockVideo, File: "clip.mp4"},
		{Type: BlockList, Items: []string{"감자탕 대", "볶음밥"}},
		{Type: BlockText, Content: "{{slot:1}}"},
		{Type: BlockQuote, Content: " "},
	}}
	got := MeasureComposition(Sample{Slug: "a", Doc: doc, Language: LanguageKorean})
	// 18 + 2 + 5 + 3 runes; the title, a blank quote and the slot token do not count.
	if got.CharCount != 28 || got.PhotoCount != 1 {
		t.Fatalf("chars = %d, photos = %d; want 28 and 1", got.CharCount, got.PhotoCount)
	}
	// TEXT, HEADING, IMAGE, VIDEO and LIST carry something; the blank QUOTE does not.
	if got.DistinctBlockTypes != 5 {
		t.Fatalf("distinct types = %d, want 5", got.DistinctBlockTypes)
	}
	// Sentences of TEXT alone: 9 and 8 runes.
	near(t, "average sentence length", got.AvgSentenceLength, 8.5)

	english := MeasureComposition(Sample{Slug: "b", Language: LanguageEnglish, Doc: Document{Blocks: text("We ordered two lattes. The art was lovely!")}})
	near(t, "English average in words", english.AvgSentenceLength, 4)

	photos := MeasureComposition(Sample{Slug: "c", Language: LanguageKorean, Doc: Document{Blocks: []Block{{Type: BlockImage, File: "IMG_1.jpg"}}}})
	if photos.AvgSentenceLength != nil || photos.CharCount != 0 || photos.DistinctBlockTypes != 1 {
		t.Fatalf("a photo-only post = %+v", photos)
	}
}

func TestMedianIgnoresAbsentValues(t *testing.T) {
	v := func(x float64) *float64 { return &x }
	near(t, "odd", Median([]*float64{v(3), nil, v(1), v(2)}), 2)
	near(t, "even", Median([]*float64{v(0.2), v(0.4)}), 0.30000000000000004)
	near(t, "one", Median([]*float64{nil, v(7)}), 7)
	for name, values := range map[string][]*float64{"nil": nil, "empty": {}, "all absent": {nil, nil}} {
		if got := Median(values); got != nil {
			t.Errorf("%s: median = %v, want absent", name, *got)
		}
	}
}
