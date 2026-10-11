package main

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// POST-116, GEN-7: generation receives the post's photos in the owner's order, through the
// adapter's existing walk of the post's photos, and the videos still follow them.
func TestGenerationReceivesPhotosInTheOwnersOrder(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(filepath.Join(t.TempDir(), "order.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
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
	defaultVoice, err := firstTestVoice(ctx, voiceSvc, "alice")
	if err != nil {
		t.Fatal(err)
	}
	makeVoice(t, handle, "alice", defaultVoice.ID)
	store := poststore.New(handle.Writer, handle.Reader)
	postSvc := post.NewService(store, noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))

	language := post.LanguageKorean
	created, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "제주", VoiceID: &defaultVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"a", "b", "c"} {
		if err := store.CreateImage(ctx, post.Image{ID: id, PostSlug: created.Slug, Filename: id + ".jpg", Key: post.ObjectKey(created.Slug, id), Width: 1, Height: 1, Bytes: 1, CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CreateVideo(ctx, post.Video{ID: "v", PostSlug: created.Slug, Filename: "clip.mp4", Key: post.VideoObjectKey(created.Slug, "v", "mp4"), ContentType: "video/mp4", Bytes: 1, DurationMs: 1000, Width: 1, Height: 1, CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err := postSvc.ReorderImages(ctx, "alice", created.Slug, []string{"c", "a", "b"}); err != nil {
		t.Fatal(err)
	}

	input, err := generationPosts{service: postSvc}.AttachedImages(ctx, "alice", created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, image := range input.Images {
		files = append(files, image.Filename)
	}
	if want := []string{"c.jpg", "a.jpg", "b.jpg", "clip.mp4"}; !reflect.DeepEqual(files, want) {
		t.Fatalf("generation reads %v, want %v", files, want)
	}
}
