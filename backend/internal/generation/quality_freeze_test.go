package generation

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/post"
)

// recordingRules is the quality rule port: it records every question and answers a mutable list.
type recordingRules struct {
	answer   []string
	calls    int
	userID   string
	slug     string
	ticked   []string
	language Language
}

func (r *recordingRules) RulesFor(_ context.Context, userID, slug string, ticked []string, language Language) ([]string, error) {
	r.calls++
	r.userID, r.slug, r.ticked, r.language = userID, slug, append([]string(nil), ticked...), language
	return r.answer, nil
}

func freezingService(posts *fakePosts, jobs *fakeJobs, models *fakeModels, rules *recordingRules) *Service {
	deps := testDeps()
	deps.QualityRules = rules
	return NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
}

func tickedPost() *fakePosts {
	return &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모",
		TargetLanguage: LanguageEnglish, QualityRuleIDs: []string{"title_saturation", "composition"}, Field: "restaurant",
	}}
}

func startOnce(t *testing.T, svc *Service) {
	t.Helper()
	if _, err := svc.Start(context.Background(), StartRequest{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
}

// GEN-51, POST-81: Start asks once, with the post's ticks and its frozen target language, and
// freezes exactly what comes back, in order. No ticks never reach the port; a tick that comes
// back with nothing freezes nothing.
func TestStartFreezesOnlyTheReturnedRuleTexts(t *testing.T) {
	posts, jobs, rules := tickedPost(), &fakeJobs{id: "job"}, &recordingRules{answer: []string{"rule B", "rule A"}}
	svc := freezingService(posts, jobs, newFakeModels(), rules)
	startOnce(t, svc)
	if rules.calls != 1 || rules.userID != "alice" || rules.slug != "post" || rules.language != LanguageEnglish ||
		!reflect.DeepEqual(rules.ticked, []string{"title_saturation", "composition"}) {
		t.Fatalf("the port was asked %+v", rules)
	}
	if got := jobs.frozen(t, 0).QualityRules; !reflect.DeepEqual(got, []string{"rule B", "rule A"}) {
		t.Fatalf("froze %q", got)
	}

	rules.answer = nil
	startOnce(t, svc)
	if got := jobs.frozen(t, 1).QualityRules; got != nil {
		t.Fatalf("a tick with no text froze %q", got)
	}

	posts.input.QualityRuleIDs = nil
	startOnce(t, svc)
	if rules.calls != 2 || jobs.frozen(t, 2).QualityRules != nil {
		t.Fatalf("a post with nothing ticked reached the port: %d calls, froze %q", rules.calls, jobs.frozen(t, 2).QualityRules)
	}
}

// A post with nothing ticked writes the payload and the prompt it wrote before quality rules
// existed, byte for byte, and never asks the port.
func TestNoTicksLeaveThePayloadAndPromptByteIdentical(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	jobs, rules := &fakeJobs{id: "job"}, &recordingRules{answer: []string{"never"}}
	svc := freezingService(posts, jobs, newFakeModels(), rules)
	startOnce(t, svc)
	raw := jobs.generatePayloads[0]
	// Exactly what the enqueue wrote before quality rules: its language, its resolved tag count
	// and no observation decision.
	if string(raw) != `{"target_language":"ko","tag_count":4,"observe_files":null}` {
		t.Fatalf("payload = %s", raw)
	}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	wantSystem, wantUser := loadGolden(t, "write_prompt_no_template.golden")
	system, user := BuildWritePromptForLanguage(WritePromptInput{
		Language:     LanguageKorean,
		Profile:      goldenProfile(),
		Observations: goldenObservations(),
		Memo:         "MEMO 본문",
		Title:        "가제 TITLE",
		Photos:       []string{"IMG_1.jpg", "IMG_2.jpg"},
		TagCount:     post.TagCountRange.Default,
		QualityRules: decoded.QualityRules,
	})
	if system != wantSystem || user != wantUser {
		t.Fatal("the no-tick prompt moved off the golden")
	}
	if rules.calls != 0 {
		t.Fatalf("the port was asked %d times", rules.calls)
	}
}

// GEN-14: a post's 분야 reaches no part of the write. The same post with and without one is
// drained into a byte-identical request: no phrase section, no replacements instruction, and the
// plain write answer schema.
func TestAFieldLeavesTheWriteRequestByteIdentical(t *testing.T) {
	var requests []llm.Request
	for _, field := range []string{"restaurant", ""} {
		posts, jobs := tickedPost(), &fakeJobs{id: "job"}
		posts.input.Field = field
		models := newFakeModels()
		models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
		deps := testDeps()
		deps.QualityRules = &recordingRules{answer: []string{"frozen rule"}}
		deps.Guidelines = &fakeGuidelines{texts: testGuidelines(), preset: "PRESET"}
		svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
		startOnce(t, svc)
		if err := svc.Generate(context.Background(), jobs.queued(0), func(string, int, int) {}); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, models.calls[len(models.calls)-1].request)
	}
	with, without := requests[0], requests[1]
	if with.System != without.System || with.Messages[0].Parts[0].Text != without.Messages[0].Parts[0].Text {
		t.Fatalf("a 분야 moved the write prompt:\n%s\n%s\n---\n%s\n%s", with.System, with.Messages[0].Parts[0].Text, without.System, without.Messages[0].Parts[0].Text)
	}
	prompt := with.System + "\n" + with.Messages[0].Parts[0].Text
	for _, retired := range []string{"[분야 상위 글 문구]", "상위 결과의 제목과 요약", "replacements", "PRESET"} {
		if strings.Contains(prompt, retired) {
			t.Errorf("the write prompt carries %q", retired)
		}
	}
	for _, request := range requests {
		if !bytes.Equal(request.JSONSchema, WriteAnswerSchema()) {
			t.Fatalf("the write asked for %s, want the plain write answer schema", request.JSONSchema)
		}
	}
}

// The drain reads the payload alone: ticks edited or posts published after the enqueue reach
// nothing in that run, and Generate never asks the port.
func TestTheDrainIgnoresRowsChangedAfterEnqueue(t *testing.T) {
	posts, jobs := tickedPost(), &fakeJobs{id: "job"}
	rules := &recordingRules{answer: []string{"frozen rule"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := freezingService(posts, jobs, models, rules)
	startOnce(t, svc)
	raw := jobs.generatePayloads[0]

	rules.answer = []string{"live rule"}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{TargetLanguage: decoded.TargetLanguage, writeMaterial: writeMaterial{QualityRules: decoded.QualityRules}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	sent := models.calls[len(models.calls)-1].request
	prompt := sent.System + "\n" + sent.Messages[0].Parts[0].Text
	if !strings.Contains(prompt, "frozen rule") {
		t.Error("the write lost the frozen rule")
	}
	if strings.Contains(prompt, "live rule") {
		t.Error("the write read the live rule")
	}
	if rules.calls != 1 {
		t.Fatalf("the drain asked the port: %d rule calls", rules.calls)
	}
}

// QUAL-29: a run with frozen rules writes once and measures nothing; there is no correction pass.
func TestAFrozenRuleRunMakesOneWriteCall(t *testing.T) {
	posts := tickedPost()
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	rules := &recordingRules{}
	svc := freezingService(posts, &fakeJobs{id: "job"}, models, rules)
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{TargetLanguage: LanguageEnglish, writeMaterial: writeMaterial{QualityRules: []string{"frozen rule"}}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || models.calls[0].ref != writeRef {
		t.Fatalf("calls = %+v, want exactly one write call", models.calls)
	}
	if rules.calls != 0 {
		t.Fatalf("the run asked the port: %d rule calls", rules.calls)
	}
}

// GEN-51 is write-only: StartRevision never asks the port, and its payload carries no rules.
func TestTheRevisionFreezesNoRules(t *testing.T) {
	posts := tickedPost()
	posts.input.Content = revisionContent("body")
	jobs, rules := &fakeJobs{id: "job"}, &recordingRules{answer: []string{"rule"}}
	svc := freezingService(posts, jobs, newFakeModels(), rules)
	if _, err := svc.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if rules.calls != 0 {
		t.Fatalf("a revision asked the port: %d rule calls", rules.calls)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(jobs.payloads[0], &keys); err != nil {
		t.Fatal(err)
	}
	if _, carried := keys["quality_rules"]; carried {
		t.Error("the revision payload carries quality_rules")
	}
}

// Both candidates read one frozen set from the snapshot, and a different set is a different
// frozen input; with none the snapshot has no rules key.
func TestWriteSnapshotFreezesRulesForBothCandidates(t *testing.T) {
	ctx := context.Background()
	posts := tickedPost()
	rules := &recordingRules{answer: []string{"frozen rule"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := freezingService(posts, &fakeJobs{id: "job"}, models, rules)

	snapshot, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(ctx, snapshot, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	rules.answer = []string{"live rule"}
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, writeRef); err != nil {
		t.Fatal(err)
	}
	if _, _, err := svc.RunWriteCandidate(ctx, prepared, observeRef); err != nil {
		t.Fatal(err)
	}
	first, second := models.calls[0].request, models.calls[1].request
	if first.System != second.System || first.Messages[0].Parts[0].Text != second.Messages[0].Parts[0].Text {
		t.Fatal("the two candidates received different requests")
	}
	if prompt := first.System + first.Messages[0].Parts[0].Text; !strings.Contains(prompt, "frozen rule") {
		t.Fatalf("the candidates did not receive the frozen set:\n%s", prompt)
	}

	// A different set is different bytes; none is bytes without the key.
	changed, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(changed) == string(snapshot) {
		t.Fatal("a different rule set left the snapshot identical")
	}
	rules.answer = nil
	none, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(none), `"quality_rules"`) {
		t.Error("a snapshot with none carries quality_rules")
	}
}
