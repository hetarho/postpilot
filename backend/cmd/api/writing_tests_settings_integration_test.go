package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/authoring"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/template"
	templatestore "github.com/postpilot/backend/internal/template/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

func writingSettingPlan(h *writingIntegrationHarness, factor v1.WritingTestFactor, kind v1.ConfigurationKind, ids, versions []string) *v1.WritingTestPlan {
	plan := h.plan(2)
	plan.Factor = factor
	plan.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_UNSPECIFIED
	plan.Context.TargetLanguage = v1.ContentLanguage_CONTENT_LANGUAGE_KOREAN
	plan.Context.WriteModel = &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}
	plan.Entrants = nil
	for i, id := range ids {
		plan.Entrants = append(plan.Entrants, &v1.WritingTestEntrant{Source: &v1.WritingTestEntrant_Setting{Setting: &v1.WritingTestSettingRef{Kind: kind, Id: id, Revision: versions[i]}}})
	}
	return plan
}
func runWritingSettingTournament(t *testing.T, h *writingIntegrationHarness, plan *v1.WritingTestPlan) *v1.WritingTest {
	t.Helper()
	quote, err := h.client.EstimateWritingTest(t.Context(), writingRPCRequest(h, &v1.EstimateWritingTestRequest{Plan: plan}))
	if err != nil {
		t.Fatal(err)
	}
	started, err := h.client.StartWritingTest(t.Context(), writingRPCRequest(h, &v1.StartWritingTestRequest{Plan: plan, QuoteKey: quote.Msg.QuoteKey, RequestKey: "start-setting"}))
	if err != nil {
		t.Fatal(err)
	}
	current := started.Msg.Test
	for deadline := time.Now().Add(30 * time.Second); current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		current = h.get(t, current.Id)
		if current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_FAILED || current.Status == v1.WritingTestStatus_WRITING_TEST_STATUS_PARTIAL {
			t.Fatalf("setting generation failed=%+v", current.Failure)
		}
	}
	if current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_REVIEW || len(current.Candidates) != 2 || len(current.Matches) != 1 {
		t.Fatalf("all-success pair missing=%+v", current)
	}
	for _, candidate := range current.Candidates {
		if candidate.Identity != nil || candidate.Usage != nil || candidate.Output == nil || candidate.Storyline == nil {
			t.Fatalf("blind full candidate missing=%+v", candidate)
		}
	}
	match := current.Matches[0]
	// Terminal ledger confirmation may advance the public revision after the
	// all-success barrier. A stale vote is refused; reload the same pair/key.
	for attempt := 0; attempt < 4; attempt++ {
		picked, err := h.client.DecideTestMatch(t.Context(), writingRPCRequest(h, &v1.DecideTestMatchRequest{TestId: current.Id, ExpectedRevision: current.Revision, RequestKey: "human-setting-winner", MatchId: match.Id, WinnerCandidateId: match.LeftCandidateId}))
		if err == nil {
			current = picked.Msg.Test
			break
		}
		if connect.CodeOf(err) != connect.CodeAborted {
			t.Fatal(err)
		}
		current = h.get(t, current.Id)
	}
	if current.Status != v1.WritingTestStatus_WRITING_TEST_STATUS_COMPLETED || !current.Revealed || current.WinnerCandidateId == "" {
		t.Fatalf("champion missing=%+v", current)
	}
	return current
}
func writingWinner(t *testing.T, test *v1.WritingTest) *v1.WritingTestCandidate {
	t.Helper()
	for _, candidate := range test.Candidates {
		if candidate.Id == test.WinnerCandidateId {
			if candidate.Identity == nil {
				t.Fatal("completed winner did not reveal frozen source")
			}
			return candidate
		}
	}
	t.Fatal("winner missing from stored outputs")
	return nil
}
func assertSettingTournamentAccounting(t *testing.T, h *writingIntegrationHarness) {
	t.Helper()
	h.provider.mu.Lock()
	calls := len(h.provider.requests)
	h.provider.mu.Unlock()
	var admissions, events int
	if err := h.platform.db.Reader.QueryRow("SELECT count(*) FROM usage_admissions").Scan(&admissions); err != nil {
		t.Fatal(err)
	}
	if err := h.platform.db.Reader.QueryRow("SELECT count(*) FROM usage_events").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || events != 2 || admissions != 1 {
		t.Fatalf("setting tournament/publication calls=%d events=%d holds=%d", calls, events, admissions)
	}
}

func TestWritingTestsProductionTransportPublishesExactTemplateWinnerNumbersAndPreservesLaterEdits(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	store := templatestore.New(h.platform.db.Writer, h.platform.db.Reader)
	authoring := template.NewAuthoring(h.app.template, store)
	var ids, versions []string
	for i := 0; i < 2; i++ {
		length, tags := 4100+i*1000, 7+i
		saved, err := h.app.template.Create(t.Context(), "alice", template.Authored{Name: fmt.Sprintf("Source template %d", i), Body: fmt.Sprintf(`<ask label="visit" required="true">What did you observe?</ask><write>Structure %d</write>`, i), Numbers: template.Numbers{TargetLength: &length, TagCount: &tags}})
		if err != nil {
			t.Fatal(err)
		}
		_, _, version, err := authoring.SeedWithNumbers(t.Context(), "alice", saved.ID)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, saved.ID)
		versions = append(versions, version)
	}
	plan := writingSettingPlan(h, v1.WritingTestFactor_WRITING_TEST_FACTOR_TEMPLATE, v1.ConfigurationKind_CONFIGURATION_KIND_POST_TEMPLATE, ids, versions)
	plan.Context.Material.TemplateAnswers = []*v1.TemplateAnswer{{Label: "visit", Text: "Explicit owner-approved facts shared by both contenders", Enabled: true}}
	current := runWritingSettingTournament(t, h, plan)
	winner := writingWinner(t, current)
	sourceID := winner.Identity.Source.GetSetting().Id
	expectedDraft, expectedNumbers, _, err := authoring.SeedWithNumbers(t.Context(), "alice", sourceID)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.SaveWritingTestWinnerRequest{TestId: current.Id, ExpectedRevision: current.Revision, WinnerCandidateId: current.WinnerCandidateId, RequestKey: "save-tested-template", Action: v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_SAVE_SETTING, Name: "Named tested template"}
	saved, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil {
		t.Fatal(err)
	}
	actual, numbers, _, err := authoring.SeedWithNumbers(t.Context(), "alice", saved.Msg.Publication.TargetId)
	if err != nil || actual.Body != expectedDraft.Body || !reflect.DeepEqual(numbers, expectedNumbers) || *numbers.TargetLength == int(plan.Context.TargetLength) || *numbers.TagCount == int(plan.Context.TagCount) {
		t.Fatalf("tested structure/numbers changed=%+v/%+v err=%v", actual, numbers, err)
	}
	later := "Later manual template name"
	if _, err := h.app.template.Update(t.Context(), "alice", saved.Msg.Publication.TargetId, template.Patch{Name: &later}); err != nil {
		t.Fatal(err)
	}
	replay, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil || replay.Msg.Publication.TargetId != saved.Msg.Publication.TargetId {
		t.Fatalf("template publication replay=%v", err)
	}
	actual, _, _, _ = authoring.SeedWithNumbers(t.Context(), "alice", saved.Msg.Publication.TargetId)
	if actual.Name != later {
		t.Fatal("receipt replay reversed later template edit")
	}
	assertSettingTournamentAccounting(t, h)
}

func TestWritingTestsProductionTransportPublishesExplicitGuidelineScopeWithoutRescopingSources(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	store := guidelinestore.New(h.platform.db.Writer, h.platform.db.Reader)
	authoring := guideline.NewAuthoring(h.app.guideline, store)
	global, err := h.app.guideline.Create(t.Context(), "alice", guideline.KindPost, "Global source", "Use the explicit owner facts", guideline.ScopePatch{Scope: guideline.ScopeGlobal}, "")
	if err != nil {
		t.Fatal(err)
	}
	field, err := h.app.guideline.Create(t.Context(), "alice", guideline.KindPost, "Field source", "Use short clear transitions", guideline.ScopePatch{Scope: guideline.ScopeFields, Fields: []string{"restaurant"}}, "")
	if err != nil {
		t.Fatal(err)
	}
	var ids, versions []string
	for _, source := range []guideline.Guideline{global, field} {
		_, _, version, err := authoring.SeedWithScope(t.Context(), "alice", guideline.KindPost, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, source.ID)
		versions = append(versions, version)
	}
	plan := writingSettingPlan(h, v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE, v1.ConfigurationKind_CONFIGURATION_KIND_POST_GUIDELINE, ids, versions)
	plan.Context.GuidelineSlotId = global.ID
	current := runWritingSettingTournament(t, h, plan)
	winner := writingWinner(t, current)
	sourceID := winner.Identity.Source.GetSetting().Id
	expected, expectedScope, _, err := authoring.SeedWithScope(t.Context(), "alice", guideline.KindPost, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	request := &v1.SaveWritingTestWinnerRequest{TestId: current.Id, ExpectedRevision: current.Revision, WinnerCandidateId: current.WinnerCandidateId, RequestKey: "use-tested-guideline", Action: v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_USE_SETTING}
	if _, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request)); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("implicit guideline scope admitted=%v", err)
	}
	request.Scope = string(expectedScope.Scope)
	request.ScopeIds = append(append([]string(nil), expectedScope.TemplateIDs...), expectedScope.Fields...)
	saved, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil {
		t.Fatal(err)
	}
	actual, scope, _, err := authoring.SeedWithScope(t.Context(), "alice", guideline.KindPost, saved.Msg.Publication.TargetId)
	if err != nil || actual.Body != expected.Body || !reflect.DeepEqual(scope, expectedScope) || saved.Msg.Publication.TargetId != sourceID {
		t.Fatalf("published guideline=%+v/%+v err=%v", actual, scope, err)
	}
	_, originalScope, _, _ := authoring.SeedWithScope(t.Context(), "alice", guideline.KindPost, field.ID)
	if originalScope.Scope != guideline.ScopeFields || !reflect.DeepEqual(originalScope.Fields, []string{"restaurant"}) {
		t.Fatal("champion save rescaled original scope")
	}
	later := "Later manual guideline text"
	if _, err := h.app.guideline.Update(t.Context(), "alice", saved.Msg.Publication.TargetId, guideline.Patch{Text: &later}); err != nil {
		t.Fatal(err)
	}
	replay, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil || replay.Msg.Publication.TargetId != saved.Msg.Publication.TargetId {
		t.Fatalf("guideline publication replay=%v", err)
	}
	actual, _, _, _ = authoring.SeedWithScope(t.Context(), "alice", guideline.KindPost, saved.Msg.Publication.TargetId)
	if actual.Body != later {
		t.Fatal("receipt replay reversed later guideline edit")
	}
	assertSettingTournamentAccounting(t, h)
}

func TestWritingTestsProductionTransportKeepsSavedVoiceWinnerSyntheticAndDefaultsExplicit(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	store := voicestore.New(h.platform.db.Writer, h.platform.db.Reader)
	publisher := voice.NewTestStylePublisher(store)
	var ids, versions []string
	for i := 0; i < 2; i++ {
		draft := voice.WritingStyleDraft{Name: []string{"차분한 말투", "경쾌한 말투"}[i], Description: []string{"짧고 차분하게 하루를 이야기해요.", "밝고 경쾌하게 작은 경험을 이야기해요."}[i], Sample: strings.Repeat([]string{"가상의 가게에서 차를 마셨어요. 창가에서 쉬니 마음이 편안했어요. 조용히 하루를 정리했어요. ", "가상의 가게에서 차를 마셨어요! 창가에 앉으니 즐거운 마음이 들었어요! 작은 시간을 밝게 이야기했어요! "}[i], 5)}
		analysis, err := voice.BuildSyntheticAnalysis(draft, "fixture/writer-0", time.Now())
		if err != nil {
			t.Fatal(err)
		}
		receipt, err := publisher.PublishTestWinner(t.Context(), voice.TestStylePublication{UserID: "alice", TestID: fmt.Sprintf("seed-%d", i), WinnerID: "seed", Action: "save_setting", RequestKey: fmt.Sprintf("seed-voice-%d", i), Fingerprint: fmt.Sprintf("seed-%d", i), Name: draft.Name, Analysis: analysis, MakeDefault: i == 0})
		if err != nil {
			t.Fatal(err)
		}
		frozen, err := h.app.voice.FreezeTestProfile(t.Context(), "alice", receipt.VoiceID, "", voice.LanguageKorean, "")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, receipt.VoiceID)
		versions = append(versions, frozen.Revision)
	}
	plan := writingSettingPlan(h, v1.WritingTestFactor_WRITING_TEST_FACTOR_VOICE, v1.ConfigurationKind_CONFIGURATION_KIND_WRITING_VOICE, ids, versions)
	current := runWritingSettingTournament(t, h, plan)
	winner := writingWinner(t, current)
	sourceID := winner.Identity.Source.GetSetting().Id
	source, err := h.app.voice.FreezeTestProfile(t.Context(), "alice", sourceID, "", voice.LanguageKorean, "")
	if err != nil {
		t.Fatal(err)
	}
	before, err := h.app.voice.ListVoices(t.Context(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range before {
		if v.IsDefault != (v.ID == ids[0]) {
			t.Fatal("champion reveal changed a default")
		}
	}
	request := &v1.SaveWritingTestWinnerRequest{TestId: current.Id, ExpectedRevision: current.Revision, WinnerCandidateId: current.WinnerCandidateId, RequestKey: "save-tested-voice", Action: v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_SAVE_SETTING, Name: "새로 저장한 말투", MakeDefault: true}
	saved, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := h.app.voice.FreezeTestProfile(t.Context(), "alice", saved.Msg.Publication.TargetId, "", voice.LanguageKorean, "")
	if err != nil || actual.Revision != source.Revision || actual.Analysis.Origin != voice.OriginSynthetic || actual.Analysis.SyntheticSample != source.Analysis.SyntheticSample || len(actual.Analysis.AcceptedMaterials) != 0 || !actual.Voice.IsDefault {
		t.Fatalf("synthetic provenance/default lost=%+v err=%v", actual, err)
	}
	var samples int
	if err := h.platform.db.Reader.QueryRow("SELECT count(*) FROM voice_samples WHERE user_id='alice'").Scan(&samples); err != nil || samples != 0 {
		t.Fatalf("style winner created personal materials=%d err=%v", samples, err)
	}
	if _, err := h.app.voice.SetDefaultVoice(t.Context(), "alice", ids[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := h.app.voice.RenameVoice(t.Context(), "alice", saved.Msg.Publication.TargetId, "나중에 바꾼 이름"); err != nil {
		t.Fatal(err)
	}
	replay, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil || replay.Msg.Publication.TargetId != saved.Msg.Publication.TargetId {
		t.Fatalf("voice publication replay=%v", err)
	}
	actual, err = h.app.voice.FreezeTestProfile(t.Context(), "alice", saved.Msg.Publication.TargetId, "", voice.LanguageKorean, "")
	if err != nil || actual.Voice.IsDefault || actual.Voice.Name != "나중에 바꾼 이름" {
		t.Fatalf("receipt replay reversed later voice choices=%+v err=%v", actual.Voice, err)
	}
	assertSettingTournamentAccounting(t, h)
}
func TestWritingTestsProductionTransportSavesUnsavedPreparedGuidelineWithExplicitScope(t *testing.T) {
	h := newWritingIntegrationHarness(t)
	plan := h.plan(2)
	plan.Factor = v1.WritingTestFactor_WRITING_TEST_FACTOR_GUIDELINE
	plan.ModelStage = v1.WritingTestStage_WRITING_TEST_STAGE_UNSPECIFIED
	plan.Context.WriteModel = &v1.ModelRef{ProviderId: "fixture", ModelId: "writer-0"}
	plan.Entrants = nil
	expected := map[string]authoring.Artifact{}
	for i := 0; i < 2; i++ {
		session, err := h.app.authoring.Create(t.Context(), "alice", authoring.PostGuideline, "", fmt.Sprintf("prepared-rule-%d", i))
		if err != nil {
			t.Fatal(err)
		}
		source := *session.WorkingSource
		source.Name = fmt.Sprintf("Private prepared rule %d", i)
		source.Body = []string{"Write the owner-supplied facts plainly", "Explain uncertainty in the supplied material"}[i]
		updated, err := h.app.authoring.PatchDraft(t.Context(), authoring.DraftMutation{UserID: "alice", SessionID: session.ID, OperationKey: fmt.Sprintf("patch-rule-%d", i), ExpectedRevision: session.Revision, WorkingSource: source})
		if err != nil {
			t.Fatal(err)
		}
		artifact := updated.WorkingSource
		if artifact == nil || updated.DraftState != authoring.DraftValid {
			t.Fatalf("valid current prepared source missing=%+v", updated)
		}
		expected[artifact.ID] = *artifact
		plan.Entrants = append(plan.Entrants, &v1.WritingTestEntrant{Source: &v1.WritingTestEntrant_AuthoringCandidate{AuthoringCandidate: &v1.WritingTestAuthoringRef{SessionId: updated.ID, CandidateId: artifact.ID, Revision: artifact.Revision}}})
	}
	current := runWritingSettingTournament(t, h, plan)
	winner := writingWinner(t, current)
	ref := winner.Identity.Source.GetAuthoringCandidate()
	if ref == nil || !winner.Identity.Synthetic {
		t.Fatalf("unpublished contender provenance lost=%+v", winner.Identity)
	}
	request := &v1.SaveWritingTestWinnerRequest{TestId: current.Id, ExpectedRevision: current.Revision, WinnerCandidateId: current.WinnerCandidateId, RequestKey: "save-prepared-guideline", Action: v1.WritingTestPublicationAction_WRITING_TEST_PUBLICATION_ACTION_SAVE_SETTING, Name: "Named prepared winner", Scope: "fields", ScopeIds: []string{"restaurant"}}
	saved, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil {
		t.Fatal(err)
	}
	owner := guideline.NewAuthoring(h.app.guideline, guidelinestore.New(h.platform.db.Writer, h.platform.db.Reader))
	draft, scope, _, err := owner.SeedWithScope(t.Context(), "alice", guideline.KindPost, saved.Msg.Publication.TargetId)
	if err != nil || draft.Body != expected[ref.CandidateId].Body || scope.Scope != guideline.ScopeFields || !reflect.DeepEqual(scope.Fields, []string{"restaurant"}) {
		t.Fatalf("prepared winner/scoped save lost=%+v/%+v err=%v", draft, scope, err)
	}
	session, err := h.app.authoring.Get(t.Context(), "alice", ref.SessionId)
	if err != nil || session.Saved != nil {
		t.Fatalf("test implicitly published or rewrote authoring session=%+v err=%v", session.Saved, err)
	}
	replay, err := h.client.SaveWritingTestWinner(t.Context(), writingRPCRequest(h, request))
	if err != nil || replay.Msg.Publication.TargetId != saved.Msg.Publication.TargetId {
		t.Fatalf("prepared publication receipt replay=%v", err)
	}
	assertSettingTournamentAccounting(t, h)
}
