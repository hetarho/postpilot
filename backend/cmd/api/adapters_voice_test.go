package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/job"
	jobstore "github.com/postpilot/backend/internal/job/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/config"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// VOICE-62: every block a post can hold either reaches the fingerprint as its own prose type or,
// for IMAGE and VIDEO, is skipped with its caption.
func TestFingerprintBlocksKeepProseAndSkipMedia(t *testing.T) {
	kept := map[string]string{}
	for value, name := range postpilotv1.BlockType_name {
		if value == int32(postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED) {
			continue
		}
		block := post.Block{Type: post.BlockType(name), Content: "본문", Caption: "캡션", Items: []string{"하나", "둘"}}
		for _, got := range fingerprintBlocks([]post.Block{block}) {
			kept[name] = got.Type
		}
	}
	want := map[string]string{"TEXT": voice.BlockText, "HEADING": voice.BlockHeading, "LIST": voice.BlockList, "QUOTE": voice.BlockQuote}
	if len(kept) != len(want) {
		t.Fatalf("kept %v, want %v", kept, want)
	}
	for name, kind := range want {
		if kept[name] != kind {
			t.Fatalf("%s reached the fingerprint as %q", name, kept[name])
		}
	}
	list := fingerprintBlocks([]post.Block{{Type: post.BlockList, Items: []string{"하나", "둘"}}})
	if len(list) != 1 || len(list[0].Items) != 2 || list[0].Content != "" {
		t.Fatalf("a list reached the fingerprint as %+v", list)
	}
}

// POST-102: the adapter hands the voice context an owned post's voice, revision and prose blocks
// through post.CurrentContent, "" for 말투 없음 and no blocks before the first write; another
// account's post and an unknown one become the voice context's own sentinels.
func TestVoicePostsReadsAnOwnedPostsBlocks(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-posts.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	users := authstore.New(handle.Writer, handle.Reader)
	for _, id := range []string{"alice", "bob"} {
		if err := users.CreateUser(ctx, auth.User{ID: id, PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
			t.Fatal(err)
		}
	}
	if err := createTestVoice(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))
	aliceVoice, err := firstTestVoice(ctx, voiceSvc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	makeVoice(t, handle, "alice", aliceVoice.ID)
	language := post.LanguageKorean
	withVoice, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "국숫집", VoiceID: &aliceVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	noVoice, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "말투 없이", TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	adapter := voicePosts{service: postSvc}

	voiceID, revision, blocks, err := adapter.PostForFingerprint(ctx, "alice", withVoice.Slug)
	if err != nil || voiceID != aliceVoice.ID || revision != 0 || blocks != nil {
		t.Fatalf("before the first write = %q %d %v err=%v", voiceID, revision, blocks, err)
	}
	content := post.PostContent{Title: "국숫집", Blocks: []post.Block{
		{Type: post.BlockHeading, Content: "첫 방문", Level: 2},
		{Type: post.BlockText, Content: "국물이 진했다."},
		{Type: post.BlockQuote, Content: "또 올게요."},
		{Type: post.BlockList, Items: []string{"가격 9000원", "주차 가능"}},
	}}
	if err := postSvc.SetGeneratedContent(ctx, "alice", withVoice.Slug, content, post.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	voiceID, revision, blocks, err = adapter.PostForFingerprint(ctx, "alice", withVoice.Slug)
	if err != nil || voiceID != aliceVoice.ID || revision != 1 || len(blocks) != 4 || blocks[3].Type != voice.BlockList || len(blocks[3].Items) != 2 {
		t.Fatalf("after a write = %q %d %+v err=%v", voiceID, revision, blocks, err)
	}
	if voiceID, _, _, err := adapter.PostForFingerprint(ctx, "alice", noVoice.Slug); err != nil || voiceID != "" {
		t.Fatalf("말투 없음 = %q err=%v", voiceID, err)
	}
	if _, _, _, err := adapter.PostForFingerprint(ctx, "bob", withVoice.Slug); !errors.Is(err, voice.ErrPostForbidden) {
		t.Fatalf("a foreign post = %v", err)
	}
	if _, _, _, err := adapter.PostForFingerprint(ctx, "alice", "nope"); !errors.Is(err, voice.ErrPostNotFound) {
		t.Fatalf("an unknown post = %v", err)
	}
}

type capturedAdmission struct {
	starts []job.Start
	refuse error
}

func (a *capturedAdmission) Hold(_ context.Context, start job.Start) error {
	a.starts = append(a.starts, start)
	return a.refuse
}
func (a *capturedAdmission) Release(context.Context, string)             {}
func (a *capturedAdmission) Settle(context.Context, string, string)      {}
func (a *capturedAdmission) OpenHolds(context.Context) ([]string, error) { return nil, nil }

// QUOTA-13, VOICE-43: a 검증 passes the shared admission as one write call at the write stage's
// floor, one runs per voice at a time, and a refused admission leaves no job.
func TestAVoiceCheckIsAdmittedAsOneWriteCall(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "voice-check-jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := createTestVoice(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	found, err := firstTestVoice(ctx, voiceSvc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	queue := job.New(jobstore.New(handle.Writer, handle.Reader, jobKindsForTest()), time.Millisecond, jobReportingForTest())
	admission := &capturedAdmission{}
	queue.Admit(admission)
	budget := config.LLMCompletionBudget{WriteFloor: 8192, Ceiling: 32768}
	adapter := voiceJobs{queue: queue, budget: budget}
	request := voice.CheckJobRequest{UserID: "alice", VoiceID: found.ID, CheckID: "check-1", WriteModel: "stub/write"}

	id, err := adapter.EnqueueCheck(ctx, request)
	if err != nil || id == "" {
		t.Fatalf("enqueue = %q %v", id, err)
	}
	if len(admission.starts) != 1 {
		t.Fatalf("admissions = %+v", admission.starts)
	}
	start := admission.starts[0]
	if start.Kind != job.KindCheckVoice || len(start.Calls) != 1 || start.Calls[0] != (job.PlannedCall{Ref: "stub/write", Count: 1, CompletionTokens: 8192}) {
		t.Fatalf("admitted %+v", start)
	}
	var kind, payload string
	if err := handle.Reader.QueryRow(`SELECT kind, payload FROM generation_jobs WHERE id = ? AND voice_id = ?`, id, found.ID).Scan(&kind, &payload); err != nil || kind != job.KindCheckVoice || payload != "check-1" {
		t.Fatalf("the job = %s %q err=%v", kind, payload, err)
	}
	var active *voice.JobAlreadyInProgressError
	if _, err := adapter.EnqueueCheck(ctx, voice.CheckJobRequest{UserID: "alice", VoiceID: found.ID, CheckID: "check-2", WriteModel: "stub/write"}); !errors.As(err, &active) || active.ActiveID != id {
		t.Fatalf("a second 검증 of one voice = %v", err)
	}

	refused := errors.New("no credits")
	admission.refuse = refused
	other, err := voiceSvc.CreateVoice(ctx, "alice", "일상")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.EnqueueCheck(ctx, voice.CheckJobRequest{UserID: "alice", VoiceID: other.ID, CheckID: "check-3", WriteModel: "stub/write"}); !errors.Is(err, refused) {
		t.Fatalf("a refused admission = %v", err)
	}
	var jobs int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM generation_jobs WHERE voice_id = ?`, other.ID).Scan(&jobs); err != nil || jobs != 0 {
		t.Fatalf("a refused admission left %d jobs: %v", jobs, err)
	}
}
