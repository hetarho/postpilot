package voice_test

import (
	"testing"

	"github.com/postpilot/backend/internal/voice"
)

func habitVoice() voice.Analysis {
	var a voice.Analysis
	a.Counted.Sentences = 50
	a.Counted.Endings.Suffixes = []voice.Suffix{{Text: "라구요", Count: 20}, {Text: "더라구요", Count: 10}}
	a.Counted.Adverbs.Words = []voice.WordRate{{Word: "진짜", PerHundred: 18}}
	a.Counted.OpenClose.Openings = []string{"안녕하세요~"}
	a.Counted.Endings.Example = voice.Example{Sentence: "저는 이게 더라구요.", MaterialID: "m1"}
	a.Counted.Adverbs.Example = voice.Example{Sentence: "진짜 맛있었어요.", MaterialID: "gone"}
	a.AI.SignaturePhrases = []string{"국물이 끝내줘요"}
	a.AI.Examples = []voice.AIExample{{Field: voice.AISignaturePhrases, Sentence: "국물이 끝내줘요!", MaterialID: "m1"}}
	return a
}

func runesOf(text string, start, end int) string { return string([]rune(text)[start:end]) }

// VOICE-77: a voice's countable habits are found in a post's prose and located by half-open
// Unicode scalar offsets into their block's text, with the voice's rate and the post's count.
func TestFindHabitPhrasesLocatesCountedHabits(t *testing.T) {
	text := "안녕하세요! 요즘 시원한 게 계속 당기더라구요. 육수가 진짜 깔끔해서…"
	blocks := []voice.Block{{Index: 2, Type: voice.BlockText, Content: text}}
	got := voice.FindHabitPhrases(habitVoice(), blocks, map[string]bool{"m1": true})
	if len(got) != 3 {
		t.Fatalf("want 3 phrases, got %+v", got)
	}
	want := []struct {
		kind  voice.HabitKind
		quote string
		rate  float64
	}{{voice.HabitOpening, "안녕하세요!", 0}, {voice.HabitEnding, "더라구요", 20}, {voice.HabitAdverb, "진짜", 18}}
	for i, w := range want {
		p := got[i]
		if p.Kind != w.kind || p.Quote != w.quote || p.VoicePerHundred != w.rate || p.BlockIndex != 2 || p.ItemIndex != -1 || p.TextCount != 1 {
			t.Errorf("phrase %d = %+v, want %+v", i, p, w)
		}
		if runesOf(text, p.Start, p.End) != w.quote {
			t.Errorf("phrase %d offsets %d-%d select %q", i, p.Start, p.End, runesOf(text, p.Start, p.End))
		}
	}
	// the longest suffix wins over 라구요, which is also a suffix of the sentence
	if got[1].Term != "더라구요" {
		t.Errorf("ending term = %q", got[1].Term)
	}
	if got[1].Example != "저는 이게 더라구요." {
		t.Errorf("ending example = %q", got[1].Example)
	}
}

// VOICE-77: a span never overlaps another (the longer wins), headings, captions and media are
// never searched, and a LIST item is located by its block and item index.
func TestFindHabitPhrasesNeverOverlapAndSkipHeadings(t *testing.T) {
	blocks := []voice.Block{
		{Index: 0, Type: voice.BlockHeading, Content: "진짜 국물이 끝내줘요"},
		{Index: 1, Type: voice.BlockText, Content: "육수가 진짜 깔끔했어요. 국물이 끝내줘요 정말"},
		{Index: 3, Type: voice.BlockList, Items: []string{"맛있다", "여기 진짜 좋더라구요"}},
	}
	got := voice.FindHabitPhrases(habitVoice(), blocks, nil)
	seen := map[[2]int]bool{}
	for _, p := range got {
		if p.BlockIndex == 0 {
			t.Errorf("heading searched: %+v", p)
		}
		for _, q := range got {
			if p != q && p.BlockIndex == q.BlockIndex && p.ItemIndex == q.ItemIndex && p.Start < q.End && q.Start < p.End {
				t.Errorf("overlap %+v %+v", p, q)
			}
		}
		seen[[2]int{p.BlockIndex, p.ItemIndex}] = true
	}
	if !seen[[2]int{1, -1}] || !seen[[2]int{3, 1}] || seen[[2]int{3, 0}] {
		t.Errorf("block/item locations = %v", seen)
	}
	var signature, adverbs int
	for _, p := range got {
		switch p.Kind {
		case voice.HabitSignature:
			signature++
			if p.TextCount != 1 {
				t.Errorf("signature count = %d (the heading is not searched)", p.TextCount)
			}
		case voice.HabitAdverb:
			adverbs++
			if p.TextCount != 2 {
				t.Errorf("adverb count = %d", p.TextCount)
			}
		}
	}
	if signature != 1 || adverbs != 2 {
		t.Errorf("signature %d adverbs %d in %+v", signature, adverbs, got)
	}
}

// VOICE-77: an overlap keeps the longer span; an equal pair keeps the earlier.
func TestFindHabitPhrasesLongerSpanWins(t *testing.T) {
	var a voice.Analysis
	a.AI.SignaturePhrases = []string{"진짜 맛있", "맛있어요"}
	a.Counted.Adverbs.Words = []voice.WordRate{{Word: "진짜", PerHundred: 5}}
	blocks := []voice.Block{{Index: 0, Type: voice.BlockText, Content: "정말 진짜 맛있어요 그리고 진짜 좋아요"}}
	got := voice.FindHabitPhrases(a, blocks, nil)
	var quotes []string
	for _, p := range got {
		quotes = append(quotes, p.Quote)
	}
	// "진짜 맛있" beats 진짜 and the overlapping "맛있어요"; the second 진짜 stands alone
	if len(quotes) != 2 || quotes[0] != "진짜 맛있" || quotes[1] != "진짜" {
		t.Errorf("quotes = %q", quotes)
	}
}

// VOICE-77, VOICE-21, VOICE-69: an example comes only from the analysis's own examples whose
// 학습 글 still exists; a synthetic voice shows its fictional sample, flagged.
func TestFindHabitPhrasesExamples(t *testing.T) {
	blocks := []voice.Block{{Index: 0, Type: voice.BlockText, Content: "국물이 끝내줘요 정말. 진짜 좋아요."}}
	find := func(a voice.Analysis, present map[string]bool, kind voice.HabitKind) voice.HabitPhrase {
		for _, p := range voice.FindHabitPhrases(a, blocks, present) {
			if p.Kind == kind {
				return p
			}
		}
		t.Fatalf("no %s phrase", kind)
		return voice.HabitPhrase{}
	}
	a := habitVoice()
	if got := find(a, map[string]bool{"m1": true}, voice.HabitSignature); got.Example != "국물이 끝내줘요!" || got.ExampleFictional {
		t.Errorf("signature example = %+v", got)
	}
	if got := find(a, map[string]bool{}, voice.HabitSignature); got.Example != "" {
		t.Errorf("withdrawn material's example = %q", got.Example)
	}
	// the adverb's own example belongs to a deleted 학습 글
	if got := find(a, map[string]bool{"m1": true}, voice.HabitAdverb); got.Example != "" {
		t.Errorf("adverb example from a deleted material = %q", got.Example)
	}
	synthetic := habitVoice()
	synthetic.Origin = voice.OriginSynthetic
	synthetic.SyntheticSample = "오늘은 어디를 다녀왔어요. 진짜 좋은 곳이었어요. 국물이 끝내줘요."
	if got := find(synthetic, nil, voice.HabitAdverb); got.Example != "진짜 좋은 곳이었어요." || !got.ExampleFictional {
		t.Errorf("synthetic adverb example = %+v", got)
	}
	if got := find(synthetic, nil, voice.HabitSignature); got.Example != "국물이 끝내줘요." || !got.ExampleFictional {
		t.Errorf("synthetic signature example = %+v", got)
	}
}

// VOICE-77: offsets count Unicode scalar values, so emoji and earlier blocks do not shift them,
// and a closing line spans its last sentence.
func TestFindHabitPhrasesOffsetsAreRunes(t *testing.T) {
	var a voice.Analysis
	a.Counted.OpenClose.Closings = []string{"다음에 또 올게요"}
	text := "😀😀 좋았어요.\n다음에 또 올게요! 안녕~"
	blocks := []voice.Block{{Index: 0, Type: voice.BlockText, Content: text}}
	got := voice.FindHabitPhrases(a, blocks, nil)
	if len(got) != 1 || got[0].Kind != voice.HabitClosing {
		t.Fatalf("got %+v", got)
	}
	if runesOf(text, got[0].Start, got[0].End) != got[0].Quote || got[0].Quote != "안녕~" {
		t.Errorf("closing = %+v", got[0])
	}
}
