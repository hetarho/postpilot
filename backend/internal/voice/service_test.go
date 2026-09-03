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
		return llm.Response{Text: "## 1. 종결어미 분포\nold\n## 8. never uses\nold"}, nil
	}
	return llm.Response{Text: "## 1. 종결어미 분포\nnew\n## 8. never uses\nnew"}, nil
}

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

func (f *fakeModels) ModelEnabled(ref llm.ModelRef, _ string) bool {
	return ref.ProviderID != "" && ref.ModelID != ""
}

// fakeJobs keys its active analyses by VOICE, which is the guard the service must ask for.
type fakeJobs struct {
	mu                    sync.Mutex
	active                map[string]*voice.ActiveJob
	busy                  map[string]bool
	enqueueID             string
	enqueueErr            error
	enqueueCalls          []voice.AnalysisJobRequest
	personalizationCalls  []voice.PersonalizationJobRequest
	personalizationActive map[string]bool
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

func (f *fakeJobs) EnqueuePersonalization(_ context.Context, request voice.PersonalizationJobRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.personalizationCalls = append(f.personalizationCalls, request)
	if f.personalizationActive == nil {
		f.personalizationActive = make(map[string]bool)
	}
	if f.enqueueErr == nil {
		f.personalizationActive[f.enqueueID] = true
	}
	return f.enqueueID, f.enqueueErr
}

func (f *fakeJobs) IsPersonalizationJobActive(_ context.Context, jobID, _ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.personalizationActive[jobID], nil
}

func (f *fakeJobs) FailQueuedPersonalization(context.Context, string, string, voice.Failure) (bool, error) {
	return true, nil
}

func (f *fakeJobs) calls() []voice.AnalysisJobRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]voice.AnalysisJobRequest(nil), f.enqueueCalls...)
}

type fakeExperimentGuard struct{ busy map[string]bool }

func (f fakeExperimentGuard) HasPublishableExperimentForVoice(_ context.Context, _, voiceID string) (bool, error) {
	return f.busy[voiceID], nil
}

type voiceHarness struct {
	store  *voicestore.Store
	db     *db.DB
	models *fakeModels
	jobs   *fakeJobs
	svc    *voice.Service
	voices map[string]string
}

// newVoiceHarness seeds two accounts and gives each its default voice through the same
// bootstrap adduser uses, so every test starts from the state a real account is in.
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
		created, isNew, err := h.svc.EnsureDefaultVoice(context.Background(), userID, voice.LanguageKorean)
		if err != nil || !isNew || !created.IsDefault || created.Name != voice.DefaultVoiceName {
			t.Fatalf("bootstrap %s: voice=%+v new=%v err=%v", userID, created, isNew, err)
		}
		h.voices[userID] = created.ID
	}
	return h
}

// voice returns the account's default voice id; the older single-voice tests run inside it.
func (h *voiceHarness) voice(user string) string { return h.voices[user] }

func (h *voiceHarness) addSample(t *testing.T, user, voiceID, id, label, body string, at time.Time) {
	t.Helper()
	if err := h.store.InsertSample(context.Background(), voice.Sample{ID: id, UserID: user, VoiceID: voiceID, Label: label, Body: body, CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
}

func longSample(char string) string { return strings.Repeat(char, voice.SampleMinChars) }

// --- directory (plan 10 A2–A6) ---

func TestBootstrapIsIdempotentAndRepairsAMissingDefault(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	again, isNew, err := h.svc.EnsureDefaultVoice(ctx, "alice", voice.LanguageKorean)
	if err != nil || isNew || again.ID != h.voice("alice") {
		t.Fatalf("second bootstrap = %+v new=%v err=%v", again, isNew, err)
	}
	voices, _ := h.svc.ListVoices(ctx, "alice")
	if len(voices) != 1 {
		t.Fatalf("bootstrap duplicated the default: %+v", voices)
	}
	// An account whose default flag was lost gets its oldest active voice promoted, never a
	// second `기본 말투`.
	if _, err := h.db.Writer.Exec("UPDATE voices SET is_default = 0 WHERE user_id = 'alice'"); err != nil {
		t.Fatal(err)
	}
	repaired, isNew, err := h.svc.EnsureDefaultVoice(ctx, "alice", voice.LanguageKorean)
	if err != nil || isNew || repaired.ID != h.voice("alice") || !repaired.IsDefault {
		t.Fatalf("repair = %+v new=%v err=%v", repaired, isNew, err)
	}
}

func TestBootstrapRequiresExplicitLanguageAndFreezesItOnFirstCreation(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO users (id, password_hash, created_at) VALUES ('charlie', 'hash', ?)", now); err != nil {
		t.Fatal(err)
	}
	if _, _, err := h.svc.EnsureDefaultVoice(ctx, "charlie", voice.Language("fr")); !errors.Is(err, voice.ErrLanguageRequired) {
		t.Fatalf("invalid bootstrap language = %v", err)
	}
	created, isNew, err := h.svc.EnsureDefaultVoice(ctx, "charlie", voice.LanguageEnglish)
	if err != nil || !isNew || created.SourceLanguage != voice.LanguageEnglish {
		t.Fatalf("English bootstrap = %+v new=%v err=%v", created, isNew, err)
	}
	again, isNew, err := h.svc.EnsureDefaultVoice(ctx, "charlie", voice.LanguageKorean)
	if err != nil || isNew || again.ID != created.ID || again.SourceLanguage != voice.LanguageEnglish {
		t.Fatalf("rerun changed source = %+v new=%v err=%v", again, isNew, err)
	}
}

func TestCreateRenameValidateAndUniqueNames(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	var badName *voice.VoiceNameError
	if _, _, err := h.svc.CreateVoice(ctx, "alice", "   ", voice.LanguageKorean, nil); !errors.As(err, &badName) || badName.Chars != 0 {
		t.Fatalf("blank name = %v", err)
	}
	if _, _, err := h.svc.CreateVoice(ctx, "alice", strings.Repeat("가", voice.VoiceNameMaxChars+1), voice.LanguageKorean, nil); !errors.As(err, &badName) || badName.Chars != voice.VoiceNameMaxChars+1 {
		t.Fatalf("long name = %v", err)
	}
	review, _, err := h.svc.CreateVoice(ctx, "alice", "  리뷰 말투  ", voice.LanguageKorean, nil)
	if err != nil || review.Name != "리뷰 말투" || review.IsDefault || review.Deleted() {
		t.Fatalf("create = %+v err=%v", review, err)
	}
	if _, _, err := h.svc.CreateVoice(ctx, "alice", "리뷰 말투", voice.LanguageKorean, nil); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("duplicate active name = %v", err)
	}
	if _, _, err := h.svc.CreateVoice(ctx, "bob", "리뷰 말투", voice.LanguageKorean, nil); err != nil {
		t.Fatalf("same name in another account = %v", err)
	}
	if _, err := h.svc.RenameVoice(ctx, "alice", review.ID, voice.DefaultVoiceName); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("rename onto active name = %v", err)
	}
	renamed, err := h.svc.RenameVoice(ctx, "alice", review.ID, " 제품 리뷰 ")
	if err != nil || renamed.Name != "제품 리뷰" || renamed.ID != review.ID {
		t.Fatalf("rename = %+v err=%v", renamed, err)
	}
	// A new voice is genuinely empty even though the default has data.
	if err := h.svc.AppendRule(ctx, "alice", h.voice("alice"), "default rule"); err != nil {
		t.Fatal(err)
	}
	profile, err := h.svc.Get(ctx, "alice", review.ID)
	if err != nil || profile.Rules != "" || len(profile.Samples) != 0 || !profile.Structured.Empty || profile.Voice.ID != review.ID {
		t.Fatalf("new voice inherited data: %+v err=%v", profile, err)
	}
}

func TestSetDefaultSwapsAtomicallyAndRefusesTombstones(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	second, _, _ := h.svc.CreateVoice(ctx, "alice", "둘째", voice.LanguageKorean, nil)
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
	if defaults != 1 {
		t.Fatalf("defaults=%d voices=%+v", defaults, voices)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "bob", second.ID); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign set default = %v", err)
	}
	old := h.voice("alice")
	if _, err := h.svc.DeleteVoice(ctx, "alice", old); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", old); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("set deleted default = %v", err)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", ""); !errors.Is(err, voice.ErrVoiceRequired) {
		t.Fatalf("empty id = %v", err)
	}
}

func TestDeleteAndRestoreLifecycle(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	if _, err := h.svc.DeleteVoice(ctx, "alice", h.voice("alice")); !errors.Is(err, voice.ErrVoiceIsDefault) {
		t.Fatalf("delete default = %v", err)
	}
	extra, _, _ := h.svc.CreateVoice(ctx, "alice", "일기", voice.LanguageEnglish, nil)
	h.addSample(t, "alice", extra.ID, "s1", "일기", longSample("일"), time.Now())
	if err := h.svc.AppendRule(ctx, "alice", extra.ID, "diary rule"); err != nil {
		t.Fatal(err)
	}
	deleted, err := h.svc.DeleteVoice(ctx, "alice", extra.ID)
	if err != nil || !deleted.Deleted() || deleted.Name != "일기" || deleted.SourceLanguage != voice.LanguageEnglish {
		t.Fatalf("delete = %+v err=%v", deleted, err)
	}
	// Idempotent, and the tombstone keeps its whole profile readable.
	if again, err := h.svc.DeleteVoice(ctx, "alice", extra.ID); err != nil || !again.Deleted() {
		t.Fatalf("second delete = %+v err=%v", again, err)
	}
	profile, err := h.svc.Get(ctx, "alice", extra.ID)
	if err != nil || profile.Rules != "diary rule" || len(profile.Samples) != 1 || !profile.Voice.Deleted() || profile.Voice.SourceLanguage != voice.LanguageEnglish {
		t.Fatalf("tombstone profile = %+v err=%v", profile, err)
	}
	voices, _ := h.svc.ListVoices(ctx, "alice")
	if len(voices) != 2 || voices[0].Deleted() || !voices[1].Deleted() {
		t.Fatalf("tombstone should list last: %+v", voices)
	}
	// Restore is blocked by an active voice holding the name, and unblocked by renaming
	// the tombstone; it never changes the default.
	if _, _, err := h.svc.CreateVoice(ctx, "alice", "일기", voice.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.RestoreVoice(ctx, "alice", extra.ID); !errors.Is(err, voice.ErrVoiceNameTaken) {
		t.Fatalf("conflicting restore = %v", err)
	}
	if _, err := h.svc.RenameVoice(ctx, "alice", extra.ID, "옛 일기"); err != nil {
		t.Fatal(err)
	}
	restored, err := h.svc.RestoreVoice(ctx, "alice", extra.ID)
	if err != nil || restored.Deleted() || restored.IsDefault || restored.Name != "옛 일기" || restored.SourceLanguage != voice.LanguageEnglish {
		t.Fatalf("restore = %+v err=%v", restored, err)
	}
	if len(h.jobs.calls()) != 0 || len(h.jobs.personalizationCalls) != 0 {
		t.Fatal("lifecycle enqueued work")
	}
	if h.models.completeCalls != 0 {
		t.Fatal("lifecycle called a provider")
	}
}

func TestDeleteRefusesVoiceWithPublishableWork(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	busy, _, _ := h.svc.CreateVoice(ctx, "alice", "바쁜 말투", voice.LanguageKorean, nil)
	h.jobs.busy[busy.ID] = true
	if _, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with active job = %v", err)
	}
	h.jobs.busy[busy.ID] = false
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := h.db.Writer.Exec("INSERT INTO voice_contrast_rules(id,user_id,voice_id,statement,canonical_key,layer,evidence_count,status,origin,created_at,last_evidence_at) VALUES('r','alice',?,'s','k','endings',1,'candidate','diff',?,?)", busy.ID, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_authored_sources(id,user_id,voice_id,title,tags,body,excerpt,created_at) VALUES('src','alice',?,'t','[]','b','e',?)", busy.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("INSERT INTO voice_rule_comparisons(id,user_id,voice_id,rule_id,source_id,profile_version,model_ref,target_length,input_snapshot,rule_on_side,status,created_at) VALUES('c','alice',?,'r','src',1,'m',0,'{}','left','review',?)", busy.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with undecided comparison = %v", err)
	}
	if _, err := h.db.Writer.Exec("UPDATE voice_rule_comparisons SET status='decided', chosen_side='left', decided_at=? WHERE id='c'", now); err != nil {
		t.Fatal(err)
	}
	h.svc.SetExperimentGuard(fakeExperimentGuard{busy: map[string]bool{busy.ID: true}})
	if _, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); !errors.Is(err, voice.ErrVoiceBusy) {
		t.Fatalf("delete with publishable experiment = %v", err)
	}
	h.svc.SetExperimentGuard(fakeExperimentGuard{busy: map[string]bool{}})
	if deleted, err := h.svc.DeleteVoice(ctx, "alice", busy.ID); err != nil || !deleted.Deleted() {
		t.Fatalf("delete once idle = %+v err=%v", deleted, err)
	}
}

// --- isolation (plan 10 A10, A11, A15) ---

func TestProfilesAndSamplesAreIsolatedByVoiceAndAccount(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	casual := h.voice("alice")
	formal, _, _ := h.svc.CreateVoice(ctx, "alice", "격식", voice.LanguageKorean, nil)
	if err := h.svc.AppendRule(ctx, "alice", casual, "casual rule"); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.AppendRule(ctx, "alice", formal.ID, "formal rule"); err != nil {
		t.Fatal(err)
	}
	h.addSample(t, "alice", casual, "casual-sample", "캐주얼", longSample("해"), time.Now())
	h.addSample(t, "alice", formal.ID, "formal-sample", "격식", longSample("습"), time.Now())
	h.jobs.active[casual] = &voice.ActiveJob{ID: "analysis-casual"}

	casualProfile, err := h.svc.Get(ctx, "alice", casual)
	if err != nil || casualProfile.Rules != "casual rule" || len(casualProfile.Samples) != 1 || casualProfile.Samples[0].ID != "casual-sample" || casualProfile.ActiveJobID != "analysis-casual" {
		t.Fatalf("casual profile leaked/missing: %+v err=%v", casualProfile, err)
	}
	formalProfile, err := h.svc.Get(ctx, "alice", formal.ID)
	if err != nil || formalProfile.Rules != "formal rule" || len(formalProfile.Samples) != 1 || formalProfile.Samples[0].ID != "formal-sample" || formalProfile.ActiveJobID != "" {
		t.Fatalf("formal profile leaked/missing: %+v err=%v", formalProfile, err)
	}
	_, excerpts, rules, empty, err := h.svc.ProfileForPrompt(ctx, "alice", formal.ID)
	if err != nil || empty || rules != "formal rule" || len(excerpts) != 1 || !strings.HasPrefix(excerpts[0], "습") {
		t.Fatalf("formal prompt borrowed from casual: rules=%q excerpts=%v err=%v", rules, excerpts, err)
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
	if err := h.svc.AppendRule(ctx, "bob", casual, "hijack"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice write = %v", err)
	}
	if _, _, err := h.svc.AddSample(ctx, "bob", formal.ID, "", longSample("가"), analyzeRef); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("foreign voice sample = %v", err)
	}
	// Bob's own default is untouched by any of it.
	bobProfile, err := h.svc.Get(ctx, "bob", h.voice("bob"))
	if err != nil || bobProfile.Rules != "" || len(bobProfile.Samples) != 0 {
		t.Fatalf("bob profile changed: %+v err=%v", bobProfile, err)
	}
}

func TestDeletedVoiceStaysReadableButRefusesMutations(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	gone, _, _ := h.svc.CreateVoice(ctx, "alice", "사라질 말투", voice.LanguageKorean, nil)
	if err := h.svc.AppendRule(ctx, "alice", gone.ID, "rule"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", gone.ID); err != nil {
		t.Fatal(err)
	}
	if profile, err := h.svc.Get(ctx, "alice", gone.ID); err != nil || profile.Rules != "rule" {
		t.Fatalf("tombstone read = %+v err=%v", profile, err)
	}
	if _, _, err := h.svc.AddSample(ctx, "alice", gone.ID, "", longSample("가"), analyzeRef); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("sample on deleted = %v", err)
	}
	if err := h.svc.AppendRule(ctx, "alice", gone.ID, "rule"); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("rule on deleted = %v", err)
	}
	if _, err := h.svc.PromptProfileForTopic(ctx, "alice", gone.ID, "", nil); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("prompt for deleted = %v", err)
	}
	if _, err := h.svc.SnapshotAnalysisInput(ctx, "alice", gone.ID); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("experiment snapshot for deleted = %v", err)
	}
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: gone.ID, WriteModel: analyzeRef.String()}, func(string, int, int) {}); !errors.Is(err, voice.ErrVoiceDeleted) {
		t.Fatalf("analyze deleted = %v", err)
	}
	if len(h.jobs.calls()) != 0 || h.models.completeCalls != 0 {
		t.Fatal("a deleted voice reached the queue or a provider")
	}
}

// --- samples and analysis (plan 03, now per voice) ---

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

func TestAnalyzeExperimentSnapshotDoesNotMutateAndApplyPreservesRules(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	if err := h.svc.AppendRule(context.Background(), "alice", alice, "hand rule"); err != nil {
		t.Fatal(err)
	}
	h.addSample(t, "alice", alice, "sample", "글", longSample("가"), time.Now())
	raw, err := h.svc.SnapshotAnalysisInput(context.Background(), "alice", alice)
	if err != nil {
		t.Fatal(err)
	}
	h.models.response = "## 1. 종결어미 분포\n새 분석\n## 8. never uses\n없음"
	first, _, err := h.svc.RunAnalyzeCandidate(context.Background(), raw, analyzeRef)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := h.svc.RunAnalyzeCandidate(context.Background(), raw, llm.ModelRef{ProviderID: "stub", ModelID: "other"})
	if err != nil || first != second {
		t.Fatalf("same corpus produced invalid candidates: first=%q second=%q err=%v", first, second, err)
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || profile.Structured.Version != 0 || profile.Rules != "hand rule" {
		t.Fatalf("experiment mutated profile before apply: %+v err=%v", profile, err)
	}
	// The winner lands only in the voice it was frozen for; a sibling voice is untouched.
	other, _, _ := h.svc.CreateVoice(context.Background(), "alice", "다른 말투", voice.LanguageKorean, nil)
	if err := h.svc.ApplyStyleguideWinner(context.Background(), "alice", alice, first); err != nil {
		t.Fatal(err)
	}
	// It is applied as a PUBLISHED STRUCTURED VERSION now, whose lexical description is the
	// winning analysis (change 16). The "save as rule" text is untouched by it.
	profile, err = h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || profile.Structured.Version == 0 || profile.Structured.Lexical.Description.Value != first || profile.Rules != "hand rule" {
		t.Fatalf("winner apply did not publish a structured version: %+v err=%v", profile, err)
	}
	if profile.Structured.Lexical.Description.Source != voice.SourceAnalyzed {
		t.Fatalf("winner description source = %v", profile.Structured.Lexical.Description.Source)
	}
	if otherProfile, err := h.store.GetProfile(context.Background(), "alice", other.ID); err != nil || otherProfile.Structured.Version != 0 {
		t.Fatalf("winner leaked into another voice: %+v err=%v", otherProfile, err)
	}
}

func TestProfileForPromptMostRecentTruncatedAndEmpty(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	style, excerpts, rules, empty, err := h.svc.ProfileForPrompt(context.Background(), "alice", alice)
	if err != nil || style != "" || rules != "" || len(excerpts) != 0 || !empty {
		t.Fatalf("empty profile = %q %+v %q %v err=%v", style, excerpts, rules, empty, err)
	}
	if err := h.svc.AppendRule(context.Background(), "alice", alice, "RULES"); err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	markers := []rune{'가', '나', '다', '라'}
	for i := range 4 {
		body := strings.Repeat(string(markers[i]), voice.ExcerptChars+10)
		h.addSample(t, "alice", alice, string(rune('a'+i)), "sample", body, base.Add(time.Duration(i)*time.Minute))
	}
	_, excerpts, rules, empty, err = h.svc.ProfileForPrompt(context.Background(), "alice", alice)
	if err != nil || rules != "RULES" || empty || len(excerpts) != voice.ExcerptCount {
		t.Fatalf("profile prompt = lens=%d %q %v err=%v", len(excerpts), rules, empty, err)
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

func TestAppendRuleDeduplicates(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	if err := h.svc.AppendRule(context.Background(), "alice", alice, "existing"); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"  new rule  ", "new rule"} {
		if err := h.svc.AppendRule(context.Background(), "alice", alice, line); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || profile.Rules != "existing\nnew rule" {
		t.Fatalf("profile after rule = %+v err=%v", profile, err)
	}
}

func TestConcurrentAppendRuleDoesNotLoseLines(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, line := range []string{"first", "second"} {
		go func() {
			<-start
			errs <- h.svc.AppendRule(context.Background(), "alice", alice, line)
		}()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	profile, err := h.store.GetProfile(context.Background(), "alice", alice)
	if err != nil || !strings.Contains(profile.Rules, "first") || !strings.Contains(profile.Rules, "second") {
		t.Fatalf("concurrent rules = %q err=%v", profile.Rules, err)
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

func TestAnalyzePublishesStructuredProfileAndNeverTouchesRules(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	if err := h.svc.AppendRule(context.Background(), "alice", alice, "keep this rule"); err != nil {
		t.Fatal(err)
	}
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.response = "## 평균 문장 길이\n짧음"
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(string, int, int) {}); err == nil || !strings.Contains(err.Error(), "종결어미") {
		t.Fatalf("missing ending section error = %v", err)
	}
	profile, _ := h.store.GetProfile(context.Background(), "alice", alice)
	if profile.Structured.Version != 0 || profile.Rules != "keep this rule" {
		t.Fatalf("invalid analysis mutated profile: %+v", profile)
	}

	h.models.response = "## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
	var progress [][3]any
	if err := h.svc.Analyze(context.Background(), voice.AnalysisJob{UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String()}, func(stage string, done, total int) {
		progress = append(progress, [3]any{stage, done, total})
	}); err != nil {
		t.Fatal(err)
	}
	profile, _ = h.store.GetProfile(context.Background(), "alice", alice)
	// The analysis text lands in the published structured version's lexical description, once
	// (change 16); the "save as rule" text is never touched by an analysis.
	if profile.Structured.Lexical.Description.Value != h.models.response || profile.Rules != "keep this rule" || len(progress) != 2 {
		t.Fatalf("successful analysis = profile=%+v progress=%+v", profile, progress)
	}
	if !strings.Contains(h.models.request.Messages[0].Parts[0].Text, longSample("글")) {
		t.Fatal("analysis request omitted the accumulated corpus")
	}
}

// policy/providers.md requires analyze to send no reasoning effort. That rule lives in one
// place only — the absence of a stage value on the request — so it is asserted against the
// request the provider receives rather than against a config field.
func TestAnalyzeRequestsNoReasoningEffort(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	h.addSample(t, "alice", alice, "sample", "post", longSample("글"), time.Now())
	h.models.response = "## 1. 종결어미 분포\n해요체\n## 8. 절대 사용하지 않는 표현 (never uses)\n과장"
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
	formal, _, _ := h.svc.CreateVoice(context.Background(), "alice", "격식", voice.LanguageKorean, nil)
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
	if err := h.svc.AppendRule(context.Background(), "alice", alice, "keep rule"); err != nil {
		t.Fatal(err)
	}
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
	if err != nil || profile.Structured.Version != 0 || profile.Rules != "keep rule" {
		t.Fatalf("last-delete profile = %+v err=%v", profile, err)
	}
}

type queueJobs struct{ queue *job.Queue }

func (a queueJobs) Enqueue(ctx context.Context, request voice.AnalysisJobRequest) (string, error) {
	id, err := a.queue.Enqueue(ctx, job.NewJob{Kind: job.KindAnalyzeVoice, UserID: request.UserID, VoiceID: request.VoiceID, WriteModel: request.WriteModel})
	var active *job.ErrAlreadyInProgress
	if errors.As(err, &active) {
		return "", &voice.JobAlreadyInProgressError{ActiveID: active.ActiveID}
	}
	return id, err
}

func (a queueJobs) ActiveForVoiceKind(ctx context.Context, voiceID, kind string) (*voice.ActiveJob, error) {
	found, err := a.queue.ActiveForVoiceKind(ctx, voiceID, kind)
	if err != nil || found == nil {
		return nil, err
	}
	return &voice.ActiveJob{ID: found.ID}, nil
}

func (a queueJobs) HasActiveForVoice(ctx context.Context, voiceID string) (bool, error) {
	return a.queue.HasActiveForVoice(ctx, voiceID)
}

func TestAnalyzeHandlerFailureBecomesFailedJob(t *testing.T) {
	h := newVoiceHarness(t)
	alice := h.voice("alice")
	models := h.models
	models.response = "## 문장 길이\n짧음"
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader), 5*time.Millisecond)
	svc := voice.NewService(h.store, models, queueJobs{queue: queue})
	queue.Register(job.KindAnalyzeVoice, func(ctx context.Context, found job.Job, progress job.Progress) error {
		return svc.Analyze(ctx, voice.AnalysisJob{UserID: found.UserID, VoiceID: found.VoiceID, WriteModel: found.WriteModel}, voice.Progress(progress))
	})
	h.addSample(t, "alice", alice, "sample", "post", longSample("문"), time.Now())
	id, err := queue.Enqueue(context.Background(), job.NewJob{
		Kind: job.KindAnalyzeVoice, UserID: "alice", VoiceID: alice, WriteModel: analyzeRef.String(),
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
	formal, _, _ := h.svc.CreateVoice(context.Background(), "alice", "격식", voice.LanguageKorean, nil)
	queue := job.New(jobstore.New(h.db.Writer, h.db.Reader), time.Second)
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
