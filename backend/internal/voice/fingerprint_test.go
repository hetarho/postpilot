package voice

import (
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

var fingerprintAt = time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

func post(id, text string, minutes int) Material {
	return Material{ID: id, Kind: SampleKindPost, CreatedAt: fingerprintAt.Add(time.Duration(minutes) * time.Minute), Text: text}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// repeatLines makes a one-sentence-per-line text of n copies of each line, in turn.
func repeatLines(n int, lines ...string) string {
	var out []string
	for i := range n {
		out = append(out, lines[i%len(lines)])
	}
	return strings.Join(out, "\n")
}

// ① endings: the class mix and the frequent sentence-final strings.
func TestFingerprintEndings(t *testing.T) {
	text := repeatLines(12,
		"오늘 연남동에 다녀왔더라구요.",
		"국물이 진하더라구요!",
		"다음에 또 가고 싶어요.",
		"사장님이 친절했습니다.",
	)
	f := MeasureText(text)
	if f.Endings.Unknown || !near(f.Endings.Haeyo, 0.75) || !near(f.Endings.Seumnida, 0.25) {
		t.Fatalf("endings = %+v", f.Endings)
	}
	if len(f.Endings.Suffixes) == 0 || f.Endings.Suffixes[0].Text != "더라구요" || f.Endings.Suffixes[0].Count != 6 {
		t.Fatalf("suffixes = %+v", f.Endings.Suffixes)
	}
	for _, suffix := range f.Endings.Suffixes {
		if suffix.Text == "라구요" || suffix.Text == "구요" {
			t.Fatalf("a suffix held by a longer one survived: %+v", f.Endings.Suffixes)
		}
	}
	if !strings.HasSuffix(stripTail(f.Endings.Example.Sentence), "더라구요") || f.Endings.Example.MaterialID != "" {
		t.Fatalf("example = %+v", f.Endings.Example)
	}
	if short := MeasureText(repeatLines(9, "좋았어요.")); !short.Endings.Unknown {
		t.Fatal("nine sentences showed endings")
	}
}

// ② sentence-final marks, with `!!` repeats kept on their sentence.
func TestFingerprintMarks(t *testing.T) {
	text := repeatLines(10,
		"정말 맛있었어요!!",
		"또 갈 거예요~",
		"여기 아시나요?",
		"조금 아쉬웠어요…",
		"그랬답니다.",
		"좋았어요!😊",
		"음...",
		"그래도 추천",
		"대박이에요!ㅎㅎ",
		"다들 가 보세요.",
	)
	f := MeasureText(text)
	if f.Sentences != 10 {
		t.Fatalf("sentences = %d, the marks split a sentence", f.Sentences)
	}
	want := map[string]float64{"exclaim": 0.3, "tilde": 0.1, "question": 0.1, "ellipsis": 0.2, "period": 0.2, "none": 0.1}
	got := map[string]float64{"exclaim": f.Marks.Exclaim, "tilde": f.Marks.Tilde, "question": f.Marks.Question, "ellipsis": f.Marks.Ellipsis, "period": f.Marks.Period, "none": f.Marks.None}
	for mark, share := range want {
		if !near(got[mark], share) {
			t.Fatalf("%s share = %v, want %v (%+v)", mark, got[mark], share, f.Marks)
		}
	}
	if !near(f.Marks.Repeat, 0.1) || f.Marks.Example.Sentence != "정말 맛있었어요!!" {
		t.Fatalf("repeat/example = %+v", f.Marks)
	}
}

// ③ emoji clusters and ㅎㅎ · ㅋㅋ · ㅠㅠ runs per 100 sentences.
func TestFingerprintEmojiAndJamo(t *testing.T) {
	answer := Material{ID: "a", Kind: SampleKindAnswer, Part: PartDescription, CreatedAt: fingerprintAt, Text: repeatLines(10,
		"짜장면 맛있어요😋😋",
		"👨‍👩‍👧 가족이랑 갔어요",
		"웃겼어요ㅋㅋㅋ",
		"좋았어요ㅎㅎ",
		"아쉬웠어요ㅠㅠ",
		"ㅜㅠ 비가 왔어요",
		"ㅎ 한 번",
		"그냥 그랬어요",
		"👍🏻 추천해요",
		"끝!",
	)}
	f := FingerprintOf([]Material{answer})
	if f.Emoji.Unknown || !near(f.Emoji.Emoji, 30) || !near(f.Emoji.Kk, 10) || !near(f.Emoji.Hh, 10) || !near(f.Emoji.Tears, 20) {
		t.Fatalf("emoji = %+v", f.Emoji)
	}
	if f.Emoji.Example.Sentence != "짜장면 맛있어요😋😋" || f.Emoji.Example.MaterialID != "a" {
		t.Fatalf("example = %+v", f.Emoji.Example)
	}
}

// ④ shape: length without trailing marks, paragraph sizes and one sentence per line.
func TestFingerprintShape(t *testing.T) {
	ownLines := MeasureText("첫째 줄이에요.\n둘째 줄이에요.\n\n셋째 줄이에요.\n넷째 줄이에요.\n다섯째 줄이에요.")
	if ownLines.Shape.Unknown || !ownLines.Shape.OwnLine || !near(ownLines.Shape.LineBreakShare, 1) {
		t.Fatalf("own-line shape = %+v", ownLines.Shape)
	}
	if ownLines.Shape.ParagraphMin != 2 || ownLines.Shape.ParagraphMax != 3 || !near(ownLines.Shape.ParagraphAverage, 2.5) {
		t.Fatalf("paragraphs = %+v", ownLines.Shape)
	}
	flowing := MeasureText("하나예요. 둘이에요. 셋이에요. 넷이에요. 다섯이에요.")
	if flowing.Shape.OwnLine || !near(flowing.Shape.LineBreakShare, 0.2) || !near(flowing.Shape.AverageChars, 4.2) {
		t.Fatalf("flowing shape = %+v", flowing.Shape)
	}
	if short := MeasureText("하나예요. 둘이에요."); !short.Shape.Unknown {
		t.Fatal("two sentences showed a shape")
	}
}

// ⑤ openings and closings: a post's first and last lines, an opening answer's first sentence and
// a closing answer's last, keeping only the lines that recur in two or more 학습 글.
func TestFingerprintOpeningsAndClosings(t *testing.T) {
	materials := []Material{
		post("p1", "안녕하세요! 오늘은 국수예요.\n맛있었어요.\n다음에 또 만나요!", 1),
		post("p2", "안녕하세요!   오늘은 국수예요.\n별로였어요.\n그럼 이만.", 2),
		{ID: "o", Kind: SampleKindAnswer, Part: PartOpening, CreatedAt: fingerprintAt.Add(3 * time.Minute), Text: "여러분 반가워요. 오늘도 시작해요."},
		{ID: "c", Kind: SampleKindAnswer, Part: PartClosing, CreatedAt: fingerprintAt, Text: "읽어 주셔서 고마워요. 다음에 또 만나요!"},
		{ID: "d", Kind: SampleKindAnswer, Part: PartDescription, CreatedAt: fingerprintAt, Text: "설명하는 답이에요."},
	}
	f := FingerprintOf(materials)
	if f.OpenClose.Unknown || !reflect.DeepEqual(f.OpenClose.Openings, []string{"안녕하세요! 오늘은 국수예요."}) {
		t.Fatalf("openings = %q", f.OpenClose.Openings)
	}
	if !reflect.DeepEqual(f.OpenClose.Closings, []string{"다음에 또 만나요!"}) {
		t.Fatalf("closings = %q", f.OpenClose.Closings)
	}
	// Newest first: p2 is newer than p1, so the shared opening is first seen in p2.
	if f.OpenClose.Example.Sentence != "안녕하세요! 오늘은 국수예요." || f.OpenClose.Example.MaterialID != "p2" {
		t.Fatalf("example = %+v", f.OpenClose.Example)
	}
	if none := FingerprintOf([]Material{{ID: "d", Kind: SampleKindAnswer, Part: PartDescription, Text: "설명뿐이에요."}}); !none.OpenClose.Unknown {
		t.Fatal("a description answer alone showed openings")
	}
}

// ⑥ the adverb lexicon, alone or with a particle, per 100 sentences; none is real from 30.
func TestFingerprintAdverbs(t *testing.T) {
	text := repeatLines(10,
		"진짜 맛있었어요.",
		"진짜로 추천해요.",
		"정말요 대박이에요.",
		"정말 좋았어요!",
		"“근데” 좀 멀어요.",
		"근데 괜찮아요.",
		"좀 비싸요.",
		"완전 만족해요.",
		"진짜예요.",
		"그냥 그래요.",
	)
	f := MeasureText(text)
	words := make([]string, 0, len(f.Adverbs.Words))
	for _, word := range f.Adverbs.Words {
		words = append(words, word.Word)
	}
	if f.Adverbs.Unknown || !reflect.DeepEqual(words, []string{"진짜", "정말", "좀", "근데"}) || !near(f.Adverbs.Words[0].PerHundred, 20) {
		t.Fatalf("adverbs = %+v", f.Adverbs)
	}
	if f.Adverbs.Rates["완전"] != 10 || f.Adverbs.Example.Sentence != "진짜 맛있었어요." {
		t.Fatalf("rates/example = %+v", f.Adverbs)
	}
	if plain := MeasureText(repeatLines(30, "밥을 먹었어요.")); plain.Adverbs.Unknown || !plain.Adverbs.None {
		t.Fatalf("thirty plain sentences = %+v", plain.Adverbs)
	}
	if few := MeasureText(repeatLines(12, "밥을 먹었어요.")); !few.Adverbs.Unknown || few.Adverbs.None {
		t.Fatalf("twelve plain sentences = %+v", few.Adverbs)
	}
}

// ⑦ 저 · 우리 · 나 forms and the dominant one.
func TestFingerprintFirstPerson(t *testing.T) {
	text := repeatLines(10,
		"저는 국수를 좋아해요.",
		"제가 고른 집이에요.",
		"우리 가족도 왔어요.",
		"제 입맛에 맞았어요.",
		"맛있었어요.",
		"저도 놀랐어요.",
		"나는 몰랐어요.",
		"또 올게요.",
		"추천해요.",
		"끝이에요.",
	)
	f := MeasureText(text)
	if f.Person.Unknown || f.Person.Dominant != PersonJeo || !near(f.Person.Jeo, 40) || !near(f.Person.Uri, 10) || !near(f.Person.Na, 10) {
		t.Fatalf("person = %+v", f.Person)
	}
	if f.Person.Example.Sentence != "저는 국수를 좋아해요." {
		t.Fatalf("example = %+v", f.Person.Example)
	}
	if none := MeasureText(repeatLines(10, "맛있었어요.")); none.Person.Dominant != "" || none.Person.Unknown {
		t.Fatalf("no first person = %+v", none.Person)
	}
}

// ⑧ headings with emoji and question marks, `-` and ① lists.
func TestFingerprintHeadingsAndLists(t *testing.T) {
	text := strings.Join([]string{
		"🍜 오늘의 메뉴",
		"",
		"국수가 정말 맛있었어요. 국물이 진했어요.",
		"",
		"가격은 어땠을까?",
		"",
		"- 비빔국수 9천원",
		"- 잔치국수 8천원",
		"① 주차는 없어요",
		"",
		"1. 다음에 또 갈 곳",
	}, "\n")
	f := MeasureText(text)
	if f.Headings.Unknown || f.Headings.Count != 2 || !near(f.Headings.EmojiShare, 0.5) || !near(f.Headings.QuestionShare, 0.5) {
		t.Fatalf("headings = %+v", f.Headings)
	}
	if f.Headings.Marker != "-" || !near(f.Headings.ListShare, 4.0/7) || f.Headings.Example.Sentence != "🍜 오늘의 메뉴" {
		t.Fatalf("lists = %+v", f.Headings)
	}
	blocks := MeasureBlocks([]Block{
		{Type: BlockHeading, Content: "오늘의 메뉴는 국수였어요"},
		{Type: BlockText, Content: "짧은 글"},
		{Type: BlockList, Items: []string{"비빔국수", "잔치국수"}},
		{Type: "IMAGE", Content: "사진 설명은 세지 않아요"},
	})
	if blocks.Headings.Count != 1 || blocks.Headings.Marker != "-" || blocks.Headings.Example.MaterialID != "" {
		t.Fatalf("block headings = %+v", blocks.Headings)
	}
	if plain := MeasureText("그냥 글이에요. 제목도 목록도 없어요."); !plain.Headings.Unknown {
		t.Fatalf("a text with neither = %+v", plain.Headings)
	}
}

// VOICE-61 through T469's prose filter: the place card, hours, hashtags and `[출처]` change nothing.
func TestAPastedNaverPostCountsTheSameWithoutItsPlaceCard(t *testing.T) {
	prose := repeatLines(12, "연남동 국숫집에 다녀왔어요!", "국물이 진짜 진했어요.", "저는 또 갈 거예요ㅎㅎ")
	pasted := strings.Join([]string{
		prose,
		"📍 연남국수",
		"영업시간 11:00 - 21:00",
		"주소: 서울 마포구 연남로 12",
		"#연남동맛집 #국수",
		"[출처] 연남국수 후기|작성자 누구",
	}, "\n")
	with := FingerprintOf([]Material{post("p", pasted, 0)})
	without := FingerprintOf([]Material{post("p", prose, 0)})
	if !reflect.DeepEqual(with, without) {
		t.Fatalf("the place card changed the fingerprint:\n%+v\n%+v", with, without)
	}
}

// Examples come from the newest 학습 글 first and name it.
func TestFingerprintExamplesNameTheNewestMaterial(t *testing.T) {
	older := post("old", "안녕하세요! 예전 글이에요.\n"+repeatLines(10, "예전 글이에요!"), 0)
	newer := post("new", "안녕하세요! 새 글이에요.\n"+repeatLines(10, "새 글이에요!"), 10)
	f := FingerprintOf([]Material{older, newer})
	for item, example := range map[Item]Example{ItemEndings: f.Endings.Example, ItemMarks: f.Marks.Example, ItemShape: f.Shape.Example, ItemOpenings: f.OpenClose.Example} {
		if example.MaterialID != "new" {
			t.Errorf("%s example = %+v", item, example)
		}
	}
	if text := MeasureText(repeatLines(10, "그냥 글이에요!")); text.Marks.Example.MaterialID != "" || text.Marks.Example.Sentence == "" {
		t.Fatalf("a text example = %+v", text.Marks.Example)
	}
}

// VOICE-62: the reported case — each item leads with the voice's dominant value and carries a
// verdict; 다름 come first, then 비슷함, then the items without one, each group in item order.
func TestCompareJudgesEachItem(t *testing.T) {
	voice := Fingerprint{
		Sentences: 100,
		Endings:   Endings{Haeyo: 0.61, Other: 0.39},
		Marks:     Marks{Exclaim: 0.3, Period: 0.7},
		Emoji:     Emoji{Hh: 2},
		Shape:     Shape{AverageChars: 20, LineBreakShare: 0.5},
		OpenClose: OpenClose{Unknown: true},
		Adverbs:   Adverbs{None: true},
		Person:    Person{Jeo: 5, Dominant: PersonJeo},
		Headings:  Headings{Unknown: true},
	}
	text := Fingerprint{
		Sentences: 20,
		Endings:   Endings{Haeyo: 0.58, Da: 0.02, Other: 0.40},
		Marks:     Marks{Exclaim: 0.25, Period: 0.75},
		Emoji:     Emoji{Emoji: 24, Hh: 2},
		Shape:     Shape{Unknown: true},
		OpenClose: OpenClose{Unknown: true},
		Adverbs:   Adverbs{Rates: map[string]float64{}},
		Person:    Person{},
		Headings:  Headings{EmojiShare: 1},
	}
	got := Compare(voice, text)
	type reading struct {
		item     Item
		headline string
		verdict  Verdict
		unknown  bool
	}
	var read []reading
	for _, comparison := range got {
		read = append(read, reading{comparison.Item, comparison.Headline, comparison.Verdict, comparison.Unknown})
	}
	want := []reading{
		{ItemEmoji, "ㅎㅎ", VerdictDifferent, false},
		{ItemEndings, "해요", VerdictSimilar, false},
		{ItemMarks, markPeriod, VerdictSimilar, false},
		{ItemShape, "average", VerdictNone, true},
		{ItemAdverbs, "", VerdictNone, false},
		{ItemPerson, "dominant", VerdictNone, false}, // 저 5/100 over 20 sentences expects 1 < CompareMinExpected
	}
	if !reflect.DeepEqual(read, want) {
		t.Fatalf("comparison = %+v\nwant %+v", read, want)
	}
}

// VOICE-62's tolerances at their edges.
func TestCompareTolerances(t *testing.T) {
	verdictOf := func(item Item, v, x Fingerprint) Verdict {
		for _, comparison := range Compare(v, x) {
			if comparison.Item == item {
				return comparison.Verdict
			}
		}
		t.Fatalf("%s left out", item)
		return VerdictNone
	}
	base := func() Fingerprint {
		return Fingerprint{Sentences: 100, Endings: Endings{Haeyo: 0.6, Da: 0.4}, Emoji: Emoji{Emoji: 10},
			Shape: Shape{AverageChars: 20, LineBreakShare: 0.5}, OpenClose: OpenClose{Openings: []string{"안녕하세요 여러분"}},
			Adverbs: Adverbs{Words: []WordRate{{"진짜", 10}, {"완전", 10}, {"근데", 1}}, Rates: map[string]float64{}},
			Person:  Person{Dominant: ""}, Headings: Headings{EmojiShare: 0.5}}
	}
	v := base()
	x := base()
	x.Endings = Endings{Haeyo: 0.45, Da: 0.55}
	if got := verdictOf(ItemEndings, v, x); got != VerdictDifferent {
		t.Errorf("another largest class = %q", got)
	}
	x.Endings = Endings{Haeyo: 0.75, Da: 0.25}
	if got := verdictOf(ItemEndings, v, x); got != VerdictSimilar {
		t.Errorf("해요 0.60 vs 0.75 = %q", got)
	}
	x.Emoji = Emoji{Emoji: 15}
	if got := verdictOf(ItemEmoji, v, x); got != VerdictSimilar {
		t.Errorf("emoji 10 vs 15 = %q", got)
	}
	x.Emoji = Emoji{Emoji: 16}
	if got := verdictOf(ItemEmoji, v, x); got != VerdictDifferent {
		t.Errorf("emoji 10 vs 16 = %q", got)
	}
	x.Shape = Shape{AverageChars: 25, LineBreakShare: 0.75}
	if got := verdictOf(ItemShape, v, x); got != VerdictSimilar {
		t.Errorf("shape at both edges = %q", got)
	}
	x.Shape = Shape{AverageChars: 26, LineBreakShare: 0.5}
	if got := verdictOf(ItemShape, v, x); got != VerdictDifferent {
		t.Errorf("shape past the length edge = %q", got)
	}
	x.OpenClose = OpenClose{Openings: []string{"안녕하세요!"}}
	if got := verdictOf(ItemOpenings, v, x); got != VerdictSimilar {
		t.Errorf("a shared opening = %q", got)
	}
	x.OpenClose = OpenClose{Closings: []string{"다음에 또 봐요"}}
	if got := verdictOf(ItemOpenings, v, x); got != VerdictDifferent {
		t.Errorf("no shared line = %q", got)
	}
	// 근데 (expected 1) is not judged; one of 진짜 · 완전 is half of the judged words.
	x.Adverbs = Adverbs{Rates: map[string]float64{"진짜": 5}}
	if got := verdictOf(ItemAdverbs, v, x); got != VerdictSimilar {
		t.Errorf("half the expected words = %q", got)
	}
	x.Adverbs = Adverbs{Rates: map[string]float64{"근데": 5}}
	if got := verdictOf(ItemAdverbs, v, x); got != VerdictDifferent {
		t.Errorf("only the unexpected word = %q", got)
	}
	x.Sentences = 10
	if got := verdictOf(ItemAdverbs, v, x); got != VerdictNone {
		t.Errorf("a text too short to expect any word = %q", got)
	}
	x.Person = Person{Dominant: PersonJeo}
	if got := verdictOf(ItemPerson, v, x); got != VerdictDifferent {
		t.Errorf("voice none, text 저 = %q", got)
	}
	v.Person, x.Sentences = Person{Jeo: 10, Dominant: PersonJeo}, 20
	if got := verdictOf(ItemPerson, v, x); got != VerdictSimilar {
		t.Errorf("저 expected 2 and used = %q", got)
	}
	x.Headings = Headings{EmojiShare: 0.8, QuestionShare: 0.3}
	if got := verdictOf(ItemHeadings, v, x); got != VerdictSimilar {
		t.Errorf("headings at the edge = %q", got)
	}
	x.Headings = Headings{EmojiShare: 0.85}
	if got := verdictOf(ItemHeadings, v, x); got != VerdictDifferent {
		t.Errorf("headings past the edge = %q", got)
	}
}

// VOICE-62 on measured texts: the voice's own text is 비슷함 wherever judged, dropping the marks
// makes ② 다름 first, and a one-sentence text leaves its items unknown without a verdict.
func TestCompareMeasuredTexts(t *testing.T) {
	voiceText := strings.Join([]string{
		"안녕하세요! 오늘은 국수예요.",
		repeatLines(20, "진짜 맛있었어요!", "저는 또 갈 거예요ㅎㅎ", "국물이 진했어요!"),
		"다음에 또 만나요!",
	}, "\n")
	voice := FingerprintOf([]Material{post("p", voiceText, 0)})

	for _, comparison := range Compare(voice, MeasureText(voiceText)) {
		if voice.Unknown(comparison.Item) || comparison.Verdict == VerdictDifferent {
			t.Fatalf("the voice's own text on %s = %+v", comparison.Item, comparison)
		}
	}

	dropped := Compare(voice, MeasureText(strings.ReplaceAll(voiceText, "!", ".")))
	if dropped[0].Item != ItemMarks || dropped[0].Verdict != VerdictDifferent || dropped[0].Headline != markExclaim {
		t.Fatalf("the first item after dropping the marks = %+v", dropped[0])
	}

	for _, comparison := range Compare(voice, MeasureText("짧아요!")) {
		if comparison.Unknown && comparison.Verdict != VerdictNone {
			t.Fatalf("an unknown item with a verdict: %+v", comparison)
		}
	}
}

// ② keeps a run of marks and a trailing emoji with the sentence it closes.
func TestSegmentSentencesKeepsTheFinalRun(t *testing.T) {
	got := SegmentSentences("좋아요!!😊 다음에 또?! 그래요~ 끝.")
	want := []string{"좋아요!!😊", "다음에 또?!", "그래요~ 끝."}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sentences = %q, want %q", got, want)
	}
}

// VOICE-62: a post is counted as its prose; its HEADING blocks feed only ⑧.
func TestMeasureBlocksCountsProseWithoutTemplateHeadings(t *testing.T) {
	f := MeasureBlocks([]Block{
		{Type: BlockHeading, Content: "📝 어쩌다 가게 됐냐면요..."},
		{Type: BlockText, Content: "안녕하세요! 오늘은 국수 먹었어요.\n정말 맛있었어요."},
		{Type: BlockHeading, Content: "🍜 오늘의 메뉴"},
		{Type: BlockText, Content: "비빔국수를 시켰어요.\n다음에 또 갈게요!"},
	})
	if f.Sentences != 5 {
		t.Fatalf("sentences = %d, a heading was counted as prose", f.Sentences)
	}
	if f.Emoji.Emoji != 0 || f.Emoji.Example.Sentence != "" {
		t.Fatalf("emoji = %+v, a heading's emoji was counted", f.Emoji)
	}
	if f.Headings.Count != 2 || !near(f.Headings.EmojiShare, 1) {
		t.Fatalf("headings = %+v", f.Headings)
	}
	if !near(f.Shape.LineBreakShare, 0.8) {
		t.Fatalf("line-break share = %v, a heading added a line", f.Shape.LineBreakShare)
	}
	if got := f.OpenClose.Openings; !reflect.DeepEqual(got, []string{"안녕하세요! 오늘은 국수 먹었어요."}) {
		t.Fatalf("openings = %q", got)
	}
	if got := f.OpenClose.Closings; !reflect.DeepEqual(got, []string{"다음에 또 갈게요!"}) {
		t.Fatalf("closings = %q", got)
	}
	if only := MeasureBlocks([]Block{{Type: BlockHeading, Content: "🍜 오늘의 메뉴"}}); !only.OpenClose.Unknown {
		t.Fatalf("a post of headings alone has opening and closing: %+v", only.OpenClose)
	}
}

// VOICE-24: ⑤ keeps a first or last line only when it recurs in two or more 학습 글.
func TestFingerprintKeepsOnlyRecurringOpeningsAndClosings(t *testing.T) {
	distinct := FingerprintOf([]Material{
		post("a", "안녕하세요~ 요즘 국수예요.\n끝.", 0),
		post("b", "오늘은 비가 와요.\n끝.", 1),
		post("c", "여러분 반가워요.\n끝.", 2),
	})
	if len(distinct.OpenClose.Openings) != 0 {
		t.Fatalf("openings = %q", distinct.OpenClose.Openings)
	}
	recurring := FingerprintOf([]Material{
		post("a", "안녕하세요~ 요즘 국수예요.\n맛있어요.", 0),
		post("b", "안녕하세요! 오늘 비가 와요.\n별로예요.", 1),
	})
	if got := recurring.OpenClose.Openings; !reflect.DeepEqual(got, []string{"안녕하세요! 오늘 비가 와요."}) || recurring.OpenClose.Example.MaterialID != "b" {
		t.Fatalf("openings = %q example = %+v", got, recurring.OpenClose.Example)
	}
	answers := FingerprintOf([]Material{
		{ID: "x", Kind: SampleKindAnswer, Part: PartOpening, CreatedAt: fingerprintAt, Text: "안녕하세요 여러분. 시작해요."},
		{ID: "y", Kind: SampleKindAnswer, Part: PartOpening, CreatedAt: fingerprintAt.Add(time.Minute), Text: "안녕하세요 친구들. 오늘은 달라요."},
	})
	if got := answers.OpenClose.Openings; !reflect.DeepEqual(got, []string{"안녕하세요 친구들."}) {
		t.Fatalf("answer openings = %q", got)
	}
	// A text keeps its own first and last line unfiltered.
	if text := MeasureText("처음 보는 줄이에요.\n마지막 줄이에요."); len(text.OpenClose.Openings) != 1 || len(text.OpenClose.Closings) != 1 {
		t.Fatalf("a text's ⑤ = %+v", text.OpenClose)
	}
}
