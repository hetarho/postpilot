package post

import (
	"context"
	"reflect"
	"testing"
)

// POST-99, GEN-71: a write stores its storyline with its content, never as an owner edit; a
// revision (nil annotations) and an older candidate (nil storyline) keep it; a write that
// planned no paragraph leaves none.
func TestAWriteStoresItsStorylineAndARevisionKeepsIt(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "성수")
	content := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	written := &Storyline{
		Paragraphs:   []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"a.jpg"}}, {Text: "마무리", Files: []string{}}},
		EditedByHand: true,
		MadeWith:     []string{"a.jpg"},
	}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, &WriteAnnotations{Storyline: written}); err != nil {
		t.Fatal(err)
	}
	want := &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞을 보여줍니다.", Files: []string{"a.jpg"}}, {Text: "마무리"}},
		MadeWith:   []string{"a.jpg"},
	}
	stored := func() *Storyline {
		t.Helper()
		found, err := svc.Get(ctx, alice, p.Slug)
		if err != nil {
			t.Fatal(err)
		}
		return found.Storyline
	}
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("stored %+v, want %+v", got, want)
	}
	// The same write again is the idempotent no-op it always was.
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, &WriteAnnotations{Storyline: written}); err != nil {
		t.Fatalf("an identical retry: %v", err)
	}

	revised := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "고친 본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, revised, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a revision moved the storyline to %+v", got)
	}
	older := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "비교 후보"}}}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, older, LanguageKorean, &WriteAnnotations{Nouns: []string{"성수"}}); err != nil {
		t.Fatal(err)
	}
	if got := stored(); !reflect.DeepEqual(got, want) {
		t.Fatalf("a candidate without a storyline moved it to %+v", got)
	}

	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, &WriteAnnotations{Storyline: &Storyline{MadeWith: []string{"a.jpg"}}}); err != nil {
		t.Fatal(err)
	}
	if got := stored(); got != nil {
		t.Fatalf("a write with no paragraph left %+v, want none", got)
	}
}

// POST-18: deleting a photo or a video takes its name out of every paragraph and out of
// MadeWith, with its observation; the paragraphs stay.
func TestDeletingAnAttachmentTakesItsNameOutOfTheStoryline(t *testing.T) {
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "성수")
	photo := attachPhoto(t, svc, blobs, p.Slug, "a.jpg")
	attachPhoto(t, svc, blobs, p.Slug, "b.jpg")
	video := mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)
	if err := svc.SetObservations(ctx, alice, p.Slug, []Observation{{File: "a.jpg", Scene: "가게"}, {File: "b.jpg", Scene: "커피"}}); err != nil {
		t.Fatal(err)
	}
	content := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, &WriteAnnotations{Storyline: &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg", "clip.mp4"}}, {Text: "커피", Files: []string{"b.jpg"}}},
		MadeWith:   []string{"a.jpg", "b.jpg", "clip.mp4"},
	}}); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteImage(ctx, alice, photo.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteVideo(ctx, alice, video.ID); err != nil {
		t.Fatal(err)
	}
	found, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	want := &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞"}, {Text: "커피", Files: []string{"b.jpg"}}},
		MadeWith:   []string{"b.jpg"},
	}
	if !reflect.DeepEqual(found.Storyline, want) {
		t.Fatalf("storyline after the deletes = %+v, want %+v", found.Storyline, want)
	}
	if len(found.Observations) != 1 || found.Observations[0].File != "b.jpg" {
		t.Fatalf("observations after the deletes = %+v", found.Observations)
	}
}

// POST-99: added is what the write was not shown; taken out is what it was shown, still
// attached, that no paragraph holds. A name gone from the post is neither.
func TestStorylineAddedAndTakenOutFiles(t *testing.T) {
	storyline := Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg"}}, {Text: "커피"}},
		MadeWith:   []string{"a.jpg", "b.jpg", "gone.jpg"},
	}
	attached := []string{"a.jpg", "b.jpg", "new.jpg", "clip.mp4"}
	if got := storyline.AddedFiles(attached); !reflect.DeepEqual(got, []string{"new.jpg", "clip.mp4"}) {
		t.Errorf("added = %v", got)
	}
	if got := storyline.TakenOutFiles(attached); !reflect.DeepEqual(got, []string{"b.jpg"}) {
		t.Errorf("taken out = %v", got)
	}
}

// GEN-68, GEN-69: a storyline job's answer replaces the storyline and nothing else; it is never
// an owner edit, and an answer with no paragraph leaves none.
func TestSetStorylineReplacesTheStorylineAlone(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "성수")
	content := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	before, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetStoryline(ctx, alice, p.Slug, Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{}}}, EditedByHand: true, MadeWith: []string{},
	}); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after.Storyline, &Storyline{Paragraphs: []StorylineParagraph{{Text: "가게 앞"}}}) {
		t.Fatalf("storyline = %+v", after.Storyline)
	}
	if after.ContentRevision != before.ContentRevision || after.Status != before.Status || !reflect.DeepEqual(after.Content, before.Content) {
		t.Fatalf("the storyline write moved the post: %+v", after)
	}
	if err := svc.SetStoryline(ctx, alice, p.Slug, Storyline{MadeWith: []string{"a.jpg"}}); err != nil {
		t.Fatal(err)
	}
	if cleared, _ := svc.Get(ctx, alice, p.Slug); cleared.Storyline != nil {
		t.Fatalf("an answer with no paragraph left %+v", cleared.Storyline)
	}
}
