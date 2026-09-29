package voice_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/voice"
)

type fakePost struct {
	owner    string
	voiceID  string
	revision int64
	blocks   []voice.Block
}

type fakePosts map[string]fakePost

func (f fakePosts) PostForFingerprint(_ context.Context, userID, slug string) (string, int64, []voice.Block, error) {
	found, ok := f[slug]
	switch {
	case !ok:
		return "", 0, nil, voice.ErrPostNotFound
	case found.owner != userID:
		return "", 0, nil, voice.ErrPostForbidden
	}
	return found.voiceID, found.revision, found.blocks, nil
}

// fingerprintProse is ten sentences that end in 해요 with an exclamation mark — the voice's side.
var fingerprintProse = strings.Repeat("정말 맛있었어요! ", 12)

// VOICE-62, POST-102: ② counts the post's blocks against its voice's current analysis, with no
// model call; a post with 말투 없음, no content, or a deleted or unmade voice is not applicable,
// and another account's post or an unknown one is refused.
func TestPostFingerprintComparesThePostWithItsVoice(t *testing.T) {
	h := newVoiceHarness(t)
	ctx := context.Background()
	alice := h.voice("alice")
	counted := voice.FingerprintOf([]voice.Material{{ID: "m1", Kind: voice.SampleKindPost, CreatedAt: time.Now(), Text: fingerprintProse}})
	if err := h.store.PublishAnalysis(ctx, "alice", alice, voice.Analysis{Counted: counted, AnalyzeModel: analyzeRef.String(), CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	unmade, err := h.svc.CreateVoice(ctx, "alice", "아직")
	if err != nil {
		t.Fatal(err)
	}
	gone, err := h.svc.CreateVoice(ctx, "alice", "지운 말투")
	if err != nil {
		t.Fatal(err)
	}
	h.makeVoice(t, "alice", gone.ID)
	if _, err := h.svc.DeleteVoice(ctx, "alice", gone.ID); err != nil {
		t.Fatal(err)
	}
	post := []voice.Block{
		{Type: voice.BlockHeading, Content: "첫 방문"},
		{Type: voice.BlockText, Content: strings.Repeat("국물이 진했다. ", 12)},
		{Type: "IMAGE", Content: "IMG_1.jpg"},
	}
	h.svc.ConfigurePosts(fakePosts{
		"made":     {owner: "alice", voiceID: alice, revision: 7, blocks: post},
		"no-voice": {owner: "alice", revision: 3, blocks: post},
		"empty":    {owner: "alice", voiceID: alice, revision: 0},
		"unmade":   {owner: "alice", voiceID: unmade.ID, revision: 2, blocks: post},
		"deleted":  {owner: "alice", voiceID: gone.ID, revision: 4, blocks: post},
		"bobs":     {owner: "bob", voiceID: h.voice("bob"), revision: 1, blocks: post},
	})
	h.models.completeCalls = 0

	found, err := h.svc.PostFingerprint(ctx, "alice", "made")
	if err != nil || !found.Applicable || found.Revision != 7 || len(found.Items) != len(voice.Items()) {
		t.Fatalf("made voice = %+v err=%v", found, err)
	}
	var endings voice.ItemComparison
	for _, item := range found.Items {
		if item.Item == voice.ItemEndings {
			endings = item
		}
	}
	if endings.Unknown || endings.Distance < 0.9 || endings.Headline == "" {
		t.Fatalf("해요 against 다 = %+v", endings)
	}
	for _, facet := range endings.Facets {
		if facet.Key == "해요" && (facet.Unit != voice.UnitShare || facet.Voice < 0.9 || facet.Text != 0) {
			t.Fatalf("the 해요 facet = %+v", facet)
		}
	}
	for slug, revision := range map[string]int64{"no-voice": 3, "empty": 0, "unmade": 2, "deleted": 4} {
		found, err := h.svc.PostFingerprint(ctx, "alice", slug)
		if err != nil || found.Applicable || len(found.Items) != 0 || found.Revision != revision {
			t.Fatalf("%s = %+v err=%v", slug, found, err)
		}
	}
	if _, err := h.svc.PostFingerprint(ctx, "alice", "bobs"); !errors.Is(err, voice.ErrPostForbidden) {
		t.Fatalf("a foreign post = %v", err)
	}
	if _, err := h.svc.PostFingerprint(ctx, "alice", "nope"); !errors.Is(err, voice.ErrPostNotFound) {
		t.Fatalf("an unknown post = %v", err)
	}
	if h.models.completeCalls != 0 {
		t.Fatalf("the comparison called a model %d times", h.models.completeCalls)
	}
}
