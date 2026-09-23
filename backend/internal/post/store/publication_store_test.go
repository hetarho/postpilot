package store_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
	"github.com/postpilot/backend/internal/post/store"
)

const (
	firstAddress  = "https://blog.naver.com/alice/223000000001"
	secondAddress = "https://blog.naver.com/alice/223000000002"
)

// finalizedRow reaches finalized through the real statements: machine content, then the
// finalization of revision 1.
func finalizedRow(t *testing.T, s *store.Store, slug, userID string) {
	t.Helper()
	ctx := context.Background()
	seedPost(t, s, slug, userID, testNow)
	content := post.PostContent{Title: "제주 3일", Blocks: []post.Block{{Type: post.BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if updated, err := s.UpdateGeneratedContent(ctx, slug, userID, content, post.LanguageKorean, testNow); err != nil || !updated {
		t.Fatalf("machine save: updated=%v err=%v", updated, err)
	}
	if updated, err := s.Finalize(ctx, slug, userID, "제주 3일", 1, testNow.Add(time.Minute)); err != nil || !updated {
		t.Fatalf("finalize: updated=%v err=%v", updated, err)
	}
}

func TestPublishReplaceAndClearRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	finalizedRow(t, s, "jeju", "alice")

	at := time.Date(2026, 9, 24, 21, 30, 45, 123456789, time.FixedZone("KST", 9*3600))
	if ok, err := s.PublishPost(ctx, "jeju", "alice", firstAddress, at); err != nil || !ok {
		t.Fatalf("publish: ok=%v err=%v", ok, err)
	}
	got, err := s.GetPost(ctx, "jeju")
	if err != nil || got.Status != post.StatusPublished || got.PublishedURL != firstAddress || got.PublishedAt == nil || !got.PublishedAt.Equal(at) {
		t.Fatalf("published row = %s %q at %v, err %v", got.Status, got.PublishedURL, got.PublishedAt, err)
	}
	if got.FinalizedRevision != 1 || got.FinalizedAt == nil {
		t.Fatal("publishing moved the finalization")
	}

	later := at.Add(time.Hour)
	if ok, err := s.PublishPost(ctx, "jeju", "alice", secondAddress, later); err != nil || !ok {
		t.Fatalf("replace: ok=%v err=%v", ok, err)
	}
	if got, _ := s.GetPost(ctx, "jeju"); got.PublishedURL != secondAddress || !got.PublishedAt.Equal(later) {
		t.Fatalf("replaced row = %q at %v", got.PublishedURL, got.PublishedAt)
	}

	if ok, err := s.UnpublishPost(ctx, "jeju", "alice", later.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("clear: ok=%v err=%v", ok, err)
	}
	got, _ = s.GetPost(ctx, "jeju")
	if got.Status != post.StatusFinalized || got.PublishedURL != "" || got.PublishedAt != nil || got.FinalizedRevision != 1 || got.FinalizedAt == nil {
		t.Fatalf("cleared row = %s %q at %v, finalized %d", got.Status, got.PublishedURL, got.PublishedAt, got.FinalizedRevision)
	}
	// Clearing what is no longer published changes nothing.
	if ok, err := s.UnpublishPost(ctx, "jeju", "alice", later); err != nil || ok {
		t.Fatalf("a second clear: ok=%v err=%v", ok, err)
	}
}

func TestPublishRefusesADraftAStaleFinalizationAndAnotherAccount(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	seedPost(t, s, "draft", "alice", testNow)
	if ok, err := s.PublishPost(ctx, "draft", "alice", firstAddress, testNow); err != nil || ok {
		t.Fatalf("a draft published: ok=%v err=%v", ok, err)
	}
	finalizedRow(t, s, "stale", "alice")
	edited := post.PostContent{Title: "고친 글", Blocks: []post.Block{{Type: post.BlockText, Content: "고친 문장"}}}
	if ok, err := s.SaveContent(ctx, "stale", "alice", edited, 1, testNow.Add(2*time.Minute)); err != nil || !ok {
		t.Fatalf("edit: ok=%v err=%v", ok, err)
	}
	if ok, err := s.PublishPost(ctx, "stale", "alice", firstAddress, testNow); err != nil || ok {
		t.Fatalf("a stale finalization published: ok=%v err=%v", ok, err)
	}
	finalizedRow(t, s, "hers", "alice")
	if ok, err := s.PublishPost(ctx, "hers", "bob", firstAddress, testNow); err != nil || ok {
		t.Fatalf("another account published: ok=%v err=%v", ok, err)
	}
	for _, slug := range []string{"draft", "stale", "hers"} {
		if got, _ := s.GetPost(ctx, slug); got.Status == post.StatusPublished || got.PublishedURL != "" {
			t.Errorf("%s = %s %q", slug, got.Status, got.PublishedURL)
		}
	}
}

func TestPublishedWindowIsNewestFirstOwnerScopedAndBounded(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	for i, slug := range []string{"a-first", "b-second", "c-third"} {
		finalizedRow(t, s, slug, "alice")
		if ok, err := s.PublishPost(ctx, slug, "alice", firstAddress, testNow.Add(time.Duration(i)*time.Hour)); err != nil || !ok {
			t.Fatalf("publish %s: ok=%v err=%v", slug, ok, err)
		}
	}
	// Same instant: the slug breaks the tie, descending.
	finalizedRow(t, s, "d-tied", "alice")
	if ok, err := s.PublishPost(ctx, "d-tied", "alice", firstAddress, testNow.Add(2*time.Hour)); err != nil || !ok {
		t.Fatalf("publish tie: ok=%v err=%v", ok, err)
	}
	finalizedRow(t, s, "unpublished", "alice")
	finalizedRow(t, s, "theirs", "bob")
	if ok, err := s.PublishPost(ctx, "theirs", "bob", "https://blog.naver.com/bob/1", testNow.Add(9*time.Hour)); err != nil || !ok {
		t.Fatalf("publish theirs: ok=%v err=%v", ok, err)
	}

	window, err := s.ListPublishedPosts(ctx, "alice", 3)
	if err != nil {
		t.Fatal(err)
	}
	var slugs []string
	for _, p := range window {
		slugs = append(slugs, p.Slug)
	}
	if want := []string{"d-tied", "c-third", "b-second"}; !reflect.DeepEqual(slugs, want) {
		t.Fatalf("window = %v, want %v", slugs, want)
	}
	first := window[0]
	if first.ContentRevision != 1 || first.Content.Title != "제주 3일" || first.ContentLanguage == nil || *first.ContentLanguage != post.LanguageKorean || !first.PublishedAt.Equal(testNow.Add(2*time.Hour)) {
		t.Fatalf("window[0] = %+v", first)
	}
	if all, err := s.ListPublishedPosts(ctx, "alice", 100); err != nil || len(all) != 4 {
		t.Fatalf("the whole window = %d rows, %v", len(all), err)
	}
}

// NULL and an empty array both mean "the write pass returned none"; an array reads as it is.
func TestContentNounsDecodeThroughGetPostAndTheWindow(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	finalizedRow(t, s, "jeju", "alice")
	if ok, err := s.PublishPost(ctx, "jeju", "alice", firstAddress, testNow); err != nil || !ok {
		t.Fatalf("publish: ok=%v err=%v", ok, err)
	}
	for _, test := range []struct {
		stored any
		want   []string
	}{
		{nil, nil},
		{`[]`, nil},
		{`["협재","해변"]`, []string{"협재", "해변"}},
	} {
		if _, err := handle.Writer.Exec(`UPDATE posts SET content_nouns = ? WHERE slug = 'jeju'`, test.stored); err != nil {
			t.Fatal(err)
		}
		got, err := s.GetPost(ctx, "jeju")
		if err != nil || !reflect.DeepEqual(got.ContentNouns, test.want) {
			t.Errorf("GetPost nouns for %v = %q, %v; want %q", test.stored, got.ContentNouns, err, test.want)
		}
		window, err := s.ListPublishedPosts(ctx, "alice", 1)
		if err != nil || len(window) != 1 || !reflect.DeepEqual(window[0].Nouns, test.want) {
			t.Errorf("window nouns for %v = %+v, %v; want %q", test.stored, window, err, test.want)
		}
	}
}

func TestAPublishedRowIsLearnableAndListed(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	finalizedRow(t, s, "jeju", "alice")
	if ok, err := s.PublishPost(ctx, "jeju", "alice", firstAddress, testNow.Add(time.Hour)); err != nil || !ok {
		t.Fatalf("publish: ok=%v err=%v", ok, err)
	}
	snapshot, err := s.LearningSnapshot(ctx, "jeju", "alice")
	if err != nil || snapshot.ContentRevision != 1 || snapshot.Current.Title != "제주 3일" {
		t.Fatalf("snapshot of a published row = %+v, %v", snapshot, err)
	}
	listed, err := s.ListPosts(ctx, "alice")
	if err != nil || len(listed) != 1 || listed[0].Status != post.StatusPublished {
		t.Fatalf("list = %+v, %v", listed, err)
	}
}

// Deleting a published post takes its measurement row with it (POST-85).
func TestDeletingAPublishedPostCascadesToItsMeasurement(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	finalizedRow(t, s, "jeju", "alice")
	if ok, err := s.PublishPost(ctx, "jeju", "alice", firstAddress, testNow); err != nil || !ok {
		t.Fatalf("publish: ok=%v err=%v", ok, err)
	}
	if _, err := handle.Writer.Exec(`INSERT INTO post_measurements(post_slug, user_id, content_revision, measure_version,
		char_count, photo_count, distinct_block_types, computed_at) VALUES('jeju', 'alice', 1, 1, 14, 0, 1, ?)`, testNow.Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if deleted, err := s.DeletePost(ctx, "jeju", "alice"); err != nil || !deleted {
		t.Fatalf("delete: deleted=%v err=%v", deleted, err)
	}
	var left int
	if err := handle.Reader.QueryRow(`SELECT count(*) FROM post_measurements`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("measurements left = %d, %v", left, err)
	}
}
