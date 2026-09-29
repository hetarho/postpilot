package voice_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice"
)

var visionRef = llm.ModelRef{ProviderID: "stub", ModelID: "vision"}

// checkHarness is a made voice holding one pasted post and two answers: a situation prompt and a
// photo prompt whose photo is in storage.
func checkHarness(t *testing.T) (*voiceHarness, string) {
	t.Helper()
	h := newVoiceHarness(t)
	objects := h.withPhotos()
	alice := h.voice("alice")
	ctx := context.Background()
	at := time.Now().Add(-time.Hour)
	h.addSample(t, "alice", alice, "post", "국숫집", strings.Repeat("국물이 정말 진했어요! ", 10), at)
	for _, sample := range []voice.Sample{
		{ID: "answer", Kind: voice.SampleKindAnswer, PromptKey: "opening_greeting", Body: "안녕하세요, 오늘은 동네 빵집 이야기예요.", CreatedAt: at.Add(time.Minute)},
		{ID: "photo", Kind: voice.SampleKindAnswer, PromptKey: "photo_food", Body: "노릇한 크루아상이 먹음직스러웠어요.", PhotoKey: "voices/alice/photo.jpg", PhotoWidth: 800, PhotoHeight: 600, CreatedAt: at.Add(2 * time.Minute)},
	} {
		sample.UserID, sample.VoiceID = "alice", alice
		if err := h.store.InsertSample(ctx, sample); err != nil {
			t.Fatal(err)
		}
	}
	objects.objects["voices/alice/photo.jpg"] = 1024
	counted := voice.FingerprintOf([]voice.Material{{ID: "post", Kind: voice.SampleKindPost, CreatedAt: at, Text: strings.Repeat("국물이 정말 진했어요! ", 10)}})
	if err := h.store.PublishAnalysis(ctx, "alice", alice, voice.Analysis{Counted: counted, AnalyzeModel: analyzeRef.String(), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	return h, alice
}

// VOICE-43, VOICE-15: 검증 needs an active, made voice, an answered prompt, a write model and,
// for a photo prompt, a model that reads images; each refusal enqueues nothing.
func TestStartVoiceCheckRefusals(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	unmade, err := h.svc.CreateVoice(ctx, "alice", "아직")
	if err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		voiceID, prompt string
		model           llm.ModelRef
		want            error
	}{
		"not made":        {unmade.ID, "opening_greeting", writeOnlyRef, voice.ErrVoiceNotMade},
		"unknown prompt":  {alice, "no_such_prompt", writeOnlyRef, voice.ErrPromptNotFound},
		"unanswered":      {alice, "closing_greeting", writeOnlyRef, voice.ErrCheckPromptUnanswered},
		"not a writer":    {alice, "opening_greeting", analyzeRef, voice.ErrWriteModelRequired},
		"unknown model":   {alice, "opening_greeting", llm.ModelRef{ProviderID: "stub", ModelID: "nope"}, voice.ErrWriteModelRequired},
		"photo, no image": {alice, "photo_food", writeOnlyRef, voice.ErrCheckPhotoUnsupported},
	} {
		if _, _, err := h.svc.StartVoiceCheck(ctx, "alice", tc.voiceID, tc.prompt, tc.model); !errors.Is(err, tc.want) {
			t.Fatalf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", alice); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("a tombstone's 검증 = %v", err)
	}
	if len(h.jobs.checkCalls) != 0 {
		t.Fatalf("a refused 검증 enqueued %d jobs", len(h.jobs.checkCalls))
	}
	checks, _, err := h.svc.ListVoiceChecks(ctx, "alice", alice)
	if err != nil || len(checks) != 0 {
		t.Fatalf("a refused 검증 left %d results: %v", len(checks), err)
	}
}

// VOICE-43: the start freezes the projection with the answer withheld and enqueues one job;
// the handler makes one write call and stores the trimmed piece, measured on the list.
func TestAVoiceCheckWritesOnceWithTheAnswerWithheld(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	started, jobID, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef)
	if err != nil {
		t.Fatal(err)
	}
	if jobID == "" || started.Status != voice.CheckQueued || started.MaterialID != "answer" || started.Answer == "" || started.Prompt.Key != "opening_greeting" {
		t.Fatalf("started = %+v job=%q", started, jobID)
	}
	if !strings.HasPrefix(started.Projection, "[말투]") || !strings.Contains(started.Projection, "국물이 정말 진했어요!") {
		t.Fatalf("the projection = %q", started.Projection)
	}
	if strings.Contains(started.Projection, "동네 빵집") {
		t.Fatal("the checked answer reached the projection")
	}
	if calls := h.jobs.checkCalls; len(calls) != 1 || calls[0].CheckID != started.ID || calls[0].WriteModel != writeOnlyRef.String() {
		t.Fatalf("enqueued %+v", calls)
	}

	h.models.completeCalls = 0
	h.models.response = "  안녕하세요! 오늘도 빵 이야기예요.  \n"
	if err := h.svc.CheckVoice(ctx, voice.CheckJob{UserID: "alice", VoiceID: alice, CheckID: started.ID, WriteModel: writeOnlyRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	request := h.models.request
	if h.models.completeCalls != 1 || request.System != started.Projection || len(request.Messages) != 1 || len(request.Messages[0].Parts) != 1 {
		t.Fatalf("calls=%d request=%+v", h.models.completeCalls, request)
	}
	task := request.Messages[0].Parts[0].Text
	if !strings.Contains(task, "[문항]\n블로그 글을 시작할 때") || !strings.Contains(task, "10~15문장") {
		t.Fatalf("the task = %q", task)
	}
	checks, active, err := h.svc.ListVoiceChecks(ctx, "alice", alice)
	if err != nil || len(checks) != 1 || active != "" {
		t.Fatalf("list = %+v active=%q err=%v", checks, active, err)
	}
	done := checks[0]
	if done.Status != voice.CheckDone || done.Piece != "안녕하세요! 오늘도 빵 이야기예요." || done.Answer != "안녕하세요, 오늘은 동네 빵집 이야기예요." || done.Stale {
		t.Fatalf("done = %+v", done)
	}
	if len(done.Comparison) != len(voice.Items()) {
		t.Fatalf("comparison = %+v", done.Comparison)
	}
	// Two sentences cannot show a ratio: the short piece reads unknown where it cannot count.
	for _, item := range done.Comparison {
		if item.Item == voice.ItemMarks && !item.Unknown {
			t.Fatalf("a two-sentence piece measured its marks: %+v", item)
		}
	}
	// A second run of the same job finds nothing waiting and calls nothing.
	if err := h.svc.CheckVoice(ctx, voice.CheckJob{UserID: "alice", VoiceID: alice, CheckID: started.ID, WriteModel: writeOnlyRef.String()}, func(string, int, int) {}); err == nil || h.models.completeCalls != 1 {
		t.Fatalf("a finished check ran again: err=%v calls=%d", err, h.models.completeCalls)
	}
}

// VOICE-43: a photo prompt sends the answer's photo as an image part.
func TestAPhotoCheckSendsTheAnswersPhoto(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	started, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "photo_food", visionRef)
	if err != nil {
		t.Fatal(err)
	}
	h.models.response = "크루아상이 정말 바삭했어요."
	if err := h.svc.CheckVoice(ctx, voice.CheckJob{UserID: "alice", VoiceID: alice, CheckID: started.ID, WriteModel: visionRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	parts := h.models.request.Messages[0].Parts
	if len(parts) != 2 || string(parts[1].Image) != "jpeg:voices/alice/photo.jpg" || parts[1].MIME != voice.PhotoContentType {
		t.Fatalf("parts = %+v", parts)
	}
}

// VOICE-44, VOICE-45: a failed call stores its failure and is not repeated, an empty answer is
// MODEL_OUTPUT_INVALID, a failed enqueue leaves no result, an interrupted check reads as
// JOB_INTERRUPTED, and a retry is a new check on the same prompt.
func TestAFailedCheckKeepsItsFailureAndRetries(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	run := func(checkID string) error {
		return h.svc.CheckVoice(ctx, voice.CheckJob{UserID: "alice", VoiceID: alice, CheckID: checkID, WriteModel: writeOnlyRef.String()}, func(string, int, int) {})
	}

	h.jobs.checkErr = errors.New("queue down")
	if _, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef); err == nil {
		t.Fatal("a failed enqueue started a check")
	}
	if checks, _, _ := h.svc.ListVoiceChecks(ctx, "alice", alice); len(checks) != 0 {
		t.Fatalf("a failed enqueue left %d results", len(checks))
	}
	h.jobs.checkErr = nil

	failed, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef)
	if err != nil {
		t.Fatal(err)
	}
	h.models.completeCalls = 0
	h.models.err = llm.ErrRateLimited
	if err := run(failed.ID); err == nil || h.models.completeCalls != 1 {
		t.Fatalf("a rate-limited check = %v after %d calls", err, h.models.completeCalls)
	}
	h.models.err, h.models.response = nil, "   "
	empty, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef)
	if err != nil {
		t.Fatal(err)
	}
	if err := run(empty.ID); err == nil {
		t.Fatal("an empty piece was stored")
	}
	interrupted, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef)
	if err != nil {
		t.Fatal(err)
	}

	checks, _, err := h.svc.ListVoiceChecks(ctx, "alice", alice)
	if err != nil || len(checks) != 3 {
		t.Fatalf("list = %d err=%v", len(checks), err)
	}
	want := map[string]string{failed.ID: "MODEL_RATE_LIMITED", empty.ID: "MODEL_OUTPUT_INVALID", interrupted.ID: "JOB_INTERRUPTED"}
	for _, check := range checks {
		if check.Status != voice.CheckFailed || check.Failure == nil || check.Failure.Reason != want[check.ID] {
			t.Fatalf("%s = %+v %+v", check.ID, check.Status, check.Failure)
		}
	}
	if checks[0].ID != interrupted.ID {
		t.Fatalf("the list is not newest first: %s", checks[0].ID)
	}
	// While its job is still queued, the newest check is live, and the list names the job.
	h.jobs.activeChecks[alice] = &voice.ActiveJob{ID: "job-live"}
	checks, active, _ := h.svc.ListVoiceChecks(ctx, "alice", alice)
	if active != "job-live" || checks[0].Status != voice.CheckQueued || checks[0].Failure != nil {
		t.Fatalf("a live check = %+v active=%q", checks[0], active)
	}
	delete(h.jobs.activeChecks, alice)

	retried, jobID, err := h.svc.RetryVoiceCheck(ctx, "alice", failed.ID, writeOnlyRef)
	if err != nil || jobID == "" || retried.ID == failed.ID || retried.PromptKey != failed.PromptKey || retried.Status != voice.CheckQueued {
		t.Fatalf("retry = %+v job=%q err=%v", retried, jobID, err)
	}
	if _, _, err := h.svc.RetryVoiceCheck(ctx, "bob", failed.ID, writeOnlyRef); !errors.Is(err, voice.ErrCheckNotFound) {
		t.Fatalf("another account's retry = %v", err)
	}
}

// VOICE-43: a result written before the current analysis is marked, and a deleted answer reads
// as deleted while the piece stays.
func TestAnOlderCheckIsMarkedAndKeepsItsPiece(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	started, _, err := h.svc.StartVoiceCheck(ctx, "alice", alice, "opening_greeting", writeOnlyRef)
	if err != nil {
		t.Fatal(err)
	}
	h.models.response = "안녕하세요! 오늘은 빵집이에요."
	if err := h.svc.CheckVoice(ctx, voice.CheckJob{UserID: "alice", VoiceID: alice, CheckID: started.ID, WriteModel: writeOnlyRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if err := h.store.PublishAnalysis(ctx, "alice", alice, voice.Analysis{AnalyzeModel: analyzeRef.String(), CreatedAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.DeleteSample(ctx, "alice", alice, "answer"); err != nil {
		t.Fatal(err)
	}
	checks, _, err := h.svc.ListVoiceChecks(ctx, "alice", alice)
	if err != nil || len(checks) != 1 {
		t.Fatalf("list = %d err=%v", len(checks), err)
	}
	if !checks[0].Stale || !checks[0].AnswerDeleted || checks[0].Answer != "" || checks[0].Piece == "" {
		t.Fatalf("an older check = %+v", checks[0])
	}
}

// MODEL-67: 말투 반영 비교 freezes 검증's input — the projection with the answer withheld, the
// prompt, the answer's text and a photo prompt's photo — refuses what 검증 refuses, and each
// candidate makes 검증's one call over that snapshot, reporting its usage.
func TestAReflectionFreezesAndRunsTheCheckPrompt(t *testing.T) {
	h, alice := checkHarness(t)
	ctx := context.Background()
	if _, err := h.svc.SnapshotReflectionInput(ctx, "alice", alice, "closing_greeting"); !errors.Is(err, voice.ErrCheckPromptUnanswered) {
		t.Fatalf("an unanswered prompt = %v", err)
	}
	input, err := h.svc.SnapshotReflectionInput(ctx, "alice", alice, "photo_food")
	if err != nil || !input.Photo || input.PromptKey != "photo_food" || input.MaterialID != "photo" {
		t.Fatalf("input = %+v err=%v", input, err)
	}
	prompt, answer, err := voice.ReflectionView(input.Content)
	if err != nil || prompt.Key != "photo_food" || answer != "노릇한 크루아상이 먹음직스러웠어요." {
		t.Fatalf("view = %+v %q err=%v", prompt, answer, err)
	}
	h.models.completeCalls = 0
	h.models.response = " 크루아상이 정말 바삭했어요. "
	result, err := h.svc.RunReflectionCandidate(ctx, input.Content, visionRef)
	if err != nil || result.Piece != "크루아상이 정말 바삭했어요." || h.models.completeCalls != 1 {
		t.Fatalf("run = %+v err=%v calls=%d", result, err, h.models.completeCalls)
	}
	request := h.models.request
	if strings.Contains(request.System, "먹음직스러웠어요") || len(request.Messages[0].Parts) != 2 || !strings.Contains(request.Messages[0].Parts[0].Text, "[문항]") {
		t.Fatalf("the call = %+v", request)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", alice); err != nil {
		t.Fatal(err)
	}
	// The snapshot is the retry boundary: a deleted voice's comparison still runs, and still reads.
	if _, err := h.svc.RunReflectionCandidate(ctx, input.Content, visionRef); err != nil {
		t.Fatalf("a frozen snapshot on a deleted voice = %v", err)
	}
	if comparison, err := h.svc.CompareText(ctx, "alice", alice, result.Piece); err != nil || len(comparison) != len(voice.Items()) {
		t.Fatalf("compare = %d err=%v", len(comparison), err)
	}
	if _, err := h.svc.SnapshotReflectionInput(ctx, "alice", alice, "opening_greeting"); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("a tombstone's snapshot = %v", err)
	}
	if _, err := h.svc.CompareText(ctx, "bob", alice, "글"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("another account's compare = %v", err)
	}
}
