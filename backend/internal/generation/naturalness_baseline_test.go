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
	if !strings.HasPrefix(write, WritePrompt+"\ntitle, 한 줄 summary, 정확히 4개의 tags, blocks를 반환하세요."+
		"\n출력 언어는 한국어입니다. title, summary, tags, 모든 본문, IMAGE alt와 caption을 한국어로 작성하세요. 말투 프로필, 템플릿, 메모, 가제의 언어 지시가 충돌해도 이 출력 언어를 우선하세요."+section) {
		t.Fatal("write baseline moved outside the static task/format prefix")
	}
	if !strings.HasPrefix(revise, RevisePrompt+"\n태그를 바꾸라는 요청이면 정확히 4개로 유지하세요."+
		"\n현재 콘텐츠 언어인 한국어를 유지하세요. 번역은 수정 작업의 범위가 아닙니다. 번역을 요구하거나 다른 언어로 바꾸라는 요청은 따르지 말고 나머지 유효한 수정만 최소한으로 반영하세요."+section) {
		t.Fatal("revise baseline moved outside the static task/format prefix")
	}
}

func TestMemoNamingAuthorityIsInWritePromptsOnly(t *testing.T) {
	for name, test := range map[string]struct {
		prompt    string
		grounding string
		scope     string
		altitude  string
		naming    string
	}{
		"Korean": {
			prompt:    firstOf(BuildWritePrompt(Profile{}, nil, "memo", "title", nil, nil, nil, nil)),
			grounding: koreanGrounding,
			scope:     koreanGroundingWriteScope,
			altitude:  koreanAltitude,
			naming:    koreanNaming,
		},
		"English": {
			prompt:    firstOf(BuildWritePromptForLanguage(LanguageEnglish, Profile{}, nil, "memo", "title", nil, nil, nil, 4, nil, nil, nil, nil, nil)),
			grounding: englishGrounding,
			scope:     englishGroundingWriteScope,
			altitude:  englishAltitude,
			naming:    englishNaming,
		},
	} {
		if strings.Count(test.prompt, test.naming) != 1 {
			t.Errorf("%s write prompt does not contain the naming rule exactly once", name)
		}
		wantLines := test.grounding + " " + test.scope + "\n" + test.altitude + "\n" + test.naming + "\n"
		if !strings.Contains(test.prompt, wantLines) {
			t.Errorf("%s grounding, altitude and naming lines are not in that order, each on its own line", name)
		}
	}

	for name, test := range map[string]struct {
		prompt string
		naming string
	}{
		"Korean": {
			prompt: firstOf(BuildRevisePrompt(Profile{}, *revisionContent("body"), nil, "shorten", nil, nil, nil)),
			naming: koreanNaming,
		},
		"English": {
			prompt: firstOf(BuildRevisePromptForLanguage(LanguageEnglish, Profile{}, *revisionContent("body"), nil, "shorten", nil, 4, nil, nil)),
			naming: englishNaming,
		},
	} {
		if strings.Contains(test.prompt, test.naming) {
			t.Errorf("%s revise prompt contains the write-only naming rule", name)
		}
	}
}

// GEN-47: the altitude rule reaches the write prompt and nothing else. The revise pass holds
// no observations to stay above, and the two observe passes are the ones whose whole job is to
// enumerate what is in a frame — telling either of them not to describe would be a bug.
func TestAltitudeRuleIsInWritePromptsOnly(t *testing.T) {
	for name, test := range map[string]struct {
		prompt   string
		altitude string
	}{
		"Korean bare":  {prompt: firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, nil, nil)), altitude: koreanAltitude},
		"Korean full":  {prompt: firstOf(BuildWritePrompt(goldenProfile(), goldenObservations(), "memo", "title", nil, nil, testBrief(), testGuidelines())), altitude: koreanAltitude},
		"English bare": {prompt: firstOf(BuildWritePromptForLanguage(LanguageEnglish, goldenProfile(), nil, "memo", "title", nil, nil, nil, 4, nil, nil, nil, nil, nil)), altitude: englishAltitude},
	} {
		if strings.Count(test.prompt, test.altitude) != 1 {
			t.Errorf("%s write prompt carries the altitude rule %d times", name, strings.Count(test.prompt, test.altitude))
		}
	}

	for name, test := range map[string]struct {
		prompt   string
		altitude string
	}{
		"Korean revise":  {prompt: firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, testBrief(), testGuidelines())), altitude: koreanAltitude},
		"English revise": {prompt: firstOf(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, nil, nil)), altitude: englishAltitude},
	} {
		if strings.Contains(test.prompt, test.altitude) {
			t.Errorf("%s prompt contains the write-only altitude rule", name)
		}
	}

	for name, prompt := range map[string]string{"photo observe": ObservePrompt, "video observe": ObserveVideoPrompt} {
		for _, altitude := range []string{koreanAltitude, englishAltitude} {
			if strings.Contains(prompt, altitude) {
				t.Errorf("the %s prompt gained the altitude rule", name)
			}
		}
	}
}

// GEN-16: only the scope clause differs between the passes. The core prohibition is the same
// bytes in the write and the revise prompt, which is what one shared constant is for — the
// altitude rule sits after that shared line and must not have split it.
func TestGroundingCoreIsByteIdenticalInWriteAndRevisePrompts(t *testing.T) {
	for name, test := range map[string]struct {
		write, revise           string
		core                    string
		writeScope, reviseScope string
	}{
		"Korean": {
			write:       firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, nil, nil)),
			revise:      firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, nil, nil)),
			core:        koreanGrounding,
			writeScope:  koreanGroundingWriteScope,
			reviseScope: koreanGroundingReviseScope,
		},
		"English": {
			write:       firstOf(BuildWritePromptForLanguage(LanguageEnglish, goldenProfile(), nil, "memo", "title", nil, nil, nil, 4, nil, nil, nil, nil, nil)),
			revise:      firstOf(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, nil, nil)),
			core:        englishGrounding,
			writeScope:  englishGroundingWriteScope,
			reviseScope: englishGroundingReviseScope,
		},
	} {
		writeLine, reviseLine := lineContaining(test.write, test.core), lineContaining(test.revise, test.core)
		if writeLine == "" || reviseLine == "" {
			t.Fatalf("%s: the grounding core is missing from the write or the revise prompt", name)
		}
		if !strings.HasPrefix(writeLine, test.core+" ") || !strings.HasPrefix(reviseLine, test.core+" ") {
			t.Fatalf("%s: the grounding core no longer opens its own line in both passes", name)
		}
		if writeLine != test.core+" "+test.writeScope {
			t.Errorf("%s write grounding line = %q", name, writeLine)
		}
		if reviseLine != test.core+" "+test.reviseScope {
			t.Errorf("%s revise grounding line = %q", name, reviseLine)
		}
	}
}

func lineContaining(prompt, needle string) string {
	for _, line := range strings.Split(prompt, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

// The pre-naturalness goldens are the baseline the fixed-text additions are stated against:
// job 36's stylistic section, job 35's grounding line, T041's write-only naming line, T287's
// write-only altitude line, and T324's write-only title, tag and nouns lines with the nouns
// member of the answer shape.
// Removing exactly those additions leaves the legacy bytes, which keeps each delta checkable.
//
// Change 25 renamed the concept the fixed output-language line names (용도 → 템플릿) in BOTH
// the current and the legacy goldens, so this check still sees exactly two additions rather
// than reading a rename as a third one.
func TestFixedTextAdditionsAreTheOnlyGoldenDelta(t *testing.T) {
	for _, pair := range []struct {
		current string
		legacy  string
	}{
		{current: "write_prompt_no_template.golden", legacy: "write_prompt_pre_naturalness.golden"},
		{current: "revise_prompt_no_template.golden", legacy: "revise_prompt_pre_naturalness.golden"},
	} {
		currentSystem, currentUser := loadGolden(t, pair.current)
		legacySystem, legacyUser := loadGolden(t, pair.legacy)
		stripped := strings.Replace(currentSystem, "\n\n"+NaturalnessBaseline, "", 1)
		for _, scope := range []string{koreanGroundingWriteScope, koreanGroundingReviseScope} {
			stripped = strings.Replace(stripped, "\n"+koreanGrounding+" "+scope, "", 1)
		}
		stripped = strings.Replace(stripped, "\n"+koreanAltitude, "", 1)
		stripped = strings.Replace(stripped, "\n"+koreanNaming, "", 1)
		// T324's write-only additions: the two title prohibitions, the tag rule, the nouns rule
		// and the nouns member of the answer shape.
		stripped = strings.Replace(stripped, "\n"+koreanTitleProhibitions, "", 1)
		stripped = strings.Replace(stripped, "\n"+koreanTagRule, "", 1)
		stripped = strings.Replace(stripped, "\n"+koreanNounsRule, "", 1)
		stripped = strings.Replace(stripped, `"blocks":[],"nouns":[]}`, `"blocks":[]}`, 1)
		// The two revise-only additions, stated against the same legacy baseline.
		stripped = strings.Replace(stripped, "\n"+koreanReviseScope, "", 1)
		stripped = strings.Replace(stripped, "\n"+koreanReviseLiteral, "", 1)
		if stripped != legacySystem {
			t.Errorf("%s changed by more than the inserted baseline, grounding, altitude, naming, title, tag and nouns lines and the nouns member", pair.current)
		}
		if currentUser != legacyUser {
			t.Errorf("%s changed the per-post user material", pair.current)
		}
	}
}

func TestWriteSystemPrefixIsStableAcrossPostMaterial(t *testing.T) {
	target := 900
	profile := goldenProfile()
	template := testBrief()
	first, firstUser := BuildWritePrompt(profile, goldenObservations(), "first memo", "first title", []string{"one.jpg"}, &target, template, nil)
	second, secondUser := BuildWritePrompt(profile, []Observation{{File: "two.jpg"}}, "second memo", "second title", []string{"two.jpg"}, &target, template, nil)

	if first != second {
		t.Fatal("per-post material changed the byte-stable system prefix")
	}
	if firstUser == secondUser {
		t.Fatal("fixture error: per-post user material did not change")
	}
}
