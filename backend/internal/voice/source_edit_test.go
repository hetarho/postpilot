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

func captureAnalysisJob(t *testing.T, h *voiceHarness, job voice.AnalysisJob) voice.AnalysisJob {
	t.Helper()
	for _, id := range job.MaterialIDs {
		sample, err := h.store.GetSampleBody(context.Background(), job.UserID, job.VoiceID, id)
		if err != nil || sample == nil {
			t.Fatal("capture owned sample", id, err)
		}
		source := voice.AcceptedSource{SampleID: id, ContentRevision: sample.ContentRevision}
		job.AcceptedSources = append(job.AcceptedSources, source)
		job.AcceptedMaterials = append(job.AcceptedMaterials, voice.AcceptedMaterial{Source: source, Body: sample.Body, PhotoKey: sample.PhotoKey, PhotoWidth: sample.PhotoWidth, PhotoHeight: sample.PhotoHeight, Kind: sample.Kind, PromptKey: sample.PromptKey, Label: sample.Label, CreatedAt: sample.CreatedAt})
	}
	return job
}

func analysisJob(request voice.AnalysisJobRequest) voice.AnalysisJob {
	return voice.AnalysisJob{UserID: request.UserID, VoiceID: request.VoiceID, WriteModel: request.WriteModel, MaterialIDs: request.MaterialIDs, AcceptedSources: request.AcceptedSources, AcceptedMaterials: request.AcceptedMaterials}
}

func editBody(t *testing.T, h *voiceHarness, sample voice.Sample, key, body string) voice.Sample {
	t.Helper()
	updated, err := h.svc.UpdateVoiceSample(context.Background(), voice.SampleMutation{UserID: sample.UserID, VoiceID: sample.VoiceID, SampleID: sample.ID, ExpectedContentRevision: sample.ContentRevision, OperationKey: key, Body: &body})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}

func TestMaterialEditingUsesOwnerVoiceCASAndReceiptsWithoutAI(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	sample, err := h.svc.AddSample(ctx, "alice", h.voice("alice"), "첫 글", readyPost())
	if err != nil || sample.ContentRevision != 1 {
		t.Fatal("new material revision", err)
	}
	label := "새 제목"
	metadata, err := h.svc.UpdateVoiceSample(ctx, voice.SampleMutation{UserID: "alice", VoiceID: sample.VoiceID, SampleID: sample.ID, ExpectedContentRevision: 1, OperationKey: "label", Label: &label})
	if err != nil || metadata.ContentRevision != 1 || metadata.Label != label {
		t.Fatal("metadata edit changed content revision", err)
	}
	body := strings.TrimSpace(readyPost()) + "\n새 문장을 더했어요."
	first := voice.SampleMutation{UserID: "alice", VoiceID: sample.VoiceID, SampleID: sample.ID, ExpectedContentRevision: 1, OperationKey: "body", Body: &body}
	changed, err := h.svc.UpdateVoiceSample(ctx, first)
	if err != nil || changed.ContentRevision != 2 {
		t.Fatal("body edit did not increment once", err)
	}
	noOp := editBody(t, h, changed, "same-body", "  "+body+"  ")
	if noOp.ContentRevision != 2 {
		t.Fatal("equivalent prose changed revision")
	}
	later := editBody(t, h, noOp, "later-body", body+" 또 썼어요.")
	replay, err := h.svc.UpdateVoiceSample(ctx, first)
	if err != nil || replay.ContentRevision != changed.ContentRevision || replay.Body != changed.Body {
		t.Fatal("lost-response replay changed its result", err)
	}
	current, _, _ := h.svc.GetSample(ctx, "alice", sample.VoiceID, sample.ID)
	if current.ContentRevision != later.ContentRevision || current.Body != later.Body {
		t.Fatal("replay rewound later prose")
	}
	stale := first
	stale.OperationKey = "stale"
	if _, err := h.svc.UpdateVoiceSample(ctx, stale); !errors.Is(err, voice.ErrSampleRevisionConflict) {
		t.Fatal("stale edit accepted", err)
	}
	short := "짧아요"
	invalid := first
	invalid.ExpectedContentRevision = later.ContentRevision
	invalid.OperationKey = "short"
	invalid.Body = &short
	var tooShort *voice.SampleTooShortError
	if _, err := h.svc.UpdateVoiceSample(ctx, invalid); !errors.As(err, &tooShort) {
		t.Fatal("short paste edit accepted", err)
	}
	other, _ := h.svc.CreateVoice(ctx, "alice", "다른 말투")
	for _, scope := range [][2]string{{"bob", sample.VoiceID}, {"alice", other.ID}} {
		foreign := first
		foreign.UserID, foreign.VoiceID, foreign.OperationKey = scope[0], scope[1], "foreign"
		if _, err := h.svc.UpdateVoiceSample(ctx, foreign); !(errors.Is(err, voice.ErrVoiceNotFound) || errors.Is(err, voice.ErrSampleNotFound)) {
			t.Fatal("foreign material accepted", err)
		}
	}
	answer, err := h.svc.AnswerPrompt(ctx, "alice", sample.VoiceID, voice.Answer{PromptKey: "opening_greeting", Body: "안녕하세요!"})
	if err != nil {
		t.Fatal(err)
	}
	rewritten := editBody(t, h, answer, "saved-answer", "안녕하세요! 반가운 마음으로 인사해요.")
	if rewritten.ID != answer.ID || rewritten.PromptKey != answer.PromptKey || rewritten.Kind != answer.Kind || rewritten.ContentRevision != 2 {
		t.Fatal("saved answer changed identity")
	}
	if h.models.completeCalls != 0 || len(h.jobs.calls()) != 0 {
		t.Fatal("source editing started AI")
	}
}

func TestAcceptedProseSurvivesQueuedAndInflightEditsUntilExplicitReanalysis(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	voiceID := h.voice("alice")
	oldBody := readyPost() + "\n처음 받아들인 문장이에요."
	sample, err := h.svc.AddSample(ctx, "alice", voiceID, "원본", oldBody)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", voiceID, analyzeRef); err != nil {
		t.Fatal(err)
	}
	job := analysisJob(h.jobs.calls()[0])
	queued := editBody(t, h, sample, "queued-edit", readyPost()+"\n대기 중에 새로 쓴 문장이에요.")
	h.models.response = analysisAnswer("담담한 인상이에요.", "처음 받아들인 문장이에요.")
	if err := h.svc.Analyze(ctx, job, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(h.models.request.Messages[0].Parts[0].Text, "대기 중에 새로") || !strings.Contains(h.models.request.Messages[0].Parts[0].Text, "처음 받아들인") {
		t.Fatal("queued analysis replaced admitted prose")
	}
	current, _ := h.store.CurrentAnalysis(ctx, "alice", voiceID)
	if !current.SourceVersionsKnown || current.AcceptedSources[0].ContentRevision != 1 || current.AcceptedMaterials[0].Body != oldBody {
		t.Fatal("accepted source version was not stored")
	}
	profile, _ := h.svc.Get(ctx, "alice", voiceID)
	if profile.Notice.Kind != voice.NoticeChanged {
		t.Fatal("queued edit lost its pending marker")
	}
	projection, err := h.svc.PromptProfileForTopic(ctx, "alice", voiceID, "", voice.LanguageKorean, "")
	if err != nil || len(projection.Excerpts) != 1 || strings.Contains(projection.Excerpts[0], "대기 중") || len(projection.AcceptedSources) != 1 || projection.AcceptedSources[0].ContentRevision != 1 {
		t.Fatal("new prose entered the accepted projection", projection, err)
	}
	if err := h.svc.ValidateAcceptedSources(ctx, "alice", voiceID, projection.AcceptedSources); err != nil {
		t.Fatal("later edit invalidated present accepted input", err)
	}
	label := "설명만 수정"
	if _, err := h.svc.UpdateVoiceSample(ctx, voice.SampleMutation{UserID: "alice", VoiceID: voiceID, SampleID: sample.ID, ExpectedContentRevision: queued.ContentRevision, OperationKey: "metadata", Label: &label}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", voiceID, analyzeRef); err != nil {
		t.Fatal(err)
	}
	if err := h.svc.Analyze(ctx, analysisJob(h.jobs.calls()[1]), func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	profile, _ = h.svc.Get(ctx, "alice", voiceID)
	if profile.Notice.Kind != voice.NoticeNone {
		t.Fatal("reanalysis did not accept current revision")
	}
	currentSample, _, err := h.svc.GetSample(ctx, "alice", voiceID, sample.ID)
	if err != nil {
		t.Fatal(err)
	}
	metadataLabel := "분석 뒤 제목만 수정"
	metadataOnly, err := h.svc.UpdateVoiceSample(ctx, voice.SampleMutation{UserID: "alice", VoiceID: voiceID, SampleID: sample.ID, ExpectedContentRevision: currentSample.ContentRevision, OperationKey: "accepted-metadata", Label: &metadataLabel})
	if err != nil || metadataOnly.ContentRevision != currentSample.ContentRevision {
		t.Fatal("metadata changed accepted revision", err)
	}
	profile, _ = h.svc.Get(ctx, "alice", voiceID)
	if profile.Notice.Kind != voice.NoticeNone || h.models.completeCalls != 2 || len(h.jobs.calls()) != 2 {
		t.Fatal("metadata dirtied the profile or performed reanalysis")
	}
	restored, err := h.svc.RestorePreviousAnalysis(ctx, "alice", voiceID)
	if err != nil || restored.Notice.Kind != voice.NoticeChanged || restored.Analysis.AcceptedSources[0].ContentRevision != 1 {
		t.Fatal("restore treated unchanged ids as a fresh source", err)
	}
	if err := h.svc.DeleteSample(ctx, "alice", voiceID, sample.ID); err != nil {
		t.Fatal(err)
	}
	profile, _ = h.svc.Get(ctx, "alice", voiceID)
	if len(profile.Analysis.AI.Examples) != 0 {
		t.Fatal("withdrawn AI example survived")
	}
	projection, _ = h.svc.PromptProfileForTopic(ctx, "alice", voiceID, "", voice.LanguageKorean, "")
	if len(projection.Excerpts) != 0 || len(projection.AcceptedSources) != 0 {
		t.Fatal("withdrawn source survived projection")
	}
	if err := h.svc.ValidateAcceptedSources(ctx, "alice", voiceID, job.AcceptedSources); !errors.Is(err, voice.ErrAcceptedSourceWithdrawn) {
		t.Fatal("frozen source withdrawal was not fenced", err)
	}
}

func TestUnknownAcceptedLegacyProseIsUsableWithoutReconstruction(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id := h.voice("alice")
	h.addSample(t, "alice", id, "sample", "current", readyPost()+" 최신 개인 글이에요.", time.Now())
	if err := h.store.PublishAnalysis(ctx, "alice", id, voice.Analysis{MaterialIDs: []string{"sample"}, AI: voice.AIPart{Impression: "저장된 이전 인상"}, AnalyzeModel: analyzeRef.String(), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	profile, _ := h.svc.Get(ctx, "alice", id)
	projection, err := h.svc.PromptProfileForTopic(ctx, "alice", id, "", voice.LanguageKorean, "")
	if err != nil || profile.Analysis.SourceVersionsKnown || len(projection.Excerpts) != 0 || !strings.Contains(projection.Text, "저장된 이전 인상") || strings.Contains(projection.Text, "최신 개인 글") {
		t.Fatal("legacy source was reconstructed", projection, err)
	}
	if err := h.svc.Analyze(ctx, voice.AnalysisJob{UserID: "alice", VoiceID: id, WriteModel: analyzeRef.String(), MaterialIDs: []string{"sample"}}, func(string, int, int) {}); !errors.Is(err, voice.ErrAcceptedSourceWithdrawn) {
		t.Fatal("unprovable queued legacy snapshot reached provider", err)
	}
	if h.models.completeCalls != 0 {
		t.Fatal("legacy input performed a call")
	}
}

type sourceEstimate struct {
	inputs, outputs []int64
	credits         int
	available       bool
}

func (s *sourceEstimate) CallCredits(_ context.Context, _ llm.ModelInfo, input, output int64) (int, bool) {
	s.inputs = append(s.inputs, input)
	s.outputs = append(s.outputs, output)
	return s.credits, s.available
}

func TestAnalysisEstimateMatchesAdmittedPromptWithoutWork(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id := h.voice("alice")
	h.addSample(t, "alice", id, "ready", "ready", readyPost(), time.Now())
	if estimate, err := h.svc.EstimateAnalysis(ctx, "alice", id, analyzeRef); err != nil || estimate.Available {
		t.Fatal("missing estimator invented a quote", err)
	}
	estimator := &sourceEstimate{credits: 7, available: true}
	h.svc.WithAnalysisEstimates(estimator, 8192)
	quote, err := h.svc.EstimateAnalysis(ctx, "alice", id, analyzeRef)
	if err != nil || !quote.Available || quote.Free || quote.Credits != 7 || h.models.completeCalls != 0 || len(h.jobs.calls()) != 0 {
		t.Fatal("estimate performed work or lost price", quote, err)
	}
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", id, analyzeRef); err != nil {
		t.Fatal(err)
	}
	if estimator.inputs[0] != int64(h.jobs.calls()[0].PromptTokens) || estimator.outputs[0] != 8192 {
		t.Fatal("estimate and admission use different inputs")
	}
	if _, err := h.svc.EstimateAnalysis(ctx, "bob", id, analyzeRef); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatal("foreign estimate accepted", err)
	}
	if _, err := h.svc.EstimateAnalysis(ctx, "alice", id, writeOnlyRef); !errors.Is(err, voice.ErrAnalyzeModelRequired) {
		t.Fatal("wrong stage estimate accepted", err)
	}
}

func TestAnalysisPublishesFrozenVersionWhileOwnerEditsDuringCall(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id := h.voice("alice")
	sample, err := h.svc.AddSample(ctx, "alice", id, "old", readyPost()+" 처음 쓴 내용이에요.")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.AnalyzeVoice(ctx, "alice", id, analyzeRef); err != nil {
		t.Fatal(err)
	}
	job := analysisJob(h.jobs.calls()[0])
	models := &changingCorpusModels{started: make(chan struct{}), release: make(chan struct{})}
	svc := voice.NewService(h.store, models, h.jobs)
	done := make(chan error, 1)
	go func() { done <- svc.Analyze(ctx, job, func(string, int, int) {}) }()
	select {
	case <-models.started:
	case <-time.After(5 * time.Second):
		t.Fatal("provider did not start")
	}
	editBody(t, h, sample, "during-call", readyPost()+" 분석 중 새로 쓴 내용이에요.")
	close(models.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	profile, err := h.svc.Get(ctx, "alice", id)
	if err != nil || profile.Notice.Kind != voice.NoticeChanged || profile.Analysis.AcceptedMaterials[0].Body != job.AcceptedMaterials[0].Body || profile.Analysis.AcceptedSources[0].ContentRevision != 1 {
		t.Fatal("inflight edit replaced or falsely refreshed accepted input", err)
	}
	models.mu.Lock()
	defer models.mu.Unlock()
	if len(models.requests) != 1 || strings.Contains(models.requests[0], "분석 중 새로") {
		t.Fatal("analysis repeated or substituted current prose")
	}
}

func TestSavedPhotoAnswerEditingKeepsPromptAndAdoptsOwnedPhotoOnce(t *testing.T) {
	h := newVoiceHarness(t)
	objects := h.withPhotos()
	ctx := context.Background()
	id := h.voice("alice")
	first, _, err := h.svc.CreatePhotoUpload(ctx, "alice", id, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[first.Key] = 5000
	answer, err := h.svc.AnswerPrompt(ctx, "alice", id, voice.Answer{PromptKey: "photo_food", Body: "따뜻한 국물이 맛있어요.", UploadID: first.ID, PhotoWidth: 1024, PhotoHeight: 768})
	if err != nil {
		t.Fatal(err)
	}
	kept := editBody(t, h, answer, "photo-text", "국물이 따뜻하고 맛있어요.")
	if kept.PhotoKey != first.Key || kept.ID != answer.ID || kept.PromptKey != answer.PromptKey {
		t.Fatal("text editing detached its photo/prompt")
	}
	empty := ""
	if _, err := h.svc.UpdateVoiceSample(ctx, voice.SampleMutation{UserID: "alice", VoiceID: id, SampleID: answer.ID, ExpectedContentRevision: kept.ContentRevision, OperationKey: "remove-required-photo", PhotoUploadID: &empty}); !errors.Is(err, voice.ErrPhotoRequired) {
		t.Fatal("required prompt photo removed", err)
	}
	other, _, err := h.svc.CreatePhotoUpload(ctx, "bob", h.voice("bob"), "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[other.Key] = 5000
	width, height := 800, 600
	foreign := voice.SampleMutation{UserID: "alice", VoiceID: id, SampleID: answer.ID, ExpectedContentRevision: kept.ContentRevision, OperationKey: "foreign-photo", PhotoUploadID: &other.ID, PhotoWidth: &width, PhotoHeight: &height}
	if _, err := h.svc.UpdateVoiceSample(ctx, foreign); !errors.Is(err, voice.ErrPhotoRequired) {
		t.Fatal("foreign pending upload used", err)
	}
	second, _, err := h.svc.CreatePhotoUpload(ctx, "alice", id, "photo_food")
	if err != nil {
		t.Fatal(err)
	}
	objects.objects[second.Key] = 5000
	request := foreign
	request.OperationKey = "replace-photo"
	request.PhotoUploadID = &second.ID
	replaced, err := h.svc.UpdateVoiceSample(ctx, request)
	if err != nil || replaced.ID != answer.ID || replaced.PhotoKey != second.Key || replaced.ContentRevision != kept.ContentRevision+1 || len(objects.deleted) != 1 || objects.deleted[0] != first.Key {
		t.Fatal("photo replacement was not one identity/CAS", err)
	}
	replay, err := h.svc.UpdateVoiceSample(ctx, request)
	if err != nil || replay.PhotoKey != second.Key || replay.ContentRevision != replaced.ContentRevision || len(objects.deleted) != 1 {
		t.Fatal("lost response repeated photo adoption", err)
	}
	if h.models.completeCalls != 0 || len(h.jobs.calls()) != 0 {
		t.Fatal("photo editing performed AI work")
	}
}

func TestAcceptedAnalysisCodecRetainsExactVersionsAndLegacyUnknownInput(t *testing.T) {
	h := newVoiceHarness(t)
	id := h.voice("alice")
	h.addSample(t, "alice", id, "source", "body", readyPost(), time.Now())
	job := captureAnalysisJob(t, h, voice.AnalysisJob{UserID: "alice", VoiceID: id, MaterialIDs: []string{"source"}})
	raw, err := voice.EncodeAcceptedAnalysisSnapshot(job.AcceptedSources, job.AcceptedMaterials)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := voice.DecodeAcceptedAnalysisSnapshot(raw)
	if err != nil || len(decoded.AcceptedSources) != 1 || decoded.AcceptedSources[0] != job.AcceptedSources[0] || decoded.AcceptedMaterials[0] != job.AcceptedMaterials[0] {
		t.Fatal("snapshot round trip changed accepted input", err)
	}
	legacy, err := voice.EncodeAnalysisSnapshot([]string{"source"})
	if err != nil {
		t.Fatal(err)
	}
	unknown, err := voice.DecodeAcceptedAnalysisSnapshot(legacy)
	if err != nil || len(unknown.MaterialIDs) != 1 || len(unknown.AcceptedMaterials) != 0 {
		t.Fatal("legacy payload invented captured input", err)
	}
	job.AcceptedMaterials[0].Source.ContentRevision++
	if _, err := voice.EncodeAcceptedAnalysisSnapshot(job.AcceptedSources, job.AcceptedMaterials); !errors.Is(err, voice.ErrAcceptedSourceWithdrawn) {
		t.Fatal("mismatched snapshot accepted", err)
	}
}
