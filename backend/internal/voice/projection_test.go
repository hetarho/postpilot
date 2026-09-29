package voice

import (
	"os"
	"strings"
	"testing"
)

// goldenAnalysis is the fixed analysis behind the projection goldens.
func goldenAnalysis() Analysis {
	return Analysis{
		Counted: Fingerprint{
			Sentences: 120,
			Endings:   Endings{Da: 0.28, Haeyo: 0.31, Seumnida: 0.07, Other: 0.34, Suffixes: []Suffix{{Text: "더라구요", Count: 9}, {Text: "했어요", Count: 7}}},
			Marks:     Marks{Exclaim: 0.32, Question: 0.05, Tilde: 0.10, Ellipsis: 0.03, Period: 0.40, None: 0.10, Repeat: 0.08},
			Emoji:     Emoji{Emoji: 12, Hh: 4, Kk: 0, Tears: 2},
			Shape:     Shape{AverageChars: 24.4, ParagraphAverage: 2, ParagraphMin: 1, ParagraphMax: 3, LineBreakShare: 0.95, OwnLine: true},
			OpenClose: OpenClose{Openings: []string{"안녕하세요!"}, Closings: []string{"오늘도 좋은 하루 보내세요~"}},
			Adverbs:   Adverbs{Words: []WordRate{{Word: "진짜", PerHundred: 8}, {Word: "완전", PerHundred: 5}, {Word: "근데", PerHundred: 4}}},
			Person:    Person{Jeo: 6, Dominant: PersonJeo},
			Headings:  Headings{Count: 10, EmojiShare: 0.7, QuestionShare: 0.4, Marker: "-"},
		},
		AI: AIPart{
			Impression:       "들뜬 목소리로 친구에게 말하듯 써요.",
			Tics:             []Tic{{Phrase: "진짜", When: "맛에 감탄할 때"}},
			SignaturePhrases: []string{"~더라구요", "완전 추천"},
		},
	}
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := "testdata/" + name
	if os.Getenv("VOICE_GOLDEN_REGEN") == "1" {
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got+"\n" != string(want) {
		t.Fatalf("%s moved:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// VOICE-46: a Korean target receives every known item as a plain Korean sentence with its value,
// then the AI part.
func TestTheKoreanProjectionIsPlainKorean(t *testing.T) {
	got := koreanSection(goldenAnalysis())
	checkGolden(t, "projection_ko.golden", got)
	for _, bare := range []string{"exclaim", "Da", "0.32", "per_hundred"} {
		if strings.Contains(got, bare) {
			t.Fatalf("the projection carries a bare key or number %q", bare)
		}
	}
}

// LANG-15: another target receives the portable habits alone — marks, emoji, paragraphs and
// headings — with no ending, word, AI part or excerpt.
func TestTheEnglishProjectionIsPortable(t *testing.T) {
	got := portableSection(goldenAnalysis().Counted)
	checkGolden(t, "projection_en.golden", got)
	for _, korean := range []string{"~다", "진짜", "들뜬", "안녕하세요", "'저'"} {
		if strings.Contains(got, korean) {
			t.Fatalf("the portable projection carries %q", korean)
		}
	}
}

// VOICE-46: an unknown item has no line, and a real "none" says so.
func TestTheProjectionSkipsUnknownItemsAndSaysNone(t *testing.T) {
	analysis := goldenAnalysis()
	analysis.Counted.Emoji.Unknown = true
	analysis.Counted.Adverbs = Adverbs{None: true}
	analysis.Counted.Person = Person{}
	analysis.Counted.Shape.OwnLine = false
	got := koreanSection(analysis)
	if strings.Contains(got, "이모지와 자모") {
		t.Fatal("an unknown item has a line")
	}
	for _, want := range []string{"눈에 띄게 반복하는 부사는 없습니다.", "1인칭을 거의 쓰지 않습니다.", "문장을 여러 개 이어 한 줄에 씁니다."} {
		if !strings.Contains(got, want) {
			t.Fatalf("the projection lacks %q:\n%s", want, got)
		}
	}
}
