package post

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	firstAddress  = "https://blog.naver.com/alice/223000000001"
	secondAddress = "https://blog.naver.com/alice/223000000002"
)

// publishedURLCase is one entry of the shared address fixture; a nil Stored means refused.
type publishedURLCase struct {
	Name   string  `json:"name"`
	Input  string  `json:"input"`
	Stored *string `json:"stored"`
}

// The same file the frontend pre-check runs, so the two parsers cannot disagree (POST-77).
func TestParseNaverBlogURL(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "published_url", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		MaxChars int                `json:"maxChars"`
		Cases    []publishedURLCase `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the address fixture is empty")
	}
	// The length bound is the fixture's, pinned to this side's constant as the frontend pins its own.
	if fixture.MaxChars != PublishedURLMaxChars {
		t.Fatalf("the fixture's maxChars is %d, PublishedURLMaxChars is %d", fixture.MaxChars, PublishedURLMaxChars)
	}
	prefix := "https://blog.naver.com/alice/"
	atLimit := prefix + strings.Repeat("a", fixture.MaxChars-len(prefix))
	cases := append(fixture.Cases,
		publishedURLCase{Name: "an address at the length limit", Input: atLimit, Stored: &atLimit},
		publishedURLCase{Name: "an address over the length limit", Input: atLimit + "a"},
	)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			stored, err := ParseNaverBlogURL(tc.Input)
			if tc.Stored == nil {
				if !errors.Is(err, ErrPublishedURLInvalid) || stored != "" {
					t.Fatalf("ParseNaverBlogURL(%q) = %q, %v; want a refusal", tc.Input, stored, err)
				}
				return
			}
			if err != nil || stored != *tc.Stored {
				t.Fatalf("ParseNaverBlogURL(%q) = %q, %v; want %q", tc.Input, stored, err, *tc.Stored)
			}
		})
	}
}

func TestPostStatusSetIsTheFourLifecycleValues(t *testing.T) {
	got := []string{StatusDraft, StatusReview, StatusFinalized, StatusPublished}
	if want := []string{"draft", "review", "finalized", "published"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("statuses = %v, want %v", got, want)
	}
}

// finalizedPost reaches finalized through the real path: generated content, then finalization.
func finalizedPost(t *testing.T, svc *Service, userID string) Post {
	t.Helper()
	ctx := context.Background()
	created := mustCreatePost(t, svc, userID, "제주 3일")
	content := PostContent{Title: "제주 3일 기록", Blocks: []Block{{Type: BlockText, Content: "협재 해변은 물빛이 맑았다."}}}
	if err := svc.SetGeneratedContent(ctx, userID, created.Slug, content, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	finalized, err := svc.Finalize(ctx, userID, created.Slug, 1)
	if err != nil {
		t.Fatal(err)
	}
	return finalized
}

func TestSavePublishedURLPublishesReplacesAndClears(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	clock := testNow
	svc.now = func() time.Time { return clock }

	clock = testNow.Add(time.Hour)
	published, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, "  https://m.blog.naver.com/alice/223000000001#top ")
	if err != nil {
		t.Fatal(err)
	}
	if published.Status != StatusPublished || published.PublishedURL != firstAddress || published.PublishedAt == nil || !published.PublishedAt.Equal(clock) {
		t.Fatalf("published = %s %q at %v", published.Status, published.PublishedURL, published.PublishedAt)
	}
	if published.FinalizedRevision != finalized.FinalizedRevision || !published.FinalizedAt.Equal(*finalized.FinalizedAt) {
		t.Fatal("publishing moved the finalization")
	}

	// The identical address again writes nothing and keeps the stamp.
	clock = testNow.Add(2 * time.Hour)
	before := store.posts[finalized.Slug]
	again, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, "http://blog.naver.com/alice/223000000001")
	if err != nil || !reflect.DeepEqual(store.posts[finalized.Slug], before) || !again.PublishedAt.Equal(testNow.Add(time.Hour)) {
		t.Fatalf("the same address rewrote the post: %+v, %v", again, err)
	}

	// A different one replaces it and restamps.
	replaced, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, secondAddress)
	if err != nil || replaced.Status != StatusPublished || replaced.PublishedURL != secondAddress || !replaced.PublishedAt.Equal(clock) {
		t.Fatalf("replaced = %+v, %v", replaced, err)
	}

	// Clearing returns it to finalized with the finalization untouched.
	cleared, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, "   ")
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Status != StatusFinalized || cleared.PublishedURL != "" || cleared.PublishedAt != nil ||
		cleared.FinalizedRevision != finalized.FinalizedRevision || !cleared.FinalizedAt.Equal(*finalized.FinalizedAt) {
		t.Fatalf("cleared = %s %q at %v, finalized %d at %v", cleared.Status, cleared.PublishedURL, cleared.PublishedAt, cleared.FinalizedRevision, cleared.FinalizedAt)
	}

	// An empty address on a post that is not published is a no-op.
	before = store.posts[finalized.Slug]
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, ""); err != nil || !reflect.DeepEqual(store.posts[finalized.Slug], before) {
		t.Fatalf("clearing an unpublished post wrote something: %v", err)
	}
	draft := mustCreatePost(t, svc, alice, "초안")
	if got, err := svc.SavePublishedURL(ctx, alice, draft.Slug, ""); err != nil || got.Status != StatusDraft {
		t.Fatalf("clearing a draft = %s, %v", got.Status, err)
	}
}

func TestSavePublishedURLRefusesAnInvalidAddressAndWritesNothing(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	before := store.posts[finalized.Slug]
	for _, address := range []string{
		"https://blog.naver.com.evil.com/alice/1", "https://cafe.naver.com/alice/1", "ftp://blog.naver.com/alice/1",
		"https://blog.naver.com:443/alice/1", "https://user@blog.naver.com/alice/1", "https://blog.naver.com/",
		"https://blog.naver.com/alice/" + strings.Repeat("a", PublishedURLMaxChars),
	} {
		if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, address); !errors.Is(err, ErrPublishedURLInvalid) {
			t.Errorf("%q: err = %v, want ErrPublishedURLInvalid", address, err)
		}
	}
	if !reflect.DeepEqual(store.posts[finalized.Slug], before) {
		t.Fatal("a refused address changed the post")
	}
	// No uniqueness across the account: another post may carry the same address (POST-78).
	other := finalizedPost(t, svc, alice)
	if got, err := svc.SavePublishedURL(ctx, alice, other.Slug, firstAddress); err != nil || got.PublishedURL != firstAddress {
		t.Fatalf("a second post with the same address = %+v, %v", got, err)
	}
}

func TestSavePublishedURLNeedsTheCurrentFinalizedRevision(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	draft := mustCreatePost(t, svc, alice, "초안")
	if _, err := svc.SavePublishedURL(ctx, alice, draft.Slug, firstAddress); !errors.Is(err, ErrPostNotFinalized) {
		t.Fatalf("a draft = %v", err)
	}
	review := mustCreatePost(t, svc, alice, "검토")
	if err := svc.SetGeneratedContent(ctx, alice, review.Slug, PostContent{Title: "검토", Blocks: []Block{{Type: BlockText, Content: "본문"}}}, LanguageKorean, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SavePublishedURL(ctx, alice, review.Slug, firstAddress); !errors.Is(err, ErrPostNotFinalized) {
		t.Fatalf("a review post = %v", err)
	}
	// A finalized post edited since: its finalized revision is stale.
	stale := finalizedPost(t, svc, alice)
	row := store.posts[stale.Slug]
	row.ContentRevision++
	store.posts[stale.Slug] = row
	if _, err := svc.SavePublishedURL(ctx, alice, stale.Slug, firstAddress); !errors.Is(err, ErrPostNotFinalized) {
		t.Fatalf("a stale finalization = %v", err)
	}
}

func TestSavePublishedURLIsBusyOnlyForContentWritingJobs(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)

	svc.jobs = fakeActiveJobs{finalized.Slug: {ID: "job-1", Kind: "generate", WritesContent: true}}
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("a content-writing job = %v", err)
	}
	svc.jobs = fakeActiveJobs{finalized.Slug: {ID: "job-2", Kind: "learn_voice"}}
	published, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress)
	if err != nil || published.Status != StatusPublished {
		t.Fatalf("a learning job blocked the save: %+v, %v", published, err)
	}
	// Replacing and clearing wait for a writer too.
	svc.jobs = fakeActiveJobs{finalized.Slug: {ID: "job-3", Kind: "model_experiment", WritesContent: true}}
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, secondAddress); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("replacing under a writer = %v", err)
	}
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, ""); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("clearing under a writer = %v", err)
	}
}

func TestSavePublishedURLIsOwnerScoped(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, bob, finalized.Slug, firstAddress); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another account = %v", err)
	}
	if _, err := svc.SavePublishedURL(ctx, alice, "no-such-post", firstAddress); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown slug = %v", err)
	}
	if _, err := svc.SavePublishedURL(ctx, bob, finalized.Slug, ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("another account clearing = %v", err)
	}
}

func TestLearningSnapshotReadsAPublishedPost(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	want, err := svc.LearningSnapshot(ctx, alice, finalized.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	got, err := svc.LearningSnapshot(ctx, alice, finalized.Slug)
	if err != nil {
		t.Fatalf("a published post refused learning: %v", err)
	}
	// Publishing restamps updated_at and moves the row's status, which the service's rule reads
	// and the learner does not; nothing the learner reads changes.
	want.UpdatedAt, got.UpdatedAt = time.Time{}, time.Time{}
	want.Status = StatusPublished
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("snapshot of the published post =\n%+v\nwant\n%+v", got, want)
	}
}

func TestDeletePostRemovesAPublishedPostAndStillWaitsForAnyJob(t *testing.T) {
	svc, store, _ := newTestService(t)
	ctx := context.Background()
	finalized := finalizedPost(t, svc, alice)
	if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
		t.Fatal(err)
	}
	listed, err := svc.List(ctx, alice)
	if err != nil || len(listed) != 1 || listed[0].Status != StatusPublished {
		t.Fatalf("list = %+v, %v", listed, err)
	}
	// A delete waits for ANY job, a learning one included (POST-29).
	svc.jobs = fakeActiveJobs{finalized.Slug: {ID: "job-1", Kind: "learn_voice"}}
	if err := svc.DeletePost(ctx, alice, finalized.Slug); !errors.Is(err, ErrPostBusy) {
		t.Fatalf("delete under a job = %v", err)
	}
	svc.jobs = neutralJobs{}
	if err := svc.DeletePost(ctx, alice, finalized.Slug); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.posts[finalized.Slug]; ok {
		t.Fatal("the published post survived its delete")
	}
}

func TestPublishedPostsAreNewestFirstAndBounded(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	clock := testNow
	svc.now = func() time.Time { return clock }
	var slugs []string
	for i := 0; i < 3; i++ {
		finalized := finalizedPost(t, svc, alice)
		clock = testNow.Add(time.Duration(i+1) * time.Hour)
		if _, err := svc.SavePublishedURL(ctx, alice, finalized.Slug, firstAddress); err != nil {
			t.Fatal(err)
		}
		slugs = append(slugs, finalized.Slug)
	}
	unpublished := finalizedPost(t, svc, alice)
	theirs := finalizedPost(t, svc, bob)
	if _, err := svc.SavePublishedURL(ctx, bob, theirs.Slug, "https://blog.naver.com/bob/1"); err != nil {
		t.Fatal(err)
	}

	window, err := svc.PublishedPosts(ctx, alice, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(window) != 2 || window[0].Slug != slugs[2] || window[1].Slug != slugs[1] {
		t.Fatalf("window = %+v, want the two newest of %v", window, slugs)
	}
	if !window[0].PublishedAt.Equal(testNow.Add(3*time.Hour)) || window[0].Content.Title != "제주 3일 기록" || window[0].ContentRevision != 1 {
		t.Fatalf("window[0] = %+v", window[0])
	}
	all, err := svc.PublishedPosts(ctx, alice, 100)
	if err != nil || len(all) != 3 {
		t.Fatalf("the whole window = %+v, %v", all, err)
	}
	for _, p := range all {
		if p.Slug == unpublished.Slug || p.Slug == theirs.Slug {
			t.Fatalf("the window holds %s", p.Slug)
		}
	}
	if _, err := svc.PublishedPosts(ctx, alice, 0); err == nil {
		t.Fatal("a zero limit was accepted")
	}
}
