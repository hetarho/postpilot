package experiment

import (
	"context"
	"errors"
	"testing"
)

// fakeReflection is the voice context's 말투 반영 비교 behaviour: one voice with one answered
// prompt and one photo prompt, a candidate writing "piece:<model>".
type fakeReflection struct {
	snapshotErr error
	runs        []ModelRef
	runContent  [][]byte
	fail        map[string]error
	// compares is the texts of each comparison call.
	compares [][]string
}

func (f *fakeReflection) Snapshot(_ context.Context, _, _, promptKey string) (ReflectionSnapshot, error) {
	if f.snapshotErr != nil {
		return ReflectionSnapshot{}, f.snapshotErr
	}
	return ReflectionSnapshot{
		Content: []byte(`{"prompt":"` + promptKey + `","answer":"내 답"}`), PromptVersion: "voice-reflection-v1",
		PromptKey: promptKey, MaterialID: "answer-1", Photo: promptKey == "photo_food",
	}, nil
}

func (f *fakeReflection) Run(_ context.Context, content []byte, model ModelRef) (CandidateResult, error) {
	f.runs = append(f.runs, model)
	f.runContent = append(f.runContent, content)
	if err := f.fail[model.ModelID]; err != nil {
		return CandidateResult{}, err
	}
	return CandidateResult{Output: []byte("piece:" + model.ModelID), Usage: UsageReport{PromptTokens: 5, CompletionTokens: 3}}, nil
}

func (f *fakeReflection) PromptText(key string) string  { return "문항:" + key }
func (f *fakeReflection) Answer([]byte) (string, error) { return "내 답", nil }

// Compare answers each text with a comparison headed by the text itself, so a test can see which
// piece's comparison landed on which candidate.
func (f *fakeReflection) Compare(_ context.Context, _, _ string, texts []string) ([][]ItemComparison, error) {
	f.compares = append(f.compares, texts)
	out := make([][]ItemComparison, 0, len(texts))
	for _, text := range texts {
		out = append(out, []ItemComparison{{Item: "endings", Headline: text, Facets: []ComparisonFacet{{Key: "해요", Unit: "share", Voice: 0.9, Text: float64(len(text)) / 100}}}})
	}
	return out, nil
}

func reflectionService(t *testing.T) (*Service, *memoryStore, *fakeCatalog, *fakeJobs, *fakeRunner, *fakeReflection) {
	t.Helper()
	svc, store, catalog, jobs, runner := newTestService()
	svc.SetVoiceDirectory(fakeVoices{deleted: map[string]bool{"voice-gone": true}})
	reflection := &fakeReflection{fail: map[string]error{}}
	svc.SetVoiceReflection(reflection)
	return svc, store, catalog, jobs, runner, reflection
}

var (
	refA = ModelRef{ProviderID: "p", ModelID: "a"}
	refB = ModelRef{ProviderID: "p", ModelID: "b"}
)

func TestRankedReflectionChecksAllFivePhotoCandidates(t *testing.T) {
	svc, store, catalog, jobs, _, _ := reflectionService(t)
	refs := []ModelRef{refA, refB, {ProviderID: "p", ModelID: "c"}, {ProviderID: "p", ModelID: "d"}, {ProviderID: "p", ModelID: "e"}}
	for _, ref := range refs[2:] {
		catalog.models[ref] = Model{Ref: ref, Label: ref.ModelID, Enabled: true, Vision: true, Stages: []string{"write"}}
	}
	request := ReflectionStartRequest{UserID: "alice", VoiceID: "voice-a", PromptKey: "photo_food", Candidates: refs}
	last := catalog.models[refs[4]]
	last.Vision = false
	catalog.models[refs[4]] = last
	if _, err := svc.StartVoiceReflection(context.Background(), request); !errors.Is(err, ErrPhotoUnsupported) {
		t.Fatalf("last candidate without vision: %v", err)
	}
	if len(store.rows) != 0 || len(jobs.ids) != 0 {
		t.Fatal("refused photo comparison created work")
	}
	last.Vision = true
	catalog.models[refs[4]] = last
	started, err := svc.StartVoiceReflection(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	found, err := store.Get(context.Background(), started.ExperimentID)
	if err != nil {
		t.Fatal(err)
	}
	if found.ReviewMode != ReviewCandidateRanking || len(found.Candidates) != 5 || len(jobs.requests[0].Models) != 5 {
		t.Fatalf("reflection = %+v, job=%+v", found, jobs.requests[0])
	}
}

// MODEL-67: the review measures every delivered piece in one comparison call, so a poll reads the
// voice and its analysis once however many candidates there are; a failed candidate is not
// measured and a run with nothing delivered asks nothing.
func TestAReflectionReviewComparesEveryPieceInOneCall(t *testing.T) {
	svc, store, catalog, _, _, reflection := reflectionService(t)
	refs := []ModelRef{refA, refB, {ProviderID: "p", ModelID: "c"}, {ProviderID: "p", ModelID: "d"}, {ProviderID: "p", ModelID: "e"}}
	for _, ref := range refs[2:] {
		catalog.models[ref] = Model{Ref: ref, Label: ref.ModelID, Enabled: true, Vision: true, Stages: []string{"write"}}
	}
	ctx := context.Background()
	started, err := svc.StartVoiceReflection(ctx, ReflectionStartRequest{UserID: "alice", VoiceID: "voice-a", PromptKey: "opening_greeting", Candidates: refs})
	if err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(ctx, started.ExperimentID)
	if detail, err := svc.ReflectionDetail(ctx, found); err != nil || len(detail.Comparisons) != 0 || len(reflection.compares) != 0 {
		t.Fatalf("an undelivered review = %+v compares=%v err=%v", detail, reflection.compares, err)
	}
	reflection.fail["c"] = errors.New("provider down")
	if err := svc.Handle(ctx, found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ = store.Get(ctx, found.ID)
	detail, err := svc.ReflectionDetail(ctx, found)
	if err != nil || len(reflection.compares) != 1 || len(reflection.compares[0]) != 4 || len(detail.Comparisons) != 4 {
		t.Fatalf("detail = %+v compares=%v err=%v", detail, reflection.compares, err)
	}
	for _, candidate := range found.Candidates {
		comparison, measured := detail.Comparisons[candidate.ID]
		if candidate.Model.ModelID == "c" {
			if measured {
				t.Fatalf("a failed candidate was measured: %+v", comparison)
			}
			continue
		}
		if !measured || comparison[0].Headline != string(candidate.Output) {
			t.Fatalf("candidate %s got %+v", candidate.Model.ModelID, comparison)
		}
	}
}

// MODEL-31, MODEL-67: 말투 반영 비교 names a voice and one of its answered prompts and two different
// write models, both reading images for a photo prompt; each refusal creates nothing and queues
// nothing.
func TestAReflectionStartRefusesBeforeAnyWork(t *testing.T) {
	svc, store, catalog, jobs, runner, reflection := reflectionService(t)
	writeOnly := ModelRef{ProviderID: "p", ModelID: "w"}
	catalog.models[writeOnly] = Model{Ref: writeOnly, Label: "W", Enabled: true, Stages: []string{"write"}}
	observeOnly := ModelRef{ProviderID: "p", ModelID: "o"}
	catalog.models[observeOnly] = Model{Ref: observeOnly, Label: "O", Enabled: true, Vision: true, Stages: []string{"observe"}}
	ctx := context.Background()
	start := func(voiceID, prompt string, a, b ModelRef) error {
		_, err := svc.StartVoiceReflection(ctx, ReflectionStartRequest{UserID: "alice", VoiceID: voiceID, PromptKey: prompt, ModelA: a, ModelB: b})
		return err
	}
	for name, tc := range map[string]struct {
		err  error
		want error
	}{
		"no voice":        {start("", "opening_greeting", refA, refB), ErrVoiceRequired},
		"same model":      {start("voice-a", "opening_greeting", refA, refA), ErrDuplicateCandidates},
		"not a writer":    {start("voice-a", "opening_greeting", refA, observeOnly), ErrModelRequired},
		"tombstone":       {start("voice-gone", "opening_greeting", refA, refB), ErrVoiceUnavailable},
		"photo, no image": {start("voice-a", "photo_food", refA, writeOnly), ErrPhotoUnsupported},
	} {
		if !errors.Is(tc.err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", name, tc.err, tc.want)
		}
	}
	for _, refusal := range []error{ErrVoiceNotMade, ErrPromptUnanswered, ErrVoiceNotFound} {
		reflection.snapshotErr = refusal
		if err := start("voice-a", "opening_greeting", refA, refB); !errors.Is(err, refusal) {
			t.Fatalf("a snapshot refusal = %v, want %v", err, refusal)
		}
	}
	if len(store.rows) != 0 || len(jobs.ids) != 0 || len(reflection.runs) != 0 || runner.snapshotCalls != 0 {
		t.Fatalf("a refused start left rows=%d jobs=%d runs=%d", len(store.rows), len(jobs.ids), len(reflection.runs))
	}
}

// MODEL-30, MODEL-67, MODEL-36: the start freezes the voice's snapshot into a lab write
// comparison sourced from the voice, with one job naming neither a post nor the voice; each
// candidate writes through the reflection port, the verdict counts on the write board, the
// decided result offers adoption and never an application, and the review reads the prompt, the
// answer and each piece's comparison.
func TestAReflectionRunsVotesOnTheWriteBoardAndAdoptsOnly(t *testing.T) {
	svc, store, catalog, jobs, runner, reflection := reflectionService(t)
	ctx := context.Background()
	started, err := svc.StartVoiceReflection(ctx, ReflectionStartRequest{UserID: "alice", VoiceID: "voice-a", PromptKey: "opening_greeting", ModelA: refA, ModelB: refB})
	if err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(ctx, started.ExperimentID)
	if found.Source != SourceVoice || found.Stage != StageWrite || found.Origin != OriginLab || found.PostSlug != "" ||
		found.VoiceID != "voice-a" || found.VoicePromptKey != "opening_greeting" || found.VoiceMaterialID != "answer-1" ||
		found.TargetLanguage == nil || *found.TargetLanguage != LanguageKorean || found.InputHash == "" || found.PromptVersion != "voice-reflection-v1" {
		t.Fatalf("started = %+v", found)
	}
	if request := jobs.requests[0]; request.PostSlug != "" || request.VoiceID != "" || len(request.Models) != 2 {
		t.Fatalf("the job = %+v", request)
	}

	if err := svc.Handle(ctx, found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ = store.Get(ctx, found.ID)
	if found.Status != StatusReview || len(reflection.runs) != 2 || runner.runCalls != 0 {
		t.Fatalf("after handle = %s runs=%v runner=%d", found.Status, reflection.runs, runner.runCalls)
	}
	for _, content := range reflection.runContent {
		if string(content) != string(found.InputSnapshot) {
			t.Fatalf("a candidate read %q, not the frozen snapshot", content)
		}
	}
	detail, err := svc.ReflectionDetail(ctx, found)
	if err != nil || detail.PromptText != "문항:opening_greeting" || detail.Answer != "내 답" || len(detail.Comparisons) != 2 {
		t.Fatalf("detail = %+v err=%v", detail, err)
	}

	if _, err := svc.DecideWrite(ctx, "alice", found.ID, found.Candidates[0].ID, false, nil); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("a committing verdict = %v", err)
	}
	decided, err := svc.Choose(ctx, "alice", found.ID, found.Candidates[0].ID, false, []CandidateBadges{{CandidateID: found.Candidates[0].ID, Badges: []Badge{BadgeInVoice}}})
	if err != nil || decided.Status != StatusDecided || decided.Outcome != OutcomeWinner {
		t.Fatalf("verdict = %+v err=%v", decided, err)
	}
	board, err := svc.Leaderboard(ctx, "alice", StageWrite, WindowWeek, ScopeMe)
	if err != nil || len(board) != 2 {
		t.Fatalf("the write board = %+v err=%v", board, err)
	}
	if _, err := svc.ApplyWinner(ctx, "alice", found.ID); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("an application = %v", err)
	}
	adopted, stage, err := svc.AdoptWinner(ctx, "alice", found.ID)
	if err != nil || stage != StageWrite || adopted != decided.Winner().Model || len(catalog.adopted) != 1 {
		t.Fatalf("adoption = %v %s %v adopted=%v", adopted, stage, err, catalog.adopted)
	}

	voiced, _ := svc.List(ctx, "alice", StageWrite, SourceVoice)
	posts, _ := svc.List(ctx, "alice", StageWrite, SourcePost)
	if len(voiced) != 1 || len(posts) != 0 {
		t.Fatalf("history voice=%d post=%d", len(voiced), len(posts))
	}
}

// MODEL-35: a failed candidate is retried alone against the original snapshot.
func TestAReflectionRetriesTheFailedCandidateAlone(t *testing.T) {
	svc, store, _, _, _, reflection := reflectionService(t)
	ctx := context.Background()
	reflection.fail["b"] = errors.New("provider down")
	started, err := svc.StartVoiceReflection(ctx, ReflectionStartRequest{UserID: "alice", VoiceID: "voice-a", PromptKey: "opening_greeting", ModelA: refA, ModelB: refB})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, started.ExperimentID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ := store.Get(ctx, started.ExperimentID)
	if found.Status != StatusPartial {
		t.Fatalf("after a failed candidate = %s", found.Status)
	}
	delete(reflection.fail, "b")
	reflection.runs, reflection.runContent = nil, nil
	if _, err := svc.Retry(ctx, "alice", found.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.Handle(ctx, found.ID, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	found, _ = store.Get(ctx, found.ID)
	if found.Status != StatusReview || len(reflection.runs) != 1 || reflection.runs[0] != refB || string(reflection.runContent[0]) != string(found.InputSnapshot) {
		t.Fatalf("retry ran %v on %q, status %s", reflection.runs, reflection.runContent, found.Status)
	}
}
