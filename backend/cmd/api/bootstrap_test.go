package main

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/purpose"
	purposestore "github.com/postpilot/backend/internal/purpose/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

type noBlobs struct{}

func (noBlobs) PresignPut(context.Context, string, string, time.Duration) (string, error) {
	return "", nil
}
func (noBlobs) PresignGet(context.Context, string, time.Duration) (string, error) { return "", nil }
func (noBlobs) Head(context.Context, string) (int64, error)                       { return 0, post.ErrObjectNotFound }
func (noBlobs) Delete(context.Context, string) error                              { return nil }
func (noBlobs) List(context.Context, string) ([]post.Object, error)               { return nil, nil }

// Plan 10 A2: a new account cannot create a post until the adduser bootstrap has given it
// an active default voice, and rerunning the bootstrap never duplicates that voice.
func TestAccountBootstrapPrecedesPostCreation(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "bootstrap.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})

	if _, err := voiceSvc.DefaultVoice(ctx, "alice"); !errors.Is(err, voice.ErrVoiceNotFound) {
		t.Fatalf("default before bootstrap = %v", err)
	}
	guess := "any"
	if _, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &guess, nil); !errors.Is(err, post.ErrVoiceNotFound) {
		t.Fatalf("post before bootstrap = %v", err)
	}
	for range 2 {
		if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
			t.Fatal(err)
		}
	}
	voices, err := voiceSvc.ListVoices(ctx, "alice")
	if err != nil || len(voices) != 1 || !voices[0].IsDefault || voices[0].Name != voice.DefaultVoiceName {
		t.Fatalf("voices after two bootstraps = %+v err=%v", voices, err)
	}
	created, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &voices[0].ID, nil)
	if err != nil || created.VoiceID != voices[0].ID || created.Voice.Name != voice.DefaultVoiceName {
		t.Fatalf("post after bootstrap = %+v err=%v", created, err)
	}
}

// Plan 11 A4/A7: the composition root is the ONLY place the post's purpose id crosses into
// generation, and every prompt hangs off it. This walks the real adapters end to end — a post
// saved with a 용도, read back through generationPosts, resolved through generationPurposes —
// because both contexts' own tests inject the id into a fake and so cannot see it dropped here.
func TestGenerationAdapterCarriesThePostPurposeThroughToTheFrozenBrief(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "purpose.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}

	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, time.Minute, time.Minute, 1<<20)
	postSvc.SetVoiceDirectory(postVoices{service: voiceSvc})
	purposeSvc := purpose.NewService(
		purposestore.New(handle.Writer, handle.Reader),
		purpose.Limits{NameMaxChars: 40, DescriptionMaxChars: 200, InstructionsMaxChars: 2000},
	)
	postSvc.SetPurposeDirectory(postPurposes{service: purposeSvc})

	created, err := purposeSvc.Create(ctx, "alice", "정보성 식당 리뷰", "협찬 방문 리뷰", "사진마다 설명하세요")
	if err != nil {
		t.Fatal(err)
	}
	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := postSvc.SaveDraft(ctx, "alice", "", "제주", "", &defaultVoice.ID, &created.ID)
	if err != nil {
		t.Fatal(err)
	}

	input, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if input.PurposeID != created.ID {
		t.Fatalf("the adapter dropped the purpose: PurposeID=%q, want %q", input.PurposeID, created.ID)
	}

	brief, ok, err := (generationPurposes{service: purposeSvc}).BriefFor(ctx, "alice", input.PurposeID)
	if err != nil || !ok {
		t.Fatalf("brief lookup: ok=%v err=%v", ok, err)
	}
	want := generation.PurposeBrief{Name: "정보성 식당 리뷰", Description: "협찬 방문 리뷰", Instructions: "사진마다 설명하세요"}
	if brief != want {
		t.Fatalf("brief = %+v, want %+v", brief, want)
	}
	// And the prompt that brief produces actually carries it.
	system, _ := generation.BuildWritePrompt(generation.Profile{}, nil, "", "", nil, nil, &brief)
	if !strings.Contains(system, "[글의 용도: 정보성 식당 리뷰]") {
		t.Fatalf("the frozen brief did not reach the prompt:\n%s", system)
	}

	// A post left on 없음 resolves to no brief, so the prompt is the pre-purpose one.
	plain, err := postSvc.SaveDraft(ctx, "alice", "", "용도 없는 글", "", &defaultVoice.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	bare, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", plain.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if bare.PurposeID != "" {
		t.Fatalf("a post with no purpose reported %q", bare.PurposeID)
	}
}
