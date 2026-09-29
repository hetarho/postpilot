package rpc_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type models struct{}

// Resolve knows two models: one on the analyze stage, one on the write stage.
func (models) Resolve(ref llm.ModelRef) (llm.ModelInfo, bool) {
	if ref == (llm.ModelRef{ProviderID: "stub", ModelID: "analyze"}) {
		return llm.ModelInfo{Stages: []string{llm.StageNameAnalyze}}, true
	}
	if ref == (llm.ModelRef{ProviderID: "stub", ModelID: "write"}) {
		return llm.ModelInfo{Stages: []string{llm.StageNameWrite}}, true
	}
	return llm.ModelInfo{}, false
}
func (models) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	return llm.Response{}, nil
}

type jobs struct{}

func (jobs) Enqueue(context.Context, voice.AnalysisJobRequest) (string, error) { return "job", nil }
func (jobs) EnqueueCheck(context.Context, voice.CheckJobRequest) (string, error) {
	return "job-check", nil
}
func (jobs) ActiveForVoiceKind(context.Context, string, string) (*voice.ActiveJob, error) {
	return nil, nil
}
func (jobs) HasActiveForVoice(context.Context, string) (bool, error) { return false, nil }

// openVoiceTestDB opens a migrated database with both accounts already provisioned.
func openVoiceTestDB(t *testing.T) *db.DB {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-rpc.db"))
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
	return handle
}

func TestVoiceRPCIsScopedOnlyByAuthenticatedContext(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-rpc.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
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
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	voices := map[string]string{}
	for _, userID := range []string{"alice", "bob"} {
		created, err := service.CreateVoice(context.Background(), userID, "기본 말투")
		if err != nil {
			t.Fatal(err)
		}
		voices[userID] = created.ID
	}
	handler := voicerpc.NewHandler(service)
	for _, userID := range []string{"alice", "bob"} {
		response, err := handler.GetVoiceProfile(
			auth.WithUser(context.Background(), userID),
			connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{VoiceId: voices[userID]}),
		)
		if err != nil {
			t.Fatal(err)
		}
		// The profile no longer carries a styleguide or a rules string at all (VOICE-6):
		// what it reports is the structured profile, its samples and its voice.
		if got := response.Msg.GetProfile(); got.GetVoice().GetId() != voices[userID] {
			t.Fatalf("%s received foreign profile: %+v", userID, got)
		}
	}
	// A crafted voice id from another account is NotFound, never that account's profile.
	_, err = handler.GetVoiceProfile(
		auth.WithUser(context.Background(), "bob"),
		connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{VoiceId: voices["alice"]}),
	)
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign voice code = %v", err)
	}
	_, err = handler.GetVoiceProfile(
		auth.WithUser(context.Background(), "bob"),
		connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{}),
	)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing voice id code = %v", err)
	}

	_, err = handler.AddVoiceSample(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{VoiceId: voices["alice"], Body: strings.Repeat("가", 199)}),
	)
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("short sample RPC error = %v", err)
	}
	// A pasted post needs no model and starts nothing (VOICE-21).
	added, err := handler.AddVoiceSample(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{VoiceId: voices["alice"], Body: strings.Repeat("가", 200)}),
	)
	if err != nil || added.Msg.GetSample().GetKind() != postpilotv1.VoiceSampleKind_VOICE_SAMPLE_KIND_POST {
		t.Fatalf("add sample = %+v err=%v", added, err)
	}
}

// VOICE-60, VOICE-23: the prompt set, answering, opening a 학습 글 and 말투 만들기 at the edge.
func TestVoiceMaterialRPCs(t *testing.T) {
	handle := openVoiceTestDB(t)
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	ctx := context.Background()
	created, err := service.CreateVoice(ctx, "alice", "리뷰")
	if err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)
	alice := auth.WithUser(ctx, "alice")

	prompts, err := handler.ListVoicePrompts(alice, connect.NewRequest(&postpilotv1.ListVoicePromptsRequest{}))
	if err != nil || len(prompts.Msg.GetPrompts()) != 20 {
		t.Fatalf("prompts = %d err=%v", len(prompts.Msg.GetPrompts()), err)
	}
	photos := 0
	parts := map[postpilotv1.VoicePromptPart]int{}
	for _, prompt := range prompts.Msg.GetPrompts() {
		parts[prompt.GetPart()]++
		if prompt.GetPhoto() {
			photos++
		}
	}
	if photos != 6 || parts[postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_OPENING] != 4 || parts[postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_DESCRIPTION] != 12 || parts[postpilotv1.VoicePromptPart_VOICE_PROMPT_PART_CLOSING] != 4 {
		t.Fatalf("prompt parts = %v photos=%d", parts, photos)
	}

	for request, want := range map[*postpilotv1.AnswerVoicePromptRequest]connect.Code{
		{VoiceId: created.ID, PromptKey: "nope", Body: "안녕"}:            connect.CodeNotFound,
		{VoiceId: created.ID, PromptKey: "opening_greeting", Body: " "}: connect.CodeInvalidArgument,
		{VoiceId: created.ID, PromptKey: "photo_food", Body: "맛있어요"}:    connect.CodeFailedPrecondition,
	} {
		if _, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(request)); connect.CodeOf(err) != want {
			t.Fatalf("answer %+v code = %v, want %v", request, err, want)
		}
	}
	answered, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "opening_greeting", Body: "안녕하세요!"}))
	if err != nil || answered.Msg.GetSample().GetKind() != postpilotv1.VoiceSampleKind_VOICE_SAMPLE_KIND_ANSWER || answered.Msg.GetSample().GetPromptKey() != "opening_greeting" {
		t.Fatalf("answer = %+v err=%v", answered, err)
	}
	if _, err := handler.AnswerVoicePrompt(alice, connect.NewRequest(&postpilotv1.AnswerVoicePromptRequest{VoiceId: created.ID, PromptKey: "opening_greeting", Body: "또"})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("second answer code = %v", err)
	}
	opened, err := handler.GetVoiceSample(alice, connect.NewRequest(&postpilotv1.GetVoiceSampleRequest{VoiceId: created.ID, SampleId: answered.Msg.GetSample().GetId()}))
	if err != nil || opened.Msg.GetBody() != "안녕하세요!" || opened.Msg.GetPhotoUrl() != "" {
		t.Fatalf("opened = %+v err=%v", opened, err)
	}
	if _, err := handler.GetVoiceSample(auth.WithUser(ctx, "bob"), connect.NewRequest(&postpilotv1.GetVoiceSampleRequest{VoiceId: created.ID, SampleId: answered.Msg.GetSample().GetId()})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign open code = %v", err)
	}

	analyze := &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "analyze"}
	if _, err := handler.AnalyzeVoice(alice, connect.NewRequest(&postpilotv1.AnalyzeVoiceRequest{VoiceId: created.ID, Model: analyze})); voiceReason(t, err) != "VOICE_NOT_READY" {
		t.Fatalf("an unready analysis = %v", err)
	}
	if _, err := handler.AnalyzeVoice(alice, connect.NewRequest(&postpilotv1.AnalyzeVoiceRequest{VoiceId: created.ID, Model: &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "other"}})); voiceReason(t, err) != "VOICE_ANALYZE_MODEL_REQUIRED" {
		t.Fatalf("an unregistered model = %v", err)
	}
	profile, err := handler.GetVoiceProfile(alice, connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{VoiceId: created.ID}))
	if err != nil || profile.Msg.GetProfile().GetMade() || profile.Msg.GetProfile().GetReadiness().GetNeeded() != 60 {
		t.Fatalf("profile readiness = %+v err=%v", profile.Msg.GetProfile().GetReadiness(), err)
	}
}

func voiceReason(t *testing.T, err error) string {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		return ""
	}
	for _, detail := range connectErr.Details() {
		value, valueErr := detail.Value()
		if valueErr != nil {
			continue
		}
		if app, ok := value.(*postpilotv1.AppErrorDetail); ok {
			return app.GetReason()
		}
	}
	return ""
}

// The directory RPCs map every lifecycle refusal to a code the client can act on, and never
// let one account see or move another account's voices.
func TestVoiceDirectoryRPCCodes(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-directory.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	if err := db.Migrate(context.Background(), handle.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, userID := range []string{"alice", "bob"} {
		if _, err := handle.Writer.Exec("INSERT INTO users (id, password_hash, created_at) VALUES (?, 'hash', ?)", userID, now); err != nil {
			t.Fatal(err)
		}
	}
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	defaultVoice, err := service.CreateVoice(context.Background(), "alice", "기본 말투")
	if err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)
	alice := auth.WithUser(context.Background(), "alice")
	bob := auth.WithUser(context.Background(), "bob")

	if _, err := handler.CreateVoice(alice, connect.NewRequest(&postpilotv1.CreateVoiceRequest{Name: "  "})); connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("blank name code = %v", err)
	}
	created, err := handler.CreateVoice(alice, connect.NewRequest(&postpilotv1.CreateVoiceRequest{Name: "리뷰"}))
	if err != nil || created.Msg.GetVoice().GetName() != "리뷰" || created.Msg.GetVoice().GetIsDefault() {
		t.Fatalf("create = %+v err=%v", created, err)
	}
	if _, err := handler.CreateVoice(alice, connect.NewRequest(&postpilotv1.CreateVoiceRequest{Name: "리뷰"})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("duplicate name code = %v", err)
	}
	review := created.Msg.GetVoice().GetId()
	if _, err := handler.RenameVoice(bob, connect.NewRequest(&postpilotv1.RenameVoiceRequest{VoiceId: review, Name: "훔친 이름"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("foreign rename code = %v", err)
	}
	// A voice not yet made cannot be the 기본 (VOICE-32).
	if _, err := handler.SetDefaultVoice(alice, connect.NewRequest(&postpilotv1.SetDefaultVoiceRequest{VoiceId: review})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("unmade default code = %v", err)
	}
	store := voicestore.New(handle.Writer, handle.Reader)
	for _, id := range []string{review, defaultVoice.ID} {
		if err := store.PublishAnalysis(context.Background(), "alice", id, voice.Analysis{AnalyzeModel: "stub/analyze", CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := handler.SetDefaultVoice(alice, connect.NewRequest(&postpilotv1.SetDefaultVoiceRequest{VoiceId: defaultVoice.ID})); err != nil {
		t.Fatal(err)
	}
	swapped, err := handler.SetDefaultVoice(alice, connect.NewRequest(&postpilotv1.SetDefaultVoiceRequest{VoiceId: review}))
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, v := range swapped.Msg.GetVoices() {
		if v.GetIsDefault() {
			defaults++
		}
		if v.GetId() == review && (v.GetAnalyzedAt() == "" || !v.GetMade()) {
			t.Fatalf("a made voice lacks its analysis date: %+v", v)
		}
	}
	if defaults != 1 {
		t.Fatalf("defaults after swap = %d: %+v", defaults, swapped.Msg.GetVoices())
	}
	// An empty id clears the 기본 (VOICE-12).
	cleared, err := handler.SetDefaultVoice(alice, connect.NewRequest(&postpilotv1.SetDefaultVoiceRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range cleared.Msg.GetVoices() {
		if v.GetIsDefault() {
			t.Fatalf("a cleared directory still has a 기본: %+v", cleared.Msg.GetVoices())
		}
	}
	if _, err := handler.SetDefaultVoice(alice, connect.NewRequest(&postpilotv1.SetDefaultVoiceRequest{VoiceId: defaultVoice.ID})); err != nil {
		t.Fatal(err)
	}
	// The 기본 deletes like any other voice (VOICE-13).
	deleted, err := handler.DeleteVoice(alice, connect.NewRequest(&postpilotv1.DeleteVoiceRequest{VoiceId: defaultVoice.ID}))
	if err != nil || !deleted.Msg.GetVoice().GetDeleted() || deleted.Msg.GetVoice().GetDeletedAt() == "" || deleted.Msg.GetVoice().GetIsDefault() {
		t.Fatalf("delete = %+v err=%v", deleted, err)
	}
	if _, err := handler.AddVoiceSample(alice, connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{VoiceId: defaultVoice.ID, Body: strings.Repeat("가", 200)})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("mutate deleted code = %v", err)
	}
	listed, err := handler.ListVoices(alice, connect.NewRequest(&postpilotv1.ListVoicesRequest{}))
	if err != nil || len(listed.Msg.GetVoices()) != 2 || listed.Msg.GetVoices()[1].GetId() != defaultVoice.ID || !listed.Msg.GetVoices()[1].GetDeleted() {
		t.Fatalf("list = %+v err=%v", listed, err)
	}
	if _, err := handler.CreateVoice(alice, connect.NewRequest(&postpilotv1.CreateVoiceRequest{Name: "기본 말투"})); err != nil {
		t.Fatal(err)
	}
	if _, err := handler.RestoreVoice(alice, connect.NewRequest(&postpilotv1.RestoreVoiceRequest{VoiceId: defaultVoice.ID})); connect.CodeOf(err) != connect.CodeAlreadyExists {
		t.Fatalf("conflicting restore code = %v", err)
	}
	if _, err := handler.RenameVoice(alice, connect.NewRequest(&postpilotv1.RenameVoiceRequest{VoiceId: defaultVoice.ID, Name: "옛 기본"})); err != nil {
		t.Fatal(err)
	}
	restored, err := handler.RestoreVoice(alice, connect.NewRequest(&postpilotv1.RestoreVoiceRequest{VoiceId: defaultVoice.ID}))
	if err != nil || restored.Msg.GetVoice().GetDeleted() || restored.Msg.GetVoice().GetIsDefault() {
		t.Fatalf("restore = %+v err=%v", restored, err)
	}
	if bobList, err := handler.ListVoices(bob, connect.NewRequest(&postpilotv1.ListVoicesRequest{})); err != nil || len(bobList.Msg.GetVoices()) != 0 {
		t.Fatalf("bob sees alice's voices: %+v err=%v", bobList, err)
	}
}

// VOICE-30, VOICE-63: the profile carries the current analysis and whether a previous one
// exists, and 이전 분석으로 되돌리기 answers with the voice as it now reads — or refuses without a
// previous analysis.
func TestTheAnalysisAndItsUndoAtTheEdge(t *testing.T) {
	handle := openVoiceTestDB(t)
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	ctx := context.Background()
	created, err := service.CreateVoice(ctx, "alice", "리뷰")
	if err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)
	alice := auth.WithUser(ctx, "alice")
	if _, err := handler.RestorePreviousVoiceAnalysis(alice, connect.NewRequest(&postpilotv1.RestorePreviousVoiceAnalysisRequest{VoiceId: created.ID})); voiceReason(t, err) != "VOICE_NO_PREVIOUS_ANALYSIS" {
		t.Fatalf("an undo with nothing to return to = %v", err)
	}
	store := voicestore.New(handle.Writer, handle.Reader)
	for _, impression := range []string{"첫 분석", "다시 분석"} {
		if err := store.PublishAnalysis(ctx, "alice", created.ID, voice.Analysis{
			Counted:      voice.FingerprintOf([]voice.Material{{ID: "m", Kind: voice.SampleKindPost, Text: "안녕하세요!\n좋았어요."}}),
			AI:           voice.AIPart{Impression: impression},
			AnalyzeModel: "stub/analyze", CreatedAt: time.Now(),
		}); err != nil {
			t.Fatal(err)
		}
	}
	profile, err := handler.GetVoiceProfile(alice, connect.NewRequest(&postpilotv1.GetVoiceProfileRequest{VoiceId: created.ID}))
	got := profile.Msg.GetProfile()
	if err != nil || !got.GetMade() || !got.GetHasPrevious() || got.GetAnalysis().GetAi().GetImpression() != "다시 분석" {
		t.Fatalf("profile = %+v err=%v", got, err)
	}
	if !got.GetAnalysis().GetCounted().GetMarks().GetUnknown() || got.GetAnalysis().GetCounted().GetOpenings().GetOpenings()[0] != "안녕하세요!" {
		t.Fatalf("counted items = %+v", got.GetAnalysis().GetCounted())
	}
	restored, err := handler.RestorePreviousVoiceAnalysis(alice, connect.NewRequest(&postpilotv1.RestorePreviousVoiceAnalysisRequest{VoiceId: created.ID}))
	if err != nil || restored.Msg.GetProfile().GetAnalysis().GetAi().GetImpression() != "첫 분석" || restored.Msg.GetProfile().GetHasPrevious() {
		t.Fatalf("restored = %+v err=%v", restored, err)
	}
}

type posts map[string]string

// PostForFingerprint knows alice's posts: slug → voice id, each with the same prose.
func (p posts) PostForFingerprint(_ context.Context, userID, slug string) (string, int64, []voice.Block, error) {
	voiceID, ok := p[slug]
	switch {
	case !ok:
		return "", 0, nil, voice.ErrPostNotFound
	case userID != "alice":
		return "", 0, nil, voice.ErrPostForbidden
	}
	return voiceID, 5, []voice.Block{{Type: voice.BlockText, Content: strings.Repeat("국물이 진했다. ", 12)}}, nil
}

// VOICE-62, POST-102: GetPostFingerprint answers the comparison in the domain's order with each
// facet's unit and value, applicable=false for 말투 없음, PermissionDenied for another account's
// post and NotFound for an unknown one.
func TestGetPostFingerprintOverTheWire(t *testing.T) {
	handle := openVoiceTestDB(t)
	store := voicestore.New(handle.Writer, handle.Reader)
	service := voice.NewService(store, models{}, jobs{})
	ctx := auth.WithUser(context.Background(), "alice")
	created, err := service.CreateVoice(ctx, "alice", "리뷰")
	if err != nil {
		t.Fatal(err)
	}
	counted := voice.FingerprintOf([]voice.Material{{ID: "m1", Kind: voice.SampleKindPost, CreatedAt: time.Now(), Text: strings.Repeat("정말 맛있었어요! ", 12)}})
	if err := store.PublishAnalysis(ctx, "alice", created.ID, voice.Analysis{Counted: counted, AnalyzeModel: "stub/analyze", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	service.ConfigurePosts(posts{"made": created.ID, "no-voice": ""})
	handler := voicerpc.NewHandler(service)
	ask := func(ctx context.Context, slug string) (*postpilotv1.GetPostFingerprintResponse, error) {
		res, err := handler.GetPostFingerprint(ctx, connect.NewRequest(&postpilotv1.GetPostFingerprintRequest{PostSlug: slug}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	made, err := ask(ctx, "made")
	if err != nil || !made.GetApplicable() || made.GetRevision() != 5 || len(made.GetItems()) != len(voice.Items()) {
		t.Fatalf("made = %+v err=%v", made, err)
	}
	want := voice.Compare(counted, voice.MeasureBlocks([]voice.Block{{Type: voice.BlockText, Content: strings.Repeat("국물이 진했다. ", 12)}}))
	for i, item := range made.GetItems() {
		if item.GetItem() == postpilotv1.FingerprintItem_FINGERPRINT_ITEM_UNSPECIFIED || item.GetUnknown() != want[i].Unknown || item.GetHeadline() != want[i].Headline || len(item.GetFacets()) != len(want[i].Facets) {
			t.Fatalf("item %d = %+v, want %+v", i, item, want[i])
		}
		for j, facet := range item.GetFacets() {
			domain := want[i].Facets[j]
			if facet.GetKey() != domain.Key || facet.GetUnit() == postpilotv1.FingerprintFacetUnit_FINGERPRINT_FACET_UNIT_UNSPECIFIED {
				t.Fatalf("facet %s = %+v", domain.Key, facet)
			}
			if domain.Unit == voice.UnitText {
				if facet.GetVoice().GetTerms() == nil || strings.Join(facet.GetVoice().GetTerms().GetTerms(), ",") != strings.Join(domain.VoiceTerms, ",") {
					t.Fatalf("terms facet %s = %+v", domain.Key, facet)
				}
			} else if facet.GetVoice().GetNumber() != domain.Voice || facet.GetText().GetNumber() != domain.Text {
				t.Fatalf("number facet %s = %+v, want %+v", domain.Key, facet, domain)
			}
		}
	}
	if none, err := ask(ctx, "no-voice"); err != nil || none.GetApplicable() || len(none.GetItems()) != 0 {
		t.Fatalf("말투 없음 = %+v err=%v", none, err)
	}
	if _, err := ask(auth.WithUser(context.Background(), "bob"), "made"); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Fatalf("a foreign post = %v", err)
	}
	if _, err := ask(ctx, "nope"); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("an unknown post = %v", err)
	}
}

// VOICE-43: 검증 over the wire — the start answers the queued check with its prompt and answer
// and the job, the list carries it with the active job, a retry is a new check, and the
// refusals arrive as their reasons.
func TestVoiceChecksOverTheWire(t *testing.T) {
	handle := openVoiceTestDB(t)
	store := voicestore.New(handle.Writer, handle.Reader)
	service := voice.NewService(store, models{}, jobs{})
	ctx := auth.WithUser(context.Background(), "alice")
	created, err := service.CreateVoice(ctx, "alice", "리뷰")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertSample(ctx, voice.Sample{ID: "answer", UserID: "alice", VoiceID: created.ID, Kind: voice.SampleKindAnswer, PromptKey: "opening_greeting", Body: "안녕하세요, 동네 빵집이에요.", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.PublishAnalysis(ctx, "alice", created.ID, voice.Analysis{AnalyzeModel: "stub/analyze", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)
	write := &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "write"}

	started, err := handler.StartVoiceCheck(ctx, connect.NewRequest(&postpilotv1.StartVoiceCheckRequest{VoiceId: created.ID, PromptKey: "opening_greeting", Model: write}))
	if err != nil {
		t.Fatal(err)
	}
	check := started.Msg.GetCheck()
	if started.Msg.GetJobId() != "job-check" || check.GetStatus() != postpilotv1.VoiceCheckStatus_VOICE_CHECK_STATUS_QUEUED ||
		check.GetPrompt().GetKey() != "opening_greeting" || check.GetAnswer() != "안녕하세요, 동네 빵집이에요." || check.GetWriteModel().GetModelId() != "write" {
		t.Fatalf("started = %+v", started.Msg)
	}
	listed, err := handler.ListVoiceChecks(ctx, connect.NewRequest(&postpilotv1.ListVoiceChecksRequest{VoiceId: created.ID}))
	if err != nil || len(listed.Msg.GetChecks()) != 1 || listed.Msg.GetChecks()[0].GetId() != check.GetId() {
		t.Fatalf("listed = %+v err=%v", listed, err)
	}
	// The fake queue holds no job, so the waiting check reads as interrupted.
	if failure := listed.Msg.GetChecks()[0].GetFailure(); failure.GetReason() != "JOB_INTERRUPTED" {
		t.Fatalf("a check with no job = %+v", listed.Msg.GetChecks()[0])
	}
	retried, err := handler.RetryVoiceCheck(ctx, connect.NewRequest(&postpilotv1.RetryVoiceCheckRequest{CheckId: check.GetId(), Model: write}))
	if err != nil || retried.Msg.GetCheck().GetId() == check.GetId() {
		t.Fatalf("retried = %+v err=%v", retried, err)
	}
	for name, tc := range map[string]struct {
		call func() error
		code connect.Code
	}{
		"unanswered": {func() error {
			_, err := handler.StartVoiceCheck(ctx, connect.NewRequest(&postpilotv1.StartVoiceCheckRequest{VoiceId: created.ID, PromptKey: "closing_greeting", Model: write}))
			return err
		}, connect.CodeFailedPrecondition},
		"not a writer": {func() error {
			_, err := handler.StartVoiceCheck(ctx, connect.NewRequest(&postpilotv1.StartVoiceCheckRequest{VoiceId: created.ID, PromptKey: "opening_greeting", Model: &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "analyze"}}))
			return err
		}, connect.CodeFailedPrecondition},
		"foreign check": {func() error {
			_, err := handler.RetryVoiceCheck(auth.WithUser(context.Background(), "bob"), connect.NewRequest(&postpilotv1.RetryVoiceCheckRequest{CheckId: check.GetId(), Model: write}))
			return err
		}, connect.CodeNotFound},
		"foreign voice": {func() error {
			_, err := handler.ListVoiceChecks(auth.WithUser(context.Background(), "bob"), connect.NewRequest(&postpilotv1.ListVoiceChecksRequest{VoiceId: created.ID}))
			return err
		}, connect.CodeNotFound},
	} {
		if err := tc.call(); connect.CodeOf(err) != tc.code {
			t.Fatalf("%s = %v, want %v", name, err, tc.code)
		}
	}
}
