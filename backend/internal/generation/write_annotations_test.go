package generation

import (
	"context"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

// An answer with a storyline and nouns, plus a replacements member a model may still send: the
// write answer carries the storyline and nouns only (GEN-55, GEN-67), so the parse ignores it.
const annotatedAnswer = `{"storyline":[{"text":"성수 카페에 간 이유를 보여줍니다.","files":[]}],
	"title":"성수 카페 투어","summary":"s","tags":["성수"],"nouns":["성수","카페"],
	"blocks":[{"type":"TEXT","content":"성수 카페에 갔다."}],
	"replacements":[{"surface":"title","index":0,"source":"성수 카페","phrases":["분위기 좋은 카페"]}]}`

var annotatedNouns = []string{"성수", "카페"}

func annotatingModels(text string) *fakeModels {
	models := newFakeModels()
	models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) {
		return llm.Response{Text: text}, nil
	}
	return models
}

// annotatedPost is a post with a 분야, which plays no part in the write (GEN-14).
func annotatedPost() *fakePosts {
	return &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Title: "가제", Memo: "메모", Field: "cafe",
	}}
}

func annotatedService(posts *fakePosts, models *fakeModels) *Service {
	return NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
}

// GEN-55, GEN-67: a generation hands the post the write's nouns and storyline beside its
// content, and nothing else.
func TestGenerateHandsTheWriteAnswerToThePost(t *testing.T) {
	posts := annotatedPost()
	svc := annotatedService(posts, annotatingModels(annotatedAnswer))
	if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustGeneratePayload(t, generationOptions{})}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 {
		t.Fatalf("annotations = %+v", posts.annotations)
	}
	want := &WriteAnnotations{Nouns: annotatedNouns, Storyline: &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "성수 카페에 간 이유를 보여줍니다."}},
	}}
	if got := posts.annotations[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("the post was handed %+v (storyline %+v)", got, got.Storyline)
	}
}

// GEN-67, GEN-12: the write keeps its storyline's files against what the run was shown, and
// hands those names on as what the storyline was made with.
func TestTheWriteStorylineIsMadeWithTheAttachmentsItWasShown(t *testing.T) {
	models := annotatingModels(`{"storyline":[{"text":"가게 앞","files":["a.jpg","ghost.jpg"]},{"text":"영상","files":["clip.mp4","a.jpg"]}],
		"title":"t","summary":"s","tags":["a"],"blocks":[{"type":"TEXT","content":"ok"}],"nouns":[]}`)
	svc := annotatedService(annotatedPost(), models)
	answer, err := svc.write(context.Background(), PostInput{
		UserID: "alice", Voice: liveVoice, TargetLanguage: LanguageKorean,
		Images: []Image{{Filename: "a.jpg", Key: "k1"}, {Filename: "clip.mp4", Key: "k2", Kind: AttachmentVideo}, {Filename: "b.jpg", Key: "k3"}},
	}, nil, writeRef)
	if err != nil {
		t.Fatal(err)
	}
	want := &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg"}}, {Text: "영상", Files: []string{"clip.mp4"}}},
		MadeWith:   []string{"a.jpg", "b.jpg", "clip.mp4"},
	}
	if !reflect.DeepEqual(answer.Storyline, want) {
		t.Fatalf("storyline = %+v, want %+v", answer.Storyline, want)
	}
}

// GEN-55: a write whose answer names no nouns still hands the post an answer — non-nil, with
// none — so the last generation's nouns are cleared, not kept.
func TestANounlessWriteClearsTheNouns(t *testing.T) {
	posts := annotatedPost()
	svc := annotatedService(posts, annotatingModels(okContent().Text))
	if err := svc.Generate(context.Background(), GenerateJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String()}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 || posts.annotations[0] == nil {
		t.Fatalf("a noun-less write handed %+v, want a non-nil answer", posts.annotations)
	}
	if got := posts.annotations[0]; got.Nouns != nil {
		t.Fatalf("a noun-less write handed %+v", got)
	}
}

// GEN-55: a revision has no nouns answer, so it keeps what the post holds by handing nil.
func TestReviseKeepsTheStoredAnnotations(t *testing.T) {
	posts := &fakePosts{input: PostInput{
		Slug: "post", UserID: "alice", Voice: liveVoice, Content: revisionContent("body"), Field: "cafe",
	}}
	models := annotatingModels(`{"title":"제목","summary":"요약","tags":["a"],"blocks":[{"type":"TEXT","content":"고친 본문"}]}`)
	svc := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
	if err := svc.Revise(context.Background(), RevisionJob{
		UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: mustRevisionPayload(t, "고쳐줘"),
	}, func(string, int, int) {}); err != nil {
		t.Fatal(err)
	}
	if len(posts.annotations) != 1 || posts.annotations[0] != nil {
		t.Fatalf("a revision handed %+v, want nil", posts.annotations)
	}
}

// A write-experiment candidate returns its whole answer, so a winner can carry its own nouns.
func TestRunWriteCandidateReturnsTheAnswer(t *testing.T) {
	posts := annotatedPost()
	svc := annotatedService(posts, annotatingModels(annotatedAnswer))
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
	if answer.Content.Title != "성수 카페 투어" || !reflect.DeepEqual(answer.Nouns, annotatedNouns) {
		t.Fatalf("candidate answer = %+v", answer)
	}
	if len(posts.contents) != 0 {
		t.Fatal("running a candidate wrote the post")
	}
}

// GEN-4: an applied winner's annotations replace the post's; one recorded before they existed
// carries none, and applying it clears them.
func TestApplyWriteWinnerForwardsTheWinnersAnnotations(t *testing.T) {
	posts := annotatedPost()
	svc := annotatedService(posts, annotatingModels(annotatedAnswer))
	snapshot, err := svc.SnapshotWriteInput(context.Background(), "alice", "post", llm.ModelRef{}, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	content := PostContent{Title: "성수 카페 투어", Blocks: []Block{{Type: BlockText, Content: "성수 카페에 갔다."}}}
	winner := WriteAnswer{Content: content, Nouns: annotatedNouns}
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
	if got := posts.annotations[0]; !reflect.DeepEqual(got.Nouns, annotatedNouns) {
		t.Fatalf("the winner handed %+v", got)
	}
	if got := posts.annotations[1]; got.Nouns != nil {
		t.Fatalf("a legacy winner handed %+v, want none", got)
	}
}
