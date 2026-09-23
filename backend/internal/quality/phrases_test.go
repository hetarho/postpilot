package quality

import (
	"fmt"
	"reflect"
	"testing"

	"golang.org/x/text/unicode/norm"
)

func phraseTexts(phrases []Phrase) []string {
	out := make([]string, len(phrases))
	for i, phrase := range phrases {
		out[i] = phrase.Text
	}
	return out
}

func TestExtractPhrases(t *testing.T) {
	many := make([]string, 0, 60)
	for i := 0; i < 60; i++ {
		many = append(many, fmt.Sprintf("단어%02d 뒤%02d", i, i))
	}
	for name, test := range map[string]struct {
		units []string
		want  []string
	}{
		"a single token is never a phrase": {[]string{"카페", "라떼"}, []string{}},
		// Six words give runs of at most five, and each five-word run already says its shorter ones.
		"five words at most":                  {[]string{"하나 둘 셋 넷 다섯 여섯"}, []string{"둘 셋 넷 다섯 여섯", "하나 둘 셋 넷 다섯"}},
		"a run of stopwords alone is dropped": {[]string{"그리고 그러나", "그리고 그러나"}, []string{}},
		"a stopword inside a run stays":       {[]string{"정말 맛있는 곳", "정말 맛있는 곳"}, []string{"정말 맛있는 곳"}},
		"one count per title or description":  {[]string{"성수 카페 성수 카페", "성수 카페"}, []string{"성수 카페", "성수 카페 성수 카페"}},
		"a longer run with the same count subsumes its parts": {
			[]string{"성수 카페 라떼", "성수 카페 라떼", "성수 카페 쿠키"},
			[]string{"성수 카페", "성수 카페 라떼", "성수 카페 쿠키"},
		},
		// Lexicographic order alone would put 가 바 사 first.
		"ties go to fewer words, then lexicographic": {
			[]string{"라 마", "가 바 사", "나 다"}, []string{"나 다", "라 마", "가 바 사"},
		},
		"English, stopwords matched without case": {
			[]string{"The Best Coffee", "the best coffee", "The and"},
			[]string{"The Best Coffee", "the best coffee"},
		},
		"a punctuation-only field breaks a run": {
			[]string{"성수 카페 | 분위기 좋은", "성수 카페 - 분위기 좋은"},
			[]string{"분위기 좋은", "성수 카페"},
		},
		"NFD input composes": {[]string{norm.NFD.String("성수 카페"), "성수 카페"}, []string{"성수 카페"}},
	} {
		got := phraseTexts(ExtractPhrases(test.units))
		if !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: %q, want %q", name, got, test.want)
		}
	}

	// Sixty distinct phrases once each: the list stops at fifty, in rank order.
	cut := ExtractPhrases(many)
	if len(cut) != PhraseListMax || cut[0].Text != "단어00 뒤00" || cut[PhraseListMax-1].Text != "단어49 뒤49" {
		t.Fatalf("cut = %d phrases from %q to %q", len(cut), cut[0].Text, cut[len(cut)-1].Text)
	}
	// A phrase twice in one title still counts once for it: two units, a count of two.
	if twice := ExtractPhrases([]string{"성수 카페 성수 카페", "성수 카페"}); twice[0] != (Phrase{Text: "성수 카페", Tokens: 2, Count: 2}) {
		t.Fatalf("a repeat inside one unit = %+v", twice[0])
	}
	// Counts and word counts ride along.
	ranked := ExtractPhrases([]string{"성수 카페 라떼", "성수 카페 라떼", "성수 카페 쿠키"})
	if ranked[0] != (Phrase{Text: "성수 카페", Tokens: 2, Count: 3}) || ranked[1] != (Phrase{Text: "성수 카페 라떼", Tokens: 3, Count: 2}) {
		t.Fatalf("ranked = %+v", ranked)
	}
}
