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
	"unicode/utf8"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

var (
	analyzeRef   = llm.ModelRef{ProviderID: "stub", ModelID: "analyze"}
	writeOnlyRef = llm.ModelRef{ProviderID: "stub", ModelID: "write"}
	disabledRef  = llm.ModelRef{ProviderID: "stub", ModelID: "disabled"}
)

type fakeModels struct {
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
		return llm.Response{Text: analysisAnswer("old")}, nil
	}
	return llm.Response{Text: analysisAnswer("new")}, nil
}

// analysisAnswer is the analysis call's JSON answer: the AI part alone (VOICE-24).
func analysisAnswer(impression string, examples ...string) string {
	cited := make([]map[string]string, 0, len(examples))
	for _, sentence := range examples {
		cited = append(cited, map[string]string{"field": "impression", "sentence": sentence})
	}
	encoded, err := json.Marshal(map[string]any{
		"impression": impression, "tics": []map[string]string{{"phrase": "진짜", "when": "감탄할 때"}},
		"signature_phrases": []string{"~더라구요"}, "examples": cited,
	})
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// structured makes Resolve report a model that declares structured output, so a test can
// assert the analysis call attaches its schema only then. analyzeRef serves the analyze stage,
// writeOnlyRef only the write stage and disabledRef is switched off; anything else is unknown.
func (f *fakeModels) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	switch ref {
	case analyzeRef:
		return llm.ModelInfo{StructuredOutput: f.structured, Stages: []string{llm.StageNameAnalyze}}, true
	case writeOnlyRef:
		return llm.ModelInfo{Stages: []string{llm.StageNameWrite}}, true
	case visionRef:
		return llm.ModelInfo{Stages: []string{llm.StageNameWrite}, Vision: true}, true
	case disabledRef:
		return llm.ModelInfo{Stages: []string{llm.StageNameAnalyze}, Disabled: true}, true
	}
	return llm.ModelInfo{}, false
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
	// activeChecks is each voice's queued or running 검증 job; checkErr refuses a check enqueue.
	activeChecks map[string]*voice.ActiveJob
	checkErr     error
	checkCalls   []voice.CheckJobRequest
}

func (f *fakeJobs) EnqueueCheck(_ context.Context, request voice.CheckJobRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkCalls = append(f.checkCalls, request)
	if f.checkErr != nil {
		return "", f.checkErr
	}
	return "job-check-" + request.CheckID, nil
}

func (f *fakeJobs) Enqueue(_ context.Context, request voice.AnalysisJobRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enqueueCalls = append(f.enqueueCalls, request)
	return f.enqueueID, f.enqueueErr
}

func (f *fakeJobs) ActiveForVoiceKind(_ context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	if kind == voice.CheckJobKind {
		return f.activeChecks[voiceID], nil
	}
	return f.active[voiceID], nil
}

func (f *fakeJobs) HasActiveForVoice(_ context.Context, voiceID string) (bool, error) {
	return f.active[voiceID] != nil || f.activeChecks[voiceID] != nil || f.busy[voiceID], nil
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
	models := &fakeModels{}
	jobs := &fakeJobs{active: map[string]*voice.ActiveJob{}, activeChecks: map[string]*voice.ActiveJob{}, busy: map[string]bool{}, enqueueID: "job-new"}
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
	if err := h.store.PublishAnalysis(context.Background(), user, voiceID, voice.Analysis{AnalyzeModel: analyzeRef.String(), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
}

// countingStore is the voice store with a count of the reads and writes a test pins.
type countingStore struct {
	*voicestore.Store
	voiceReads, analysisReads int
	// bodyReads is per-voice 학습 글 reads, batchBodyReads several voices' at once.
	bodyReads, batchBodyReads int
	answerWrites              int
}

func (c *countingStore) ListSampleBodies(ctx context.Context, userID, voiceID string) ([]voice.Sample, error) {
	c.bodyReads++
	return c.Store.ListSampleBodies(ctx, userID, voiceID)
}

func (c *countingStore) ListSampleBodiesForVoices(ctx context.Context, userID string, voiceIDs []string) (map[string][]voice.Sample, error) {
	c.batchBodyReads++
	return c.Store.ListSampleBodiesForVoices(ctx, userID, voiceIDs)
}

func (c *countingStore) AnswerPrompt(ctx context.Context, sample voice.Sample, uploadID, replaceID string) error {
	c.answerWrites++
	return c.Store.AnswerPrompt(ctx, sample, uploadID, replaceID)
}

func (c *countingStore) GetVoice(ctx context.Context, userID, voiceID string) (voice.Voice, error) {
	c.voiceReads++
	return c.Store.GetVoice(ctx, userID, voiceID)
}

func (c *countingStore) CurrentAnalysis(ctx context.Context, userID, voiceID string) (*voice.Analysis, error) {
	c.analysisReads++
	return c.Store.CurrentAnalysis(ctx, userID, voiceID)
}

// voice returns the account's first voice id; the older single-voice tests run inside it.
func (h *voiceHarness) voice(user string) string { return h.voices[user] }

func (h *voiceHarness) addSample(t *testing.T, user, voiceID, id, label, body string, at time.Time) {
	t.Helper()
	if err := h.store.InsertSample(context.Background(), voice.Sample{ID: id, UserID: user, VoiceID: voiceID, Kind: voice.SampleKindPost, Label: label, Body: body, CreatedAt: at}); err != nil {
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
	if err != nil || len(profile.Samples) != 0 || profile.Analysis != nil || profile.Voice.ID != review.ID {
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
	h.makeVoice(t, "alice", formal.ID)
	formalPrompt, err := h.svc.PromptProfileForTopic(ctx, "alice", formal.ID, "", voice.LanguageKorean, "")
	if err != nil || len(formalPrompt.Excerpts) != 1 || !strings.HasPrefix(formalPrompt.Excerpts[0], "습") {
		t.Fatalf("formal prompt borrowed from casual: excerpts=%v err=%v", formalPrompt.Excerpts, err)
	}
	// A same-account sample id from the other voice is unreachable, as is a foreign voice.
	if err := h.svc.DeleteSample(ctx, "alice", formal.ID, "casual-sample"); !errors.Is(err, voice.ErrSampleNotFound) {
		t.Fatalf("cross-voice sample delete = %v", err)
	}
	if count, _ := h.store.CountSamples(ctx, "alice", casual); count != 1 {
		t.Fatalf("cross-voice delete removed a sample: %d", count)
	}
	if _, err := h.svc.Get(ctx, "bob", casual); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice read = %v", err)
	}
	if _, err := h.svc.AddSample(ctx, "bob", formal.ID, "", longSample("가")); !errors.Is(err, voice.ErrVoiceNotFound) {
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
	if _, err := h.svc.AddSample(ctx, "alice", gone.ID, "", longSample("가")); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("sample on deleted = %v", err)
	}
	if _, err := h.svc.PromptProfileForTopic(ctx, "alice", gone.ID, "", voice.LanguageKorean, ""); !errors.Is(err, voice.ErrVoiceDeleted) {
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

// readyPost is one pasted post holding exactly the sentences 말투 만들기 needs (VOICE-32).
func readyPost() string {
	return strings.Repeat("오늘도 정말 맛있게 먹었어요.\n", voice.ReadySentences)
}

// VOICE-20, VOICE-21: a pasted post is validated, stored, and enqueues nothing.
func TestAddSampleValidatesAndEnqueuesNothing(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	_, err := h.svc.AddSample(context.Background(), "alice", alice, "", strings.Repeat("가", 199))
	var short *voice.SampleTooShortError
	if !errors.As(err, &short) || short.Chars != 199 {
		t.Fatalf("short sample error = %v", err)
	}
	sample, err := h.svc.AddSample(context.Background(), "alice", alice, "", longSample("다"))
	if err != nil || sample.Kind != voice.SampleKindPost || sample.Label != strings.Repeat("다", voice.LabelFallbackChars) || sample.Chars != voice.SampleMinChars || sample.VoiceID != alice {
		t.Fatalf("sample = %+v err=%v", sample, err)
	}
	if err := h.svc.DeleteSample(context.Background(), "alice", alice, sample.ID); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.DeleteSample(context.Background(), "alice", alice, sample.ID); !errors.Is(err, voice.ErrSampleNotFound) {
		t.Fatalf("second delete = %v", err)
	}
	if len(h.jobs.calls()) != 0 || h.models.completeCalls != 0 {
		t.Fatal("gathering enqueued work or called a provider")
	}
}

func TestAssembleCorpusHeadsEachPieceAndKeepsProseOnly(t *testing.T) {
	corpus := voice.AssembleCorpus([]voice.Sample{
		{Kind: voice.SampleKindPost, Label: "첫 글", Body: "첫 번째 본문이에요.\n#맛집 #연남동\n📍 서울 마포구 연남동 123"},
		{Kind: voice.SampleKindAnswer, PromptKey: "closing_greeting", Body: "다음에 또 만나요!"},
	})
	for _, expected := range []string{"===== 학습 글 1: 첫 글 =====", "첫 번째 본문이에요.", "===== 학습 글 2: 글을 마무리할 때", "다음에 또 만나요!"} {
		if !strings.Contains(corpus, expected) {
			t.Errorf("corpus missing %q: %s", expected, corpus)
		}
	}
	for _, excluded := range []string{"#맛집", "📍"} {
		if strings.Contains(corpus, excluded) {
			t.Errorf("corpus kept the non-prose %q: %s", excluded, corpus)
		}
	}
}

// fakeObjects is the private bucket as the voice context sees it.
type fakeObjects struct {
	mu      sync.Mutex
	objects map[string]int64
	deleted []string
}

func (f *fakeObjects) PresignPut(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://storage.test/put/" + key, nil
}
func (f *fakeObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://storage.test/get/" + key, nil
}
func (f *fakeObjects) Head(_ context.Context, key string) (voice.ObjectHead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	size, ok := f.objects[key]
	if !ok {
		return voice.ObjectHead{}, voice.ErrObjectNotFound
	}
	return voice.ObjectHead{Size: size, ContentType: voice.PhotoContentType}, nil
}
func (f *fakeObjects) Read(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.objects[key]; !ok {
		return nil, voice.ErrObjectNotFound
	}
	return []byte("jpeg:" + key), nil
}
func (f *fakeObjects) Delete(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.objects, key)
	f.deleted = append(f.deleted, key)
	return nil
}
func (f *fakeObjects) List(context.Context, string) ([]voice.StoredObject, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]voice.StoredObject, 0, len(f.objects))
	for key := range f.objects {
		out = append(out, voice.StoredObject{Key: key, LastModified: time.Now().Add(-48 * time.Hour)})
	}
	return out, nil
}

const photoMaxBytes = 1 << 20

func (h *voiceHarness) withPhotos() *fakeObjects {
	objects := &fakeObjects{objects: map[string]int64{}}
	h.svc.ConfigurePhotos(objects, voice.PhotoLimits{PutTTL: time.Minute, GetTTL: time.Minute, MaxBytes: photoMaxBytes})
	return objects
}

// VOICE-60: an answer is trimmed and non-empty, one per prompt, and names a known prompt; a
// second answer rewrites the first.
func TestAnswerPromptValidatesAndHoldsOneAnswerPerPrompt(t *testing.T) {
	h := newVoiceHarness(t)
	h.withPhotos()
	ctx := context.Background()
	alice := h.voice("alice")
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "no_such_prompt", Body: "안녕하세요"}); !errors.Is(err, voice.ErrPromptNotFound) {
		t.Fatalf("unknown prompt = %v", err)
	}
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "   "}); !errors.Is(err, voice.ErrAnswerRequired) {
		t.Fatalf("empty answer = %v", err)
	}
	answer, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "  안녕하세요! 오늘도 반가워요.  "})
	if err != nil || answer.Kind != voice.SampleKindAnswer || answer.PromptKey != "opening_greeting" || answer.Body != "안녕하세요! 오늘도 반가워요." || answer.Label != "" || answer.HasPhoto() {
		t.Fatalf("answer = %+v err=%v", answer, err)
	}
	rewritten, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "또 안녕하세요. 오늘은 두 문장을 더 써요. 반가워요."})
	if err != nil || rewritten.ID == answer.ID {
		t.Fatalf("rewrite = %+v err=%v", rewritten, err)
	}
	samples, err := h.store.ListSampleBodies(ctx, "alice", alice)
	if err != nil || len(samples) != 1 || samples[0].ID != rewritten.ID {
		t.Fatalf("after the rewrite the prompt holds %+v err=%v", samples, err)
	}
	answer = rewritten
	// Another voice's prompt is its own: bob answers the same prompt freely.
	if _, err := h.svc.AnswerPrompt(ctx, "bob", h.voice("bob"), voice.Answer{PromptKey: "opening_greeting", Body: "반가워요"}); err != nil {
		t.Fatalf("another voice's answer = %v", err)
	}
	// Deleting the answer frees the prompt.
	if err := h.svc.DeleteSample(ctx, "alice", alice, answer.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "다시 안녕하세요"}); err != nil {
		t.Fatalf("answer after delete = %v", err)
	}
	if len(h.jobs.calls()) != 0 || h.models.completeCalls != 0 {
		t.Fatal("answering enqueued work or called a provider")
	}
}

// VOICE-60, VOICE-21: saving the answer a prompt already holds — the same words, and no new photo
// — writes nothing and keeps the 학습 글's id, so the profile's notice stays where it was and no
// paid 다시 분석 is nudged; changed words, or a new photo, still replace the answer.
func TestAnUnchangedAnswerIsLeftAlone(t *testing.T) {
	h := newVoiceHarness(t)
	objects := &fakeObjects{objects: map[string]int64{}}
	counting := &countingStore{Store: h.store}
	svc := voice.NewService(counting, h.models, h.jobs)
	svc.ConfigurePhotos(objects, voice.PhotoLimits{PutTTL: time.Minute, GetTTL: time.Minute, MaxBytes: photoMaxBytes})
	ctx := context.Background()
	alice := h.voice("alice")
	answer, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "안녕하세요! 오늘도 반가워요."})
	if err != nil {
		t.Fatal(err)
	}
	upload, _, err := svc.CreatePhotoUpload(ctx, "alice", alice, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[upload.Key] = 5000
	photo, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요.", UploadID: upload.ID, PhotoWidth: 1024, PhotoHeight: 768})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.PublishAnalysis(ctx, "alice", alice, voice.Analysis{AnalyzeModel: analyzeRef.String(), MaterialIDs: []string{answer.ID, photo.ID}, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	notice := func() voice.Notice {
		t.Helper()
		profile, err := svc.Get(ctx, "alice", alice)
		if err != nil {
			t.Fatal(err)
		}
		return profile.Notice
	}
	if got := notice(); got != (voice.Notice{}) {
		t.Fatalf("a fresh analysis = %+v", got)
	}
	counting.answerWrites = 0
	again, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "  안녕하세요! 오늘도 반가워요.\n"})
	if err != nil || again.ID != answer.ID || again.Body != answer.Body {
		t.Fatalf("the same words = %+v err=%v, want %s", again, err, answer.ID)
	}
	// No upload is the photo the answer already has.
	kept, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요."})
	if err != nil || kept.ID != photo.ID || kept.PhotoKey != upload.Key || kept.PhotoWidth != 1024 {
		t.Fatalf("the same photo answer = %+v err=%v, want %s", kept, err, photo.ID)
	}
	if counting.answerWrites != 0 || len(objects.deleted) != 0 {
		t.Fatalf("an unchanged answer wrote %d times and deleted %v", counting.answerWrites, objects.deleted)
	}
	if got := notice(); got != (voice.Notice{}) {
		t.Fatalf("an unchanged answer moved the notice to %+v", got)
	}
	// Changed words are a new 학습 글: the analysis read one that is gone.
	changed, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "opening_greeting", Body: "안녕하세요! 오늘은 다르게 써요."})
	if err != nil || changed.ID == answer.ID || counting.answerWrites != 1 {
		t.Fatalf("changed words = %+v err=%v writes=%d", changed, err, counting.answerWrites)
	}
	if got := notice(); got.Kind != voice.NoticeChanged {
		t.Fatalf("after changed words the notice = %+v", got)
	}
	// So is the same text on a new photo.
	second, _, err := svc.CreatePhotoUpload(ctx, "alice", alice, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[second.Key] = 5000
	rephotographed, err := svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요.", UploadID: second.ID, PhotoWidth: 800, PhotoHeight: 600})
	if err != nil || rephotographed.ID == photo.ID || rephotographed.PhotoKey != second.Key || counting.answerWrites != 2 {
		t.Fatalf("a new photo = %+v err=%v writes=%d", rephotographed, err, counting.answerWrites)
	}
}

// VOICE-9, VOICE-32: the directory reads the 학습 글 of every voice not yet made in one read, each
// voice's readiness from its own 학습 글; a directory of made voices reads none. The profile reads
// its 학습 글 once for both the list and the meter.
func TestTheDirectoryAndTheProfileReadTheirSamplesOnce(t *testing.T) {
	h := newVoiceHarness(t)
	counting := &countingStore{Store: h.store}
	svc := voice.NewService(counting, h.models, h.jobs)
	ctx := context.Background()
	alice := h.voice("alice")
	second, err := svc.CreateVoice(ctx, "alice", "리뷰 말투")
	if err != nil {
		t.Fatal(err)
	}
	third, err := svc.CreateVoice(ctx, "alice", "일기 말투")
	if err != nil {
		t.Fatal(err)
	}
	made, err := svc.CreateVoice(ctx, "alice", "다 된 말투")
	if err != nil {
		t.Fatal(err)
	}
	h.makeVoice(t, "alice", made.ID)
	at := time.Now().Add(-time.Hour)
	h.addSample(t, "alice", alice, "alice-1", "국숫집", strings.Repeat("국물이 정말 진했어요! ", 30), at)
	h.addSample(t, "alice", alice, "alice-2", "빵집", strings.Repeat("빵이 바삭했어요. ", 12), at.Add(time.Minute))
	h.addSample(t, "alice", second.ID, "second-1", "카페", strings.Repeat("커피가 고소했어요. ", 9), at)
	// Another account's 학습 글 never reach this directory.
	h.addSample(t, "bob", h.voice("bob"), "bob-1", "남의 글", strings.Repeat("남의 문장이에요. ", 40), at)

	voices, err := svc.ListVoices(ctx, "alice")
	if err != nil || len(voices) != 4 {
		t.Fatalf("directory = %d voices err=%v", len(voices), err)
	}
	if counting.batchBodyReads != 1 || counting.bodyReads != 0 {
		t.Fatalf("the directory read bodies %d times at once and %d times per voice", counting.batchBodyReads, counting.bodyReads)
	}
	readiness := map[string]int{}
	for _, found := range voices {
		readiness[found.ID] = found.ReadinessPercent
	}
	for _, voiceID := range []string{alice, second.ID, third.ID} {
		bodies, err := h.store.ListSampleBodies(ctx, "alice", voiceID)
		if err != nil {
			t.Fatal(err)
		}
		if want := voice.ReadinessOf(bodies).Percent; readiness[voiceID] != want {
			t.Fatalf("voice %s readiness = %d, want %d", voiceID, readiness[voiceID], want)
		}
	}
	if readiness[alice] == readiness[second.ID] || readiness[second.ID] == 0 || readiness[third.ID] != 0 || readiness[made.ID] != 0 {
		t.Fatalf("readiness = %v", readiness)
	}
	h.makeVoice(t, "bob", h.voice("bob"))
	counting.batchBodyReads = 0
	if _, err := svc.ListVoices(ctx, "bob"); err != nil || counting.batchBodyReads != 0 {
		t.Fatalf("a made directory read bodies %d times err=%v", counting.batchBodyReads, err)
	}

	counting.bodyReads, counting.batchBodyReads = 0, 0
	profile, err := svc.Get(ctx, "alice", alice)
	if err != nil || counting.bodyReads != 1 || counting.batchBodyReads != 0 {
		t.Fatalf("the profile read 학습 글 %d+%d times err=%v", counting.bodyReads, counting.batchBodyReads, err)
	}
	bodies, err := h.store.ListSampleBodies(ctx, "alice", alice)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Readiness.Percent != voice.ReadinessOf(bodies).Percent || len(profile.Samples) != 2 {
		t.Fatalf("profile = %+v", profile)
	}
	for i, sample := range profile.Samples {
		if sample.ID != bodies[i].ID || sample.Body != "" || sample.Chars != utf8.RuneCountInString(bodies[i].Body) || sample.Label != bodies[i].Label {
			t.Fatalf("listed sample %d = %+v", i, sample)
		}
	}
}

// VOICE-60, POST-34 … POST-39: a photo prompt's photo is presigned under the voice, confirmed
// by a HEAD within the post photo's limits, opened through a fresh view URL, and removed after
// its answer's row.
func TestAPhotoPromptIsAnsweredOnTheOwnersUploadedPhoto(t *testing.T) {
	h := newVoiceHarness(t)
	objects := h.withPhotos()
	ctx := context.Background()
	alice := h.voice("alice")
	if _, _, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "opening_greeting"); !errors.Is(err, voice.ErrPromptNotFound) {
		t.Fatalf("a photo upload for a prompt with no photo = %v", err)
	}
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요."}); !errors.Is(err, voice.ErrPhotoRequired) {
		t.Fatalf("a photo prompt with no photo = %v", err)
	}
	upload, url, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "photo_food")
	if err != nil || !strings.HasPrefix(upload.Key, "voices/"+alice+"/") || !strings.HasSuffix(upload.Key, ".jpg") || !strings.Contains(url, upload.Key) {
		t.Fatalf("upload = %+v url=%q err=%v", upload, url, err)
	}
	answer := voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요.", UploadID: upload.ID, PhotoWidth: 1024, PhotoHeight: 768}
	// The PUT has not landed yet: the answer waits for it.
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, answer); !errors.Is(err, voice.ErrPhotoRequired) {
		t.Fatalf("an answer before the PUT = %v", err)
	}
	// bob cannot use alice's upload even for his own voice.
	objects.objects[upload.Key] = 5000
	if _, err := h.svc.AnswerPrompt(ctx, "bob", h.voice("bob"), answer); !errors.Is(err, voice.ErrPhotoRequired) {
		t.Fatalf("a foreign upload = %v", err)
	}
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요.", UploadID: upload.ID}); !errors.Is(err, voice.ErrInvalidPhoto) {
		t.Fatalf("an answer without dimensions = %v", err)
	}
	saved, err := h.svc.AnswerPrompt(ctx, "alice", alice, answer)
	if err != nil || saved.PhotoKey != upload.Key || saved.PhotoWidth != 1024 {
		t.Fatalf("photo answer = %+v err=%v", saved, err)
	}
	opened, view, err := h.svc.GetSample(ctx, "alice", alice, saved.ID)
	if err != nil || opened.Body != "짜장면이에요." || !strings.Contains(view, upload.Key) {
		t.Fatalf("opened = %+v view=%q err=%v", opened, view, err)
	}
	if _, _, err := h.svc.GetSample(ctx, "bob", alice, saved.ID); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("a foreign read = %v", err)
	}
	// A rewrite with no new photo stays on the photo the answer was written about.
	kept, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짜장면이에요. 면이 쫄깃했어요."})
	if err != nil || kept.PhotoKey != upload.Key || kept.PhotoWidth != 1024 || kept.PhotoHeight != 768 || len(objects.deleted) != 0 {
		t.Fatalf("rewrite keeping the photo = %+v err=%v deleted=%v", kept, err, objects.deleted)
	}
	// A rewrite on a new photo drops the old object.
	second, _, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[second.Key] = 5000
	replaced, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_food", Body: "짬뽕이에요.", UploadID: second.ID, PhotoWidth: 800, PhotoHeight: 600})
	if err != nil || replaced.PhotoKey != second.Key || len(objects.deleted) != 1 || objects.deleted[0] != upload.Key {
		t.Fatalf("rewrite on a new photo = %+v err=%v deleted=%v", replaced, err, objects.deleted)
	}
	if err := h.svc.DeleteSample(ctx, "alice", alice, replaced.ID); err != nil {
		t.Fatal(err)
	}
	if len(objects.deleted) != 2 || objects.deleted[1] != second.Key {
		t.Fatalf("deleted objects = %v", objects.deleted)
	}
}

// POST-36: an object past the size limit is dropped at once and never becomes a 학습 글.
func TestAnOversizedPhotoIsDroppedAtOnce(t *testing.T) {
	h := newVoiceHarness(t)
	objects := h.withPhotos()
	ctx := context.Background()
	alice := h.voice("alice")
	upload, _, err := h.svc.CreatePhotoUpload(ctx, "alice", alice, "photo_space")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[upload.Key] = photoMaxBytes + 1
	if _, err := h.svc.AnswerPrompt(ctx, "alice", alice, voice.Answer{PromptKey: "photo_space", Body: "넓어요.", UploadID: upload.ID, PhotoWidth: 10, PhotoHeight: 10}); !errors.Is(err, voice.ErrInvalidPhoto) {
		t.Fatalf("oversized photo = %v", err)
	}
	if _, ok := objects.objects[upload.Key]; ok {
		t.Fatal("the oversized object was left behind")
	}
	if count, _ := h.store.CountSamples(ctx, "alice", alice); count != 0 {
		t.Fatalf("an oversized photo became %d 학습 글", count)
	}
}

// VOICE-23, VOICE-32: 말투 만들기 is one analyze_voice job at 100% on a model registered to the
// analyze stage; a voice already analysing is refused.
func TestAnalyzeVoiceNeedsReadinessAndAnAnalyzeModel(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef); !errors.Is(err, voice.ErrVoiceNotReady) {
		t.Fatalf("an empty voice = %v", err)
	}
	h.addSample(t, "alice", alice, "almost", "거의", strings.Repeat("거의 다 됐어요.\n", voice.ReadySentences-1), time.Now())
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef); !errors.Is(err, voice.ErrVoiceNotReady) {
		t.Fatalf("a voice at 59 sentences = %v", err)
	}
	h.addSample(t, "alice", alice, "ready", "충분", readyPost(), time.Now())
	for _, ref := range []llm.ModelRef{{}, writeOnlyRef, disabledRef, {ProviderID: "stub", ModelID: "unknown"}} {
		if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, ref); !errors.Is(err, voice.ErrAnalyzeModelRequired) {
			t.Fatalf("model %v = %v", ref, err)
		}
	}
	id, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef)
	if calls := h.jobs.calls(); err != nil || id != "job-new" || len(calls) != 1 || calls[0].VoiceID != alice || calls[0].WriteModel != analyzeRef.String() {
		t.Fatalf("analyze = %q err=%v calls=%+v", id, err, calls)
	}
	h.jobs.enqueueErr = &voice.JobAlreadyInProgressError{ActiveID: "job-new"}
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("a second analysis = %v", err)
	}
	if h.models.completeCalls != 0 {
		t.Fatal("starting an analysis called a provider")
	}
}

// VOICE-9, VOICE-32: the directory and the profile carry the readiness of a voice not yet made.
func TestReadinessReachesTheDirectoryAndTheProfile(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "half", "절반", strings.Repeat("반쯤 왔어요.\n", voice.ReadySentences/2), time.Now())
	voices, _ := h.svc.ListVoices(ctx, "alice")
	if voices[0].ReadinessPercent != 50 {
		t.Fatalf("directory readiness = %d", voices[0].ReadinessPercent)
	}
	profile, err := h.svc.Get(ctx, "alice", alice)
	if err != nil || profile.Readiness.Percent != 50 || profile.Readiness.Sentences != voice.ReadySentences/2 || len(profile.Readiness.MissingParts) != 0 {
		t.Fatalf("profile readiness = %+v err=%v", profile.Readiness, err)
	}
}

// VOICE-46: the projection carries FewShotMax excerpts, newest first, each cut at the excerpt
// maximum when no sentence ends near the target; a voice not yet made projects nothing.
func TestProfileForPromptMostRecentTruncatedAndUnmade(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	if _, err := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", voice.LanguageKorean, ""); !errors.Is(err, voice.ErrVoiceNotMade) {
		t.Fatalf("an unmade voice projected: %v", err)
	}
	base := time.Now().Add(-time.Hour)
	markers := []rune{'가', '나', '다', '라'}
	for i := range 4 {
		body := strings.Repeat(string(markers[i]), voice.FewShotExcerptMaxChars+10)
		h.addSample(t, "alice", alice, string(rune('a'+i)), "sample", body, base.Add(time.Duration(i)*time.Minute))
	}
	h.makeVoice(t, "alice", alice)
	projection, err := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", voice.LanguageKorean, "")
	excerpts := projection.Excerpts
	if err != nil || len(excerpts) != voice.FewShotMax || !strings.HasPrefix(projection.Text, "[말투]") {
		t.Fatalf("profile prompt = %d excerpts, %q err=%v", len(excerpts), projection.Text, err)
	}
	if []rune(excerpts[0])[0] != '라' {
		t.Fatalf("first excerpt is not most recent: %q", []rune(excerpts[0])[0])
	}
	for _, excerpt := range excerpts {
		if len([]rune(excerpt)) != voice.FewShotExcerptMaxChars {
			t.Fatalf("excerpt length = %d", len([]rune(excerpt)))
		}
	}
	// The checked answer is never excerpted (VOICE-43), and a topic match comes first.
	excluded, _ := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", voice.LanguageKorean, "d")
	for _, excerpt := range excluded.Excerpts {
		if []rune(excerpt)[0] == '라' {
			t.Fatal("the excluded 학습 글 was excerpted")
		}
	}
	topical, _ := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "가가 여행", voice.LanguageKorean, "")
	if []rune(topical.Excerpts[0])[0] != '가' {
		t.Fatalf("a topic match did not come first: %q", []rune(topical.Excerpts[0])[0])
	}
	english, err := h.svc.PromptProfileForTopic(context.Background(), "alice", alice, "", voice.LanguageEnglish, "")
	if err != nil || !english.Portable || len(english.Excerpts) != 0 || !strings.HasPrefix(english.Text, "[Portable voice habits]") {
		t.Fatalf("English projection = %+v err=%v", english, err)
	}
}

// VOICE-23 … VOICE-26: an analysis counts the fingerprint, makes one call, keeps only the AI's
// examples quoted verbatim from a 학습 글 and publishes; a failed or incomplete call publishes
// nothing.
func TestAnalyzeCountsCallsOnceAndPublishes(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	body := strings.Repeat("진짜 맛있었어요!\n", 12) + "국물이 정말   진했어요."
	h.addSample(t, "alice", alice, "sample", "국숫집", body, time.Now())
	job := voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(), MaterialIDs: []string{"sample"}}
	for _, broken := range []string{"## 평균 문장 길이\n짧음", `{"impression": "짧아요"}`} {
		h.models.response = broken
		if err := h.svc.Analyze(context.Background(), job, func(string, int, int) {}); err == nil {
			t.Fatalf("a broken answer %q published", broken)
		}
		if current, _ := h.store.CurrentAnalysis(context.Background(), "alice", alice); current != nil {
			t.Fatalf("a broken answer published %+v", current)
		}
	}
	h.models.completeCalls = 0
	h.models.response = analysisAnswer("밝고 들뜬 말투예요. 느낌표가 많아요. 세 번째 문장은 버려요.", "국물이 정말 진했어요.", "지어낸 문장이에요.")
	var progress [][3]any
	if err := h.svc.Analyze(context.Background(), job, func(stage string, done, total int) {
		progress = append(progress, [3]any{stage, done, total})
	}); err != nil {
		t.Fatal(err)
	}
	current, err := h.store.CurrentAnalysis(context.Background(), "alice", alice)
	if err != nil || current == nil || h.models.completeCalls != 1 || len(progress) != 2 {
		t.Fatalf("analysis = %+v calls=%d progress=%v err=%v", current, h.models.completeCalls, progress, err)
	}
	if current.AI.Impression != "밝고 들뜬 말투예요. 느낌표가 많아요." || len(current.AI.Examples) != 1 || current.AI.Examples[0].MaterialID != "sample" {
		t.Fatalf("AI part = %+v", current.AI)
	}
	if current.Counted.Sentences != 13 || current.Counted.Marks.Unknown || current.Counted.Marks.Exclaim < 0.9 || strings.Join(current.MaterialIDs, ",") != "sample" {
		t.Fatalf("counted = %+v ids=%v", current.Counted.Marks, current.MaterialIDs)
	}
	request := h.models.request.Messages[0].Parts[0].Text
	if !strings.Contains(request, "[제품이 센 습관]") || !strings.Contains(request, "===== 학습 글 1: 국숫집 =====") || !strings.Contains(request, "느낌표") {
		t.Fatalf("analysis request = %s", request)
	}
	if !strings.Contains(h.models.request.System, "수치는 다시 세지 말고") {
		t.Fatalf("analysis system prompt = %q", h.models.request.System)
	}
}

// VOICE-30: 이전 분석으로 되돌리기 makes the previous analysis current and discards the replaced
// one; there is no redo, nothing to return to without a previous one, and a tombstone refuses.
func TestRestoreThePreviousAnalysisOnce(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	if _, err := h.svc.RestorePreviousAnalysis(ctx, "alice", alice); !errors.Is(err, voice.ErrNoPreviousAnalysis) {
		t.Fatalf("an unmade voice's undo = %v", err)
	}
	first := voice.Analysis{AI: voice.AIPart{Impression: "첫 분석"}, AnalyzeModel: "stub/analyze", CreatedAt: time.Now()}
	second := voice.Analysis{AI: voice.AIPart{Impression: "다시 분석"}, AnalyzeModel: "stub/analyze", CreatedAt: time.Now()}
	for _, analysis := range []voice.Analysis{first, second} {
		if err := h.store.PublishAnalysis(ctx, "alice", alice, analysis); err != nil {
			t.Fatal(err)
		}
	}
	profile, _ := h.svc.Get(ctx, "alice", alice)
	if profile.Analysis.AI.Impression != "다시 분석" || !profile.HasPrevious {
		t.Fatalf("before undo = %+v previous=%v", profile.Analysis, profile.HasPrevious)
	}
	restored, err := h.svc.RestorePreviousAnalysis(ctx, "alice", alice)
	if err != nil || restored.Analysis.AI.Impression != "첫 분석" || restored.HasPrevious {
		t.Fatalf("after undo = %+v err=%v", restored, err)
	}
	if _, err := h.svc.RestorePreviousAnalysis(ctx, "alice", alice); !errors.Is(err, voice.ErrNoPreviousAnalysis) {
		t.Fatalf("a second undo = %v", err)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", alice); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RestorePreviousAnalysis(ctx, "alice", alice); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("a tombstone's undo = %v", err)
	}
}

// VOICE-21: the notice says 새 학습 글 N편 after additions and 학습 글이 바뀌었어요 once a 학습 글 the
// analysis read is gone — whose examples leave at once.
func TestTheNoticeAndADeletedExample(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "s1", "하나", "첫 글이에요.", time.Now().Add(-time.Hour))
	h.addSample(t, "alice", alice, "s2", "둘", "둘째 글이에요.", time.Now().Add(-time.Minute))
	analysis := voice.Analysis{
		Counted:     voice.Fingerprint{Marks: voice.Marks{Example: voice.Example{Sentence: "첫 글이에요.", MaterialID: "s1"}}},
		AI:          voice.AIPart{Impression: "담담해요.", Examples: []voice.AIExample{{Field: voice.AIImpression, Sentence: "첫 글이에요.", MaterialID: "s1"}}},
		MaterialIDs: []string{"s1", "s2"}, AnalyzeModel: "stub/analyze", CreatedAt: time.Now(),
	}
	if err := h.store.PublishAnalysis(ctx, "alice", alice, analysis); err != nil {
		t.Fatal(err)
	}
	if profile, _ := h.svc.Get(ctx, "alice", alice); profile.Notice != (voice.Notice{}) {
		t.Fatalf("a fresh analysis has a notice: %+v", profile.Notice)
	}
	h.addSample(t, "alice", alice, "s3", "셋", "셋째 글이에요.", time.Now())
	if profile, _ := h.svc.Get(ctx, "alice", alice); profile.Notice != (voice.Notice{Kind: voice.NoticeAdded, Count: 1}) {
		t.Fatalf("after an addition = %+v", profile.Notice)
	}
	if err := h.svc.DeleteSample(ctx, "alice", alice, "s1"); err != nil {
		t.Fatal(err)
	}
	profile, _ := h.svc.Get(ctx, "alice", alice)
	if profile.Notice.Kind != voice.NoticeChanged {
		t.Fatalf("after a deletion = %+v", profile.Notice)
	}
	if profile.Analysis.Counted.Marks.Example != (voice.Example{}) || len(profile.Analysis.AI.Examples) != 0 {
		t.Fatalf("a deleted 학습 글's example survived: %+v %+v", profile.Analysis.Counted.Marks.Example, profile.Analysis.AI.Examples)
	}
}

// MODEL-9 and GEN-22 require analyze to send no reasoning effort. That rule lives in one
// place only — the absence of a stage value on the request — so it is asserted against the
// request the provider receives rather than against a config field.
func TestAnalyzeRequestsNoReasoningEffort(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.response = analysisAnswer("담담해요.")
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(), MaterialIDs: []string{"sample"}}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	// Unspecified is "no stage decision", which registry.go forwards as nothing. Unset would
	// be a different statement — the yaml sentinel for a model override that omits the key.
	if h.models.request.Reasoning != llm.ReasoningUnspecified {
		t.Fatalf("analyze request reasoning = %q, want no stage value", h.models.request.Reasoning)
	}
}

// VOICE-22: an analysis publishes the snapshot it read, even when a 학습 글 arrived meanwhile,
// and never repeats the call on its own.
func TestAnAnalysisPublishesWhatItReadAndNeverRepeats(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "first", "first", longSample("첫"), time.Now())
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() {
		done <- svc.Analyze(context.Background(), voice.AnalysisJob{
			UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(), MaterialIDs: []string{"first"},
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
	current, err := h.store.CurrentAnalysis(context.Background(), "alice", alice)
	if err != nil || current == nil || current.AI.Impression != "old" || len(current.MaterialIDs) != 1 {
		t.Fatalf("published analysis = %+v err=%v", current, err)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 1 || strings.Contains(models.requests[0], longSample("둘")) {
		t.Fatalf("analysis calls = %d: %+v", len(models.requests), models.requests)
	}
}

// sentPromptTokens is the request the provider received, at one token per Unicode character.
func sentPromptTokens(request llm.Request) int {
	tokens := utf8.RuneCountInString(request.System)
	for _, message := range request.Messages {
		for _, part := range message.Parts {
			tokens += utf8.RuneCountInString(part.Text)
		}
	}
	return tokens
}

// QUOTA-14, VOICE-22: the start freezes the 학습 글 it read, newest first, and declares the prompt
// the run then sends over them, character for character; a large corpus declares all of it.
func TestAnAnalysisStartFreezesItsSnapshotAndDeclaresItsPrompt(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "older", "예전", longSample("예"), time.Now().Add(-time.Hour))
	h.addSample(t, "alice", alice, "ready", "충분", readyPost(), time.Now())
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef); err != nil {
		t.Fatal(err)
	}
	calls := h.jobs.calls()
	if len(calls) != 1 || strings.Join(calls[0].MaterialIDs, ",") != "ready,older" {
		t.Fatalf("enqueued %+v", calls)
	}
	h.models.response = analysisAnswer("담담해요.")
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(), MaterialIDs: calls[0].MaterialIDs}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if sent := sentPromptTokens(h.models.request); calls[0].PromptTokens != sent || sent >= 30_000 {
		t.Fatalf("declared %d prompt tokens, the run sent %d", calls[0].PromptTokens, sent)
	}

	large, _ := h.svc.CreateVoice(ctx, "alice", "긴 말투")
	h.addSample(t, "alice", large.ID, "long", "긴 글", strings.Repeat("오늘도 정말 맛있게 먹었어요.\n", 3_000), time.Now())
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", large.ID, analyzeRef); err != nil {
		t.Fatal(err)
	}
	if calls := h.jobs.calls(); len(calls) != 2 || calls[1].PromptTokens < 50_000 || strings.Join(calls[1].MaterialIDs, ",") != "long" {
		t.Fatalf("a 50 000-character corpus declared %+v", calls[1:])
	}
}

// VOICE-22: the run reads the snapshot its start froze: a 학습 글 added after the start is not
// read, and one deleted after it is skipped.
func TestAnAnalysisReadsOnlyItsFrozenSnapshot(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "s1", "하나", readyPost(), time.Now().Add(-time.Hour))
	h.addSample(t, "alice", alice, "s2", "둘", longSample("둘"), time.Now().Add(-time.Minute))
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", alice, analyzeRef); err != nil {
		t.Fatal(err)
	}
	frozen := h.jobs.calls()[0].MaterialIDs
	h.addSample(t, "alice", alice, "s3", "셋", longSample("셋"), time.Now())
	if err := h.svc.DeleteSample(ctx, "alice", alice, "s1"); err != nil {
		t.Fatal(err)
	}
	h.models.response = analysisAnswer("담담해요.")
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(), MaterialIDs: frozen}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	current, err := h.store.CurrentAnalysis(ctx, "alice", alice)
	if err != nil || current == nil || strings.Join(current.MaterialIDs, ",") != "s2" {
		t.Fatalf("published analysis = %+v err=%v", current, err)
	}
	request := h.models.request.Messages[0].Parts[0].Text
	if strings.Contains(request, longSample("셋")) || strings.Contains(request, "오늘도 정말") || !strings.Contains(request, longSample("둘")) {
		t.Fatalf("analysis read outside its snapshot: %s", request)
	}
}

// Two voices analyzing at the same time do not see each other's corpus or overwrite each
// other's published profile: the profile head is per voice.
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
		casualDone <- svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: casual, WriteModel: analyzeRef.String(), MaterialIDs: []string{"c"}}, func(string, int, int) {})
	}()
	select {
	case <-models.started:
	case <-time.After(time.Second):
		t.Fatal("casual analysis did not start")
	}
	// The formal analysis completes entirely while the casual provider call is still open.
	if err := svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: formal.ID, WriteModel: analyzeRef.String(), MaterialIDs: []string{"f"}}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	close(models.release)
	if err := <-casualDone; err != nil {
		t.Fatal(err)
	}
	casualCurrent, _ := h.store.CurrentAnalysis(context.Background(), "alice", casual)
	formalCurrent, _ := h.store.CurrentAnalysis(context.Background(), "alice", formal.ID)
	casualAnalysis := casualCurrent.AI.Impression
	formalAnalysis := formalCurrent.AI.Impression
	if !strings.Contains(casualAnalysis, "old") || !strings.Contains(formalAnalysis, "new") {
		t.Fatalf("published analyses crossed: casual=%q formal=%q", casualAnalysis, formalAnalysis)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 2 || strings.Contains(models.requests[0], longSample("습")) || strings.Contains(models.requests[1], longSample("해")) {
		t.Fatalf("a voice saw the other voice's corpus: %+v", models.requests)
	}
}

type queueJobs struct{ queue *job.Queue }

func (a queueJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	payload, err := voice.EncodeAnalysisSnapshot(request.MaterialIDs)
	if err != nil {
		return "", err
	}
	id, err := a.queue.Enqueue(ctx, attach(job.NewJob{Kind: job.KindAnalyzeVoice, UserID: request.UserID, WriteModel: request.WriteModel, Payload: payload}, "", request.VoiceID))
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a queueJobs) EnqueueCheck(ctx context.Context, request voice.CheckJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, attach(job.NewJob{
		Kind: job.KindCheckVoice, UserID: request.UserID, WriteModel: request.WriteModel, Payload: []byte(request.CheckID),
	}, "", request.VoiceID))
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
		materialIDs, err := voice.DecodeAnalysisSnapshot(found.Payload)
		if err != nil {
			return err
		}
		return svc.Analyze(ctx, voice.AnalysisJob{UserID: found.UserID, VoiceID: found.Subject(voice.JobSubject), WriteModel: found.WriteModel, MaterialIDs: materialIDs}, voice.Progress(progress))
	})
	h.addSample(t, "alice", alice, "sample", "post", longSample("문"), time.Now())
	payload, err := voice.EncodeAnalysisSnapshot([]string{"sample"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := queue.Enqueue(context.Background(), attach(job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: "alice", WriteModel: analyzeRef.String(), Payload: payload}, "", alice))
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
// a second analysis for the same voice is refused.
func TestAnalysesAreGuardedPerVoiceThroughTheQueue(t *testing.T) {
	h := newVoiceHarness(t)
	casual := h.voice("alice")
	formal, _ := h.svc.CreateVoice(context.Background(), "alice", "격식")
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader, jobKindsForTest()), time.Second, jobReportingForTest())
	svc := voice.NewService(h.store, h.models, queueJobs{queue: queue})
	h.addSample(t, "alice", casual, "c", "casual", readyPost(), time.Now())
	h.addSample(t, "alice", formal.ID, "f", "formal", readyPost(), time.Now())
	casualJob, err := svc.AnalyzeVoice(context.Background(), "alice", casual, analyzeRef)
	if err != nil {
		t.Fatal(err)
	}
	formalJob, err := svc.AnalyzeVoice(context.Background(), "alice", formal.ID, analyzeRef)
	if err != nil || formalJob == casualJob {
		t.Fatalf("second voice could not analyze concurrently: job=%q err=%v", formalJob, err)
	}
	if _, err := svc.AnalyzeVoice(context.Background(), "alice", casual, analyzeRef); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("same voice analysing twice = %v", err)
	}
	profile, err := svc.Get(context.Background(), "alice", formal.ID)
	if err != nil || profile.ActiveJobID != formalJob {
		t.Fatalf("formal active job = %q want %q err=%v", profile.ActiveJobID, formalJob, err)
	}
	if _, err := svc.DeleteVoice(context.Background(), "alice", formal.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with a queued analysis = %v", err)
	}
}
