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

// GEN-14, GEN-51: the ticked rules are the first thing in the per-post half, closed by the one
// line that ranks them below a 지침. The stable half is byte-identical to the same input with no
// ticks — the ticks differ per post and must not move the cached prefix — and the per-post half
// is the section followed by the unticked per-post half. Ticking nothing leaves the prompt of a
// run from before the rules existed, byte for byte (POST-81).
func TestTickedQualityRulesOpenThePerPostHalf(t *testing.T) {
	rules := testQualityRules()
	portable := goldenProfile()
	portable.Portable = true
	for name, test := range map[string]struct {
		language Language
		profile  Profile
		videos   []string
	}{
		"Korean target":            {LanguageKorean, goldenProfile(), []string{"a.mp4"}},
		"English full profile":     {LanguageEnglish, goldenProfile(), nil},
		"English portable profile": {LanguageEnglish, portable, nil},
	} {
		input := WritePromptInput{
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
		}
		systemWithout, userWithout := BuildWritePromptForLanguage(input)
		input.QualityRules = rules
		system, user := BuildWritePromptForLanguage(input)
		if system != systemWithout {
			t.Errorf("%s: the ticks moved the stable half", name)
		}
		if strings.Contains(system, "[발행 글 측정 규칙]") {
			t.Errorf("%s: the section reached the stable half", name)
		}
		want := "[발행 글 측정 규칙]\n- " + rules[0] + "\n- " + rules[1] + "\n지침이 위 규칙과 충돌하면 지침을 우선하세요.\n\n" + userWithout
		if user != want {
			t.Errorf("%s: the per-post half is not the section followed by the unticked half:\n%s", name, user)
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
