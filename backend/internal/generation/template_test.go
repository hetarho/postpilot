package generation

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// These goldens define the current fixed prompt for a post without a template. Update them
// only for an explicitly accepted change to that fixed prompt; ordinary template work must
// remain byte-identical to this baseline.
//
// Change 25 regenerated them exactly once: the fixed output-language line named 용도 /
// "purpose", a concept that no longer exists. Every byte-identity rule below is stated
// relative to that new baseline, the way plan 16's grounding constraint did.
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

// testBrief is what the template context hands over: a name, a body already expanded and
// rendered for this post's photos, and the slots that body declared.
func testBrief() *TemplateBrief {
	return &TemplateBrief{
		Name: "정보성 식당 리뷰",
		Body: "<write>인트로를 작성합니다.</write>\n\n=========================\n{{slot:1}}\n\n{{photo:IMG_1.jpg}}\n<write>이 사진에 대한 설명</write>",
		Slots: []TemplateSlot{
			{Kind: "place", Label: "네이버 지도"},
		},
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

// A5: the word 지침 must name exactly one thing in the prompt. The retired purpose section
// used it for its own field AND the guideline section used it for the entity, one section
// apart, which made "지침이 …의 요구와 충돌하면 지침을 우선하고" self-referential.
func TestTheWord지침AppearsOnlyInTheGuidelineSection(t *testing.T) {
	system, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO", "TITLE", []string{"IMG_1.jpg"}, nil, testBrief(), testGuidelines())

	guidelineSection := system[strings.Index(system, "[작문 지침]"):]
	withoutGuidelines := system[:strings.Index(system, "[작문 지침]")]
	if strings.Contains(withoutGuidelines, "지침") {
		t.Fatalf("지침 appears outside the guideline section:\n%s", withoutGuidelines)
	}
	if !strings.Contains(guidelineSection, "지침이 템플릿의 요구와 충돌하면 지침을 우선하고") {
		t.Fatalf("the guideline precedence no longer names the template:\n%s", guidelineSection)
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
	raw, err := EncodeGenerationPayload(GenerationOptions{TargetLanguage: LanguageKorean, Template: frozen})
	if err != nil {
		t.Fatal(err)
	}
	// The row is edited beyond recognition and then deleted; the payload is unaffected
	// because it holds text, not a reference.
	frozen.Name = "편집된 이름"
	frozen.Body = "<write>편집된 본문</write>"
	frozen.Slots[0].Label = "편집된 라벨"

	decoded, err := DecodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Template == nil || decoded.Template.Name != "정보성 식당 리뷰" ||
		!strings.Contains(decoded.Template.Body, "인트로를 작성합니다") ||
		len(decoded.Template.Slots) != 1 || decoded.Template.Slots[0].Label != "네이버 지도" {
		t.Fatalf("payload followed the live row: %+v", decoded.Template)
	}
	// Decoding twice is what a resume and a retry each do; both must build the same prompt.
	again, err := DecodeGenerationPayload(raw)
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
		decoded, err := DecodeGenerationPayload(raw)
		if err != nil || decoded.Template != nil {
			t.Fatalf("payload %s decoded to %+v err=%v", raw, decoded.Template, err)
		}
	}
}

// The revision payload freezes the template the same way the generate payload does.
func TestTheRevisionPayloadFreezesTheTemplateToo(t *testing.T) {
	raw, err := encodeRevisionPayload("INSTRUCTION", false, testBrief(), nil)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := parseRevisionPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	brief := decodeTemplate(decoded.Template)
	want := testBrief()
	if brief == nil || brief.Name != want.Name || brief.Body != want.Body || len(brief.Slots) != 1 {
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
	brief     TemplateBrief
	deleted   bool
	calls     int
	filenames []string
}

func (f *fakeTemplateBriefs) RenderedFor(_ context.Context, _, templateID string, filenames []string) (TemplateBrief, bool, error) {
	f.calls++
	f.filenames = filenames
	if f.deleted || templateID == "" {
		return TemplateBrief{}, false, nil
	}
	return f.brief, true, nil
}

func templateAwareService(t *testing.T, briefs *fakeTemplateBriefs, posts *fakePosts, jobs *fakeJobs, models *fakeModels) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget)
	svc.SetTemplateBriefs(briefs)
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
	if len(jobs.generations) != 1 || jobs.generations[0].Template == nil {
		t.Fatalf("the start did not freeze a template: %+v", jobs.generations)
	}
	frozen := *jobs.generations[0].Template

	// Between the enqueue and the drain the template is edited and then deleted outright.
	briefs.brief = TemplateBrief{Name: "편집됨", Body: "<write>편집된 본문</write>"}
	briefs.deleted = true

	if err := svc.Generate(ctx, GenerateJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Template: &frozen,
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

// A6: the render sees exactly the attachment set the enqueue froze, so a photo attached after
// the start cannot change the expansion.
func TestTheEnqueuePassesTheFrozenAttachmentOrderToTheRender(t *testing.T) {
	ctx := context.Background()
	briefs := &fakeTemplateBriefs{brief: *testBrief()}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review",
		Images: []Image{{Filename: "IMG_1.jpg"}, {Filename: "IMG_2.jpg"}},
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := templateAwareService(t, briefs, posts, &fakeJobs{id: "job"}, models)

	if _, err := svc.Start(ctx, StartRequest{
		UserID: "alice", PostSlug: "post", ObserveModel: observeRef.String(), WriteModel: writeRef.String(),
	}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(briefs.filenames, ","); got != "IMG_1.jpg,IMG_2.jpg" {
		t.Fatalf("render saw filenames %q, want the post's attachment order", got)
	}
}
