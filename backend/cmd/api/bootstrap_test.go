package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
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
	if _, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &guess); !errors.Is(err, post.ErrVoiceNotFound) {
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
	created, err := postSvc.SaveDraft(ctx, "alice", "", "first", "", &voices[0].ID)
	if err != nil || created.VoiceID != voices[0].ID || created.Voice.Name != voice.DefaultVoiceName {
		t.Fatalf("post after bootstrap = %+v err=%v", created, err)
	}
}
