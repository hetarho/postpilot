package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// These goldens define the current fixed prompt for a post without a template. Update them
// only for an explicitly accepted change to that fixed prompt; ordinary template work must
// remain byte-identical to this baseline.
//
// The 용도 → 템플릿 rename regenerated them exactly once: the fixed output-language line named
// 용도 / "purpose", a concept that no longer exists. Every byte-identity rule below is stated
// relative to that new baseline, the way the grounding constraint (GUIDE-16) did.
func loadGolden(t *testing.T, name string) (system, user string) {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.SplitN(string(raw), "\n@@USER@@\n", 2)
	if len(parts) != 2 {
		t.Fatalf("golden %s is malformed", name)
	}
	return parts[0], strings.TrimSuffix(parts[1], "\n")
}

// testBrief is what the template context hands over: a name and a body already expanded and
// rendered for this post's photos. A stored place slot has already become its label's text.
func testBrief() *TemplateBrief {
	return &TemplateBrief{
		Name: "정보성 식당 리뷰",
		Body: "<write>인트로를 작성합니다.</write>\n\n=========================\n네이버 지도\n\n{{사진 자리}}\n<write>이 사진에 대한 설명</write>",
	}
}

// A4: no template means no template bytes at all.
func TestWritePromptWithoutATemplateIsByteIdenticalToTheBaseline(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_template.golden")
	system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, nil)
	if system != wantSystem {
		t.Fatalf("system prompt drifted from the no-template baseline:\n--- got ---\n%s\n--- want ---\n%s", system, wantSystem)
	}
	if user != wantUser {
		t.Fatalf("user prompt drifted from the no-template baseline:\n--- got ---\n%s\n--- want ---\n%s", user, wantUser)
	}
}

// A4: the same, for revision.
func TestRevisePromptWithoutATemplateIsByteIdenticalToTheBaseline(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "revise_prompt_no_template.golden")
	system, user := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, nil, nil)
	if system != wantSystem || user != wantUser {
		t.Fatalf("revise prompt drifted from the no-template baseline:\n--- got ---\n%s\n--- want ---\n%s", system, wantSystem)
	}
}

// A5: exactly one section, in exactly one place — after the whole voice profile and before
// the per-post material — and the profile half is untouched by its presence.
func TestWritePromptAppendsOneTemplateSectionAfterTheCompleteVoiceProfile(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "write_prompt_no_template.golden")
	brief := testBrief()
	system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, brief, nil)

	// The cached profile prefix must be unchanged across posts of different templates.
	if !strings.HasPrefix(system, baseline) {
		t.Fatalf("the template changed the voice profile prefix:\n%s", system)
	}
	if user != baselineUser {
		t.Fatalf("the template leaked into the per-post material:\n%s", user)
	}

	section := strings.TrimPrefix(system, baseline)
	want := "\n\n[글 템플릿: 정보성 식당 리뷰]" +
		"\n아래 템플릿의 구성을 그대로 따르세요. " + templateLegend +
		"\n---\n" + brief.Body + "\n---" +
		"\n" + templatePrecedence
	if section != want {
		t.Fatalf("template section =\n%q\nwant\n%q", section, want)
	}
	if strings.Count(system, "[글 템플릿:") != 1 {
		t.Fatalf("the template was injected more than once:\n%s", system)
	}
}

// TMPL-21: the legend explains the two markers an unbound body carries — a photo place with its
// row size and a repeat the writer repeats per photo group — and names no photo token and no
// slot token (TMPL-37), so there is no filename for a model to copy.
func TestWritePromptExplainsThePlaceAndRepeatMarkers(t *testing.T) {
	baseline, _ := loadGolden(t, "write_prompt_no_template.golden")
	brief := testBrief()
	brief.Body = "<repeat>{{사진 자리 · 2장 묶음}}\n<write>이 사진들에 대한 설명</write></repeat>"

	system, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, brief, nil)
	section := strings.TrimPrefix(system, baseline)
	for _, line := range []string{
		"- {{사진 자리}}: 첨부 사진 가운데 이 자리 앞뒤 내용이 다루는 사진을 골라 놓는 자리입니다.",
		"- {{사진 자리 · n장 묶음}}: 같은 자리이고, 템플릿 작성자가 사진 n장 정도를 GALLERY 블록 하나로 묶어 보여 주기를 제안한 자리입니다. 몇 장을 어떤 배치로 묶을지, 묶지 않을지는 사진에 맞게 정하세요.",
		"- <repeat>…</repeat>: 스토리라인에서 이 부분에 해당하는 사진 묶음마다 안쪽을 한 번씩 되풀이해 쓰는 부분입니다. 묶음마다 안쪽의 사진 자리에는 그 묶음의 사진을 놓고, 태그 자체는 출력하지 마세요.",
	} {
		if !strings.Contains(section, line) {
			t.Fatalf("the legend lacks %q:\n%s", line, section)
		}
	}
	for _, token := range []string{"{{photo", "{{slot"} {
		if strings.Contains(section, token) {
			t.Fatalf("the template section names %q:\n%s", token, section)
		}
	}
}

// A5: the word 지침 must name exactly one thing in the prompt, the [작문 지침] section. The
// retired purpose section used it for its own field AND the guideline section used it for the
// entity, one section apart, which made "지침이 …의 요구와 충돌하면 지침을 우선하고"
// self-referential (TMPL-13). The quality rules' closing line may name 지침, because it points
// at that same section; the template section, title form included, never may.
func TestTheWord지침NeverAppearsInTheTemplateSection(t *testing.T) {
	system, _ := BuildWritePromptForLanguage(WritePromptInput{
		Language:     LanguageKorean,
		Profile:      goldenProfile(),
		Observations: goldenObservations(),
		Memo:         "MEMO",
		Title:        "TITLE",
		Photos:       []string{"IMG_1.jpg"},
		TagCount:     4,
		Template:     titleAreaBrief(),
		Guidelines:   testGuidelines(),
		QualityRules: testQualityRules(),
	})

	start, end := strings.Index(system, "[글 템플릿:"), strings.Index(system, "\n\n[작문 지침]")
	if start < 0 || end < start {
		t.Fatalf("template section at %d, guideline section at %d:\n%s", start, end, system)
	}
	if section := system[start:end]; strings.Contains(section, "지침") {
		t.Fatalf("지침 appears in the template section:\n%s", section)
	}
	for name, text := range map[string]string{
		"legend": templateLegend, "fact legend": templateFactLegend,
		"precedence": templatePrecedence, "title instruction": templateTitleInstruction,
		"revise title instruction": reviseTemplateTitleInstruction,
	} {
		if strings.Contains(text, "지침") {
			t.Errorf("the template %s says 지침: %q", name, text)
		}
	}
	if !strings.Contains(system[end:], "지침이 템플릿과 충돌하면 지침을 우선하고") {
		t.Fatalf("the guideline precedence no longer names the template:\n%s", system[end:])
	}
}

// GEN-52, TMPL-50: a frozen title area is its own instruction inside the brief, fenced above
// the body form; the revise prompt carries the same section with its own title line (TMPL-51);
// and an empty title area adds no bytes at all. The sections are compared by slicing from
// their heading, because a title-area write prompt's static prefix is the title-form variant.
func TestTemplateTitleAreaPrecedesTheBodyFence(t *testing.T) {
	brief := titleAreaBrief()
	templateSection := func(system string) string { return system[strings.Index(system, "\n\n[글 템플릿:"):] }
	write, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg"}, nil, brief, nil)
	revise, _ := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, brief, nil)

	want := "\n\n[글 템플릿: 정보성 식당 리뷰]" +
		"\n아래 템플릿의 구성을 그대로 따르세요. " + templateLegend +
		"\n" + templateTitleInstruction + "\n---\n" + brief.TitleArea + "\n---" +
		"\n---\n" + brief.Body + "\n---" +
		"\n" + templatePrecedence
	if got := templateSection(write); got != want {
		t.Fatalf("template section =\n%q\nwant\n%q", got, want)
	}
	// TMPL-51: the revision carries the same section with its own title line, which binds only a
	// request that asks to change the title.
	if want := strings.Replace(templateSection(write), templateTitleInstruction, reviseTemplateTitleInstruction, 1); templateSection(revise) != want {
		t.Fatalf("the revise section is not the write section with the revise title line:\n%q", templateSection(revise))
	}

	baseline, _ := loadGolden(t, "write_prompt_no_template.golden")
	plain, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg"}, nil, testBrief(), nil)
	withoutTitle := strings.Replace(want, "\n"+templateTitleInstruction+"\n---\n"+brief.TitleArea+"\n---", "", 1)
	if got := strings.TrimPrefix(plain, baseline); got != withoutTitle {
		t.Fatalf("an empty title area changed the section:\n%q\nwant\n%q", got, withoutTitle)
	}
}

// TMPL-51: in a revision the template's title form binds only a request that asks to change the
// title; any other revision keeps the title as it stands. The line is pinned literally, it sits
// where the write's title line sits, for either content language, and the write keeps its own.
func TestTheReviseTitleFormBindsOnlyATitleRequest(t *testing.T) {
	if reviseTemplateTitleInstruction != "수정 요청이 제목을 바꾸라고 할 때만 JSON의 title을 바로 다음 --- 사이의 제목 형식에 맞춰 쓰고, 그 밖의 수정에서는 현재 제목을 그대로 두세요. 그 뒤 --- 사이의 내용은 본문의 형식입니다." {
		t.Fatalf("the revise title line changed: %q", reviseTemplateTitleInstruction)
	}
	brief := titleAreaBrief()
	fenced := "\n" + reviseTemplateTitleInstruction + "\n---\n" + brief.TitleArea + "\n---\n---\n" + brief.Body
	korean, _ := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, brief, nil)
	english, _ := BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, brief, FrozenGuidelines{})
	for name, system := range map[string]string{"Korean": korean, "English": english} {
		if strings.Count(system, fenced) != 1 {
			t.Errorf("the %s revise prompt does not carry the revise title line in the fence once:\n%s", name, system)
		}
		if strings.Contains(system, templateTitleInstruction) {
			t.Errorf("the %s revise prompt carries the write's title line", name)
		}
	}
	write, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg"}, nil, brief, nil)
	if !strings.Contains(write, templateTitleInstruction) || strings.Contains(write, reviseTemplateTitleInstruction) {
		t.Fatal("the write prompt lost its own title line or gained the revise one")
	}
}

// A5: revision injects the same block at the same relative position, so a post keeps being
// revised under the template it was written for.
func TestRevisePromptInjectsTheSameSectionAtTheSamePosition(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "revise_prompt_no_template.golden")
	writeBaseline, _ := loadGolden(t, "write_prompt_no_template.golden")
	writeSystem, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, testBrief(), nil)
	reviseSystem, reviseUser := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, testBrief(), nil)

	if !strings.HasPrefix(reviseSystem, baseline) || reviseUser != baselineUser {
		t.Fatalf("the template moved something else in the revise prompt:\n%s", reviseSystem)
	}
	if got, want := strings.TrimPrefix(reviseSystem, baseline), strings.TrimPrefix(writeSystem, writeBaseline); got != want {
		t.Fatalf("revise section =\n%q\nwrite section =\n%q", got, want)
	}
}

// A6: what the payload froze is what the prompt uses. Editing the live row after the
// enqueue — the case a restart-resume or an explicit retry also lands in — changes nothing.
func TestTheFrozenPayloadSurvivesAnEditOrDeletionOfTheLiveRow(t *testing.T) {
	frozen := testBrief()
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Template: frozen}})
	if err != nil {
		t.Fatal(err)
	}
	// The row is edited beyond recognition and then deleted; the payload is unaffected
	// because it holds text, not a reference.
	frozen.Name = "편집된 이름"
	frozen.Body = "<write>편집된 본문</write>"

	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Template == nil || decoded.Template.Name != "정보성 식당 리뷰" ||
		!strings.Contains(decoded.Template.Body, "인트로를 작성합니다") {
		t.Fatalf("payload followed the live row: %+v", decoded.Template)
	}
	// Decoding twice is what a resume and a retry each do; both must build the same prompt.
	again, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, decoded.Template, nil)
	second, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, again.Template, nil)
	if first != second {
		t.Fatal("a resumed run built a different prompt than the first attempt")
	}
}

// A payload written before templates existed decodes as "no template" rather than failing.
func TestAPayloadWithoutATemplateFieldDecodesAsNoTemplate(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`{"target_length":800}`)} {
		decoded, err := decodeGenerationPayload(raw)
		if err != nil || decoded.Template != nil {
			t.Fatalf("payload %s decoded to %+v err=%v", raw, decoded.Template, err)
		}
	}
}

// The revision payload freezes the template the same way the generate payload does.
func TestTheRevisionPayloadFreezesTheTemplateToo(t *testing.T) {
	raw, err := encodeRevisionPayload("INSTRUCTION", testBrief(), nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := parseRevisionPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	brief := decodeTemplate(decoded.Template)
	want := testBrief()
	if brief == nil || brief.Name != want.Name || brief.Body != want.Body {
		t.Fatalf("revision payload template = %+v", brief)
	}
	// And a revision payload from before templates existed still parses.
	old, err := parseRevisionPayload([]byte(`{"instruction":"고쳐줘","save_as_rule":false}`))
	if err != nil || old.Template != nil {
		t.Fatalf("legacy revision payload = %+v err=%v", old, err)
	}
}

// fakeTemplateBriefs is the template context's published render. `deleted` makes it answer
// like a template removed after the enqueue.
type fakeTemplateBriefs struct {
	brief         TemplateBrief
	deleted       bool
	calls         int
	newWriteCalls int
	newWriteErr   error
	hasPhotos     bool
	// answers records what the enqueue handed over, so a test can prove the post's own
	// answers reached the render rather than being dropped at the seam.
	answers []TemplateAnswer
}

func (f *fakeTemplateBriefs) RenderedFor(_ context.Context, _, templateID string, hasPhotos bool, answers []TemplateAnswer) (TemplateBrief, bool, error) {
	f.calls++
	f.hasPhotos = hasPhotos
	f.answers = answers
	if f.deleted || templateID == "" {
		return TemplateBrief{}, false, nil
	}
	return f.brief, true, nil
}

func (f *fakeTemplateBriefs) RenderedForNewWrite(ctx context.Context, userID, templateID string, hasPhotos bool, answers []TemplateAnswer) (TemplateBrief, bool, error) {
	f.newWriteCalls++
	if f.newWriteErr != nil {
		return TemplateBrief{}, false, f.newWriteErr
	}
	return f.RenderedFor(ctx, userID, templateID, hasPhotos, answers)
}

func templateAwareService(t *testing.T, briefs *fakeTemplateBriefs, posts *fakePosts, jobs *fakeJobs, models *fakeModels) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())
	svc.templates = briefs
	return svc
}

// A6, end to end: the template is rendered ONCE, at the enqueue, and the run that drains the
// job prompts with what was frozen — even though the row has since been rewritten.
func TestGenerationFreezesTheTemplateAtEnqueueAndTheDrainIgnoresTheLiveRow(t *testing.T) {
	ctx := context.Background()
	briefs := &fakeTemplateBriefs{brief: *testBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review"}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := templateAwareService(t, briefs, posts, jobs, models)

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.generations) != 1 || jobs.frozen(t, 0).Template == nil {
		t.Fatalf("the start did not freeze a template: %+v", jobs.generations)
	}
	frozen := *jobs.frozen(t, 0).Template

	// Between the enqueue and the drain the template is edited and then deleted outright.
	briefs.brief = TemplateBrief{Name: "편집됨", Body: "<write>편집된 본문</write>"}
	briefs.deleted = true

	if err := svc.Generate(ctx, GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{writeMaterial: writeMaterial{Template: &frozen}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatalf("write calls = %d", len(models.calls))
	}
	system := models.calls[0].request.System
	if !strings.Contains(system, "[글 템플릿: 정보성 식당 리뷰]") || strings.Contains(system, "편집됨") {
		t.Fatalf("the drain used the live row instead of the payload:\n%s", system)
	}
	// The handler must never consult the directory: only the enqueue may.
	if briefs.calls != 1 {
		t.Fatalf("the template context was consulted %d times, want exactly 1 (the enqueue)", briefs.calls)
	}
}

// TMPL-21: the render is told only whether the post has a photo — no filename crosses the seam
// — and a video alone is no photo: a photo place and a photo repeat are about photos.
func TestTheEnqueueTellsTheRenderWhetherThePostHasAPhoto(t *testing.T) {
	for _, tc := range []struct {
		name   string
		images []Image
		want   bool
	}{
		{"photos", []Image{{Filename: "IMG_1.jpg"}, {Filename: "IMG_2.jpg", Kind: AttachmentPhoto}}, true},
		{"a photo and a video", []Image{{Filename: "clip.mp4", Kind: AttachmentVideo}, {Filename: "IMG_1.jpg"}}, true},
		{"videos only", []Image{{Filename: "clip.mp4", Kind: AttachmentVideo}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			briefs := &fakeTemplateBriefs{brief: *testBrief()}
			posts := &fakePosts{input: PostInput{
				Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review", Images: tc.images,
			}}
			// A model that can watch video, so a clip is a valid attachment to start with.
			models := videoModels()
			models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
			svc := templateAwareService(t, briefs, posts, &fakeJobs{id: "job"}, models)

			if _, err := svc.Start(context.Background(), StartRequest{
				UserID: "alice", PostSlug: "post", ObserveModel: videoObserveRef.String(), WriteModel: writeRef.String(),
			}); err != nil {
				t.Fatal(err)
			}
			if briefs.calls != 1 || briefs.hasPhotos != tc.want {
				t.Fatalf("render calls = %d, hasPhotos = %v, want one call with %v", briefs.calls, briefs.hasPhotos, tc.want)
			}
		})
	}
}

// The post's answers ride the same seam as whether it has a photo, and for the same reason: the
// freeze has to see exactly what the author had typed when the run started, so the render is
// handed them once and no handler ever reads one (TMPL-45, POST-62).
func TestTheEnqueuePassesThePostAnswersToTheRenderOnce(t *testing.T) {
	ctx := context.Background()
	briefs := &fakeTemplateBriefs{brief: *testBrief()}
	answers := []TemplateAnswer{
		{Label: "총평 별점", Text: "4.5점", Enabled: true},
		{Label: "방문일", Text: "2026-03-01", Enabled: false},
	}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review",
		Images: []Image{{Filename: "IMG_1.jpg"}}, TemplateAnswers: answers,
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := templateAwareService(t, briefs, posts, &fakeJobs{id: "job"}, models)

	if _, err := svc.Start(ctx, StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if briefs.calls != 1 {
		t.Fatalf("the render was consulted %d times, want exactly 1 (the enqueue)", briefs.calls)
	}
	if len(briefs.answers) != 2 || briefs.answers[0] != answers[0] || briefs.answers[1] != answers[1] {
		t.Fatalf("render saw answers %+v, want %+v", briefs.answers, answers)
	}
}

// TMPL-51: the title area freezes with the body on both paths, as `template.title_area`, and a
// payload written before the member existed decodes as a template with none. A brief without
// one writes no key at all, so every payload frozen before the member stays byte-identical.
func TestTheTitleAreaRidesBothPayloadsAndALegacyOneDecodesAsNone(t *testing.T) {
	brief := titleAreaBrief()
	generate, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Template: brief}})
	if err != nil {
		t.Fatal(err)
	}
	revise, err := encodeRevisionPayload("INSTRUCTION", brief, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, raw := range map[string][]byte{"generate": generate, "revise": revise} {
		var wire struct {
			Template map[string]any `json:"template"`
		}
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Template["title_area"] != brief.TitleArea {
			t.Fatalf("%s payload template = %v", name, wire.Template)
		}
	}
	decoded, err := decodeGenerationPayload(generate)
	if err != nil || decoded.Template == nil || decoded.Template.TitleArea != brief.TitleArea {
		t.Fatalf("decoded generate template = %+v, %v", decoded.Template, err)
	}
	parsed, err := parseRevisionPayload(revise)
	if err != nil || decodeTemplate(parsed.Template).TitleArea != brief.TitleArea {
		t.Fatalf("decoded revision template = %+v, %v", parsed.Template, err)
	}

	plainGenerate, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Template: testBrief()}})
	if err != nil {
		t.Fatal(err)
	}
	plainRevise, err := encodeRevisionPayload("INSTRUCTION", testBrief(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plainGenerate, []byte("title_area")) || bytes.Contains(plainRevise, []byte("title_area")) {
		t.Fatalf("a brief without a title area wrote the key:\n%s\n%s", plainGenerate, plainRevise)
	}

	legacy, err := decodeGenerationPayload([]byte(`{"template":{"name":"정보성 식당 리뷰","body":"<write>인트로</write>"}}`))
	if err != nil || legacy.Template == nil || legacy.Template.TitleArea != "" || legacy.Template.Body != "<write>인트로</write>" {
		t.Fatalf("legacy generate payload = %+v, %v", legacy.Template, err)
	}
	old, err := parseRevisionPayload([]byte(`{"instruction":"고쳐줘","save_as_rule":false,"template":{"name":"n","body":"b"}}`))
	if err != nil || decodeTemplate(old.Template).TitleArea != "" {
		t.Fatalf("legacy revision payload = %+v, %v", old.Template, err)
	}
}

// TMPL-51: the title area the enqueue froze is the one the drain prompts with, whatever the live
// row holds by then.
func TestTheFrozenTitleAreaSurvivesAnEditOfTheLiveRow(t *testing.T) {
	ctx := context.Background()
	briefs := &fakeTemplateBriefs{brief: *titleAreaBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review"}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := templateAwareService(t, briefs, posts, jobs, models)

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	want := titleAreaBrief().TitleArea
	if len(jobs.generations) != 1 || jobs.frozen(t, 0).Template == nil || jobs.frozen(t, 0).Template.TitleArea != want {
		t.Fatalf("the start froze %+v", jobs.generations)
	}
	// The payload crosses the queue as bytes, and the live row's title area is rewritten
	// between the enqueue and the drain.
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Template: jobs.frozen(t, 0).Template}})
	if err != nil {
		t.Fatal(err)
	}
	briefs.brief.TitleArea = "[편집됨] <write>다른 제목</write>"
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Generate(ctx, GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{writeMaterial: writeMaterial{Template: decoded.Template}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	system := models.calls[0].request.System
	if !strings.Contains(system, want) || strings.Contains(system, "다른 제목") {
		t.Fatalf("the drain did not prompt with the frozen title area:\n%s", system)
	}
	if briefs.calls != 1 {
		t.Fatalf("the template context was consulted %d times, want exactly 1 (the enqueue)", briefs.calls)
	}
}

// The brief is part of the frozen comparison input, so a different title area is a different
// input and a different hash. A brief with none marshals no member at all, which is what keeps
// every snapshot frozen before the member existed byte-identical.
func TestADifferentTitleAreaIsADifferentWriteSnapshot(t *testing.T) {
	ctx := context.Background()
	briefs := &fakeTemplateBriefs{brief: *titleAreaBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review"}}
	svc := templateAwareService(t, briefs, posts, &fakeJobs{id: "job"}, newFakeModels())

	snapshot := func() []byte {
		t.Helper()
		raw, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	first := snapshot()
	if !bytes.Contains(first, []byte(`"TitleArea":`)) {
		t.Fatalf("the snapshot does not carry the title area:\n%s", first)
	}
	if again := snapshot(); !bytes.Equal(again, first) {
		t.Fatal("the same title area froze two different inputs")
	}
	briefs.brief.TitleArea = "[카페] <write>카페 이름</write>"
	if other := snapshot(); bytes.Equal(other, first) {
		t.Fatal("a different title area left the frozen input identical")
	}
	briefs.brief.TitleArea = ""
	if none := snapshot(); bytes.Contains(none, []byte("TitleArea")) {
		t.Fatalf("a brief without a title area marshalled the member:\n%s", none)
	}
}
