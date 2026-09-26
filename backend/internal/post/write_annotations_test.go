package post

import (
	"context"
	"reflect"
	"testing"
)

var (
	annotatedContent = PostContent{
		Title: "성수 카페 투어",
		Tags:  []string{"성수 카페", "라떼"},
		Blocks: []Block{
			{Type: BlockText, Content: "분위기 좋은 성수 카페를 찾았다."},
			{Type: BlockText, Content: "라떼가 맛있었다."},
		},
	}
	annotations = WriteAnnotations{Nouns: []string{"성수", "카페", "라떼"}}
)

// generatedWith writes the annotated content as a generation would and returns the post.
func generatedWith(t *testing.T, svc *Service, userID string, value *WriteAnnotations) Post {
	t.Helper()
	ctx := context.Background()
	created := mustCreatePost(t, svc, userID, "성수")
	if err := svc.SetGeneratedContent(ctx, userID, created.Slug, annotatedContent, LanguageKorean, value); err != nil {
		t.Fatal(err)
	}
	found, err := svc.Get(ctx, userID, created.Slug)
	if err != nil {
		t.Fatal(err)
	}
	return found
}

// GEN-55: a generation's nouns are stored beside its content, and a later generation's replace
// them — none included, so stale ones never outlive their content.
func TestAGeneratedWriteStoresItsNouns(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)
	if !reflect.DeepEqual(found.ContentNouns, annotations.Nouns) {
		t.Fatalf("stored nouns %v", found.ContentNouns)
	}

	// A later generation with no nouns clears them.
	next := PostContent{Title: "다른 글", Blocks: []Block{{Type: BlockText, Content: "새로 쓴 글."}}}
	if err := svc.SetGeneratedContent(context.Background(), "alice", found.Slug, next, LanguageKorean, &WriteAnnotations{}); err != nil {
		t.Fatal(err)
	}
	cleared, err := svc.Get(context.Background(), "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.ContentNouns != nil {
		t.Fatalf("a write with none kept nouns %v", cleared.ContentNouns)
	}
}

// GEN-55, R16: a revision has no nouns answer, so what the generation said stands, byte for
// byte, while the content moves on.
func TestARevisionKeepsTheNouns(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)

	revised := annotatedContent
	revised.Blocks = []Block{{Type: BlockText, Content: "고쳐 쓴 문장."}}
	if err := svc.SetGeneratedContent(context.Background(), "alice", found.Slug, revised, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	after, err := svc.Get(context.Background(), "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentRevision != found.ContentRevision+1 {
		t.Fatalf("revision %d, want %d", after.ContentRevision, found.ContentRevision+1)
	}
	if !reflect.DeepEqual(after.ContentNouns, annotations.Nouns) {
		t.Fatalf("a revision changed nouns %v", after.ContentNouns)
	}
}

// A manual edit never touches the nouns: they are the last write's, not the content's.
func TestAManualSaveKeepsTheNouns(t *testing.T) {
	svc, _, _ := newTestService(t)
	found := generatedWith(t, svc, "alice", &annotations)

	edited := annotatedContent
	edited.Title = "성수동 카페 투어"
	saved, err := svc.SaveContent(context.Background(), "alice", found.Slug, edited, found.ContentRevision)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContentRevision != found.ContentRevision+1 {
		t.Fatalf("revision %d", saved.ContentRevision)
	}
	if !reflect.DeepEqual(saved.ContentNouns, annotations.Nouns) {
		t.Fatalf("a manual save changed nouns %v", saved.ContentNouns)
	}
}

// A retried identical write is a no-op, and "identical" includes what the write said beside the
// content: the same content with different nouns is a new machine write.
func TestAnIdenticalRetriedWriteKeepsThemStored(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	found := generatedWith(t, svc, "alice", &annotations)

	retry := annotations
	if err := svc.SetGeneratedContent(ctx, "alice", found.Slug, annotatedContent, LanguageKorean, &retry); err != nil {
		t.Fatal(err)
	}
	same, err := svc.Get(ctx, "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if same.ContentRevision != found.ContentRevision || !reflect.DeepEqual(same.ContentNouns, annotations.Nouns) {
		t.Fatalf("an identical retry moved the post: revision %d nouns %v", same.ContentRevision, same.ContentNouns)
	}

	changed := WriteAnnotations{Nouns: []string{"성수"}}
	if err := svc.SetGeneratedContent(ctx, "alice", found.Slug, annotatedContent, LanguageKorean, &changed); err != nil {
		t.Fatal(err)
	}
	rewritten, err := svc.Get(ctx, "alice", found.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if rewritten.ContentRevision != found.ContentRevision+1 || !reflect.DeepEqual(rewritten.ContentNouns, changed.Nouns) {
		t.Fatalf("other nouns were not a new write: revision %d nouns %v", rewritten.ContentRevision, rewritten.ContentNouns)
	}
}
