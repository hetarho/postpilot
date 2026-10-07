package voice_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/voice"
)

func seedCandidateBatch(t *testing.T, h *voiceHarness, id, user string) {
	t.Helper()
	now := time.Now()
	store := jobstore.New(h.db.Writer, h.db.Reader, jobstore.Kinds{})
	if err := store.Insert(context.Background(), job.Job{ID: id, UserID: user, Kind: job.KindWritingVoiceCandidates, WriteModel: "stub/write", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec("UPDATE generation_jobs SET status='done' WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
}
func adoptionFixture(id, jobID, name string) voice.CandidateAdoption {
	now := time.Now()
	sample := strings.Repeat("가상의 가게에 들러 따뜻한 차를 마셨어요. 창가에서 잠깐 쉬니 마음이 편안했어요. ", 5)
	return voice.CandidateAdoption{Voice: voice.Voice{ID: id, UserID: "alice", Name: name, CreatedAt: now, UpdatedAt: now}, JobID: jobID, CandidateID: "style-1", MakeDefault: true, Analysis: voice.Analysis{Origin: voice.OriginSynthetic, SyntheticSample: sample, Counted: voice.FingerprintOf([]voice.Material{{Text: sample, Kind: voice.SampleKindPost}}), AI: voice.AIPart{Impression: "편안한 가상 말투예요."}, AnalyzeModel: "stub/write", CreatedAt: now}}
}

func TestCandidateAdoptionIsAtomicIdempotentAndPreservesLaterEdits(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	seedCandidateBatch(t, h, "candidate-batch", "alice")
	name := strings.Repeat("말", 50)
	if _, err := h.svc.CreateVoice(ctx, "alice", name); err != nil {
		t.Fatal(err)
	}
	in := adoptionFixture("adopted", "candidate-batch", name)
	found, err := h.store.AdoptCandidate(ctx, in)
	if err != nil || !found.Made || !found.IsDefault || found.Origin != voice.OriginSynthetic || found.Name == name || utf8.RuneCountInString(found.Name) > 50 {
		t.Fatalf("adopted=%+v err=%v", found, err)
	}
	profile, err := h.svc.Get(ctx, "alice", found.ID)
	if err != nil || profile.Analysis.Origin != voice.OriginSynthetic || profile.Analysis.SyntheticSample == "" || len(profile.Samples) != 0 || profile.Readiness.Percent != 0 {
		t.Fatalf("profile=%+v err=%v", profile, err)
	}
	ko, err := h.svc.PromptProfileForTopic(ctx, "alice", found.ID, "주말", voice.LanguageKorean, "")
	if err != nil || len(ko.Excerpts) != 0 || !strings.Contains(ko.Text, "가상 예시") || !strings.Contains(ko.Text, "실제 경험이 아닙니다") || strings.Contains(ko.Text, "이 글쓴이가 직접 쓴 글") {
		t.Fatalf("projection=%+v err=%v", ko, err)
	}
	en, err := h.svc.PromptProfileForTopic(ctx, "alice", found.ID, "weekend", voice.LanguageEnglish, "")
	if err != nil || !strings.Contains(en.Text, "AI-created fictional illustration") || strings.Contains(en.Text, "writer's own Korean posts") || len(en.Excerpts) != 0 {
		t.Fatalf("portable=%+v err=%v", en, err)
	}
	if _, err := h.svc.RenameVoice(ctx, "alice", found.ID, "직접 바꾼 이름"); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.DeleteVoice(ctx, "alice", found.ID); err != nil {
		t.Fatal(err)
	}
	in.Voice.ID = "duplicate-must-not-exist"
	duplicate, err := h.store.AdoptCandidate(ctx, in)
	if err != nil || duplicate.ID != found.ID || duplicate.Name != "직접 바꾼 이름" || !duplicate.Deleted() || duplicate.IsDefault {
		t.Fatalf("duplicate=%+v err=%v", duplicate, err)
	}
	if _, err := h.store.GetVoice(ctx, "alice", in.Voice.ID); err == nil {
		t.Fatal("duplicate inserted another voice")
	}
}

func TestCandidateAdoptionConcurrentAndFailedMappingLeaveNoPartialVoice(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	seedCandidateBatch(t, h, "concurrent-batch", "alice")
	const workers = 8
	results := make(chan voice.Voice, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			in := adoptionFixture(fmt.Sprintf("adoption-%d", index), "concurrent-batch", "새로운 말투")
			found, err := h.store.AdoptCandidate(ctx, in)
			results <- found
			errs <- err
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for found := range results {
		if id == "" {
			id = found.ID
		}
		if found.ID != id {
			t.Fatalf("different voices: %s %s", id, found.ID)
		}
	}
	var count int
	if err := h.db.Reader.QueryRow("SELECT count(*) FROM writing_voice_candidate_adoptions WHERE user_id='alice'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("mapping count=%d err=%v", count, err)
	}
	failed := adoptionFixture("rollback-voice", "missing-batch", "실패하면 남지 않는 말투")
	if _, err := h.store.AdoptCandidate(ctx, failed); err == nil {
		t.Fatal("missing batch mapping was accepted")
	}
	if _, err := h.store.GetVoice(ctx, "alice", failed.Voice.ID); err == nil {
		t.Fatal("failed transaction retained voice")
	}
	var currentDefault string
	if err := h.db.Reader.QueryRow("SELECT id FROM voices WHERE user_id='alice' AND is_default=1").Scan(&currentDefault); err != nil || currentDefault != id {
		t.Fatalf("rollback changed default=%s err=%v", currentDefault, err)
	}
}

func TestSyntheticAnalysisCanBecomePersonalAndRestoreOriginalProvenance(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	seedCandidateBatch(t, h, "personalize-batch", "alice")
	adopted, err := h.store.AdoptCandidate(ctx, adoptionFixture("personalize", "personalize-batch", "소소한 말투"))
	if err != nil {
		t.Fatal(err)
	}
	for _, prompt := range voice.Prompts() {
		if prompt.Starter {
			if _, err := h.svc.AnswerPrompt(ctx, "alice", adopted.ID, voice.Answer{PromptKey: prompt.Key, Body: "저는 오늘 산책하고 싶어요."}); err != nil {
				t.Fatal(err)
			}
		}
	}
	h.models.response = analysisAnswer("주인의 글에서 읽은 인상")
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", adopted.ID, analyzeRef); err != nil {
		t.Fatal(err)
	}
	requests := h.jobs.calls()
	last := requests[len(requests)-1]
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: adopted.ID, WriteModel: analyzeRef.String(), MaterialIDs: last.MaterialIDs, AcceptedSources: last.AcceptedSources, AcceptedMaterials: last.AcceptedMaterials}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	personal, err := h.svc.Get(ctx, "alice", adopted.ID)
	if err != nil || personal.Analysis.Origin != voice.OriginPersonal || personal.Voice.Origin != voice.OriginPersonal || personal.Analysis.SyntheticSample != "" || len(personal.Samples) != 10 {
		t.Fatalf("personal=%+v err=%v", personal, err)
	}
	restored, err := h.svc.RestorePreviousAnalysis(ctx, "alice", adopted.ID)
	if err != nil || restored.Analysis.Origin != voice.OriginSynthetic || restored.Voice.Origin != voice.OriginSynthetic || restored.Analysis.SyntheticSample == "" {
		t.Fatalf("restored=%+v err=%v", restored, err)
	}
	ko, err := h.svc.PromptProfileForTopic(ctx, "alice", adopted.ID, "산책", voice.LanguageKorean, "")
	if err != nil || len(ko.Excerpts) != 0 {
		t.Fatalf("synthetic projection retrieved personal excerpts=%+v err=%v", ko, err)
	}
}

func TestLegacySnapshotWithoutOriginIsPersonal(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id := h.voice("alice")
	h.makeVoice(t, "alice", id)
	if _, err := h.db.Writer.Exec("UPDATE voice_analyses SET snapshot=json_remove(snapshot,'$.origin') WHERE voice_id=?", id); err != nil {
		t.Fatal(err)
	}
	analysis, err := h.store.CurrentAnalysis(ctx, "alice", id)
	if err != nil || analysis.Origin != voice.OriginPersonal {
		t.Fatalf("analysis=%+v err=%v", analysis, err)
	}
	directory, err := h.store.GetVoice(ctx, "alice", id)
	if err != nil || directory.Origin != voice.OriginPersonal {
		t.Fatalf("directory=%+v err=%v", directory, err)
	}
}
