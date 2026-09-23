package quality

import (
	"strings"
	"testing"
)

var languages = []Language{LanguageKorean, LanguageEnglish}

// The eight texts as they render, byte for byte: ② and the writing brief quote them verbatim
// (POST-81), so a wording change must be a deliberate diff here.
func TestEveryMetricHasOneKoreanAndOneEnglishRule(t *testing.T) {
	named := map[Metric]map[Language]string{
		MetricTitleSaturation:  {LanguageKorean: "감자탕", LanguageEnglish: "latte"},
		MetricCrossPostPhrases: {LanguageKorean: sharedRun, LanguageEnglish: "a quiet cafe tucked into an alley off the main road"},
	}
	want := map[Metric]map[Language]string{
		MetricTitleSaturation: {
			LanguageKorean:  "제목에 「감자탕」 단어를 넣지 마세요. 최근 발행한 글 제목에 이미 자주 쓰인 단어입니다. 억지로 비슷한 말로 바꾸지 말고, 제목을 자연스럽게 다시 짜세요.",
			LanguageEnglish: `Do not put the word "latte" in the title; it already appears in many of the account's recent published titles. Rather than forcing a near-synonym in its place, rework the title so it reads naturally.`,
		},
		MetricCrossPostPhrases: {
			LanguageKorean:  "「을지로 골목 끝 노포에서 뼈가 푸짐한 감자탕을 먹고」 문장을 그대로 쓰지 마세요. 최근 발행한 여러 글에 똑같이 들어간 문장입니다. 같은 뜻이 필요하면 이 글의 내용에 맞게 자연스럽게 새로 쓰세요.",
			LanguageEnglish: `Do not reuse the passage "a quiet cafe tucked into an alley off the main road" word for word; the same passage appears in several recent published posts. If this post needs that point, write it fresh in words that fit this post.`,
		},
		MetricInPostRepetition: {
			LanguageKorean:  "본문에서 한 명사를 계속 되풀이하지 마세요. 반복되는 곳은 읽기에 자연스러운 범위에서 다른 표현으로 바꾸거나 줄이되, 어색한 동의어를 억지로 넣지는 마세요. 제목에 쓴 대상은 본문에서도 다루세요.",
			LanguageEnglish: "Do not keep repeating one noun through the body; where it repeats, vary or drop it only as far as it still reads naturally, never forcing an awkward synonym. Cover in the body what the title names.",
		},
		MetricComposition: {
			LanguageKorean:  "본문을 문단과 사진만으로 구성하지 말고, 소제목(HEADING)·목록(LIST)·인용(QUOTE) 중 내용에 맞는 것을 섞어 서로 다른 블록 종류를 세 가지 이상 쓰세요. 내용에 맞지 않는 블록을 억지로 넣거나, 블록을 채우려고 자료에 없는 내용을 만들지는 마세요.",
			LanguageEnglish: "Do not build the body from paragraphs and photos alone: mix in whichever of HEADING, LIST and QUOTE the material fits, so the post uses at least three distinct block types. Never force a block the content does not fit, and never invent anything the source lacks to fill one.",
		},
	}
	if len(want) != len(Metrics()) {
		t.Fatalf("%d metrics pinned, want %d", len(want), len(Metrics()))
	}
	for _, m := range Metrics() {
		for _, lang := range languages {
			got, ok := RuleText(m, lang, named[m][lang])
			if !ok || got != want[m][lang] {
				t.Errorf("%s (%s):\n got %q, %v\nwant %q", m, lang, got, ok, want[m][lang])
			}
		}
	}
	if text, ok := RuleText("score", LanguageKorean, "감자탕"); ok || text != "" {
		t.Fatalf("an unknown metric rendered %q", text)
	}
}

func TestTitleAndCrossPostRulesNameTheMeasuredString(t *testing.T) {
	// Inserted verbatim, whatever it holds: inner spacing, punctuation, a verb in the formatter.
	const measured = "성수  카페 100% 라떼!"
	quoted := map[Language]string{LanguageKorean: "「" + measured + "」", LanguageEnglish: `"` + measured + `"`}
	for _, m := range []Metric{MetricTitleSaturation, MetricCrossPostPhrases} {
		for _, lang := range languages {
			text, ok := RuleText(m, lang, measured)
			if !ok || strings.Count(text, measured) != 1 || !strings.Contains(text, quoted[lang]) {
				t.Errorf("%s (%s) = %q, want %s named once", m, lang, text, quoted[lang])
			}
		}
		// No Korean particle hangs on the inserted string: the bracket closes onto a space.
		korean, _ := RuleText(m, LanguageKorean, measured)
		if !strings.Contains(korean, "」 ") {
			t.Errorf("%s: %q attaches something to the named string", m, korean)
		}
	}
	// M3 and M4 name nothing, whatever they are handed.
	for _, m := range []Metric{MetricInPostRepetition, MetricComposition} {
		for _, lang := range languages {
			bare, _ := RuleText(m, lang, "")
			handed, ok := RuleText(m, lang, measured)
			if !ok || handed != bare || strings.Contains(handed, measured) {
				t.Errorf("%s (%s) varies with the named string: %q", m, lang, handed)
			}
		}
	}
}

func TestABlankMeasuredStringRendersNoRule(t *testing.T) {
	for _, m := range []Metric{MetricTitleSaturation, MetricCrossPostPhrases} {
		for _, lang := range languages {
			for _, blank := range []string{"", " ", "\t\n", "　"} {
				if text, ok := RuleText(m, lang, blank); ok || text != "" {
					t.Errorf("%s (%s) with %q rendered %q", m, lang, blank, text)
				}
			}
		}
	}
}
