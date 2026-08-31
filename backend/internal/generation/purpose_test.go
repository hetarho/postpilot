package generation

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// These goldens define the current fixed prompt for a post without a purpose. Update them
// only for an explicitly accepted change to that fixed prompt; ordinary purpose work must
// remain byte-identical to this baseline.
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

func testBrief() *PurposeBrief {
	return &PurposeBrief{
		Name:         "정보성 식당 리뷰",
		Description:  "협찬 방문 리뷰",
		Instructions: "사진마다 무엇인지 설명하세요.\n일기체는 쓰지 마세요.",
	}
}

// Plan 11 A4: no purpose means no purpose-specific change to the fixed prompt.
func TestWritePromptWithoutAPurposeIsByteIdenticalToTheBaseline(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_purpose.golden")
	system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, nil)
	if system != wantSystem {
		t.Fatalf("system prompt drifted from the no-purpose baseline:\n--- got ---\n%s\n--- want ---\n%s", system, wantSystem)
	}
	if user != wantUser {
		t.Fatalf("user prompt drifted from the no-purpose baseline:\n--- got ---\n%s\n--- want ---\n%s", user, wantUser)
	}
}

// Plan 11 A6: the same, for revision.
func TestRevisePromptWithoutAPurposeIsByteIdenticalToTheBaseline(t *testing.T) {
	wantSystem, wantUser := loadGolden(t, "revise_prompt_no_purpose.golden")
	system, user := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, nil)
	if system != wantSystem || user != wantUser {
		t.Fatalf("revise prompt drifted from the no-purpose baseline:\n--- got ---\n%s\n--- want ---\n%s", system, wantSystem)
	}
}

// Plan 11 A4/A5: exactly one section, in exactly one place — after the whole voice profile
// and before the per-post material — and the profile half is untouched by its presence.
func TestWritePromptAppendsOneBriefAfterTheCompleteVoiceProfile(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "write_prompt_no_purpose.golden")
	system, user := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, testBrief())

	// A5: everything up to the section is the same bytes as the no-purpose prompt, so the
	// cached profile prefix is unchanged across posts of different purposes.
	if !strings.HasPrefix(system, baseline) {
		t.Fatalf("the brief changed the voice profile prefix:\n%s", system)
	}
	if user != baselineUser {
		t.Fatalf("the brief leaked into the per-post material:\n%s", user)
	}

	section := strings.TrimPrefix(system, baseline)
	want := "\n\n[글의 용도: 정보성 식당 리뷰]" +
		"\n이 글의 용도: 협찬 방문 리뷰" +
		"\n작성 지침:\n사진마다 무엇인지 설명하세요.\n일기체는 쓰지 마세요." +
		"\n" + purposePrecedence
	if section != want {
		t.Fatalf("purpose section =\n%q\nwant\n%q", section, want)
	}
	if strings.Count(system, "[글의 용도:") != 1 {
		t.Fatalf("the brief was injected more than once:\n%s", system)
	}
}

// The description line is the only optional part: an empty one is omitted rather than
// emitted as a bare label.
func TestAnEmptyDescriptionOmitsItsLineEntirely(t *testing.T) {
	baseline, _ := loadGolden(t, "write_prompt_no_purpose.golden")
	brief := testBrief()
	brief.Description = ""
	system, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, brief)

	section := strings.TrimPrefix(system, baseline)
	if strings.Contains(section, "이 글의 용도:") {
		t.Fatalf("an empty description still emitted its line:\n%q", section)
	}
	if !strings.HasPrefix(section, "\n\n[글의 용도: 정보성 식당 리뷰]\n작성 지침:\n") {
		t.Fatalf("section without a description = %q", section)
	}
}

// Plan 11 A6: revision injects the same block at the same relative position, so a post keeps
// being revised under the brief it was written for.
func TestRevisePromptInjectsTheSameSectionAtTheSamePosition(t *testing.T) {
	baseline, baselineUser := loadGolden(t, "revise_prompt_no_purpose.golden")
	writeBaseline, _ := loadGolden(t, "write_prompt_no_purpose.golden")
	writeSystem, _ := BuildWritePrompt(goldenProfile(), goldenObservations(), "MEMO 본문", "가제 TITLE", []string{"IMG_1.jpg", "IMG_2.jpg"}, nil, testBrief())
	reviseSystem, reviseUser := BuildRevisePrompt(goldenProfile(), goldenContent(), []string{"IMG_1.jpg"}, "INSTRUCTION 수정 요청", nil, testBrief())

	if !strings.HasPrefix(reviseSystem, baseline) || reviseUser != baselineUser {
		t.Fatalf("the brief moved something else in the revise prompt:\n%s", reviseSystem)
	}
	if got, want := strings.TrimPrefix(reviseSystem, baseline), strings.TrimPrefix(writeSystem, writeBaseline); got != want {
		t.Fatalf("revise section =\n%q\nwrite section =\n%q", got, want)
	}
}

// Plan 11 A7: what the payload froze is what the prompt uses. Editing the live row after the
// enqueue — the case a restart-resume or an explicit retry also lands in — changes nothing.
func TestTheFrozenPayloadSurvivesAnEditOrDeletionOfTheLiveRow(t *testing.T) {
	frozen := testBrief()
	raw, err := EncodeGenerationPayload(GenerationOptions{Purpose: frozen})
	if err != nil {
		t.Fatal(err)
	}
	// The row is edited beyond recognition and then deleted; the payload is unaffected
	// because it holds text, not a reference.
	frozen.Name = "편집된 이름"
	frozen.Instructions = "편집된 지침"

	decoded, err := DecodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Purpose == nil || decoded.Purpose.Name != "정보성 식당 리뷰" || !strings.HasPrefix(decoded.Purpose.Instructions, "사진마다") {
		t.Fatalf("payload followed the live row: %+v", decoded.Purpose)
	}
	// Decoding twice is what a resume and a retry each do; both must build the same prompt.
	again, err := DecodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, decoded.Purpose)
	second, _ := BuildWritePrompt(goldenProfile(), nil, "", "", nil, nil, again.Purpose)
	if first != second {
		t.Fatal("a resumed run built a different prompt than the first attempt")
	}
}

// A payload written before purposes existed decodes as "no purpose" rather than failing.
func TestAPayloadWithoutAPurposeFieldDecodesAsNoPurpose(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{}`), []byte(`{"target_length":800}`)} {
		decoded, err := DecodeGenerationPayload(raw)
		if err != nil || decoded.Purpose != nil {
			t.Fatalf("payload %s decoded to %+v err=%v", raw, decoded.Purpose, err)
		}
	}
}

// The revision payload freezes the brief the same way the generate payload does.
func TestTheRevisionPayloadFreezesTheBriefToo(t *testing.T) {
	raw, err := encodeRevisionPayload("INSTRUCTION", false, testBrief())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := parseRevisionPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	brief := decodePurpose(decoded.Purpose)
	if brief == nil || *brief != *testBrief() {
		t.Fatalf("revision payload brief = %+v", brief)
	}
	// And a revision payload from before purposes existed still parses.
	old, err := parseRevisionPayload([]byte(`{"instruction":"고쳐줘","save_as_rule":false}`))
	if err != nil || old.Purpose != nil {
		t.Fatalf("legacy revision payload = %+v err=%v", old, err)
	}
}

// fakePurposeBriefs is the purpose context's published lookup. `deleted` makes it answer
// like a purpose removed after the enqueue.
type fakePurposeBriefs struct {
	brief   PurposeBrief
	deleted bool
	calls   int
}

func (f *fakePurposeBriefs) BriefFor(_ context.Context, _, purposeID string) (PurposeBrief, bool, error) {
	f.calls++
	if f.deleted || purposeID == "" {
		return PurposeBrief{}, false, nil
	}
	return f.brief, true, nil
}

func purposeAwareService(t *testing.T, briefs *fakePurposeBriefs, posts *fakePosts, jobs *fakeJobs, models *fakeModels) *Service {
	t.Helper()
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy)
	svc.SetPurposeBriefs(briefs)
	return svc
}

// Plan 11 A7, end to end: the brief is read ONCE, at the enqueue, and the run that drains
// the job prompts with what was frozen — even though the row has since been rewritten.
func TestGenerationFreezesTheBriefAtEnqueueAndTheDrainIgnoresTheLiveRow(t *testing.T) {
	ctx := context.Background()
	briefs := &fakePurposeBriefs{brief: *testBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, PurposeID: "purpose-review"}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := purposeAwareService(t, briefs, posts, jobs, models)

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if len(jobs.generations) != 1 || jobs.generations[0].Purpose == nil {
		t.Fatalf("the start did not freeze a brief: %+v", jobs.generations)
	}
	frozen := *jobs.generations[0].Purpose

	// Between the enqueue and the drain the purpose is edited and then deleted outright.
	briefs.brief = PurposeBrief{Name: "편집됨", Instructions: "편집된 지침"}
	briefs.deleted = true

	if err := svc.Generate(ctx, GenerateJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Purpose: &frozen,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 {
		t.Fatalf("write calls = %d", len(models.calls))
	}
	system := models.calls[0].request.System
	if !strings.Contains(system, "[글의 용도: 정보성 식당 리뷰]") || strings.Contains(system, "편집됨") {
		t.Fatalf("the drain used the live row instead of the payload:\n%s", system)
	}
	// The handler must never consult the directory: only the enqueue may.
	if briefs.calls != 1 {
		t.Fatalf("the brief was looked up %d times; only the enqueue may", briefs.calls)
	}
}

// A purpose deleted between the save and the start is simply absent — a post with no
// purpose, not a failed start.
func TestAPurposeDeletedBeforeTheStartYieldsAPromptWithoutABrief(t *testing.T) {
	ctx := context.Background()
	briefs := &fakePurposeBriefs{brief: *testBrief(), deleted: true}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, PurposeID: "purpose-gone"}}
	jobs := &fakeJobs{id: "job"}
	svc := purposeAwareService(t, briefs, posts, jobs, newFakeModels())

	if _, err := svc.Start(ctx, StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatalf("start refused a post whose purpose was deleted: %v", err)
	}
	if jobs.generations[0].Purpose != nil {
		t.Fatalf("a deleted purpose was frozen anyway: %+v", jobs.generations[0].Purpose)
	}
}

// Revision freezes into its own payload, and the drain reads it from there.
func TestRevisionFreezesTheBriefIntoItsPayload(t *testing.T) {
	ctx := context.Background()
	briefs := &fakePurposeBriefs{brief: *testBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, PurposeID: "purpose-review", Content: revisionContent("body")}}
	jobs := &fakeJobs{id: "job"}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := purposeAwareService(t, briefs, posts, jobs, models)

	if _, err := svc.StartRevision(ctx, StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	payload := jobs.payloads[0]
	briefs.deleted = true

	if err := svc.Revise(ctx, RevisionJob{
		UserID: "alice", PostSlug: "post", VoiceID: liveVoice.ID, WriteModel: writeRef.String(), Payload: payload,
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(models.calls[0].request.System, "[글의 용도: 정보성 식당 리뷰]") {
		t.Fatalf("revision lost the frozen brief:\n%s", models.calls[0].request.System)
	}
}

// Plan 11 A8: both candidates of a write comparison get byte-identical system prompts
// including the brief, and a different purpose is a different input.
func TestWriteExperimentGivesBothCandidatesTheSameBriefAndChangesTheSnapshot(t *testing.T) {
	ctx := context.Background()
	briefs := &fakePurposeBriefs{brief: *testBrief()}
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice, PurposeID: "purpose-review"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := purposeAwareService(t, briefs, posts, &fakeJobs{id: "job"}, models)

	snapshot, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if name := SnapshotPurposeName(snapshot); name != "정보성 식당 리뷰" {
		t.Fatalf("snapshot purpose name = %q", name)
	}
	prepared, err := svc.PrepareWriteInput(ctx, snapshot, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	// The row changes between the two candidates; neither may notice.
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, writeRef); err != nil {
		t.Fatal(err)
	}
	briefs.brief = PurposeBrief{Name: "편집됨", Instructions: "편집된 지침"}
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, observeRef); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 2 || models.calls[0].request.System != models.calls[1].request.System {
		t.Fatalf("candidates received different system prompts:\n%q\n%q", models.calls[0].request.System, models.calls[1].request.System)
	}
	if !strings.Contains(models.calls[0].request.System, "[글의 용도: 정보성 식당 리뷰]") {
		t.Fatalf("candidates did not receive the brief:\n%s", models.calls[0].request.System)
	}
	if !strings.Contains(models.calls[0].request.System, NaturalnessBaseline) {
		t.Fatalf("candidates did not receive the naturalness baseline:\n%s", models.calls[0].request.System)
	}

	// The experiment's input hash is taken over these bytes, so a different brief is a
	// different comparison rather than a rerun of the same one.
	briefs.deleted = true
	without, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(without) == string(snapshot) {
		t.Fatal("changing the purpose left the frozen input identical")
	}
	if name := SnapshotPurposeName(without); name != "" {
		t.Fatalf("snapshot without a purpose named %q", name)
	}
}
