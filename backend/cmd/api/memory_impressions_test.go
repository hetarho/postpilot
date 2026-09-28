package main

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/guideline"
	guidelinestore "github.com/postpilot/backend/internal/guideline/store"
	"github.com/postpilot/backend/internal/memory"
	memorystore "github.com/postpilot/backend/internal/memory/store"
	"github.com/postpilot/backend/internal/platform/db"
)

// GEN-73 through the real wiring: a preference memory reaches the generation context as a
// 취향:-labelled text, and the memories 기본 지침 freezes exactly beside a retrieval that found
// one. The adapters pass generation's flag through unchanged, so an account whose retrieval
// finds nothing freezes the defaults it froze before this 지침 existed.
func TestTheMemoriesDefaultFreezesOnlyBesideAMatchingMemory(t *testing.T) {
	d, err := db.Open(filepath.Join(t.TempDir(), "impressions.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	ctx := context.Background()
	if err := db.Migrate(ctx, d.Writer); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, id := range []string{"alice", "bob"} {
		if _, err := d.Writer.Exec("INSERT INTO users(id,password_hash,created_at) VALUES(?,'hash',?)", id, now); err != nil {
			t.Fatal(err)
		}
	}
	memories := memory.NewService(
		memorystore.New(d.Writer, d.Reader),
		memory.Limits{TextMaxChars: 120, TagsMax: 5, MaxPerAccount: 300, InjectMax: 8},
	)
	if _, _, err := memories.Create(ctx, "alice", "매운 음식을 좋아한다", memory.KindPreference, nil, ""); err != nil {
		t.Fatal(err)
	}
	guidelines := guideline.NewService(
		guidelinestore.New(d.Writer, d.Reader),
		blogFields{},
		guideline.Limits{TextMaxChars: 300, MaxPerAccount: 100},
		50,
	)
	memoryAdapter := generationMemories{service: memories}
	guidelineAdapter := generationGuidelines{service: guidelines}
	impressions, _ := guideline.DefaultFor(guideline.KindPost, "memory_impressions")

	for _, account := range []struct {
		user        string
		withMemory  bool
		wantDefault bool
	}{{"alice", true, true}, {"bob", false, false}} {
		texts, err := memoryAdapter.ForPost(ctx, account.user, []string{"떡볶이를 먹었다"})
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(texts, "취향: 매운 음식을 좋아한다"); got != account.withMemory || (!account.withMemory && len(texts) != 0) {
			t.Fatalf("%s retrieved %q", account.user, texts)
		}
		frozen, err := guidelineAdapter.ForPrompt(ctx, account.user, nil, nil, generation.LanguageKorean, len(texts) > 0)
		if err != nil {
			t.Fatal(err)
		}
		if got := slices.Contains(frozen.Defaults, impressions.Ko.Text); got != account.wantDefault {
			t.Fatalf("%s froze the memories default = %v, want %v: %q", account.user, got, account.wantDefault, frozen.Defaults)
		}
	}
}
