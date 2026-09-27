package post

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

// storylinePost is a post with two photos, a video and a storyline made with the photos and the
// video, plus a photo attached after it was made.
func storylineFixture(t *testing.T) (*Service, *fakeBlobs, Post) {
	t.Helper()
	svc, _, blobs := newTestService(t)
	ctx := context.Background()
	p := mustCreatePost(t, svc, alice, "성수")
	attachPhoto(t, svc, blobs, p.Slug, "a.jpg")
	attachPhoto(t, svc, blobs, p.Slug, "b.jpg")
	mustAttachVideo(t, svc, blobs, alice, p.Slug, "clip.mp4", 5_000)
	content := PostContent{Title: "성수 카페", Blocks: []Block{{Type: BlockText, Content: "본문"}}}
	if err := svc.SetGeneratedContent(ctx, alice, p.Slug, content, LanguageKorean, &WriteAnnotations{Storyline: &Storyline{
		Paragraphs: []StorylineParagraph{{Text: "가게 앞", Files: []string{"a.jpg", "clip.mp4"}}, {Text: "커피", Files: []string{"b.jpg"}}},
		MadeWith:   []string{"a.jpg", "b.jpg", "clip.mp4"},
	}}); err != nil {
		t.Fatal(err)
	}
	attachPhoto(t, svc, blobs, p.Slug, "later.jpg")
	found, err := svc.Get(ctx, alice, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return svc, blobs, found
}

func saveStoryline(svc *Service, p Post, paragraphs ...StorylineParagraph) (Post, error) {
	return svc.SaveDraft(context.Background(), alice, DraftSave{
		Slug: p.Slug, Title: p.Title, Memo: p.Memo, Storyline: &StorylineEdit{Paragraphs: paragraphs},
	})
}

// POST-96: the owner's edit replaces the texts and files, marks the storyline edited by hand, and
// changes no status, revision or baseline; an identical save sets no mark.
func TestADraftSaveEditsTheStorylineByHand(t *testing.T) {
	svc, _, p := storylineFixture(t)
	same, err := saveStoryline(svc, p, p.Storyline.Paragraphs...)
	if err != nil || same.Storyline.EditedByHand {
		t.Fatalf("an identical save = %+v, %v", same.Storyline, err)
	}
	saved, err := saveStoryline(svc, p,
		StorylineParagraph{Text: "가게 앞과 영상", Files: []string{"clip.mp4"}},
		StorylineParagraph{Text: "커피와 간판", Files: []string{"b.jpg", "a.jpg"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := &Storyline{
		Paragraphs:   []StorylineParagraph{{Text: "가게 앞과 영상", Files: []string{"clip.mp4"}}, {Text: "커피와 간판", Files: []string{"b.jpg", "a.jpg"}}},
		EditedByHand: true,
		MadeWith:     []string{"a.jpg", "b.jpg", "clip.mp4"},
	}
	if !reflect.DeepEqual(saved.Storyline, want) {
		t.Fatalf("storyline = %+v, want %+v", saved.Storyline, want)
	}
	if saved.ContentRevision != p.ContentRevision || saved.MachineBaselineRevision != p.MachineBaselineRevision || saved.Status != p.Status {
		t.Fatalf("the edit moved the post: %+v", saved)
	}
	// Taking a file out is an edit too; it then reads as taken out.
	out, err := saveStoryline(svc, saved,
		StorylineParagraph{Text: "가게 앞과 영상", Files: []string{"clip.mp4"}},
		StorylineParagraph{Text: "커피와 간판", Files: []string{"b.jpg"}},
	)
	if err != nil || out.Storyline.TakenOutFiles([]string{"a.jpg", "b.jpg", "clip.mp4", "later.jpg"})[0] != "a.jpg" {
		t.Fatalf("taking a.jpg out = %+v, %v", out.Storyline, err)
	}
}

// POST-96, POST-74: an edit is refused whole — while a job targets the post, on a published post,
// with another paragraph count, a file in two paragraphs, a paragraph past the ceiling, or a file
// the storyline was not made with or that is no longer attached — and nothing else in the save
// is applied.
func TestADraftSaveRefusesAStorylineEditItCannotTake(t *testing.T) {
	long := strings.Repeat("가", StorylineTextMaxChars+1)
	var fileUnknown *StorylineFileUnknownError
	var tooLong *StorylineTextTooLongError
	for name, test := range map[string]struct {
		paragraphs []StorylineParagraph
		busy       bool
		check      func(error) bool
	}{
		"a job targets the post": {
			paragraphs: []StorylineParagraph{{Text: "하나"}, {Text: "둘"}}, busy: true,
			check: func(err error) bool { return errors.Is(err, ErrPostBusy) },
		},
		"another count": {
			paragraphs: []StorylineParagraph{{Text: "하나"}},
			check:      func(err error) bool { return errors.Is(err, ErrStorylineInvalid) },
		},
		"a file in two paragraphs": {
			paragraphs: []StorylineParagraph{{Text: "하나", Files: []string{"a.jpg"}}, {Text: "둘", Files: []string{"a.jpg"}}},
			check:      func(err error) bool { return errors.Is(err, ErrStorylineInvalid) },
		},
		"past the ceiling": {
			paragraphs: []StorylineParagraph{{Text: long}, {Text: "둘"}},
			check:      func(err error) bool { return errors.As(err, &tooLong) && tooLong.Max == StorylineTextMaxChars },
		},
		"attached after it was made": {
			paragraphs: []StorylineParagraph{{Text: "하나", Files: []string{"later.jpg"}}, {Text: "둘"}},
			check:      func(err error) bool { return errors.As(err, &fileUnknown) && fileUnknown.File == "later.jpg" },
		},
		"not attached": {
			paragraphs: []StorylineParagraph{{Text: "하나", Files: []string{"ghost.jpg"}}, {Text: "둘"}},
			check:      func(err error) bool { return errors.As(err, &fileUnknown) && fileUnknown.File == "ghost.jpg" },
		},
	} {
		svc, _, p := storylineFixture(t)
		if test.busy {
			svc.jobs = fakeActiveJobs{p.Slug: {ID: "job-1", Status: "running"}}
		}
		_, err := svc.SaveDraft(context.Background(), alice, DraftSave{
			Slug: p.Slug, Title: "바뀐 제목", Memo: p.Memo, Storyline: &StorylineEdit{Paragraphs: test.paragraphs},
		})
		if !test.check(err) {
			t.Errorf("%s: err = %v", name, err)
		}
		svc.jobs = neutralJobs{}
		after, _ := svc.Get(context.Background(), alice, p.Slug)
		if after.Title != p.Title || !reflect.DeepEqual(after.Storyline, p.Storyline) {
			t.Errorf("%s: a refused save changed the post: title %q, storyline %+v", name, after.Title, after.Storyline)
		}
	}

	// A post with no storyline, the one being created included, has none to edit.
	svc, _, _ := newTestService(t)
	plain := mustCreatePost(t, svc, alice, "그냥")
	if _, err := saveStoryline(svc, plain, StorylineParagraph{Text: "하나"}); !errors.Is(err, ErrStorylineMissing) {
		t.Fatalf("an edit without a storyline: %v", err)
	}
	language := LanguageKorean
	voice := defaultVoiceFor(alice)
	if _, err := svc.SaveDraft(context.Background(), alice, DraftSave{
		Title: "새 글", VoiceID: &voice, TargetLanguage: &language, Storyline: &StorylineEdit{},
	}); !errors.Is(err, ErrStorylineMissing) {
		t.Fatalf("a create with a storyline edit: %v", err)
	}
}
