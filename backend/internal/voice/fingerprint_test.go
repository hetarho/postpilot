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
// a closing answer's last, deduplicated and counted.
func TestFingerprintOpeningsAndClosings(t *testing.T) {
	materials := []Material{
		post("p1", "안녕하세요! 오늘은 국수예요.\n맛있었어요.\n다음에 또 만나요!", 1),
		post("p2", "안녕하세요!   오늘은 국수예요.\n별로였어요.\n그럼 이만.", 2),
		{ID: "o", Kind: SampleKindAnswer, Part: PartOpening, CreatedAt: fingerprintAt.Add(3 * time.Minute), Text: "여러분 반가워요. 오늘도 시작해요."},
		{ID: "c", Kind: SampleKindAnswer, Part: PartClosing, CreatedAt: fingerprintAt, Text: "읽어 주셔서 고마워요. 다음에 또 만나요!"},
		{ID: "d", Kind: SampleKindAnswer, Part: PartDescription, CreatedAt: fingerprintAt, Text: "설명하는 답이에요."},
	}
	f := FingerprintOf(materials)
	if f.OpenClose.Unknown || !reflect.DeepEqual(f.OpenClose.Openings, []string{"안녕하세요! 오늘은 국수예요.", "여러분 반가워요."}) {
		t.Fatalf("openings = %q", f.OpenClose.Openings)
	}
	if !reflect.DeepEqual(f.OpenClose.Closings, []string{"다음에 또 만나요!", "그럼 이만."}) {
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
	older := post("old", repeatLines(10, "예전 글이에요!"), 0)
	newer := post("new", repeatLines(10, "새 글이에요!"), 10)
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

// VOICE-62: one comparison per item, the farthest first, the unknown last.
func TestCompareOrdersByDistance(t *testing.T) {
	voiceText := strings.Join([]string{
		"안녕하세요! 오늘은 국수예요.",
		repeatLines(20, "진짜 맛있었어요!", "저는 또 갈 거예요ㅎㅎ", "국물이 진했어요!"),
		"다음에 또 만나요!",
	}, "\n")
	voice := FingerprintOf([]Material{post("p", voiceText, 0)})

	same := Compare(voice, MeasureText(voiceText))
	if len(same) != len(Items()) {
		t.Fatalf("comparisons = %d", len(same))
	}
	for _, comparison := range same {
		if !comparison.Unknown && comparison.Distance > 1e-9 {
			t.Fatalf("a text that matches every habit is %v away on %s", comparison.Distance, comparison.Item)
		}
	}

	calm := strings.ReplaceAll(voiceText, "!", ".")
	dropped := Compare(voice, MeasureText(calm))
	if dropped[0].Item != ItemMarks || dropped[0].Headline != markExclaim || dropped[0].Distance < 0.5 {
		t.Fatalf("the farthest item after dropping the marks = %+v", dropped[0])
	}
	for i := 1; i < len(dropped); i++ {
		if !dropped[i].Unknown && !dropped[i-1].Unknown && dropped[i].Distance > dropped[i-1].Distance {
			t.Fatalf("not ordered by distance: %+v", dropped)
		}
	}

	short := Compare(voice, MeasureText("짧아요!"))
	unknownSeen := false
	for _, comparison := range short {
		if comparison.Unknown {
			unknownSeen = true
			continue
		}
		if unknownSeen {
			t.Fatalf("a known item after an unknown one: %+v", short)
		}
	}
	known := 0
	for _, comparison := range short {
		if !comparison.Unknown {
			known++
		}
	}
	if known > 2 {
		t.Fatalf("a one-sentence text showed %d items: %+v", known, short)
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
