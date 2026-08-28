package voice_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

var analyzeRef = llm.ModelRef{ProviderID: "stub", ModelID: "analyze"}

type fakeModels struct {
	selected map[string]llm.ModelRef
	response string
	err      error
	request  llm.Request
}

type changingCorpusModels struct {
	started  chan struct{}
	release  chan struct{}
	mu       sync.Mutex
	requests []string
}

func (f *changingCorpusModels) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return analyzeRef, true, nil
}

func (f *changingCorpusModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	corpus := request.Messages[0].Parts[0].Text
	f.mu.Lock()
	call := len(f.requests)
	f.requests = append(f.requests, corpus)
	f.mu.Unlock()
	if call == 0 {
		close(f.started)
		<-f.release
		return llm.Response{Text: "## 1. 종결어미 분포\nold\n## 8. never uses\nold"}, nil
	}
	return llm.Response{Text: "## 1. 종결어미 분포\nnew\n## 8. never uses\nnew"}, nil
}

func (f *fakeModels) AnalyzeModel(_ context.Context, userID string) (llm.ModelRef, bool, error) {
	ref, ok := f.selected[userID]
	return ref, ok, nil
}

func (f *fakeModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.request = request
	return llm.Response{Text: f.response}, f.err
}

type fakeJobs struct {
	mu           sync.Mutex
	active       map[string]*voice.ActiveJob
	enqueueID    string
	enqueueErr   error
	enqueueCalls []voice.AnalysisJobRequest
}

func (f *fakeJobs) Enqueue(_ context.Context, request voice.AnalysisJobRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueueCalls = append(f.enqueueCalls, request)
	return f.enqueueID, f.enqueueErr
}

func (f *fakeJobs) ActiveForUserKind(_ context.Context, userID, _ string) (*voice.ActiveJob, error) {
	return f.active[userID], nil
}

func (f *fakeJobs) calls() []voice.AnalysisJobRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]voice.AnalysisJobRequest(nil), f.enqueueCalls...)
}

type voiceHarness struct {
	store  *voicestore.Store
	db     *db.DB
	models *fakeModels
	jobs   *fakeJobs
	svc    *voice.Service
}

func newVoiceHarness(t *testing.T) *voiceHarness {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, userID := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec(
			"INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", userID, now,
		); err != nil {
			t.Fatal(err)
		}
	}
	store := voicestore.New(handle.Writer, handle.Reader)
	models := &fakeModels{selected: map[string]llm.ModelRef{"alice": analyzeRef, "bob": analyzeRef}}
	jobs := &fakeJobs{active: map[string]*voice.ActiveJob{}, enqueueID: "job-new"}
	return &voiceHarness{
		store: store, db: handle, models: models, jobs: jobs,
		svc: voice.NewService(store, models, jobs),
	}
}

func longSample(char string) string { return strings.Repeat(char, voice.SampleMinChars) }

func TestAddSampleValidatesBeforeWritingAndReturnsActiveJob(t *testing.T) {
	h := newVoiceHarness(t)
	_, _, err := h.svc.AddSample(context.Background(), "alice", "", strings.Repeat("가", 199), analyzeRef)
	var short *voice.SampleTooShortError
	if !errors.As(err, &short) || short.Chars != 199 {
		t.Fatalf("short sample error = %v", err)
	}
	delete(h.models.selected, "alice")
	_, _, err = h.svc.AddSample(context.Background(), "alice", "", longSample("나"), analyzeRef)
	if !errors.Is(err, voice.ErrAnalyzeModelRequired) {
		t.Fatalf("missing model error = %v", err)
	}
	if count, _ := h.store.CountSamples(context.Background(), "alice"); count != 0 {
		t.Fatalf("samples stored before model validation = %d", count)
	}

	h.models.selected["alice"] = analyzeRef
	h.jobs.enqueueErr = errors.New("queue unavailable")
	_, _, err = h.svc.AddSample(context.Background(), "alice", "", longSample("마"), analyzeRef)
	if !errors.Is(err, voice.ErrSampleMutation) {
		t.Fatalf("enqueue failure = %v", err)
	}
	if count, _ := h.store.CountSamples(context.Background(), "alice"); count != 0 {
		t.Fatalf("failed enqueue left %d samples", count)
	}

	h.jobs.enqueueErr = &voice.JobAlreadyInProgressError{ActiveID: "job-active"}
	sample, jobID, err := h.svc.AddSample(context.Background(), "alice", "", longSample("다"), analyzeRef)
	if err != nil || jobID != "job-active" {
		t.Fatalf("AddSample = sample=%+v job=%q err=%v", sample, jobID, err)
	}
	if sample.Label != strings.Repeat("다", voice.LabelFallbackChars) || sample.Chars != voice.SampleMinChars {
		t.Fatalf("sample fallback/count = %+v", sample)
	}
	if calls := h.jobs.calls(); len(calls) != 2 || calls[1].WriteModel != analyzeRef.String() {
		t.Fatalf("enqueue calls = %+v", calls)
	}
}

func TestAssembleCorpusIncludesEveryBody(t *testing.T) {
	corpus := voice.AssembleCorpus([]voice.Sample{
		{Label: "첫 글", Body: "첫 번째 본문"}, {Label: "둘째 글", Body: "두 번째 본문"},
	})
	for _, expected := range []string{"첫 글", "첫 번째 본문", "둘째 글", "두 번째 본문"} {
		if !strings.Contains(corpus, expected) {
			t.Errorf("corpus missing %q: %s", expected, corpus)
		}
	}
}

func TestProfileForPromptMostRecentTruncatedAndEmpty(t *testing.T) {
	h := newVoiceHarness(t)
	style, excerpts, rules, empty, err := h.svc.ProfileForPrompt(context.Background(), "alice")
	if err != nil || style != "" || rules != "" || len(excerpts) != 0 || !empty {
		t.Fatalf("empty profile = %q %+v %q %v err=%v", style, excerpts, rules, empty, err)
	}
	if err := h.store.UpsertProfile(context.Background(), voice.Profile{
		UserID: "alice", Styleguide: "STYLE", Rules: "RULES", UpdatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	markers := []rune{'가', '나', '다', '라'}
	for i := range 4 {
		body := strings.Repeat(string(markers[i]), voice.ExcerptChars+10)
		if err := h.store.InsertSample(context.Background(), voice.Sample{
			ID: string(rune('a' + i)), UserID: "alice", Label: "sample", Body: body, CreatedAt: base.Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatal(err)
		}
	}
	style, excerpts, rules, empty, err = h.svc.ProfileForPrompt(context.Background(), "alice")
	if err != nil || style != "STYLE" || rules != "RULES" || empty || len(excerpts) != voice.ExcerptCount {
		t.Fatalf("profile prompt = %q lens=%d %q %v err=%v", style, len(excerpts), rules, empty, err)
	}
	if []rune(excerpts[0])[0] != '라' {
		t.Fatalf("first excerpt is not most recent: %q", []rune(excerpts[0])[0])
	}
	for _, excerpt := range excerpts {
		if len([]rune(excerpt)) != voice.ExcerptChars {
			t.Fatalf("excerpt length = %d", len([]rune(excerpt)))
		}
	}
}

func TestAppendRuleDeduplicatesAndPreservesStyleguide(t *testing.T) {
	h := newVoiceHarness(t)
	if _, err := h.svc.Update(context.Background(), "alice", "hand edited style", "existing"); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"  new rule  ", "new rule"} {
		if err := h.svc.AppendRule(context.Background(), "alice", line); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || profile.Styleguide != "hand edited style" || profile.Rules != "existing\nnew rule" {
		t.Fatalf("profile after rule = %+v err=%v", profile, err)
	}
}

func TestConcurrentAppendRuleDoesNotLoseLines(t *testing.T) {
	h := newVoiceHarness(t)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, line := range []string{"first", "second"} {
		go func() {
			<-start
			errs <- h.svc.AppendRule(context.Background(), "alice", line)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	profile, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || !strings.Contains(profile.Rules, "first") || !strings.Contains(profile.Rules, "second") {
		t.Fatalf("concurrent rules = %q err=%v", profile.Rules, err)
	}
}

func TestProfilesAndSamplesAreIsolatedByUser(t *testing.T) {
	h := newVoiceHarness(t)
	h.jobs.active["alice"] = &voice.ActiveJob{ID: "analysis-alice"}
	for _, row := range []struct{ user, style, rules string }{
		{"alice", "alice style", "alice rule"}, {"bob", "bob style", "bob rule"},
	} {
		if _, err := h.svc.Update(context.Background(), row.user, row.style, row.rules); err != nil {
			t.Fatal(err)
		}
		if err := h.store.InsertSample(context.Background(), voice.Sample{
			ID: row.user + "-sample", UserID: row.user, Label: row.user, Body: longSample(row.user), CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, userID := range []string{"alice", "bob"} {
		profile, err := h.svc.Get(context.Background(), userID)
		if err != nil || profile.Styleguide != userID+" style" || profile.Rules != userID+" rule" || len(profile.Samples) != 1 || profile.Samples[0].Label != userID {
			t.Fatalf("%s profile leaked/missing: %+v err=%v", userID, profile, err)
		}
		if userID == "alice" && profile.ActiveJobID != "analysis-alice" {
			t.Fatalf("active analysis was not exposed: %+v", profile)
		}
		if userID == "bob" && profile.ActiveJobID != "" {
			t.Fatalf("alice active job leaked to bob: %+v", profile)
		}
	}
}

func TestDeleteReenqueuesOnlyWhileSamplesRemain(t *testing.T) {
	h := newVoiceHarness(t)
	for _, id := range []string{"one", "two"} {
		if err := h.store.InsertSample(context.Background(), voice.Sample{
			ID: id, UserID: "alice", Label: id, Body: longSample(id), CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	jobID, err := h.svc.DeleteSample(context.Background(), "alice", "one")
	if err != nil || jobID != "job-new" || len(h.jobs.calls()) != 1 {
		t.Fatalf("first delete = job=%q calls=%d err=%v", jobID, len(h.jobs.calls()), err)
	}
	jobID, err = h.svc.DeleteSample(context.Background(), "alice", "two")
	if err != nil || jobID != "" || len(h.jobs.calls()) != 1 {
		t.Fatalf("last delete = job=%q calls=%d err=%v", jobID, len(h.jobs.calls()), err)
	}
}

func TestDeleteRestoresSampleWhenEnqueueFails(t *testing.T) {
	h := newVoiceHarness(t)
	for _, id := range []string{"keep", "delete"} {
		if err := h.store.InsertSample(context.Background(), voice.Sample{
			ID: id, UserID: "alice", Label: id, Body: longSample(id), CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	h.jobs.enqueueErr = errors.New("queue unavailable")
	if _, err := h.svc.DeleteSample(context.Background(), "alice", "delete"); !errors.Is(err, voice.ErrSampleMutation) {
		t.Fatalf("delete enqueue error = %v", err)
	}
	if count, err := h.store.CountSamples(context.Background(), "alice"); err != nil || count != 2 {
		t.Fatalf("restored sample count = %d err=%v", count, err)
	}
	if restored, err := h.store.GetSampleBody(context.Background(), "alice", "delete"); err != nil || restored == nil || restored.Label != "delete" {
		t.Fatalf("restored sample = %+v err=%v", restored, err)
	}
}

func TestAnalyzeReplacesStyleguideAndNeverRules(t *testing.T) {
	h := newVoiceHarness(t)
	if _, err := h.svc.Update(context.Background(), "alice", "old style", "keep this rule"); err != nil {
		t.Fatal(err)
	}
	if err := h.store.InsertSample(context.Background(), voice.Sample{
		ID: "sample", UserID: "alice", Label: "post", Body: longSample("글"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	h.models.response = "## 평균 문장 길이\n짧음"
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", WriteModel: analyzeRef.String()}, func(string, int, int) {}); err == nil || !strings.Contains(err.Error(), "종결어미") {
		t.Fatalf("missing ending section error = %v", err)
	}
	profile, _ := h.store.GetProfile(context.Background(), "alice")
	if profile.Styleguide != "old style" || profile.Rules != "keep this rule" {
		t.Fatalf("invalid analysis mutated profile: %+v", profile)
	}

	h.models.response = "## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
	var progress [][3]any
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", WriteModel: analyzeRef.String()}, func(stage string, done, total int) {
		progress = append(progress, [3]any{stage, done, total})
	}); err != nil {
		t.Fatal(err)
	}
	profile, _ = h.store.GetProfile(context.Background(), "alice")
	if profile.Styleguide != h.models.response || profile.Rules != "keep this rule" || len(progress) != 2 {
		t.Fatalf("successful analysis = profile=%+v progress=%+v", profile, progress)
	}
	if !strings.Contains(h.models.request.Messages[0].Parts[0].Text, longSample("글")) {
		t.Fatal("analysis request omitted the accumulated corpus")
	}
}

func TestAnalyzeRetriesWhenCorpusChangesDuringProviderCall(t *testing.T) {
	h := newVoiceHarness(t)
	if err := h.store.InsertSample(context.Background(), voice.Sample{
		ID: "first", UserID: "alice", Label: "first", Body: longSample("첫"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() {
		done <- svc.Analyze(context.Background(), voice.AnalysisJob{
			UserID: "alice", WriteModel: analyzeRef.String(),
		}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("first provider call did not start")
	}
	if err := h.store.InsertSample(context.Background(), voice.Sample{
		ID: "second", UserID: "alice", Label: "second", Body: longSample("둘"), CreatedAt: time.Now().Add(time.Second),
	}); err != nil {
		t.Fatal(err)
	}
	close(models.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || !strings.Contains(profile.Styleguide, "new") {
		t.Fatalf("latest styleguide = %+v err=%v", profile, err)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 2 || strings.Contains(models.requests[0], longSample("둘")) || !strings.Contains(models.requests[1], longSample("둘")) {
		t.Fatalf("analysis snapshots = %d: %+v", len(models.requests), models.requests)
	}
}

func TestDeletingLastSampleDuringAnalysisLeavesStyleguideUntouched(t *testing.T) {
	h := newVoiceHarness(t)
	if _, err := h.svc.Update(context.Background(), "alice", "hand edited", "keep rule"); err != nil {
		t.Fatal(err)
	}
	if err := h.store.InsertSample(context.Background(), voice.Sample{
		ID: "only", UserID: "alice", Label: "only", Body: longSample("문"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() {
		done <- svc.Analyze(context.Background(), voice.AnalysisJob{
			UserID: "alice", WriteModel: analyzeRef.String(),
		}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("provider call did not start")
	}
	if jobID, err := svc.DeleteSample(context.Background(), "alice", "only"); err != nil || jobID != "" {
		t.Fatalf("delete last sample = job=%q err=%v", jobID, err)
	}
	close(models.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice")
	if err != nil || profile.Styleguide != "hand edited" || profile.Rules != "keep rule" {
		t.Fatalf("last-delete profile = %+v err=%v", profile, err)
	}
}

type queueJobs struct{ queue *job.Queue }

func (a queueJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: request.UserID, WriteModel: request.WriteModel})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a queueJobs) ActiveForUserKind(ctx context.Context, userID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveForUserKind(ctx, userID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func TestAnalyzeHandlerFailureBecomesFailedJob(t *testing.T) {
	h := newVoiceHarness(t)
	models := h.models
	models.response = "## 문장 길이\n짧음"
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader), 5*time.Millisecond)
	svc := voice.NewService(h.store, models, queueJobs{queue: queue})
	queue.Register(job.KindAnalyzeVoice, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return svc.Analyze(ctx, voice.AnalysisJob{UserID: found.UserID, WriteModel: found.WriteModel}, voice.Progress(progress))
	})
	if err := h.store.InsertSample(context.Background(), voice.Sample{
		ID: "sample", UserID: "alice", Label: "post", Body: longSample("문"), CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	id, err := queue.Enqueue(context.Background(), job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: "alice", WriteModel: analyzeRef.String(),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go queue.Run(ctx)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		found, getErr := queue.Get(context.Background(), id, "alice")
		if getErr == nil && found.Status == job.StatusFailed {
			if !strings.Contains(found.Error, "종결어미") {
				t.Fatalf("failed reason = %q", found.Error)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("analysis job did not fail")
}
