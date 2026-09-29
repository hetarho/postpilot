package voice_test

import (
	"context"
	"encoding/json"
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
	selected      map[string]llm.ModelRef
	response      string
	err           error
	request       llm.Request
	completeCalls int
	structured    bool
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

func (f *changingCorpusModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{}, false
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
		return llm.Response{Text: analysisAnswer("## 1. 종결어미 분포\nold\n## 8. never uses\nold")}, nil
	}
	return llm.Response{Text: analysisAnswer("## 1. 종결어미 분포\nnew\n## 8. never uses\nnew")}, nil
}

// analysisAnswer is the analysis call's JSON answer around a nine-section style guide, which
// is the profile's lexical description (VOICE-25, VOICE-27).
func analysisAnswer(guide string) string {
	encoded, err := json.Marshal(map[string]any{
		"lexical_description": guide, "base_register": "", "connective_style": "", "intro_pattern": "",
		"closing_pattern": "", "heading_habit": "", "list_habit": "", "emoji_use": "", "axes": map[string]int{},
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// A nine-section guide in the shape the analysis refuses anything short of.
const koreanGuide = "1. 종결어미 분포: 해요체\n8. 절대 사용하지 않는 표현 (never uses): 과장"

func (f *fakeModels) AnalyzeModel(_ context.Context, userID string) (llm.ModelRef, bool, error) {
	ref, ok := f.selected[userID]
	return ref, ok, nil
}

// structured makes Resolve report a model that declares structured output, so a test can
// assert the analysis call attaches its schema only then.
func (f *fakeModels) Resolve(llm.ModelRef) (llm.ModelInfo, bool) {
	return llm.ModelInfo{StructuredOutput: f.structured}, true
}

func (f *fakeModels) Complete(_ context.Context, _ llm.ModelRef, request llm.Request) (llm.Response, error) {
	f.request = request
	f.completeCalls++
	return llm.Response{Text: f.response}, f.err
}

// fakeJobs keys its active analyses by VOICE, which is the guard the service must ask for.
type fakeJobs struct {
	mu           sync.Mutex
	active       map[string]*voice.ActiveJob
	busy         map[string]bool
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

func (f *fakeJobs) ActiveForVoiceKind(_ context.Context, voiceID, _ string) (*voice.ActiveJob, error) {
	return f.active[voiceID], nil
}

func (f *fakeJobs) HasActiveForVoice(_ context.Context, voiceID string) (bool, error) {
	return f.active[voiceID] != nil || f.busy[voiceID], nil
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
	voices map[string]string
}

// firstVoiceName is the voice each harness account creates for the older single-voice tests.
const firstVoiceName = "기본 말투"

// newVoiceHarness seeds two accounts and gives each one voice created by name, the way an
// owner makes their first one: no account is given a voice (VOICE-4).
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
	jobs := &fakeJobs{active: map[string]*voice.ActiveJob{}, busy: map[string]bool{}, enqueueID: "job-new"}
	h := &voiceHarness{store: store, db: handle, models: models, jobs: jobs, svc: voice.NewService(store, models, jobs), voices: map[string]string{}}
	for _, userID := range []string{"alice", "bob"} {
		created, err := h.svc.CreateVoice(context.Background(), userID, firstVoiceName)
		if err != nil || created.IsDefault || created.Made {
			t.Fatalf("create %s: voice=%+v err=%v", userID, created, err)
		}
		h.voices[userID] = created.ID
	}
	return h
}

// makeVoice publishes an analysis, which is what makes a voice (VOICE-25).
func (h *voiceHarness) makeVoice(t *testing.T, user, voiceID string) {
	t.Helper()
	if _, err := h.store.PublishProfileVersion(context.Background(), user, voiceID, voice.StructuredProfile{}, "analysis", 0, time.Now()); err != nil {
		t.Fatal(err)
	}
}

// voice returns the account's first voice id; the older single-voice tests run inside it.
func (h *voiceHarness) voice(user string) string { return h.voices[user] }

func (h *voiceHarness) addSample(t *testing.T, user, voiceID, id, label, body string, at time.Time) {
	t.Helper()
	if err := h.store.InsertSample(context.Background(), voice.Sample{ID: id, UserID: user, VoiceID: voiceID, Label: label, Body: body, CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
}

func longSample(char string) string { return strings.Repeat(char, voice.SampleMinChars) }

// --- directory (VOICE-4, VOICE-10..VOICE-14) ---

// VOICE-4: an account starts with no voice, and reads never create one.
func TestANewAccountListsNoVoice(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	if _, err := h.db.Writer.Exec("INSERT INTO users (id, password_hash, created_at) VALUES ('charlie', 'hash', ?)", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		voices, err := h.svc.ListVoices(ctx, "charlie")
		if err != nil || len(voices) != 0 {
			t.Fatalf("a new account lists %+v, %v", voices, err)
		}
	}
}

func TestCreateRenameValidateAndUniqueNames(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	var badName *voice.VoiceNameError
	if _, err := h.svc.CreateVoice(ctx, "alice", "   "); !errors.As(err, &badName) || badName.Chars != 0 {
		t.Fatalf("blank name = %v", err)
	}
	if _, err := h.svc.CreateVoice(ctx, "alice", strings.Repeat("가", voice.VoiceNameMaxChars+1)); !errors.As(err, &badName) || badName.Chars != voice.VoiceNameMaxChars+1 {
		t.Fatalf("long name = %v", err)
	}
	review, err := h.svc.CreateVoice(ctx, "alice", "  리뷰 말투  ")
	if err != nil || review.Name != "리뷰 말투" || review.IsDefault || review.Deleted() {
		t.Fatalf("create = %+v err=%v", review, err)
	}
	if _, err := h.svc.CreateVoice(ctx, "alice", "리뷰 말투"); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("duplicate active name = %v", err)
	}
	if _, err := h.svc.CreateVoice(ctx, "bob", "리뷰 말투"); err != nil {
		t.Fatalf("same name in another account = %v", err)
	}
	if _, err := h.svc.RenameVoice(ctx, "alice", review.ID, firstVoiceName); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("rename onto active name = %v", err)
	}
	renamed, err := h.svc.RenameVoice(ctx, "alice", review.ID, " 제품 리뷰 ")
	if err != nil || renamed.Name != "제품 리뷰" || renamed.ID != review.ID {
		t.Fatalf("rename = %+v err=%v", renamed, err)
	}
	// A new voice is genuinely empty even though the default has data.
	h.addSample(t, "alice", h.voice("alice"), "default-sample", "기본", longSample("기"), time.Now())
	profile, err := h.svc.Get(ctx, "alice", review.ID)
	if err != nil || len(profile.Samples) != 0 || !profile.Structured.Empty || profile.Voice.ID != review.ID {
		t.Fatalf("new voice inherited data: %+v err=%v", profile, err)
	}
}

// VOICE-12: the 기본 is one active, made voice, swapped atomically, or none at all.
func TestSetDefaultSwapsAtomicallyClearsAndRefusesUnmadeAndTombstones(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	first := h.voice("alice")
	second, _ := h.svc.CreateVoice(ctx, "alice", "둘째")
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", second.ID); !errors.Is(err, voice.ErrVoiceNotMade) {
		t.Fatalf("an unmade 기본 = %v", err)
	}
	h.makeVoice(t, "alice", first)
	h.makeVoice(t, "alice", second.ID)
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", first); err != nil {
		t.Fatal(err)
	}
	voices, err := h.svc.SetDefaultVoice(ctx, "alice", second.ID)
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, v := range voices {
		if v.IsDefault {
			defaults++
			if v.ID != second.ID {
				t.Fatalf("wrong default: %+v", v)
			}
		}
	}
	if defaults != 1 || voices[0].ID != second.ID {
		t.Fatalf("defaults=%d voices=%+v", defaults, voices)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "bob", second.ID); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign set default = %v", err)
	}
	cleared, err := h.svc.SetDefaultVoice(ctx, "alice", "")
	if err != nil {
		t.Fatalf("clear = %v", err)
	}
	for _, v := range cleared {
		if v.IsDefault {
			t.Fatalf("a cleared account still has a 기본: %+v", cleared)
		}
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", first); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", first); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("set deleted default = %v", err)
	}
}

// VOICE-13: the 기본 and the last voice delete like any other and leave the account with none.
func TestDeleteTheDefaultAndTheLastVoice(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	only := h.voice("alice")
	h.makeVoice(t, "alice", only)
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", only); err != nil {
		t.Fatal(err)
	}
	deleted, err := h.svc.DeleteVoice(ctx, "alice", only)
	if err != nil || !deleted.Deleted() || deleted.IsDefault {
		t.Fatalf("delete the 기본 and last voice = %+v err=%v", deleted, err)
	}
	voices, err := h.svc.ListVoices(ctx, "alice")
	if err != nil || len(voices) != 1 || !voices[0].Deleted() || voices[0].IsDefault {
		t.Fatalf("after deleting the last voice: %+v err=%v", voices, err)
	}
	restored, err := h.svc.RestoreVoice(ctx, "alice", only)
	if err != nil || restored.Deleted() || restored.IsDefault {
		t.Fatalf("a restore made the voice the 기본 again: %+v err=%v", restored, err)
	}
}

// VOICE-9, VOICE-52: the directory carries each voice's 학습 글 count and, once made, the
// time its current analysis was published.
func TestListVoicesCarriesTheMetaLine(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id := h.voice("alice")
	h.addSample(t, "alice", id, "s1", "하나", longSample("하"), time.Now())
	h.addSample(t, "alice", id, "s2", "둘", longSample("둘"), time.Now())
	voices, _ := h.svc.ListVoices(ctx, "alice")
	if len(voices) != 1 || voices[0].SampleCount != 2 || voices[0].Made || voices[0].AnalyzedAt != nil {
		t.Fatalf("an unmade voice's row = %+v", voices)
	}
	h.makeVoice(t, "alice", id)
	voices, _ = h.svc.ListVoices(ctx, "alice")
	if !voices[0].Made || voices[0].AnalyzedAt == nil || voices[0].AnalyzedAt.IsZero() {
		t.Fatalf("a made voice's row = %+v", voices[0])
	}
}

func TestDeleteAndRestoreLifecycle(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	extra, _ := h.svc.CreateVoice(ctx, "alice", "일기")
	h.addSample(t, "alice", extra.ID, "s1", "일기", longSample("일"), time.Now())
	deleted, err := h.svc.DeleteVoice(ctx, "alice", extra.ID)
	if err != nil || !deleted.Deleted() || deleted.Name != "일기" {
		t.Fatalf("delete = %+v err=%v", deleted, err)
	}
	// Idempotent, and the tombstone keeps its whole profile readable.
	if again, err := h.svc.DeleteVoice(ctx, "alice", extra.ID); err != nil || !again.Deleted() {
		t.Fatalf("second delete = %+v err=%v", again, err)
	}
	profile, err := h.svc.Get(ctx, "alice", extra.ID)
	if err != nil || len(profile.Samples) != 1 || !profile.Voice.Deleted() {
		t.Fatalf("tombstone profile = %+v err=%v", profile, err)
	}
	voices, _ := h.svc.ListVoices(ctx, "alice")
	if len(voices) != 2 || voices[0].Deleted() || !voices[1].Deleted() {
		t.Fatalf("tombstone should list last: %+v", voices)
	}
	// Restore is blocked by an active voice holding the name, and unblocked by renaming
	// the tombstone; it never changes the default.
	if _, err := h.svc.CreateVoice(ctx, "alice", "일기"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RestoreVoice(ctx, "alice", extra.ID); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("conflicting restore = %v", err)
	}
	if _, err := h.svc.RenameVoice(ctx, "alice", extra.ID, "옛 일기"); err != nil {
		t.Fatal(err)
	}
	restored, err := h.svc.RestoreVoice(ctx, "alice", extra.ID)
	if err != nil || restored.Deleted() || restored.IsDefault || restored.Name != "옛 일기" {
		t.Fatalf("restore = %+v err=%v", restored, err)
	}
	if len(h.jobs.calls()) != 0 {
		t.Fatal("lifecycle enqueued work")
	}
	if h.models.completeCalls != 0 {
		t.Fatal("lifecycle called a provider")
	}
}

// VOICE-13: only a queued or running job frozen to the voice keeps it; no model experiment
// is asked.
func TestDeleteRefusesVoiceWithPublishableWork(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	busy, _ := h.svc.CreateVoice(ctx, "alice", "바쁜 말투")
	h.jobs.busy[busy.ID] = true
	if _, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with active job = %v", err)
	}
	h.jobs.busy[busy.ID] = false
	if deleted, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); err != nil || !deleted.Deleted() {
		t.Fatalf("delete once idle = %+v err=%v", deleted, err)
	}
}

// --- isolation (VOICE-3, VOICE-15) ---

func TestProfilesAndSamplesAreIsolatedByVoiceAndAccount(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	casual := h.voice("alice")
	formal, _ := h.svc.CreateVoice(ctx, "alice", "격식")
	h.addSample(t, "alice", casual, "casual-sample", "캐주얼", longSample("해"), time.Now())
	h.addSample(t, "alice", formal.ID, "formal-sample", "격식", longSample("습"), time.Now())
	h.jobs.active[casual] = &voice.ActiveJob{ID: "analysis-casual"}

	casualProfile, err := h.svc.Get(ctx, "alice", casual)
	if err != nil || len(casualProfile.Samples) != 1 || casualProfile.Samples[0].ID != "casual-sample" || casualProfile.ActiveJobID != "analysis-casual" {
		t.Fatalf("casual profile leaked/missing: %+v err=%v", casualProfile, err)
	}
	formalProfile, err := h.svc.Get(ctx, "alice", formal.ID)
	if err != nil || len(formalProfile.Samples) != 1 || formalProfile.Samples[0].ID != "formal-sample" || formalProfile.ActiveJobID != "" {
		t.Fatalf("formal profile leaked/missing: %+v err=%v", formalProfile, err)
	}
	formalPrompt, err := h.svc.PromptProfileForTopic(ctx, "alice", formal.ID, "", nil)
	if err != nil || formalPrompt.Empty || len(formalPrompt.Excerpts) != 1 || !strings.HasPrefix(formalPrompt.Excerpts[0], "습") {
		t.Fatalf("formal prompt borrowed from casual: excerpts=%v err=%v", formalPrompt.Excerpts, err)
	}
	// A same-account sample id from the other voice is unreachable, as is a foreign voice.
	if _, err := h.svc.DeleteSample(ctx, "alice", formal.ID, "casual-sample"); !errors.Is(err, voice.ErrSampleNotFound) {
		t.Fatalf("cross-voice sample delete = %v", err)
	}
	if count, _ := h.store.CountSamples(ctx, "alice", casual); count != 1 {
		t.Fatalf("cross-voice delete removed a sample: %d", count)
	}
	if _, err := h.svc.Get(ctx, "bob", casual); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice read = %v", err)
	}
	if _, _, err := h.svc.AddSample(ctx, "bob", formal.ID, "", longSample("가"), analyzeRef); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice sample = %v", err)
	}
	// Bob's own default is untouched by any of it.
	bobProfile, err := h.svc.Get(ctx, "bob", h.voice("bob"))
	if err != nil || len(bobProfile.Samples) != 0 {
		t.Fatalf("bob profile changed: %+v err=%v", bobProfile, err)
	}
}

func TestDeletedVoiceStaysReadableButRefusesMutations(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	gone, _ := h.svc.CreateVoice(ctx, "alice", "사라질 말투")
	h.addSample(t, "alice", gone.ID, "gone-sample", "사라질", longSample("사"), time.Now())
	if _, err := h.svc.DeleteVoice(ctx, "alice", gone.ID); err != nil {
		t.Fatal(err)
	}
	if profile, err := h.svc.Get(ctx, "alice", gone.ID); err != nil || len(profile.Samples) != 1 {
		t.Fatalf("tombstone read = %+v err=%v", profile, err)
	}
	if _, _, err := h.svc.AddSample(ctx, "alice", gone.ID, "", longSample("가"), analyzeRef); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("sample on deleted = %v", err)
	}
	if _, err := h.svc.PromptProfileForTopic(ctx, "alice", gone.ID, "", nil); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("prompt for deleted = %v", err)
	}
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: gone.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("analyze deleted = %v", err)
	}
	if len(h.jobs.calls()) != 0 || h.models.completeCalls != 0 {
		t.Fatal("a deleted voice reached the queue or a provider")
	}
}

// --- samples and analysis (VOICE-20..VOICE-25), per voice ---

func TestAddSampleValidatesBeforeWritingAndReturnsActiveJob(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	_, _, err := h.svc.AddSample(context.Background(), "alice", alice, "", strings.Repeat("가", 199), analyzeRef)
	var short *voice.SampleTooShortError
	if !errors.As(err, &short) || short.Chars != 199 {
		t.Fatalf("short sample error = %v", err)
	}
	delete(h.models.selected, "alice")
	_, _, err = h.svc.AddSample(context.Background(), "alice", alice, "", longSample("나"), analyzeRef)
	if !errors.Is(err, voice.ErrAnalyzeModelRequired) {
		t.Fatalf("missing model error = %v", err)
	}
	if count, _ := h.store.CountSamples(context.Background(), "alice", alice); count != 0 {
		t.Fatalf("samples stored before model validation = %d", count)
	}

	h.models.selected["alice"] = analyzeRef
	h.jobs.enqueueErr = errors.New("queue unavailable")
	_, _, err = h.svc.AddSample(context.Background(), "alice", alice, "", longSample("마"), analyzeRef)
	if !errors.Is(err, voice.ErrSampleMutation) {
		t.Fatalf("enqueue failure = %v", err)
	}
	if count, _ := h.store.CountSamples(context.Background(), "alice", alice); count != 0 {
		t.Fatalf("failed enqueue left %d samples", count)
	}

	h.jobs.enqueueErr = &voice.JobAlreadyInProgressError{ActiveID: "job-active"}
	sample, jobID, err := h.svc.AddSample(context.Background(), "alice", alice, "", longSample("다"), analyzeRef)
	if err != nil || jobID != "job-active" {
		t.Fatalf("AddSample = sample=%+v job=%q err=%v", sample, jobID, err)
	}
	if sample.Label != strings.Repeat("다", voice.LabelFallbackChars) || sample.Chars != voice.SampleMinChars || sample.VoiceID != alice {
		t.Fatalf("sample fallback/count = %+v", sample)
	}
	if calls := h.jobs.calls(); len(calls) != 2 || calls[1].WriteModel != analyzeRef.String() || calls[1].VoiceID != alice {
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

// VOICE-46: the projection carries FewShotMax excerpts, newest first, each cut at the excerpt
// maximum when no sentence ends near the target.
func TestProfileForPromptMostRecentTruncatedAndEmpty(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	limits := voice.PersonalizationThresholds()
	projection, err := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", nil)
	if err != nil || projection.Styleguide != "" || len(projection.Excerpts) != 0 || !projection.Empty {
		t.Fatalf("empty profile = %+v err=%v", projection, err)
	}
	base := time.Now().Add(-time.Hour)
	markers := []rune{'가', '나', '다', '라'}
	for i := range 4 {
		body := strings.Repeat(string(markers[i]), limits.FewShotExcerptMaxChars+10)
		h.addSample(t, "alice", alice, string(rune('a'+i)), "sample", body, base.Add(time.Duration(i)*time.Minute))
	}
	projection, err = h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", nil)
	excerpts := projection.Excerpts
	if err != nil || projection.Empty || len(excerpts) != limits.FewShotMax {
		t.Fatalf("profile prompt = lens=%d %v err=%v", len(excerpts), projection.Empty, err)
	}
	if []rune(excerpts[0])[0] != '라' {
		t.Fatalf("first excerpt is not most recent: %q", []rune(excerpts[0])[0])
	}
	for _, excerpt := range excerpts {
		if len([]rune(excerpt)) != limits.FewShotExcerptMaxChars {
			t.Fatalf("excerpt length = %d", len([]rune(excerpt)))
		}
	}
}

func TestDeleteReenqueuesOnlyWhileSamplesRemain(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	for _, id := range []string{"one", "two"} {
		h.addSample(t, "alice", alice, id, id, longSample(id), time.Now())
	}
	jobID, err := h.svc.DeleteSample(context.Background(), "alice", alice, "one")
	if err != nil || jobID != "job-new" || len(h.jobs.calls()) != 1 {
		t.Fatalf("first delete = job=%q calls=%d err=%v", jobID, len(h.jobs.calls()), err)
	}
	jobID, err = h.svc.DeleteSample(context.Background(), "alice", alice, "two")
	if err != nil || jobID != "" || len(h.jobs.calls()) != 1 {
		t.Fatalf("last delete = job=%q calls=%d err=%v", jobID, len(h.jobs.calls()), err)
	}
}

func TestDeleteRestoresSampleWhenEnqueueFails(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	for _, id := range []string{"keep", "delete"} {
		h.addSample(t, "alice", alice, id, id, longSample(id), time.Now())
	}
	h.jobs.enqueueErr = errors.New("queue unavailable")
	if _, err := h.svc.DeleteSample(context.Background(), "alice", alice, "delete"); !errors.Is(err, voice.ErrSampleMutation) {
		t.Fatalf("delete enqueue error = %v", err)
	}
	if count, err := h.store.CountSamples(context.Background(), "alice", alice); err != nil || count != 2 {
		t.Fatalf("restored sample count = %d err=%v", count, err)
	}
	if restored, err := h.store.GetSampleBody(context.Background(), "alice", alice, "delete"); err != nil || restored == nil || restored.Label != "delete" {
		t.Fatalf("restored sample = %+v err=%v", restored, err)
	}
}

func TestAnalyzePublishesStructuredProfile(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.response = analysisAnswer("## 평균 문장 길이\n짧음")
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err == nil || !strings.Contains(err.Error(), "종결어미") {
		t.Fatalf("missing ending section error = %v", err)
	}
	profile, _ := h.store.GetProfile(context.Background(), "alice", alice)
	if profile.Structured.Version != 0 {
		t.Fatalf("invalid analysis mutated profile: %+v", profile)
	}

	guide := "## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
	h.models.response = analysisAnswer(guide)
	var progress [][3]any
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(stage string, done, total int) {
		progress = append(progress, [3]any{stage, done, total})
	}); err != nil {
		t.Fatal(err)
	}
	profile, _ = h.store.GetProfile(context.Background(), "alice", alice)
	// The analysis text lands in the published structured version's lexical description, once
	// (VOICE-25).
	if profile.Structured.Lexical.Description.Value != guide || len(progress) != 2 {
		t.Fatalf("successful analysis = profile=%+v progress=%+v", profile, progress)
	}
	if !strings.Contains(h.models.request.Messages[0].Parts[0].Text, longSample("글")) {
		t.Fatal("analysis request omitted the accumulated corpus")
	}
}

// MODEL-9 and GEN-22 require analyze to send no reasoning effort. That rule lives in one
// place only — the absence of a stage value on the request — so it is asserted against the
// request the provider receives rather than against a config field.
func TestAnalyzeRequestsNoReasoningEffort(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.response = analysisAnswer("## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장")
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	// Unspecified is "no stage decision", which registry.go forwards as nothing. Unset would
	// be a different statement — the yaml sentinel for a model override that omits the key.
	if h.models.request.Reasoning != llm.ReasoningUnspecified {
		t.Fatalf("analyze request reasoning = %q, want no stage value", h.models.request.Reasoning)
	}
}

func TestAnalyzeRetriesWhenCorpusChangesDuringProviderCall(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "first", "first", longSample("첫"), time.Now())
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() {
		done <- svc.Analyze(context.Background(), voice.AnalysisJob{
			UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(),
		}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("first provider call did not start")
	}
	h.addSample(t, "alice", alice, "second", "second", longSample("둘"), time.Now().Add(time.Second))
	close(models.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || !strings.Contains(profile.Structured.Lexical.Description.Value, "new") {
		t.Fatalf("latest published analysis = %+v err=%v", profile, err)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 2 || strings.Contains(models.requests[0], longSample("둘")) || !strings.Contains(models.requests[1], longSample("둘")) {
		t.Fatalf("analysis snapshots = %d: %+v", len(models.requests), models.requests)
	}
}

// Two voices analyzing at the same time do not see each other's corpus or overwrite each
// other's published profile: the corpus-version claim and the profile head are both per voice.
func TestSimultaneousVoiceAnalysesDoNotOverwriteEachOther(t *testing.T) {
	h := newVoiceHarness(t)
	casual := h.voice("alice")
	formal, _ := h.svc.CreateVoice(context.Background(), "alice", "격식")
	h.addSample(t, "alice", casual, "c", "casual", longSample("해"), time.Now())
	h.addSample(t, "alice", formal.ID, "f", "formal", longSample("습"), time.Now())
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	casualDone := make(chan error, 1)
	go func() {
		casualDone <- svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: casual, WriteModel: analyzeRef.String()}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("casual analysis did not start")
	}
	// The formal analysis completes entirely while the casual provider call is still open.
	if err := svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: formal.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	close(models.release)
	if err := <-casualDone; err != nil {
		t.Fatal(err)
	}
	casualProfile, _ := h.store.GetProfile(context.Background(), "alice", casual)
	formalProfile, _ := h.store.GetProfile(context.Background(), "alice", formal.ID)
	casualAnalysis := casualProfile.Structured.Lexical.Description.Value
	formalAnalysis := formalProfile.Structured.Lexical.Description.Value
	if !strings.Contains(casualAnalysis, "old") || !strings.Contains(formalAnalysis, "new") {
		t.Fatalf("published analyses crossed: casual=%q formal=%q", casualAnalysis, formalAnalysis)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 2 || strings.Contains(models.requests[0], longSample("습")) || strings.Contains(models.requests[1], longSample("해")) {
		t.Fatalf("a voice saw the other voice's corpus: %+v", models.requests)
	}
}

func TestDeletingLastSampleDuringAnalysisLeavesProfileUntouched(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "only", "only", longSample("문"), time.Now())
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() {
		done <- svc.Analyze(context.Background(), voice.AnalysisJob{
			UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(),
		}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("provider call did not start")
	}
	if jobID, err := svc.DeleteSample(context.Background(), "alice", alice, "only"); err != nil || jobID != "" {
		t.Fatalf("delete last sample = job=%q err=%v", jobID, err)
	}
	close(models.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || profile.Structured.Version != 0 {
		t.Fatalf("last-delete profile = %+v err=%v", profile, err)
	}
}

type queueJobs struct{ queue *job.Queue }

func (a queueJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, attach(job.NewJob{Kind: job.KindAnalyzeVoice, UserID: request.UserID, WriteModel: request.WriteModel}, "", request.VoiceID))
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a queueJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{Kind: kind})
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a queueJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveFor(ctx, job.Subject{Dimension: voice.JobSubject, ID: voiceID}, job.Filter{})
}

func TestAnalyzeHandlerFailureBecomesFailedJob(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	models := h.models
	models.response = "## 문장 길이\n짧음"
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader, jobKindsForTest()), 5*time.Millisecond, jobReportingForTest())
	svc := voice.NewService(h.store, models, queueJobs{queue: queue})
	queue.Register(job.KindAnalyzeVoice, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return svc.Analyze(ctx, voice.AnalysisJob{UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), WriteModel: found.WriteModel}, voice.Progress(progress))
	})
	h.addSample(t, "alice", alice, "sample", "post", longSample("문"), time.Now())
	id, err := queue.Enqueue(context.Background(), attach(job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: "alice", WriteModel: analyzeRef.String()}, "", alice))
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
			if found.Failure == nil || found.Failure.Reason != job.FailureReasonUnknown {
				t.Fatalf("failed reason = %+v", found.Failure)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("analysis job did not fail")
}

// The queue guards analyses per voice: two voices of one account may analyze at once, while
// a second analysis for the same voice attaches to the active one.
func TestAnalysesAreGuardedPerVoiceThroughTheQueue(t *testing.T) {
	h := newVoiceHarness(t)
	casual := h.voice("alice")
	formal, _ := h.svc.CreateVoice(context.Background(), "alice", "격식")
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader, jobKindsForTest()), time.Second, jobReportingForTest())
	svc := voice.NewService(h.store, h.models, queueJobs{queue: queue})
	h.addSample(t, "alice", casual, "c", "casual", longSample("해"), time.Now())
	h.addSample(t, "alice", formal.ID, "f", "formal", longSample("습"), time.Now())
	_, casualJob, err := svc.AddSample(context.Background(), "alice", casual, "", longSample("가"), analyzeRef)
	if err != nil {
		t.Fatal(err)
	}
	_, formalJob, err := svc.AddSample(context.Background(), "alice", formal.ID, "", longSample("나"), analyzeRef)
	if err != nil || formalJob == casualJob {
		t.Fatalf("second voice could not analyze concurrently: job=%q err=%v", formalJob, err)
	}
	_, againJob, err := svc.AddSample(context.Background(), "alice", casual, "", longSample("다"), analyzeRef)
	if err != nil || againJob != casualJob {
		t.Fatalf("same voice did not attach to its active analysis: job=%q want %q err=%v", againJob, casualJob, err)
	}
	profile, err := svc.Get(context.Background(), "alice", formal.ID)
	if err != nil || profile.ActiveJobID != formalJob {
		t.Fatalf("formal active job = %q want %q err=%v", profile.ActiveJobID, formalJob, err)
	}
	if _, err := svc.DeleteVoice(context.Background(), "alice", formal.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with a queued analysis = %v", err)
	}
}
