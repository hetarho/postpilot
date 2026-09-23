package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/quality"
	qualitystore "github.com/postpilot/backend/internal/quality/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

// Post's block spelling is the proto's, so the wire enum is the list of every kind a post can
// hold: each must reach its own quality kind, or a post with that block would fail to measure.
func TestQualityAdapterMapsEveryBlockType(t *testing.T) {
	mapped := map[quality.BlockType]bool{}
	for value, name := range postpilotv1.BlockType_name {
		if value == int32(postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED) {
			continue
		}
		kind, ok := qualityBlockType(post.BlockType(name))
		if !ok {
			t.Fatalf("post block type %s has no quality kind", name)
		}
		mapped[kind] = true
	}
	if len(mapped) != 6 {
		t.Fatalf("mapped %d distinct kinds, want 6: %v", len(mapped), mapped)
	}
	if _, ok := qualityBlockType("POEM"); ok {
		t.Fatal("an unknown block type was mapped")
	}
	if _, err := qualityDocument(post.PostContent{Blocks: []post.Block{{Type: "POEM"}}}); err == nil {
		t.Fatal("a document with an unknown block was built rather than refused")
	}
}

// The aggregate reads published posts only, a measurement row belongs to its post, and a
// deleted post leaves both its row and the aggregate (POST-85).
func TestTheAggregateCountsPublishedPostsOnlyAndForgetsADeletedOne(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "quality.db"))
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
	if err := defaultVoiceBootstrap(ctx, handle, "alice"); err != nil {
		t.Fatal(err)
	}
	voiceSvc := voice.NewService(voicestore.New(handle.Writer, handle.Reader), nil, nil)
	postSvc := post.NewService(poststore.New(handle.Writer, handle.Reader), noBlobs{}, testPostLimits(), testPostDeps(voiceSvc))
	qualityStore := qualitystore.New(handle.Writer, handle.Reader)
	qualitySvc := quality.NewService(quality.Deps{
		Measurements: qualityStore, Phrases: qualityStore, Posts: qualityPosts{service: postSvc}, Now: time.Now,
	})

	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	var slugs []string
	for i := 0; i < 4; i++ {
		created, err := postSvc.SaveDraft(ctx, "alice", "", fmt.Sprintf("제주 %d일", i+1), "", &defaultVoice.ID, nil, &language, nil)
		if err != nil {
			t.Fatal(err)
		}
		content := post.PostContent{Title: fmt.Sprintf("제주 %d일 기록", i+1), Blocks: []post.Block{
			{Type: post.BlockHeading, Content: "첫날", Level: 2},
			{Type: post.BlockText, Content: fmt.Sprintf("협재 해변은 물빛이 맑았고 %d번째 날에도 바람이 잔잔했다.", i+1)},
		}}
		if err := postSvc.SetGeneratedContent(ctx, "alice", created.Slug, content, post.LanguageKorean); err != nil {
			t.Fatal(err)
		}
		if _, err := postSvc.Finalize(ctx, "alice", created.Slug, 1); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, created.Slug)
	}
	for i, slug := range slugs[:2] {
		if _, err := postSvc.SavePublishedURL(ctx, "alice", slug, fmt.Sprintf("https://blog.naver.com/alice/22300000000%d", i+1)); err != nil {
			t.Fatal(err)
		}
	}
	rows := func(slug string) int {
		t.Helper()
		var n int
		query, args := `SELECT COUNT(*) FROM post_measurements`, []any{}
		if slug != "" {
			query, args = query+` WHERE post_slug = ?`, []any{slug}
		}
		if err := handle.Reader.QueryRow(query, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}

	reading, err := qualitySvc.PostMeasurement(ctx, "alice", slugs[0])
	if err != nil {
		t.Fatal(err)
	}
	if rows("") != 1 || rows(slugs[0]) != 1 {
		t.Fatalf("one measurement wrote %d rows", rows(""))
	}
	// Both blocks crossed the adapter: a heading and a paragraph are two types.
	if got := reading.Measurement.Composition.Composition; got == nil || got.DistinctBlockTypes != 2 {
		t.Fatalf("the adapter lost a block: composition %+v", got)
	}
	// The finalized-but-unpublished posts are not the published window.
	if reading.Measurement.CrossPost.Others != 1 {
		t.Fatalf("the post was compared with %d others, want the one other published post", reading.Measurement.CrossPost.Others)
	}
	account, err := qualitySvc.AccountQuality(ctx, "alice", slugs[2])
	if err != nil || account.Account.PublishedCount != 2 {
		t.Fatalf("published count = %d, %v; want 2", account.Account.PublishedCount, err)
	}

	if err := postSvc.DeletePost(ctx, "alice", slugs[0]); err != nil {
		t.Fatal(err)
	}
	if rows(slugs[0]) != 0 {
		t.Fatal("the deleted post's measurement row survived")
	}
	account, err = qualitySvc.AccountQuality(ctx, "alice", slugs[2])
	if err != nil || account.Account.PublishedCount != 1 {
		t.Fatalf("after the delete the count = %d, %v; want 1", account.Account.PublishedCount, err)
	}
	if _, err := qualitySvc.PostMeasurement(ctx, "alice", slugs[0]); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("the deleted post still measures: %v", err)
	}
}
