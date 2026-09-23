package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/experiment"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// The experiment context mirrors the post statuses as plain strings (ARCH-7), so nothing but
// this pin keeps the two spellings in step: a drifted one would let a comparison land in a
// published post, or refuse a draft.
func TestExperimentPostStatusMirrorsThePostContext(t *testing.T) {
	for mirror, status := range map[string]string{
		experiment.PostStatusDraft:     post.StatusDraft,
		experiment.PostStatusReview:    post.StatusReview,
		experiment.PostStatusFinalized: post.StatusFinalized,
		experiment.PostStatusPublished: post.StatusPublished,
	} {
		if mirror != status {
			t.Errorf("experiment mirror %q, post status %q", mirror, status)
		}
	}
}

// Each context names the lock in its own vocabulary, and the adapters are where one becomes
// the other: a refusal that crossed untranslated would reach the wire as an unknown failure.
func TestPublishedRefusalsCrossTheAdapters(t *testing.T) {
	wrapped := fmt.Errorf("snapshot write input: %w", generation.ErrPostPublished)
	for name, got := range map[string]error{
		"generation's refusal of an editor snapshot": mapSnapshotError(generation.ErrPostPublished),
		"the same refusal wrapped":                   mapSnapshotError(wrapped),
	} {
		if !errors.Is(got, experiment.ErrPostPublished) {
			t.Errorf("%s = %v, want experiment.ErrPostPublished", name, got)
		}
	}
	if got := generationPostError(fmt.Errorf("save generated content: %w", post.ErrPostPublished)); !errors.Is(got, generation.ErrPostPublished) {
		t.Fatalf("the post context's refusal = %v, want generation.ErrPostPublished", got)
	}

	// Through the real post service: a result written to a published post comes back as
	// generation's refusal, not as a missing post.
	ctx := context.Background()
	postSvc, voiceSvc := publishedPostService(t)
	slug := publishedSlug(t, postSvc, voiceSvc)
	posts := generationPosts{service: postSvc}
	if err := posts.SetGeneratedContent(ctx, "alice", slug, generation.PostContent{
		Title: "덮어쓰기", Blocks: []generation.Block{{Type: generation.BlockText, Content: "새 문장"}},
	}, generation.LanguageKorean, nil); !errors.Is(err, generation.ErrPostPublished) {
		t.Fatalf("writing into a published post = %v, want generation.ErrPostPublished", err)
	}
}

// The generation context never reads a post's status; it reads this flag, which the adapter
// derives from the real post: true only while the post is published.
func TestGenerationPostsCarriesThePublishedFlag(t *testing.T) {
	ctx := context.Background()
	postSvc, voiceSvc := publishedPostService(t)
	posts := generationPosts{service: postSvc}
	slug := publishedSlug(t, postSvc, voiceSvc)
	input, err := posts.AttachedImages(ctx, "alice", slug)
	if err != nil || !input.Published {
		t.Fatalf("a published post read Published=%v, %v", input.Published, err)
	}
	if _, err := postSvc.SavePublishedURL(ctx, "alice", slug, ""); err != nil {
		t.Fatal(err)
	}
	if input, err := posts.AttachedImages(ctx, "alice", slug); err != nil || input.Published {
		t.Fatalf("a cleared post read Published=%v, %v", input.Published, err)
	}
}

func publishedPostService(t *testing.T) (*post.Service, *voice.Service) {
	t.Helper()
	handle, err := db.Open(filepath.Join(t.TempDir(), "published.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { handle.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, handle.Writer); err != nil {
		t.Fatal(err)
	}
	if err := authstore.New(handle.Writer, handle.Reader).CreateUser(ctx, auth.User{ID: "alice", PasswordHash: "hash", Plan: plan.Free, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	return post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc)), voiceSvc
}

// publishedSlug reaches published through the real path: generated content, the finalization,
// then the address.
func publishedSlug(t *testing.T, postSvc *post.Service, voiceSvc *voice.Service) string {
	t.Helper()
	ctx := context.Background()
	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	created, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "제주", VoiceID: &defaultVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}
	content := post.PostContent{Title: "제주 3일", Blocks: []post.Block{{Type: post.BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if err := postSvc.SetGeneratedContent(ctx, "alice", created.Slug, content, post.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := postSvc.Finalize(ctx, "alice", created.Slug, 1); err != nil {
		t.Fatal(err)
	}
	published, err := postSvc.SavePublishedURL(ctx, "alice", created.Slug, "https://blog.naver.com/alice/223000000001")
	if err != nil || published.Status != post.StatusPublished {
		t.Fatalf("publish = %s, %v", published.Status, err)
	}
	return created.Slug
}
