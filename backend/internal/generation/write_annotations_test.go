package generation

import (
	"context"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// An answer whose nouns and title span the validator keeps against the frozen phrase list.
const annotatedAnswer = `{"title":"성수 카페 투어","summary":"s","tags":["성수"],"nouns":["성수","카페"],
	"blocks":[{"type":"TEXT","content":"성수 카페에 갔다."}],
	"replacements":[{"surface":"title","index":0,"source":"성수 카페","phrases":["분위기 좋은 카페"]}]}`

var (
	annotatedNouns        = []string{"성수", "카페"}
	annotatedReplacements = []Replacement{{Surface: ReplacementTitle, Index: 0, Source: "성수 카페", Phrases: []string{"분위기 좋은 카페"}}}
)

func annotatingModels(text string) *fakeModels {
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: text}, nil
	}
	return models
}

// The frozen phrase list the validator keeps the title span against.
var annotatedPhrases = []string{"분위기 좋은 카페"}

// listedPhrases is a phrase port answering one fixed list, for the paths that freeze through it.
type listedPhrases []string

func (l listedPhrases) For(context.Context, string) ([]string, error) { return l, nil }

// phrasedPost is a post with a 분야, so the snapshot freezes its list through the port.
func phrasedPost() *fakePosts {
	return &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모", Field: "cafe",
	}}
}

func phrasedService(posts *fakePosts, models *fakeModels) *Service {
	deps := testDeps()
	deps.FieldPhrases = listedPhrases(annotatedPhrases)
	return NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, deps)
}

// GEN-53, GEN-55: a generation hands the post the write's nouns and candidates beside its content.
func TestGenerateHandsTheWriteAnswerToThePost(t *testing.T) {
	posts := phrasedPost()
	svc := phrasedService(posts, annotatingModels(annotatedAnswer))
	// The phrases arrive frozen in the job, as Start froze them.
	if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), FieldPhrases: annotatedPhrases}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 || posts.annotations[0] == nil {
		t.Fatalf("annotations = %+v", posts.annotations)
	}
	got := posts.annotations[0]
	if !reflect.DeepEqual(got.Nouns, annotatedNouns) || !reflect.DeepEqual(got.Replacements, annotatedReplacements) {
		t.Fatalf("the post was handed %+v", got)
	}
}

// GEN-48: a write with no frozen phrases offers no candidates, and it still hands the post an
// answer — non-nil, with none — so the last generation's candidates are cleared, not kept.
func TestAPhraselessWriteClearsCandidates(t *testing.T) {
	posts := phrasedPost()
	svc := phrasedService(posts, annotatingModels(annotatedAnswer))
	if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 || posts.annotations[0] == nil {
		t.Fatalf("a phrase-less write handed %+v, want a non-nil answer", posts.annotations)
	}
	if got := posts.annotations[0]; got.Replacements != nil || !reflect.DeepEqual(got.Nouns, annotatedNouns) {
		t.Fatalf("a phrase-less write handed %+v", got)
	}
}

// GEN-55, GEN-57: a revision has no nouns answer and no phrase list, so it keeps what the post
// holds by handing nil.
func TestReviseKeepsTheStoredAnnotations(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body"), Field: "cafe",
	}}
	models := annotatingModels(`{"title":"제목","summary":"요약","tags":["a"],"blocks":[{"type":"TEXT","content":"고친 본문"}]}`)
	svc := NewService(posts, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘", false),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 || posts.annotations[0] != nil {
		t.Fatalf("a revision handed %+v, want nil", posts.annotations)
	}
}

// A write-experiment candidate returns its whole answer, so a winner can carry its own.
func TestRunWriteCandidateReturnsTheAnswer(t *testing.T) {
	posts := phrasedPost()
	svc := phrasedService(posts, annotatingModels(annotatedAnswer))
	raw, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := svc.PrepareWriteInput(context.Background(), raw, func(string, int, int) {})
	if err != nil {
		t.Fatal(err)
	}
	answer, _, err := svc.RunWriteCandidate(context.Background(), prepared, writeRef)
	if err != nil {
		t.Fatal(err)
	}
	if answer.Content.Title != "성수 카페 투어" || !reflect.DeepEqual(answer.Nouns, annotatedNouns) || !reflect.DeepEqual(answer.Replacements, annotatedReplacements) {
		t.Fatalf("candidate answer = %+v", answer)
	}
	if len(posts.contents) != 0 {
		t.Fatal("running a candidate wrote the post")
	}
}

// GEN-4: an applied winner's annotations replace the post's; one recorded before they existed
// carries none, and applying it clears them.
func TestApplyWriteWinnerForwardsTheWinnersAnnotations(t *testing.T) {
	posts := phrasedPost()
	svc := phrasedService(posts, annotatingModels(annotatedAnswer))
	snapshot, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	content := PostContent{Title: "성수 카페 투어", Blocks: []Block{{Type: BlockText, Content: "성수 카페에 갔다."}}}
	winner := WriteAnswer{Content: content, Nouns: annotatedNouns, Replacements: annotatedReplacements}
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", winner, snapshot); err != nil {
		t.Fatal(err)
	}
	legacy := WriteAnswer{Content: content}
	if err := svc.ApplyWriteWinner(context.Background(), "alice", "post", legacy, snapshot); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 2 || posts.annotations[0] == nil || posts.annotations[1] == nil {
		t.Fatalf("annotations = %+v", posts.annotations)
	}
	if got := posts.annotations[0]; !reflect.DeepEqual(got.Nouns, annotatedNouns) || !reflect.DeepEqual(got.Replacements, annotatedReplacements) {
		t.Fatalf("the winner handed %+v", got)
	}
	if got := posts.annotations[1]; got.Nouns != nil || got.Replacements != nil {
		t.Fatalf("a legacy winner handed %+v, want none", got)
	}
}
