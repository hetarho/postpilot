package generation

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

func testGuidelines() []string {
	return []string{"CCTV를 언급하지 않기", "직원·주인과의 상호작용을 쓰지 않기"}
}

// fakeGuidelines is the guideline context's published resolution. Changing `texts` after an
// enqueue is how a test edits, rescopes or deletes rows between enqueue and drain.
type fakeGuidelines struct {
	texts         []string
	defaults      []string
	stock         []StockGuideline
	calls         int
	askedTemplate *string
	askedField    *string
	askedLanguage Language
	// askedMemories is whether the last caller said its run carries [기억] (GEN-73).
	askedMemories bool
}

func (f *fakeGuidelines) ForPrompt(_ context.Context, _ string, templateID, field *string, target Language, withMemories bool) (FrozenGuidelines, error) {
	f.calls++
	f.askedLanguage = target
	f.askedMemories = withMemories
	if templateID == nil {
		f.askedTemplate = nil
	} else {
		id := *templateID
		f.askedTemplate = &id
	}
	if field == nil {
		f.askedField = nil
	} else {
		id := *field
		f.askedField = &id
	}
	return FrozenGuidelines{Defaults: f.defaults, Stock: cloneStockGuidelines(f.stock), Owner: f.texts}, nil
}

func guidelineAwareService(t *testing.T, guidelines *fakeGuidelines, briefs *fakeTemplateBriefs, posts *fakePosts, jobs *fakeJobs, models *fakeModels) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())
	if briefs != nil {
		svc.templates = briefs
	}
	svc.guidelines = guidelines
	return svc
}

// A4, GUIDE-15: exactly one section, its texts verbatim as list lines in the given order under
// their group label, closed by the fixed precedence sentence, sitting after the complete voice
// profile and before [이번 글].
func TestWritePromptAppendsOneGuidelineSectionAfterTheProfile(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "write_prompt_no_template.golden")
	system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, testGuidelines())

	if !strings.HasPrefix(system, baseline) {
		t.Fatalf("the section disturbed the fixed prefix:\n%s", system)
	}
	if user != baselineUser {
		t.Fatal("the section changed the per-post material")
	}
	if strings.Count(system, "[작문 지침]") != 1 {
		t.Fatalf("guideline sections = %d", strings.Count(system, "[작문 지침]"))
	}
	want := "\n\n[작문 지침]\n사용자 지침:\n- CCTV를 언급하지 않기\n- 직원·주인과의 상호작용을 쓰지 않기\n" + guidelinePrecedence
	if got := strings.TrimPrefix(system, baseline); got != want {
		t.Fatalf("section =\n%q\nwant\n%q", got, want)
	}
}

// GUIDE-14, GUIDE-15: the enabled 기본 지침 come first under their own label, a multi-line one as
// one bullet with its continuation lines indented, then the owner's; a group with no line is left
// out, and with neither there is no section.
func TestTheSectionPutsTheDefaultsFirstUnderTheirOwnLabel(t *testing.T) {
	baseline, _ := loadGolden(t, "write_prompt_no_template.golden")
	build := func(defaults, owner []string) string {
		return firstOf(BuildWritePromptForLanguage(WritePromptInput{
			Language: LanguageKorean, Profile: goldenProfile(), Observations: goldenObservations(),
			Memo: "MEMO 본문", Title: "가제 TITLE", Photos: []string{"IMG_1.jpg", "IMG_2.jpg"}, TagCount: 4,
			DefaultGuidelines: defaults, Guidelines: owner,
		}))
	}
	both := build([]string{"첫 기본", "둘째 줄\n이어지는 줄"}, testGuidelines())
	want := "\n\n[작문 지침]\n기본 지침:\n- 첫 기본\n- 둘째 줄\n  이어지는 줄\n사용자 지침:\n- CCTV를 언급하지 않기\n- 직원·주인과의 상호작용을 쓰지 않기\n" + guidelinePrecedence
	if got := strings.TrimPrefix(both, baseline); got != want {
		t.Fatalf("section =\n%q\nwant\n%q", got, want)
	}
	if got := strings.TrimPrefix(build([]string{"첫 기본"}, nil), baseline); got != "\n\n[작문 지침]\n기본 지침:\n- 첫 기본\n"+guidelinePrecedence {
		t.Fatalf("defaults alone = %q", got)
	}
	if build(nil, nil) != baseline {
		t.Fatal("no guideline at all still wrote a section")
	}
}

// A4: with a template the one section sits AFTER the template section, still before [이번 글].
func TestGuidelinesFollowTheTemplateSectionWhenThePostHasOne(t *testing.T) {
	withTemplate, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg"}, nil, testBrief(), nil)
	both, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg"}, nil, testBrief(), testGuidelines())

	if !strings.HasPrefix(both, withTemplate) {
		t.Fatalf("the guideline section moved the template section:\n%s", both)
	}
	templateAt := strings.Index(both, "[글 템플릿:")
	guidelineAt := strings.Index(both, "[작문 지침]")
	if templateAt < 0 || guidelineAt < templateAt {
		t.Fatalf("guideline section at %d, template section at %d", guidelineAt, templateAt)
	}
	// The template section's bytes are identical with and without guidelines, so the template
	// section's own guarantees keep holding.
	if got := strings.TrimPrefix(both, withTemplate); !strings.HasPrefix(got, "\n\n[작문 지침]") {
		t.Fatalf("appended section = %q", got)
	}
}

// A6: with no applicable guidelines the prompt is byte-identical to the baseline, and the
// voice profile prefix is untouched by their presence.
func TestNoGuidelinesLeavesThePromptAtTheBaseline(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "write_prompt_no_template.golden")
	for name, empty := range map[string][]string{"nil": nil, "empty": {}} {
		system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, empty)
		if system != baseline || user != baselineUser {
			t.Fatalf("%s guidelines changed the baseline prompt", name)
		}
	}
	// The voice prefix is byte-identical either way: guidelines are appended after it, so the
	// cached prefix of PRD §5 stays stable.
	profilePrefix := func(prompt string) string {
		return prompt[:strings.Index(prompt, "예시의 고유 사실")]
	}
	with, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, testGuidelines())
	if profilePrefix(with) != profilePrefix(baseline) {
		t.Fatal("guidelines changed the voice profile prefix")
	}
}

// A10: revision injects the same section at the same relative position, and a revision
// without guidelines is unchanged from its baseline.
func TestRevisePromptInjectsTheSameGuidelineSectionAtTheSamePosition(t *testing.T) {
	reviseBaseline, reviseBaselineUser := loadGolden(t, "revise_prompt_no_template.golden")
	writeBaseline, _ := loadGolden(t, "write_prompt_no_template.golden")

	unchanged, unchangedUser := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, nil, nil)
	if unchanged != reviseBaseline || unchangedUser != reviseBaselineUser {
		t.Fatal("a revision without guidelines drifted from the baseline")
	}

	revise, reviseUser := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, nil, testGuidelines())
	write, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil, testGuidelines())
	if reviseUser != reviseBaselineUser {
		t.Fatal("the section changed the revision's per-post material")
	}
	// The same section, bound to what the request writes or touches (GEN-40).
	if got, want := strings.TrimPrefix(revise, reviseBaseline), strings.TrimPrefix(write, writeBaseline)+"\n"+reviseGuidelineScope; got != want {
		t.Fatalf("revise section =\n%q\nwant\n%q", got, want)
	}
}

func firstOf(system, _ string) string { return system }

// A8: the texts are resolved ONCE at the enqueue, from the post's current template, and the
// drain prompts with what was frozen even though every row changed since.
func TestGenerationFreezesGuidelinesAtEnqueueAndTheDrainIgnoresLiveRows(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review"}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, &fakeTemplateBriefs{brief: *testBrief()}, posts, jobs, models)

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if guidelines.askedTemplate == nil || *guidelines.askedTemplate != "template-review" {
		t.Fatalf("resolution asked for %v, want the post's current template", guidelines.askedTemplate)
	}
	frozen := jobs.frozen(t, 0).Guidelines
	if len(frozen) != 2 {
		t.Fatalf("the start froze %v", frozen)
	}

	// Between the enqueue and the drain every guideline is edited, rescoped and deleted.
	guidelines.texts = nil

	if err := svc.Generate(ctx, GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{writeMaterial: writeMaterial{Guidelines: frozen}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	system := models.calls[0].request.System
	if !strings.Contains(system, "- CCTV를 언급하지 않기") {
		t.Fatalf("the drain used live rows instead of the payload:\n%s", system)
	}
	if guidelines.calls != 1 {
		t.Fatalf("guidelines were resolved %d times; only the enqueue may", guidelines.calls)
	}
}

// A8: a restart-resume and an explicit retry both decode the same payload, so both build the
// same prompt; a payload written before guidelines existed decodes as none.
func TestFrozenGuidelinesSurviveAResumeAndALegacyPayloadDecodesAsNone(t *testing.T) {
	texts := testGuidelines()
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, writeMaterial: writeMaterial{Guidelines: texts}})
	if err != nil {
		t.Fatal(err)
	}
	// The caller's slice is rewritten after the freeze; the payload holds its own copy.
	texts[0] = "편집됨"

	first, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	again, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if first.Guidelines[0] != "CCTV를 언급하지 않기" {
		t.Fatalf("the payload followed the caller's slice: %v", first.Guidelines)
	}
	resumed, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, nil, first.Guidelines)
	retried, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, nil, again.Guidelines)
	if resumed != retried {
		t.Fatal("a resumed run built a different prompt than the retry")
	}
	for _, legacy := range [][]byte{nil, []byte(`{}`), []byte(`{"target_length":800}`)} {
		decoded, err := decodeGenerationPayload(legacy)
		if err != nil || len(decoded.Guidelines) != 0 {
			t.Fatalf("legacy payload %s decoded to %v err=%v", legacy, decoded.Guidelines, err)
		}
	}
}

// A8/A10: revision freezes into its own payload and the drain reads it from there; a legacy
// revision payload still parses as no guidelines.
func TestRevisionFreezesGuidelinesIntoItsPayload(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines(), defaults: []string{"메모의 이름으로 쓰세요"}}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body")}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, nil, posts, jobs, models)

	if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	// A post with no template resolves the global group alone, and must not be asked for a
	// template it does not have.
	if guidelines.askedTemplate != nil {
		t.Fatalf("a post with no template asked for %q", *guidelines.askedTemplate)
	}
	payload := jobs.payloads[0]
	guidelines.texts, guidelines.defaults = nil, nil

	if err := svc.Revise(ctx, RevisionJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: payload,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(models.calls[0].request.System, "기본 지침:\n- 메모의 이름으로 쓰세요\n사용자 지침:\n- CCTV를 언급하지 않기") {
		t.Fatalf("revision lost the frozen guidelines:\n%s", models.calls[0].request.System)
	}

	old, err := parseRevisionPayload([]byte(`{"instruction":"고쳐줘","save_as_rule":false}`))
	if err != nil || len(old.Guidelines) != 0 || len(old.DefaultGuidelines) != 0 {
		t.Fatalf("legacy revision payload = %v / %v err=%v", old.Guidelines, old.DefaultGuidelines, err)
	}
}

// A9: both candidates of a write comparison get byte-identical system prompts including the
// guidelines, and a different applicable set is a different frozen input (so a different hash).
func TestWriteExperimentFreezesTheSameGuidelinesForBothCandidates(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, &fakeTemplateBriefs{brief: *testBrief()}, posts, &fakeJobs{id: "job"}, models)

	snapshot, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(ctx, snapshot, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, writeRef); err != nil {
		t.Fatal(err)
	}
	// The rows change between the two candidates; neither may notice.
	guidelines.texts = []string{"편집됨"}
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, observeRef); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 2 || models.calls[0].request.System != models.calls[1].request.System {
		t.Fatalf("candidates received different system prompts:\n%q\n%q", models.calls[0].request.System, models.calls[1].request.System)
	}
	if !strings.Contains(models.calls[0].request.System, "- CCTV를 언급하지 않기") {
		t.Fatalf("candidates did not receive the guidelines:\n%s", models.calls[0].request.System)
	}

	// The experiment's input hash is taken over these bytes, so a different applicable set is
	// a different comparison rather than a rerun of the same one.
	guidelines.texts = nil
	without, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(without) == string(snapshot) {
		t.Fatal("changing the applicable guidelines left the frozen input identical")
	}
}

// [I5]: a partially wired process keeps the no-guideline behavior instead of failing.
func TestAnUnwiredResolverPromptsWithoutGuidelines(t *testing.T) {
	ctx := context.Background()
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.frozen(t, 0).Guidelines) != 0 {
		t.Fatalf("an unwired resolver produced %v", jobs.frozen(t, 0).Guidelines)
	}
}

// GUIDE-17: every entry point — the write, the revision and the comparison — freezes with the
// post's 분야. A post with no 분야 is asked for none, not for the empty id.
func TestEveryEntryPointAsksWithThePostsField(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines()}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review", Field: "cafe",
		Content: revisionContent("body"),
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, &fakeTemplateBriefs{brief: *testBrief()}, posts, &fakeJobs{id: "job"}, models)
	asked := func(entry string) {
		t.Helper()
		if guidelines.askedField == nil || *guidelines.askedField != "cafe" {
			t.Fatalf("%s asked for 분야 %v, want cafe", entry, guidelines.askedField)
		}
	}

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("Start")
	if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("StartRevision")
	if _, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	asked("SnapshotWriteInput")

	posts.input.Field = ""
	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if guidelines.askedField != nil {
		t.Fatalf("a post with no 분야 asked for %q", *guidelines.askedField)
	}
}

// GEN-73: every entry point asks for the 지침 saying whether its own prompt will carry [기억] —
// the write and the comparison exactly when their frozen memories are non-empty, the revision
// never (MEM-22) — so the memories-only 기본 지침 follows the section and nothing else.
func TestEveryEntryPointAsksWhetherItsRunCarriesMemories(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines()}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, UseMemory: true, Memo: "연남동에서 점심",
		Content: revisionContent("body"),
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, nil, posts, &fakeJobs{id: "job"}, models)
	recorder := &recordingMemories{texts: testMemories()}
	svc.memories = recorder
	asked := func(entry string, want bool) {
		t.Helper()
		if guidelines.askedMemories != want {
			t.Fatalf("%s asked withMemories = %v, want %v", entry, guidelines.askedMemories, want)
		}
	}

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("Start with memories", true)
	if _, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	asked("SnapshotWriteInput with memories", true)
	if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("StartRevision", false)

	// A retrieval that matched nothing renders no [기억], and neither does a post that never
	// opted in.
	recorder.texts = nil
	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("Start with no matching memory", false)
	recorder.texts = testMemories()
	posts.input.UseMemory = false
	if _, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	asked("SnapshotWriteInput without 기억 사용", false)
}

// GEN-18, MEM-19, MODEL-30: a comparison freezes exactly the write material Start freezes — one
// helper resolves both, so neither can carry a member the other lacks.
func TestAComparisonFreezesTheWriteMaterialStartFreezes(t *testing.T) {
	ctx := context.Background()
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "tmpl", UseMemory: true,
		QualityRuleIDs: []string{"composition"}, Field: "cafe", Memo: "메모",
	}}
	deps := testDeps()
	// Every brief member set: the payload's decoder turns an absent slice into an empty one, so
	// only a full brief compares the two freezes rather than the two codecs.
	deps.Templates = &fakeTemplateBriefs{brief: *filledGenerationOptions().Template}
	deps.Guidelines = &fakeGuidelines{texts: testGuidelines(), defaults: []string{"메모의 이름으로 쓰세요"}, stock: testStockGuidelines()}
	deps.Memories = &recordingMemories{texts: testMemories()}
	deps.QualityRules = &recordingRules{answer: testQualityRules()}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	raw, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := decodeWriteSnapshot(raw)
	if err != nil {
		t.Fatal(err)
	}
	frozen := jobs.frozen(t, 0).writeMaterial
	compared := writeMaterial{
		Template: snapshot.Post.Template, Guidelines: snapshot.Post.Guidelines,
		DefaultGuidelines: snapshot.Post.DefaultGuidelines, StockGuidelines: snapshot.Post.StockGuidelines, Memories: snapshot.Post.Memories,
		QualityRules: snapshot.Post.QualityRules,
	}
	requireNoZero(t, "frozen", reflect.ValueOf(frozen))
	if !reflect.DeepEqual(frozen, compared) {
		t.Fatalf("the comparison froze a different material:\n start %+v\n  snap %+v", frozen, compared)
	}
}
