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
	texts []string
	// preset is returned whatever forRevision is, so generation's own tests prove a revision
	// ignores it.
	preset        string
	calls         int
	askedTemplate *string
	askedField    *string
	askedRevision bool
}

func (f *fakeGuidelines) ForPrompt(_ context.Context, _ string, templateID, field *string, forRevision bool) (GuidelineTexts, error) {
	f.calls++
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
	f.askedRevision = forRevision
	return GuidelineTexts{Owner: f.texts, Preset: f.preset}, nil
}

func guidelineAwareService(t *testing.T, guidelines *fakeGuidelines, briefs *fakeTemplateBriefs, posts *fakePosts, jobs *fakeJobs, models *fakeModels) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())
	if briefs != nil {
		svc.templates = briefs
	}
	svc.guidelines = guidelines
	return svc
}

// A4: exactly one section, its texts verbatim as list lines in the given order, closed by the
// fixed precedence sentence, sitting after the complete voice profile and before [이번 글].
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
	want := "\n\n[작문 지침]\n- CCTV를 언급하지 않기\n- 직원·주인과의 상호작용을 쓰지 않기\n" + guidelinePrecedence
	if got := strings.TrimPrefix(system, baseline); got != want {
		t.Fatalf("section =\n%q\nwant\n%q", got, want)
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
	// A6: the template section's bytes are identical with and without guidelines, so plan 11's
	// acceptance criteria keep holding.
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
		return prompt[:strings.Index(prompt, "[종결어미 제약]")]
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
	if got, want := strings.TrimPrefix(revise, reviseBaseline), strings.TrimPrefix(write, writeBaseline); got != want {
		t.Fatalf("revise section =\n%q\nwrite section =\n%q", got, want)
	}
}

// A7: the grounding constraint is in the fixed text of every write and revise prompt, with or
// without guidelines and with or without a template, in both languages — and never in observe.
func TestGroundingConstraintIsInEveryWriteAndRevisePrompt(t *testing.T) {
	for name, prompt := range map[string]string{
		"write bare":            firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, nil, nil)),
		"write with template":   firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, testBrief(), nil)),
		"write with guidelines": firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, testBrief(), testGuidelines())),
		"revise bare":           firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, nil, nil)),
		"revise with both":      firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, testBrief(), testGuidelines())),
	} {
		if strings.Count(prompt, koreanGrounding) != 1 {
			t.Errorf("%s contains the grounding constraint %d times", name, strings.Count(prompt, koreanGrounding))
		}
	}
	for name, prompt := range map[string]string{
		"english write":  firstOf(BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 4})),
		"english revise": firstOf(BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 4, nil, nil)),
	} {
		if strings.Count(prompt, englishGrounding) != 1 {
			t.Errorf("%s contains the grounding constraint %d times", name, strings.Count(prompt, englishGrounding))
		}
	}
	// The write pass holds the memo and the observations, so only it is told to omit the
	// unconfirmed; the revise pass is told to leave untouched sentences alone instead, because it
	// receives neither and would otherwise strip real facts out of them.
	write := firstOf(BuildWritePrompt(goldenProfile(), nil, "memo", "title", nil, nil, nil, nil))
	revise := firstOf(BuildRevisePrompt(goldenProfile(), goldenContent(), nil, "고쳐줘", nil, nil, nil))
	if !strings.Contains(write, koreanGroundingWriteScope) || strings.Contains(write, koreanGroundingReviseScope) {
		t.Error("the write prompt carries the wrong grounding scope clause")
	}
	if !strings.Contains(revise, koreanGroundingReviseScope) || strings.Contains(revise, koreanGroundingWriteScope) {
		t.Error("the revise prompt carries the wrong grounding scope clause")
	}
	// [I3]: observation stays a photo-facts pass and gains nothing from this plan.
	if strings.Contains(ObservePrompt, koreanGrounding) {
		t.Fatal("the observe prompt gained the grounding constraint")
	}
	// It is grounding only: the stylistic floor stays change 10's section, not restated here.
	if strings.Contains(NaturalnessBaseline, koreanGrounding) || strings.Contains(koreanGrounding, "기준선") {
		t.Fatal("the grounding line and the naturalness baseline overlap")
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
	guidelines := &fakeGuidelines{texts: testGuidelines()}
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
	guidelines.texts = nil

	if err := svc.Revise(ctx, RevisionJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: payload,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(models.calls[0].request.System, "- CCTV를 언급하지 않기") {
		t.Fatalf("revision lost the frozen guidelines:\n%s", models.calls[0].request.System)
	}

	old, err := parseRevisionPayload([]byte(`{"instruction":"고쳐줘","save_as_rule":false}`))
	if err != nil || len(old.Guidelines) != 0 {
		t.Fatalf("legacy revision payload = %v err=%v", old.Guidelines, err)
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
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, testDeps())

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.frozen(t, 0).Guidelines) != 0 {
		t.Fatalf("an unwired resolver produced %v", jobs.frozen(t, 0).Guidelines)
	}
}

// GUIDE-17, GEN-57: every entry point freezes with the post's 분야, and only a revision asks
// without the preset line. A post with no 분야 is asked for none, not for the empty id.
func TestEveryEntryPointAsksWithThePostsFieldAndItsRevisionFlag(t *testing.T) {
	ctx := context.Background()
	guidelines := &fakeGuidelines{texts: testGuidelines()}
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, TemplateID: "template-review", Field: "cafe",
		Content: revisionContent("body"),
	}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := guidelineAwareService(t, guidelines, &fakeTemplateBriefs{brief: *testBrief()}, posts, &fakeJobs{id: "job"}, models)
	asked := func(entry string, revision bool) {
		t.Helper()
		if guidelines.askedField == nil || *guidelines.askedField != "cafe" || guidelines.askedRevision != revision {
			t.Fatalf("%s asked for 분야 %v with forRevision=%v, want cafe and %v", entry, guidelines.askedField, guidelines.askedRevision, revision)
		}
	}

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("Start", false)
	if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	asked("StartRevision", true)
	if _, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	asked("SnapshotWriteInput", false)

	posts.input.Field = ""
	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if guidelines.askedField != nil {
		t.Fatalf("a post with no 분야 asked for %q", *guidelines.askedField)
	}
}

// GUIDE-40, QUAL-41: the preset's line rides only with frozen phrases. A post whose 분야 froze
// none keeps every owner guideline, 분야-scoped ones included, and loses only the preset line —
// in Start's payload and in the comparison snapshot alike. With phrases it is appended last. A
// revision never carries it, even when the guideline port hands one back.
func TestThePresetLineRidesOnlyWithFrozenPhrases(t *testing.T) {
	ctx := context.Background()
	for name, phrases := range map[string][]string{"no phrases": nil, "phrases": {"성수 카페"}} {
		posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, Field: "cafe", Content: revisionContent("body")}}
		guidelines := &fakeGuidelines{texts: testGuidelines(), preset: "PRESET"}
		deps := testDeps()
		deps.Guidelines = guidelines
		deps.FieldPhrases = &recordingPhrases{answer: phrases}
		jobs := &fakeJobs{id: "job"}
		svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
		want := testGuidelines()
		if phrases != nil {
			want = append(testGuidelines(), "PRESET")
		}
		if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
			t.Fatal(err)
		}
		if got := jobs.frozen(t, 0).Guidelines; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: Start froze %q, want %q", name, got, want)
		}
		raw, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := decodeWriteSnapshot(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := snapshot.Post.Guidelines; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the snapshot froze %q, want %q", name, got, want)
		}
		if phrases == nil {
			continue
		}
		if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "짧게", WriteModel: writeRef.String()}); err != nil {
			t.Fatal(err)
		}
		revision, err := parseRevisionPayload(jobs.payloads[0])
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(revision.Guidelines, testGuidelines()) {
			t.Errorf("the revision froze %q, want the owner texts only", revision.Guidelines)
		}
	}
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
	deps.Guidelines = &fakeGuidelines{texts: testGuidelines(), preset: "PRESET"}
	deps.Memories = &recordingMemories{texts: testMemories()}
	deps.QualityRules = &recordingRules{answer: testQualityRules()}
	deps.FieldPhrases = &recordingPhrases{answer: []string{"성수 카페"}}
	jobs := &fakeJobs{id: "job"}
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, newFakeModels(), fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
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
		Template: snapshot.Post.Template, Guidelines: snapshot.Post.Guidelines, Memories: snapshot.Post.Memories,
		QualityRules: snapshot.Post.QualityRules, FieldPhrases: snapshot.Post.FieldPhrases,
	}
	requireNoZero(t, "frozen", reflect.ValueOf(frozen))
	if !reflect.DeepEqual(frozen, compared) {
		t.Fatalf("the comparison froze a different material:\n start %+v\n  snap %+v", frozen, compared)
	}
}
