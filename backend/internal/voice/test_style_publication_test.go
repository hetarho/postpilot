package voice_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/voice"
)

func frozenTestStyle(t *testing.T) voice.Analysis {
	t.Helper()
	profile, err := (&voice.Authoring{}).PrepareWritingStyle(voice.WritingStyleDraft{Name: "고요한 말투", Description: "짧고 따뜻하게 작은 경험을 설명해요.", Sample: strings.Repeat("가상의 작은 가게에서 차를 마셨어요. 창가에서 쉬니 마음이 편안했어요. 고요한 시간을 천천히 즐겼어요. ", 5)}, "stub/write", time.Date(2026, 10, 7, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return profile
}
func testStyleInput(t *testing.T) voice.TestStylePublication {
	return voice.TestStylePublication{UserID: "alice", TestID: "test", WinnerID: "winner", Action: "save_setting", RequestKey: "request", Fingerprint: "frozen-test", Name: "고요한 새 말투", Analysis: frozenTestStyle(t), MakeDefault: true}
}
func TestTestedStylePublicationRetainsExactSyntheticProfileAndReceiptAcrossLaterEdits(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	publisher := voice.NewTestStylePublisher(h.store)
	in := testStyleInput(t)
	receipt, err := publisher.PublishTestWinner(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := h.store.CurrentAnalysis(ctx, "alice", receipt.VoiceID)
	if err != nil || saved == nil || voice.AcceptedAnalysisRevision(*saved) != voice.AcceptedAnalysisRevision(in.Analysis) {
		t.Fatalf("profile changed: %+v %v", saved, err)
	}
	found, err := h.store.GetVoice(ctx, "alice", receipt.VoiceID)
	if err != nil || found.Origin != voice.OriginSynthetic || !found.Made || !found.IsDefault || found.SampleCount != 0 {
		t.Fatalf("saved=%+v %v", found, err)
	}
	if err := h.store.RenameVoice(ctx, "alice", receipt.VoiceID, "나중에 정한 이름", time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := h.svc.SetDefaultVoice(ctx, "alice", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.SoftDeleteVoice(ctx, "alice", receipt.VoiceID, time.Now()); err != nil {
		t.Fatal(err)
	}
	in.RequestKey = "lost-original-key"
	replay, err := publisher.PublishTestWinner(ctx, in)
	if err != nil || replay != receipt {
		t.Fatalf("replay=%+v original=%+v err=%v", replay, receipt, err)
	}
	found, err = h.store.GetVoice(ctx, "alice", receipt.VoiceID)
	if err != nil || !found.Deleted() || found.IsDefault || found.Name != "나중에 정한 이름" {
		t.Fatalf("replay overwrote later edit %+v %v", found, err)
	}
	if retained, found, err := publisher.ReadTestPublicationReceipt(ctx, "alice", "test", "winner", "save_setting"); err != nil || !found || retained != receipt {
		t.Fatalf("payload-free voice receipt=%+v found=%v err=%v", retained, found, err)
	}
	for _, user := range []string{"bob", "missing"} {
		if _, found, err := publisher.ReadTestPublicationReceipt(ctx, user, "test", "winner", "save_setting"); err != nil || found {
			t.Fatalf("foreign voice receipt found=%v err=%v", found, err)
		}
	}
	if _, found, err := publisher.ReadTestPublicationReceipt(ctx, "alice", "test", "winner", "use_setting"); err != nil || found {
		t.Fatalf("different action receipt found=%v err=%v", found, err)
	}
	var n int
	if err := h.db.Reader.QueryRow("SELECT count(*) FROM voice_test_publications WHERE user_id='alice'").Scan(&n); err != nil || n != 1 {
		t.Fatalf("receipts=%d %v", n, err)
	}
}
func TestTestedStyleCopiesUseBoundedUniqueNamesAndAtomicConcurrentPublication(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	publisher := voice.NewTestStylePublisher(h.store)
	in := testStyleInput(t)
	in.Name = strings.Repeat("말", voice.VoiceNameMaxChars)
	if _, err := h.svc.CreateVoice(ctx, "alice", in.Name); err != nil {
		t.Fatal(err)
	}
	const workers = 8
	receipts := make(chan voice.TestStyleReceipt, workers)
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r, e := publisher.PublishTestWinner(ctx, in); receipts <- r; errs <- e }()
	}
	wg.Wait()
	close(receipts)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var receipt voice.TestStyleReceipt
	for r := range receipts {
		if receipt.VoiceID == "" {
			receipt = r
		}
		if r != receipt {
			t.Fatal("concurrent operation made another target")
		}
	}
	found, err := h.store.GetVoice(ctx, "alice", receipt.VoiceID)
	if err != nil || utf8.RuneCountInString(found.Name) > voice.VoiceNameMaxChars || !strings.HasSuffix(found.Name, " (2)") {
		t.Fatalf("name=%s %v", found.Name, err)
	}
	in.Name = "different copy choice"
	if _, err := publisher.PublishTestWinner(ctx, in); !errors.Is(err, voice.ErrTestStylePublicationConflict) {
		t.Fatalf("changed replay err=%v", err)
	}
}
func TestTestedStyleReceiptFailureRollsBackNewProfileAndDefault(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	publisher := voice.NewTestStylePublisher(h.store)
	if _, err := h.db.Writer.Exec("CREATE TRIGGER reject_style_receipt BEFORE INSERT ON voice_test_publications BEGIN SELECT RAISE(ABORT, 'receipt unavailable'); END"); err != nil {
		t.Fatal(err)
	}
	before, err := h.store.ListVoices(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := publisher.PublishTestWinner(ctx, testStyleInput(t)); err == nil {
		t.Fatal("failed receipt committed")
	}
	after, err := h.store.ListVoices(ctx, "alice")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("partial mutation before=%+v after=%+v %v", before, after, err)
	}
}
func personalTestProfile(t *testing.T, h *voiceHarness) (string, voice.Analysis) {
	t.Helper()
	ctx := context.Background()
	id := h.voices["alice"]
	at := time.Now().UTC()
	body := strings.Repeat("저는 조용한 골목을 걸으며 하루를 정리했어요. 직접 적은 문장에는 제 생각이 담겨 있어요. ", 6)
	sample := voice.Sample{ID: "personal-source", VoiceID: id, UserID: "alice", Kind: voice.SampleKindPost, Body: body, Chars: utf8.RuneCountInString(body), CreatedAt: at}
	if err := h.store.InsertSample(ctx, sample); err != nil {
		t.Fatal(err)
	}
	source := voice.AcceptedSource{SampleID: sample.ID, ContentRevision: 1}
	profile := voice.Analysis{Origin: voice.OriginPersonal, SourceVersionsKnown: true, MaterialIDs: []string{sample.ID}, AcceptedSources: []voice.AcceptedSource{source}, AcceptedMaterials: []voice.AcceptedMaterial{{Source: source, Body: body, Kind: sample.Kind, CreatedAt: at}}, Counted: voice.FingerprintOf([]voice.Material{{Text: body, Kind: sample.Kind}}), AI: voice.AIPart{Impression: "차분한 주인의 말투"}, AnalyzeModel: "stub/analyze", CreatedAt: at}
	if err := h.store.PublishAnalysis(ctx, "alice", id, profile); err != nil {
		t.Fatal(err)
	}
	saved, err := h.store.CurrentAnalysis(ctx, "alice", id)
	if err != nil || saved == nil {
		t.Fatal(err)
	}
	return id, *saved
}
func TestPersonalTestWinnerUsesOnlyOwnedAcceptedProfileWithoutCopyingOrSubstitutingEditedMaterial(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	id, profile := personalTestProfile(t, h)
	publisher := voice.NewTestStylePublisher(h.store)
	in := testStyleInput(t)
	in.Action = "use_setting"
	in.SourceVoiceID = id
	in.Analysis = profile
	in.AcceptedRevision = voice.AcceptedAnalysisRevision(profile)
	// Editing live prose must leave the accepted private snapshot unchanged until analysis.
	if _, err := h.db.Writer.Exec("UPDATE voice_samples SET body=?,content_revision=2 WHERE id='personal-source'", strings.Repeat("지금 새로 바꾼 글이에요. ", 30)); err != nil {
		t.Fatal(err)
	}
	if err := h.store.RenameVoice(ctx, "alice", id, "메타데이터만 수정", time.Now()); err != nil {
		t.Fatal(err)
	}
	receipt, err := publisher.PublishTestWinner(ctx, in)
	if err != nil || receipt.VoiceID != id {
		t.Fatalf("accepted use=%+v %v", receipt, err)
	}
	saved, err := h.store.CurrentAnalysis(ctx, "alice", id)
	if err != nil || voice.AcceptedAnalysisRevision(*saved) != in.AcceptedRevision || saved.AcceptedMaterials[0].Body != profile.AcceptedMaterials[0].Body {
		t.Fatalf("substituted edited source %+v %v", saved, err)
	}
	in.Action = "save_setting"
	in.RequestKey = "copy-personal"
	if _, err := publisher.PublishTestWinner(ctx, in); !errors.Is(err, voice.ErrTestStylePublicationConflict) {
		t.Fatalf("personal copied %v", err)
	}
}
func TestPersonalWinnerPublicationRefusesChangedDeletedWithdrawnOrForeignSources(t *testing.T) {
	for _, scenario := range []string{"changed-profile", "deleted-voice", "withdrawn-sample", "foreign-voice"} {
		t.Run(scenario, func(t *testing.T) {
			h := newVoiceHarness(t)
			ctx := context.Background()
			id, profile := personalTestProfile(t, h)
			publisher := voice.NewTestStylePublisher(h.store)
			in := testStyleInput(t)
			in.Action = "use_setting"
			in.SourceVoiceID = id
			in.Analysis = profile
			in.AcceptedRevision = voice.AcceptedAnalysisRevision(profile)
			switch scenario {
			case "changed-profile":
				profile.AI.Impression = "새로 분석한 다른 인상"
				if err := h.store.PublishAnalysis(ctx, "alice", id, profile); err != nil {
					t.Fatal(err)
				}
			case "deleted-voice":
				if _, err := h.store.SoftDeleteVoice(ctx, "alice", id, time.Now()); err != nil {
					t.Fatal(err)
				}
			case "withdrawn-sample":
				if _, _, err := h.store.DeleteSample(ctx, "alice", id, "personal-source", time.Now()); err != nil {
					t.Fatal(err)
				}
			case "foreign-voice":
				in.UserID = "bob"
			}
			if _, err := publisher.PublishTestWinner(ctx, in); !errors.Is(err, voice.ErrTestStylePublicationConflict) {
				t.Fatalf("%s accepted: %v", scenario, err)
			}
			var n int
			if err := h.db.Reader.QueryRow("SELECT count(*) FROM voice_test_publications").Scan(&n); err != nil || n != 0 {
				t.Fatalf("unexpected receipt %d %v", n, err)
			}
		})
	}
}
