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
	Storyline: &generation.Storyline{
		Paragraphs: []generation.StorylineParagraph{{Text: "성수 카페에 간 이유를 보여줍니다.", Files: []string{}}},
		MadeWith:   []string{},
	},
}

// A lab candidate's output carries the answer's nouns and storyline, so the winner brings its
// own once it is applied (GEN-55, GEN-72).
func TestCandidateOutputCarriesTheNounsAndStoryline(t *testing.T) {
	encoded, err := json.Marshal(toOutputPost(writeAnnotationsAnswer))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"nouns":["성수","카페"]`) ||
		!strings.Contains(string(encoded), `"storyline":{"paragraphs":[{"text":"성수 카페에 간 이유를 보여줍니다.","files":[]}],"made_with":[]}`) {
		t.Fatalf("output %s lacks the nouns or the storyline", encoded)
	}
	var value outputPost
	if err := json.Unmarshal(encoded, &value); err != nil {
		t.Fatal(err)
	}
	if got := fromOutputPost(value); !reflect.DeepEqual(got, writeAnnotationsAnswer) {
		t.Fatalf("round trip = %+v", got)
	}

	// A noun-less candidate keeps the bytes it had before annotations existed.
	plain, err := json.Marshal(toOutputPost(generation.WriteAnswer{Content: writeAnnotationsAnswer.Content}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plain), `"nouns"`) || strings.Contains(string(plain), `"storyline"`) {
		t.Fatalf("a plain candidate grew members: %s", plain)
	}
}

// An output recorded before annotations existed decodes as none, and applying it clears. One
// recorded while writes offered replacement candidates decodes exactly as the same output
// without its "replacements" member: the retired key is ignored, never converted.
func TestALegacyCandidateOutputDecodesAsNone(t *testing.T) {
	legacy := `{"title":"옛 후보","summary":"s","tags":["a"],"blocks":[{"type":"TEXT","content":"본문","level":0,"file":"","alt":"","caption":"","items":null}]}`
	decode := func(raw string) generation.WriteAnswer {
		t.Helper()
		var value outputPost
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			t.Fatal(err)
		}
		return fromOutputPost(value)
	}
	answer := decode(legacy)
	if answer.Content.Title != "옛 후보" || answer.Nouns != nil {
		t.Fatalf("legacy output = %+v", answer)
	}
	if annotations := answer.Annotations(); annotations == nil || annotations.Nouns != nil || annotations.Storyline != nil {
		t.Fatalf("a legacy winner hands %+v, want a non-nil none that keeps the post's storyline", annotations)
	}

	withNouns := strings.TrimSuffix(legacy, "}") + `,"nouns":["후보"]}`
	withReplacements := strings.TrimSuffix(withNouns, "}") + `,"replacements":[{"surface":"title","index":0,"source":"옛 후보","phrases":["새 후보"]}]}`
	if got, want := decode(withReplacements), decode(withNouns); !reflect.DeepEqual(got, want) {
		t.Fatalf("an output with replacements decoded as %+v, want %+v", got, want)
	}
}

// Through the adapter into the real post store: the nouns land beside the content, in a column
// of their own, and a nil (a revision) keeps them.
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
	makeVoice(t, handle, "alice", defaultVoice.ID)
	language := post.LanguageKorean
	saved, err := postSvc.SaveDraft(ctx, "alice", post.DraftSave{Title: "성수", VoiceID: &defaultVoice.ID, TargetLanguage: &language})
	if err != nil {
		t.Fatal(err)
	}

	adapter := generationPosts{service: postSvc}
	if err := adapter.SetGeneratedContent(ctx, "alice", saved.Slug, writeAnnotationsAnswer.Content, generation.LanguageKorean, writeAnnotationsAnswer.Annotations()); err != nil {
		t.Fatal(err)
	}
	got, err := postSvc.Get(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.ContentNouns, []string{"성수", "카페"}) {
		t.Fatalf("stored nouns %v", got.ContentNouns)
	}
	wantStoryline := &post.Storyline{Paragraphs: []post.StorylineParagraph{{Text: "성수 카페에 간 이유를 보여줍니다."}}}
	if !reflect.DeepEqual(got.Storyline, wantStoryline) {
		t.Fatalf("stored storyline %+v", got.Storyline)
	}
	var content, baseline string
	if err := handle.Reader.QueryRow("SELECT content, machine_baseline FROM posts WHERE slug = ?", saved.Slug).Scan(&content, &baseline); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"content": content, "machine_baseline": baseline} {
		if strings.Contains(value, `"nouns"`) {
			t.Errorf("%s carries the annotations: %s", name, value)
		}
	}

	// A revision hands nil: the content moves and the nouns stand.
	revised := writeAnnotationsAnswer.Content
	revised.Blocks = []generation.Block{{Type: generation.BlockText, Content: "고쳐 쓴 문장."}}
	if err := adapter.SetGeneratedContent(ctx, "alice", saved.Slug, revised, generation.LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	kept, err := postSvc.Get(ctx, "alice", saved.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if kept.ContentRevision != got.ContentRevision+1 || !reflect.DeepEqual(kept.ContentNouns, got.ContentNouns) ||
		!reflect.DeepEqual(kept.Storyline, wantStoryline) {
		t.Fatalf("after a revision: revision %d nouns %v storyline %+v", kept.ContentRevision, kept.ContentNouns, kept.Storyline)
	}
}
