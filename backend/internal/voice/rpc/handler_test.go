package rpc_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/postpilot/backend/internal/auth"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/voice"
	voicerpc "github.com/postpilot/backend/internal/voice/rpc"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type models struct{}

func (models) AnalyzeModel(context.Context, string) (llm.ModelRef, bool, error) {
	return llm.ModelRef{}, false, nil
}
func (models) Resolve(llm.ModelRef) (llm.ModelInfo, bool) { return llm.ModelInfo{}, false }
func (models) Complete(context.Context, llm.ModelRef, llm.Request) (llm.Response, error) {
	return llm.Response{}, nil
}

type jobs struct{}

func (jobs) Enqueue(context.Context, voice.AnalysisJobRequest) (string, error) { return "job", nil }
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
	_, err = handler.AddVoiceSample(
		auth.WithUser(context.Background(), "alice"),
		connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{
			VoiceId: voices["alice"],
			Body:    strings.Repeat("가", 200),
			Model:   &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "analyze"},
		}),
	)
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("missing model RPC error = %v", err)
	}
	if count, err := voicestore.New(handle.Writer, handle.Reader).CountSamples(context.Background(), "alice", voices["alice"]); err != nil || count != 0 {
		t.Fatalf("model failure stored %d samples: %v", count, err)
	}
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
		if _, err := store.PublishProfileVersion(context.Background(), "alice", id, voice.StructuredProfile{}, "analysis", 0, time.Now()); err != nil {
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
	if _, err := handler.AddVoiceSample(alice, connect.NewRequest(&postpilotv1.AddVoiceSampleRequest{VoiceId: defaultVoice.ID, Body: strings.Repeat("가", 200), Model: &postpilotv1.ModelRef{ProviderId: "stub", ModelId: "analyze"}})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
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

// The version preview's RPC: it is where the stored snapshot stops being opaque text and
// becomes the post content the client already decodes.
func TestGetVoiceProfileVersionSampleIsOwnedAndOptional(t *testing.T) {
	handle := openVoiceTestDB(t)
	service := voice.NewService(voicestore.New(handle.Writer, handle.Reader), models{}, jobs{})
	ctx := context.Background()
	voices := map[string]string{}
	for _, userID := range []string{"alice", "bob"} {
		created, err := service.CreateVoice(ctx, userID, "기본 말투")
		if err != nil {
			t.Fatal(err)
		}
		voices[userID] = created.ID
	}
	store := voicestore.New(handle.Writer, handle.Reader)
	head, err := store.PublishProfileVersion(ctx, "alice", voices["alice"], voice.StructuredProfile{Empty: false}, "analysis", 0, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := protojson.Marshal(&postpilotv1.PostContent{
		Title: "제주 여행기", Blocks: []*postpilotv1.Block{{Type: postpilotv1.BlockType_TEXT, Content: "비가 왔다"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RecordVersionSample(ctx, "alice", voices["alice"], head.Version, string(encoded)); err != nil {
		t.Fatal(err)
	}
	handler := voicerpc.NewHandler(service)

	got, err := handler.GetVoiceProfileVersionSample(
		auth.WithUser(ctx, "alice"),
		connect.NewRequest(&postpilotv1.GetVoiceProfileVersionSampleRequest{VoiceId: voices["alice"], Version: head.Version}),
	)
	if err != nil || got.Msg.GetSample().GetTitle() != "제주 여행기" || len(got.Msg.GetSample().GetBlocks()) != 1 {
		t.Fatalf("own snapshot = %+v err=%v", got.Msg, err)
	}
	if got.Msg.GetCreatedAt() == "" {
		t.Fatalf("snapshot carried no timestamp: %+v", got.Msg)
	}
	// A version that never produced a post answers with no sample rather than an error.
	absent, err := handler.GetVoiceProfileVersionSample(
		auth.WithUser(ctx, "alice"),
		connect.NewRequest(&postpilotv1.GetVoiceProfileVersionSampleRequest{VoiceId: voices["alice"], Version: head.Version + 5}),
	)
	if err != nil || absent.Msg.GetSample() != nil {
		t.Fatalf("absent snapshot = %+v err=%v", absent.Msg, err)
	}
	// A crafted voice id from another account is NotFound, never that account's snapshot.
	if _, err := handler.GetVoiceProfileVersionSample(
		auth.WithUser(ctx, "bob"),
		connect.NewRequest(&postpilotv1.GetVoiceProfileVersionSampleRequest{VoiceId: voices["alice"], Version: head.Version}),
	); connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("cross-account snapshot code = %v", err)
	}
	if _, err := handler.GetVoiceProfileVersionSample(
		ctx,
		connect.NewRequest(&postpilotv1.GetVoiceProfileVersionSampleRequest{VoiceId: voices["alice"], Version: head.Version}),
	); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("unauthenticated snapshot code = %v", err)
	}
}
