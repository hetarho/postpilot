package generation

import (
	"context"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// testQualityRules stands in for what T344 freezes: rule texts already rendered in the target
// language, in the order the brief lists the ticked metrics.
func testQualityRules() []string {
	return []string{
		"최근 제목에 '솔직 후기'가 자주 나옵니다. 이번 제목에는 쓰지 마세요.",
		"본문에서 한 명사를 계속 되풀이하지 말고, 제목이 말하는 내용을 본문에서 다루세요.",
	}
}

// GEN-14, GEN-51: the ticked rules are one section after the static, tag-count, language and
// video lines, and before the naturalness baseline for a Korean target or the voice profile
// for an English one, so a 지침 further down still outranks them. Ticking nothing leaves the
// prompt of a run from before the rules existed, byte for byte (POST-81).
func TestQualityRulesSectionSitsBetweenTheStaticRulesAndTheBaseline(t *testing.T) {
	rules := testQualityRules()
	section := "\n\n" + qualityRulesHeading + "\n- " + rules[0] + "\n- " + rules[1] + "\n" + qualityRulesPrecedence
	englishLanguageLine := "This requirement overrides conflicting language instructions in the voice profile, template, memo, or title hint."
	portable := goldenProfile()
	portable.Portable = true
	for name, test := range map[string]struct {
		language      Language
		profile       Profile
		videos        []string
		before, after string
	}{
		"Korean target":            {LanguageKorean, goldenProfile(), []string{"a.mp4"}, videoWriteInstructions, "\n\n" + NaturalnessBaseline + "\n\n[스타일가이드]\n"},
		"English full profile":     {LanguageEnglish, goldenProfile(), nil, englishLanguageLine, "\n\n[스타일가이드]\n"},
		"English portable profile": {LanguageEnglish, portable, nil, englishLanguageLine, "\n\n[휴대 가능한 말투 프로필 / Portable voice profile]\n"},
	} {
		system, user := BuildWritePromptForLanguage(WritePromptInput{
			Language:     test.language,
			Profile:      test.profile,
			Observations: goldenObservations(),
			Memo:         "memo",
			Title:        "title",
			Photos:       []string{"IMG_1.jpg"},
			Videos:       test.videos,
			TagCount:     4,
			Template:     testBrief(),
			Guidelines:   testGuidelines(),
			QualityRules: rules,
		})
		if got := strings.Count(system, qualityRulesHeading); got != 1 {
			t.Errorf("%s: the section appears %d times", name, got)
		}
		if !strings.Contains(system, test.before+section+test.after) {
			t.Errorf("%s: the section is not directly between %q and %q:\n%s", name, test.before, test.after, system)
		}
		if strings.Contains(user, qualityRulesHeading) {
			t.Errorf("%s: the section reached the per-post half", name)
		}
	}

	wantSystem, wantUser := loadGolden(t, "write_prompt_no_template.golden")
	for name, none := range map[string][]string{"nil": nil, "empty": {}} {
		system, user := BuildWritePromptForLanguage(WritePromptInput{
			Language:     LanguageKorean,
			Profile:      goldenProfile(),
			Observations: goldenObservations(),
			Memo:         "MEMO 본문",
			Title:        "가제 TITLE",
			Photos:       []string{"IMG_1.jpg", "IMG_2.jpg"},
			TagCount:     post.TagCountRange.Default,
			QualityRules: none,
		})
		if system != wantSystem || user != wantUser {
			t.Errorf("%s quality rules moved the prompt off its golden", name)
		}
	}
	// The one thing that outranks a quality rule is named, and it is the one thing 지침 means.
	if !strings.Contains(qualityRulesPrecedence, "지침") {
		t.Errorf("the closing line does not say a 지침 outranks the rules: %q", qualityRulesPrecedence)
	}
}

// POST-81: the ticks are a generation option of the write pass. A revision keeps unrelated
// sentences verbatim and a rule sweep would rewrite them, so no revise prompt carries the
// section, whatever the post's frozen input holds.
func TestTheRevisePromptCarriesNoQualityRules(t *testing.T) {
	needles := append([]string{qualityRulesHeading, qualityRulesPrecedence}, testQualityRules()...)
	for name, prompt := range map[string][2]string{
		"Korean":  toPair(BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "고쳐줘", nil, testBrief(), testGuidelines())),
		"English": toPair(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, testBrief(), testGuidelines())),
	} {
		for _, half := range prompt {
			for _, needle := range needles {
				if strings.Contains(half, needle) {
					t.Errorf("the %s revise prompt carries %q", name, needle)
				}
			}
		}
	}

	// End to end: a revision of a post whose input carries ticked rules sends none of them.
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body"), QualityRules: testQualityRules(),
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: `{"title":"제목","summary":"요약","tags":["a"],"blocks":[{"type":"TEXT","content":"고친 본문"}]}`}, nil
	}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	request := models.calls[0].request
	for _, needle := range needles {
		if strings.Contains(request.System, needle) || strings.Contains(request.Messages[0].Parts[0].Text, needle) {
			t.Errorf("the revise request carries %q", needle)
		}
	}
}

func toPair(system, user string) [2]string { return [2]string{system, user} }
