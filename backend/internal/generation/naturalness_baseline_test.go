package generation

import (
	"strings"
	"testing"
	"unicode/utf8"
)

const naturalnessPrecedence = "말투 프로필, 활성 대조 규칙, 사용자 규칙이 이 기준선과 충돌하면 해당 프로필과 규칙을 우선하세요."

func TestNaturalnessBaselineContract(t *testing.T) {
	for family, marker := range map[string]string{
		"TEXT and revision scope":   "수정에서는 요청 밖의 기존 문장을 그대로 두세요",
		"non-TEXT exclusion":        "제목·요약·HEADING·LIST에는 적용하지 말고",
		"antithesis cap":            "A가 아니라 B",
		"cleft closer":              "핵심은 …이다",
		"formulaic closer":          "결국 …로 이어진다",
		"reason closer":             "~하는 이유다",
		"vague future":              "향후·앞으로",
		"empty concession":          "과제도 남아 있다",
		"obligation ending cap":     "~해야 한다",
		"connective-ending comma":   "-고/-며/-지만/-면서/-아서",
		"sentence-length variation": "짧은 문장과 긴 복문",
		"sentence-form variation":   "단문과 복문",
		"generic verbs":             "확대·강화·개선·확보·구축",
		"invented metaphors":        "잠식·청사진·신호탄",
		"abstract noun chains":      "~적 명사",
		"rhetorical decoration":     "수사·경구",
	} {
		if !strings.Contains(NaturalnessBaseline, marker) {
			t.Errorf("%s marker %q is missing", family, marker)
		}
	}
	for _, rejected := range []string{
		"에 대해", "통해", "것이다",
		"문장 첫머리", "문장 처음", "문장 시작", "첫 단어", "문두", "접속사",
		"그리고", "그러나", "하지만", "그런데", "또한", "반면", "따라서", "그러므로",
	} {
		if strings.Contains(NaturalnessBaseline, rejected) {
			t.Errorf("rejected folk-rule marker %q is present", rejected)
		}
	}
	if !strings.HasPrefix(NaturalnessBaseline, "[한국어 자연 문체 기준선]\n") {
		t.Fatal("baseline header changed")
	}
	if !strings.HasSuffix(NaturalnessBaseline, naturalnessPrecedence) {
		t.Fatal("precedence must close the baseline section")
	}
	if got := utf8.RuneCountInString(NaturalnessBaseline); got > 700 {
		t.Fatalf("baseline is %d runes, want at most 700", got)
	}
}

func TestNaturalnessBaselineIsSharedByWriteAndRevise(t *testing.T) {
	write, _ := BuildWritePrompt(Profile{}, nil, "memo", "title", nil, nil, nil, nil)
	revise, _ := BuildRevisePrompt(Profile{}, *revisionContent("body"), nil, "shorten", nil, nil, nil)
	section := "\n\n" + NaturalnessBaseline + "\n\n[스타일가이드]\n"

	for name, prompt := range map[string]string{"write": write, "revise": revise} {
		if strings.Count(prompt, NaturalnessBaseline) != 1 {
			t.Errorf("%s prompt does not contain exactly one baseline", name)
		}
		if !strings.Contains(prompt, section) {
			t.Errorf("%s prompt does not place the complete baseline before the styleguide", name)
		}
	}
	if !strings.HasPrefix(write, WritePrompt+"\ntitle, 한 줄 summary, 3–6개의 tags, blocks를 반환하세요."+
		"\n출력 언어는 한국어입니다. title, summary, tags, 모든 본문, IMAGE alt와 caption을 한국어로 작성하세요. 말투 프로필, 용도, 메모, 가제의 언어 지시가 충돌해도 이 출력 언어를 우선하세요."+section) {
		t.Fatal("write baseline moved outside the static task/format prefix")
	}
	if !strings.HasPrefix(revise, RevisePrompt+
		"\n현재 콘텐츠 언어인 한국어를 유지하세요. 번역은 수정 작업의 범위가 아닙니다. 번역을 요구하거나 다른 언어로 바꾸라는 요청은 따르지 말고 나머지 유효한 수정만 최소한으로 반영하세요."+section) {
		t.Fatal("revise baseline moved outside the static task/format prefix")
	}
}

// The pre-naturalness goldens are the pre-change baseline both fixed-text additions are
// stated against: job 36's stylistic section and job 35's grounding line. Removing exactly
// those two leaves the legacy bytes, which is what keeps "one deliberate delta each" checkable.
func TestFixedTextAdditionsAreTheOnlyGoldenDelta(t *testing.T) {
	for _, pair := range []struct {
		current string
		legacy  string
	}{
		{current: "write_prompt_no_purpose.golden", legacy: "write_prompt_pre_naturalness.golden"},
		{current: "revise_prompt_no_purpose.golden", legacy: "revise_prompt_pre_naturalness.golden"},
	} {
		currentSystem, currentUser := loadGolden(t, pair.current)
		legacySystem, legacyUser := loadGolden(t, pair.legacy)
		stripped := strings.Replace(currentSystem, "\n\n"+NaturalnessBaseline, "", 1)
		for _, scope := range []string{koreanGroundingWriteScope, koreanGroundingReviseScope} {
			stripped = strings.Replace(stripped, "\n"+koreanGrounding+" "+scope, "", 1)
		}
		if stripped != legacySystem {
			t.Errorf("%s changed by more than the inserted baseline and grounding line", pair.current)
		}
		if currentUser != legacyUser {
			t.Errorf("%s changed the per-post user material", pair.current)
		}
	}
}

func TestWriteSystemPrefixIsStableAcrossPostMaterial(t *testing.T) {
	target := 900
	profile := goldenProfile()
	purpose := testBrief()
	first, firstUser := BuildWritePrompt(profile, goldenObservations(), "first memo", "first title", []string{"one.jpg"}, &target, purpose, nil)
	second, secondUser := BuildWritePrompt(profile, []Observation{{File: "two.jpg"}}, "second memo", "second title", []string{"two.jpg"}, &target, purpose, nil)

	if first != second {
		t.Fatal("per-post material changed the byte-stable system prefix")
	}
	if firstUser == secondUser {
		t.Fatal("fixture error: per-post user material did not change")
	}
}
