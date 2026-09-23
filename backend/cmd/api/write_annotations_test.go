package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/auth"
	authstore "github.com/postpilot/backend/internal/auth/store"
	"github.com/postpilot/backend/internal/generation"
	"github.com/postpilot/backend/internal/plan"
	"github.com/postpilot/backend/internal/platform/db"
	"github.com/postpilot/backend/internal/post"
	poststore "github.com/postpilot/backend/internal/post/store"
	"github.com/postpilot/backend/internal/voice"
	voicestore "github.com/postpilot/backend/internal/voice/store"
)

var writeAnnotationsAnswer = generation.WriteAnswer{
	Content: generation.PostContent{
		Title: "성수 카페 투어", Tags: []string{"성수"},
		Blocks: []generation.Block{{Type: generation.BlockText, Content: "성수 카페에 갔다."}},
	},
	Nouns: []string{"성수", "카페"},
	Replacements: []generation.Replacement{
		{Surface: generation.ReplacementTitle, Index: 0, Source: "성수 카페", Phrases: []string{"분위기 좋은 카페"}},
		{Surface: generation.ReplacementTag, Index: 0, Source: "성수", Phrases: []string{"성수동"}},
	},
}

// A lab candidate's output carries the answer's annotations, so the winner brings its own once it
// is applied.
func TestCandidateOutputCarriesNounsAndReplacements(t *testing.T) {
	encoded, err := json.Marshal(toOutputPost(writeAnnotationsAnswer))
	if err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{`"nouns":["성수","카페"]`, `"replacements":[{"surface":"title","index":0,"source":"성수 카페","phrases":["분위기 좋은 카페"]}`} {
		if !strings.Contains(string(encoded), member) {
			t.Fatalf("output %s lacks %s", encoded, member)
		}
	}
	var value outputPost
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	if got := fromOutputPost(value); !reflect.DeepEqual(got, writeAnnotationsAnswer) {
		t.Fatalf("round trip = %+v", got)
	}

	// A noun-less, phrase-less candidate keeps the bytes it had before annotations existed.
	plain, err := json.Marshal(toOutputPost(generation.WriteAnswer{Content: writeAnnotationsAnswer.Content}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), `"nouns"`) || strings.Contains(string(plain), `"replacements"`) {
		t.Fatalf("a plain candidate grew members: %s", plain)
	}
}

// An output recorded before annotations existed decodes as none, and applying it clears.
func TestALegacyCandidateOutputDecodesAsNone(t *testing.T) {
	legacy := `{"title":"옛 후보","summary":"s","tags":["a"],"blocks":[{"type":"TEXT","content":"본문","level":0,"file":"","alt":"","caption":"","items":null}]}`
	var value outputPost
	if err := json.Unmarshal([]byte(legacy), &value); err != nil {
		t.Fatal(err)
	}
	answer := fromOutputPost(value)
	if answer.Content.Title != "옛 후보" || answer.Nouns != nil || answer.Replacements != nil {
		t.Fatalf("legacy output = %+v", answer)
	}
	if annotations := answer.Annotations(); annotations == nil || annotations.Nouns != nil || annotations.Replacements != nil {
		t.Fatalf("a legacy winner hands %+v, want a non-nil none", annotations)
	}
}

// Through the adapter into the real post store: the annotations land beside the content, in
// columns of their own, and a nil (a revision) keeps them.
func TestGenerationPostsMapsTheAnnotations(t *testing.T) {
	handle, err := db.Open(filepath.Join(t.TempDir(), "annotations.db"))
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
	defaultVoice, err := voiceSvc.DefaultVoice(ctx, "alice")
	if err != nil {
		t.Fatal(err)
	}
	language := post.LanguageKorean
	saved, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "성수", VoiceID: &defaultVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}

	adapter := generationPosts{service: postSvc}
	if err := adapter.SetGeneratedContent(ctx, "alice", saved.Slug, writeAnnotationsAnswer.Content, generation.LanguageKorean, writeAnnotationsAnswer.Annotations()); err != nil {
		t.Fatal(err)
	}
	wantCandidates := []post.ReplacementCandidate{
		{Surface: post.ReplacementSurfaceTitle, Index: 0, Source: "성수 카페", Phrases: []string{"분위기 좋은 카페"}},
		{Surface: post.ReplacementSurfaceTag, Index: 0, Source: "성수", Phrases: []string{"성수동"}},
	}
	got, err := postSvc.Get(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ContentNouns, []string{"성수", "카페"}) || !reflect.DeepEqual(got.ReplacementCandidates, wantCandidates) {
		t.Fatalf("stored nouns %v candidates %+v", got.ContentNouns, got.ReplacementCandidates)
	}
	var content, baseline string
	if err := handle.Reader.QueryRow("SELECT content, machine_baseline FROM posts WHERE slug = ?", saved.Slug).Scan(&content, &baseline); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"content": content, "machine_baseline": baseline} {
		if strings.Contains(value, `"nouns"`) || strings.Contains(value, `"replacements"`) {
			t.Errorf("%s carries the annotations: %s", name, value)
		}
	}

	// A revision hands nil: the content moves and the annotations stand.
	revised := writeAnnotationsAnswer.Content
	revised.Blocks = []generation.Block{{Type: generation.BlockText, Content: "고쳐 쓴 문장."}}
	if err := adapter.SetGeneratedContent(ctx, "alice", saved.Slug, revised, generation.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	kept, err := postSvc.Get(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if kept.ContentRevision != got.ContentRevision+1 || !reflect.DeepEqual(kept.ReplacementCandidates, wantCandidates) || !reflect.DeepEqual(kept.ContentNouns, got.ContentNouns) {
		t.Fatalf("after a revision: revision %d nouns %v candidates %+v", kept.ContentRevision, kept.ContentNouns, kept.ReplacementCandidates)
	}

	// A surface post does not know is an error, never a guess.
	unknown := &generation.WriteAnnotations{Replacements: []generation.Replacement{{Surface: "summary", Source: "s", Phrases: []string{"p"}}}}
	if err := adapter.SetGeneratedContent(ctx, "alice", saved.Slug, revised, generation.LanguageKorean, unknown); err == nil {
		t.Fatal("an unknown surface was stored")
	}
}
