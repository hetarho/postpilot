package quality

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

func TestAKoreanEojeolContainsANounItStartsWith(t *testing.T) {
	body := Tokens("을지로 노포에서 감자탕을 먹었다. 감자탕은 뼈가 푸짐했고 왕감자탕 메뉴도 따로 있었다.")
	for name, test := range map[string]struct {
		noun string
		want int
	}{
		"a particle never hides the noun":   {"감자탕", 2},
		"a compound starting elsewhere":     {"왕감자탕", 1},
		"the noun is its whole 어절":          {"을지로", 1},
		"a noun that stands nowhere":        {"라떼", 0},
		"an 어절 is not contained in its end": {"탕", 0},
	} {
		if got := Occurrences(body, Tokens(test.noun), LanguageKorean); got != test.want {
			t.Errorf("%s: Occurrences(%q) = %d, want %d", name, test.noun, got, test.want)
		}
	}
	// Deliberately loose: an 어절 holds every noun it starts with.
	for _, noun := range []string{"제주", "제주도"} {
		if !Contains(Tokens("제주도에서 사흘을 보냈다"), Tokens(noun), LanguageKorean) {
			t.Errorf("제주도에서 does not contain %q", noun)
		}
	}
	if Occurrences(body, nil, LanguageKorean) != 0 || Occurrences(body, Tokens("  "), LanguageKorean) != 0 {
		t.Error("an empty noun stands somewhere")
	}
	// Decomposed input matches its composed noun, and the other way round.
	if !Contains(Tokens(norm.NFD.String("감자탕을 먹었다")), Tokens("감자탕"), LanguageKorean) ||
		!Contains(Tokens("감자탕을 먹었다"), Tokens(norm.NFD.String("감자탕")), LanguageKorean) {
		t.Error("an NFD form did not match its NFC noun")
	}
}

func TestAnEnglishWordContainsANounOnlyWhenEqualIgnoringCase(t *testing.T) {
	body := Tokens("We ordered two lattes at a small café in Seongsu. The Latte art was lovely, and the latte itself was smooth!")
	for name, test := range map[string]struct {
		noun string
		want int
	}{
		"equal ignoring case":        {"latte", 2},
		"a plural is another word":   {"lattes", 1},
		"a prefix is not a word":     {"lat", 0},
		"an accented word as itself": {"café", 1},
	} {
		if got := Occurrences(body, Tokens(test.noun), LanguageEnglish); got != test.want {
			t.Errorf("%s: Occurrences(%q) = %d, want %d", name, test.noun, got, test.want)
		}
	}
	// The same text under the Korean rule would count the plural as the noun: the rule follows
	// the language, not the script.
	if got := Occurrences(body, Tokens("latte"), LanguageKorean); got != 2 {
		t.Errorf("the Korean rule over this text = %d, want 2 (case-sensitive prefix)", got)
	}
}

func TestAMultiTokenNounIsContainedAcrossConsecutiveEojeol(t *testing.T) {
	noun := Tokens("제주 흑돼지")
	for text, want := range map[string]int{
		"제주 흑돼지를 먹으러 협재 해변 근처로 갔다":  1,
		"제주 흑돼지 골목에서 제주 흑돼지를 또 먹었다": 2,
		"제주에서 흑돼지를 먹었다":             0, // the leading token must stand as written
		"흑돼지를 제주 식당에서 먹었다":          0, // not consecutive
		"제주 왕흑돼지를 먹었다":              0, // the last token must start with the noun's
	} {
		if got := Occurrences(Tokens(text), noun, LanguageKorean); got != want {
			t.Errorf("%q: Occurrences = %d, want %d", text, got, want)
		}
	}
	if got := Occurrences(Tokens("The Iced Latte was cold, the iced lattes colder."), Tokens("iced latte"), LanguageEnglish); got != 1 {
		t.Errorf("English multi-word noun = %d, want 1", got)
	}
	// A noun longer than the text stands nowhere.
	if Contains(Tokens("제주"), noun, LanguageKorean) {
		t.Error("a two-token noun stood in one token")
	}
}

func TestAMissingContentLanguageMeasuresAsKorean(t *testing.T) {
	if got := LanguageOf(nil); got != LanguageKorean {
		t.Fatalf("LanguageOf(nil) = %q, want Korean", got)
	}
	english := LanguageEnglish
	if got := LanguageOf(&english); got != LanguageEnglish {
		t.Fatalf("LanguageOf(en) = %q", got)
	}
	// Measured as Korean, a legacy post's particles still count as its noun.
	if !Contains(Tokens("성수동 카페에서 라떼를 마셨다"), Tokens("라떼"), LanguageOf(nil)) {
		t.Error("a post with no content language did not take the Korean rule")
	}
}

// Review F10: a trailing emoji no longer hides the word it follows.
func TestAnEnglishWordContainsANounBeforeATrailingEmoji(t *testing.T) {
	if !Contains(Tokens("I love latte☕\ufe0f"), Tokens("latte"), LanguageEnglish) {
		t.Fatal("an emoji-tailed word did not contain its noun")
	}
}
