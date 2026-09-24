package generation

import (
	"context"
	"encoding/json"
	"fmt"
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

// recordingPhrases is the phrase port, recording the same way.
type recordingPhrases struct {
	answer []string
	calls  int
	field  string
}

func (r *recordingPhrases) For(_ context.Context, field string) ([]string, error) {
	r.calls++
	r.field = field
	return r.answer, nil
}

func freezingService(posts *fakePosts, jobs *fakeJobs, models *fakeModels, rules *recordingRules, phrases *recordingPhrases) *Service {
	deps := testDeps()
	deps.QualityRules = rules
	deps.FieldPhrases = phrases
	return NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, jobs, 4, testReasoningPolicy, testBudget, deps)
}

func tickedPost() *fakePosts {
	return &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모",
		TargetLanguage: LanguageEnglish, QualityRuleIDs: []string{"title_saturation", "composition"}, Field: "restaurant",
	}}
}

func phrases(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf("문구 %02d", i)
	}
	return out
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
	svc := freezingService(posts, jobs, newFakeModels(), rules, &recordingPhrases{})
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

// GEN-48, QUAL-41: a post with a 분야 freezes the first 30 phrases in the list's order; no 분야
// never reaches the port; an empty list is none, exactly like no 분야.
func TestStartFreezesTheFirstThirtyFieldPhrases(t *testing.T) {
	posts, jobs, list := tickedPost(), &fakeJobs{id: "job"}, &recordingPhrases{answer: phrases(35)}
	svc := freezingService(posts, jobs, newFakeModels(), &recordingRules{}, list)
	startOnce(t, svc)
	if list.calls != 1 || list.field != "restaurant" {
		t.Fatalf("the port was asked %+v", list)
	}
	if got := jobs.frozen(t, 0).FieldPhrases; !reflect.DeepEqual(got, phrases(FieldPhrasesMax)) {
		t.Fatalf("froze %d phrases: %q", len(got), got)
	}

	list.answer = []string{}
	startOnce(t, svc)
	if got := jobs.frozen(t, 1).FieldPhrases; got != nil {
		t.Fatalf("an empty list froze %q", got)
	}

	posts.input.Field = ""
	startOnce(t, svc)
	if list.calls != 2 || jobs.frozen(t, 2).FieldPhrases != nil {
		t.Fatalf("a post with no 분야 reached the port: %d calls", list.calls)
	}
}

// A post with nothing ticked and no list writes the payload and the prompt it wrote before this
// task, byte for byte, and asks neither port.
func TestNoTicksAndNoListLeaveThePayloadAndPromptByteIdentical(t *testing.T) {
	posts := &fakePosts{input: PostInput{Slug: "post", UserID: "alice", Voice: liveVoice}}
	jobs, rules, list := &fakeJobs{id: "job"}, &recordingRules{answer: []string{"never"}}, &recordingPhrases{answer: []string{"never"}}
	svc := freezingService(posts, jobs, newFakeModels(), rules, list)
	startOnce(t, svc)
	raw := jobs.generatePayloads[0]
	// Exactly what HEAD's enqueue wrote for this post: its language, its resolved tag count and
	// no observation decision — and neither new member.
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
		FieldPhrases: decoded.FieldPhrases,
	})
	if system != wantSystem || user != wantUser {
		t.Fatal("the no-tick, no-list prompt moved off the golden")
	}
	if rules.calls != 0 || list.calls != 0 {
		t.Fatalf("the ports were asked: %d rule calls, %d phrase calls", rules.calls, list.calls)
	}
}

// The drain reads the payload alone: ticks edited, posts published or the list replaced after
// the enqueue reach nothing in that run, and Generate asks neither port.
func TestTheDrainIgnoresRowsChangedAfterEnqueue(t *testing.T) {
	posts, jobs := tickedPost(), &fakeJobs{id: "job"}
	rules, list := &recordingRules{answer: []string{"frozen rule"}}, &recordingPhrases{answer: []string{"frozen phrase"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := freezingService(posts, jobs, models, rules, list)
	startOnce(t, svc)
	raw := jobs.generatePayloads[0]

	rules.answer, list.answer = []string{"live rule"}, []string{"live phrase"}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{TargetLanguage: decoded.TargetLanguage, QualityRules: decoded.QualityRules, FieldPhrases: decoded.FieldPhrases}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	sent := models.calls[len(models.calls)-1].request
	prompt := sent.System + "\n" + sent.Messages[0].Parts[0].Text
	for _, frozen := range []string{"frozen rule", "frozen phrase", FieldPhrasesHeading} {
		if !strings.Contains(prompt, frozen) {
			t.Errorf("the write lost %q", frozen)
		}
	}
	for _, live := range []string{"live rule", "live phrase"} {
		if strings.Contains(prompt, live) {
			t.Errorf("the write read the live %q", live)
		}
	}
	if rules.calls != 1 || list.calls != 1 {
		t.Fatalf("the drain asked a port: %d rule calls, %d phrase calls", rules.calls, list.calls)
	}
}

// QUAL-29: a run with frozen rules writes once and measures nothing; there is no correction pass.
func TestAFrozenRuleRunMakesOneWriteCall(t *testing.T) {
	posts := tickedPost()
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	rules, list := &recordingRules{}, &recordingPhrases{}
	svc := freezingService(posts, &fakeJobs{id: "job"}, models, rules, list)
	if err := svc.Generate(context.Background(), GenerateJob{
		UserID:     "alice",
		PostSlug:   "post",
		VoiceID:    liveVoice.ID,
		WriteModel: writeRef.String(),
		Payload:    mustGeneratePayload(t, generationOptions{TargetLanguage: LanguageEnglish, QualityRules: []string{"frozen rule"}, FieldPhrases: []string{"frozen phrase"}}),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(models.calls) != 1 || models.calls[0].ref != writeRef {
		t.Fatalf("calls = %+v, want exactly one write call", models.calls)
	}
	if rules.calls != 0 || list.calls != 0 {
		t.Fatalf("the run asked a port: %d rule calls, %d phrase calls", rules.calls, list.calls)
	}
}

// GEN-51 is write-only and GEN-57 keeps a revision off the phrase list: StartRevision asks
// neither port, and its payload carries neither member.
func TestTheRevisionFreezesNeither(t *testing.T) {
	posts := tickedPost()
	posts.input.Content = revisionContent("body")
	jobs, rules, list := &fakeJobs{id: "job"}, &recordingRules{answer: []string{"rule"}}, &recordingPhrases{answer: []string{"phrase"}}
	svc := freezingService(posts, jobs, newFakeModels(), rules, list)
	if _, err := svc.StartRevision(context.Background(), StartRevisionRequest{UserID: "alice", PostSlug: "post", Instruction: "더 짧게", WriteModel: writeRef.String()}); err != nil {
		t.Fatal(err)
	}
	if rules.calls != 0 || list.calls != 0 {
		t.Fatalf("a revision asked a port: %d rule calls, %d phrase calls", rules.calls, list.calls)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(jobs.payloads[0], &keys); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{"quality_rules", "field_phrases"} {
		if _, carried := keys[member]; carried {
			t.Errorf("the revision payload carries %s", member)
		}
	}
}

// Both candidates read one frozen set from the snapshot, and a different set is a different
// frozen input; with none the snapshot has neither key.
func TestWriteSnapshotFreezesRulesAndPhrasesForBothCandidates(t *testing.T) {
	ctx := context.Background()
	posts := tickedPost()
	rules, list := &recordingRules{answer: []string{"frozen rule"}}, &recordingPhrases{answer: []string{"frozen phrase"}}
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return okContent(), nil }
	svc := freezingService(posts, &fakeJobs{id: "job"}, models, rules, list)

	snapshot, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(ctx, snapshot, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	rules.answer, list.answer = []string{"live rule"}, []string{"live phrase"}
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
	if prompt := first.System + first.Messages[0].Parts[0].Text; !strings.Contains(prompt, "frozen rule") || !strings.Contains(prompt, "frozen phrase") {
		t.Fatalf("the candidates did not receive the frozen set:\n%s", prompt)
	}

	// A different set is different bytes; none is bytes with neither key.
	changed, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if string(changed) == string(snapshot) {
		t.Fatal("a different rule set and phrase list left the snapshot identical")
	}
	rules.answer, list.answer = nil, nil
	none, err := svc.SnapshotWriteInput(ctx, "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{`"quality_rules"`, `"field_phrases"`} {
		if strings.Contains(string(none), member) {
			t.Errorf("a snapshot with none carries %s", member)
		}
	}
}
