package post

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// listSummaries unwraps a page for the tests that only read the rows.
func listSummaries(page ListPage, err error) ([]Summary, error) { return page.Summaries, err }

// seedListed puts a post straight into the fake store, so a list test controls updated_at,
// the status and the content without driving the lifecycle.
func seedListed(store *fakeStore, userID, slug, title, status string, updatedAt time.Time, tags ...string) {
	store.mu.Lock()
	defer store.mu.Unlock()
	p := Post{Slug: slug, UserID: userID, VoiceID: aliceVoice, Title: title, Status: status, UpdatedAt: updatedAt, TargetLanguage: LanguageKorean}
	if tags != nil {
		p.Content = &PostContent{Title: title, Tags: tags}
	}
	store.posts[slug] = p
}

func slugsOf(summaries []Summary) []string {
	out := make([]string, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, s.Slug)
	}
	return out
}

// walk follows the tokens from the first page to the last and returns every slug in order.
func walk(t *testing.T, svc *Service, q ListQuery) []string {
	t.Helper()
	var all []string
	for range 100 {
		page, err := svc.List(context.Background(), alice, q)
		if err != nil {
			t.Fatalf("List(%+v): %v", q, err)
		}
		if q.PageSize > 0 && len(page.Summaries) > q.PageSize {
			t.Fatalf("page holds %d rows, size %d", len(page.Summaries), q.PageSize)
		}
		all = append(all, slugsOf(page.Summaries)...)
		if page.NextPageToken == "" {
			return all
		}
		q.PageToken = page.NextPageToken
	}
	t.Fatal("the walk never reached a last page")
	return nil
}

// POST-90: a walk returns every post once, newest first, and ties on updated_at are broken by
// the slug the same way on every page — a tie split across two pages must not repeat or drop.
func TestListWalksEveryPostOnceAcrossTies(t *testing.T) {
	svc, store, _ := newTestService(t)
	for i := range 7 {
		// Pairs share a timestamp, so page boundaries fall inside ties.
		seedListed(store, alice, fmt.Sprintf("p%d", i), fmt.Sprintf("글 %d", i), StatusDraft, testNow.Add(time.Duration(i/2)*time.Minute))
	}
	seedListed(store, bob, "theirs", "남의 글", StatusDraft, testNow.Add(time.Hour))

	want := []string{"p6", "p5", "p4", "p3", "p2", "p1", "p0"}
	for _, size := range []int{1, 2, 3, 7, 50} {
		if got := walk(t, svc, ListQuery{PageSize: size}); !reflect.DeepEqual(got, want) {
			t.Errorf("walk at size %d = %v, want %v", size, got, want)
		}
	}
}

// A client that predates paging sends nothing and still gets the whole list at once.
func TestListUnpagedAnswersEverythingWithoutAToken(t *testing.T) {
	svc, store, _ := newTestService(t)
	for i := range 120 {
		seedListed(store, alice, fmt.Sprintf("p%03d", i), "글", StatusDraft, testNow.Add(time.Duration(i)*time.Second))
	}
	page, err := svc.List(context.Background(), alice, ListQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Summaries) != 120 || page.NextPageToken != "" {
		t.Fatalf("unpaged = %d rows, token %q; want 120 and none", len(page.Summaries), page.NextPageToken)
	}
}

func TestListClampsThePageSize(t *testing.T) {
	svc, store, _ := newTestService(t)
	for i := range listPageSizeMax + 5 {
		seedListed(store, alice, fmt.Sprintf("p%03d", i), "글", StatusDraft, testNow.Add(time.Duration(i)*time.Second))
	}
	page, err := svc.List(context.Background(), alice, ListQuery{PageSize: 10_000})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Summaries) != listPageSizeMax || page.NextPageToken == "" {
		t.Fatalf("clamped page = %d rows, token %q; want %d and a next token", len(page.Summaries), page.NextPageToken, listPageSizeMax)
	}
}

// POST-65's normalization, pinned against the browser's: the cases the rule names plus the
// ones where the browser's order of steps shows.
func TestNormalizeListTextMatchesTheBrowser(t *testing.T) {
	for in, want := range map[string]string{
		"제주":             "제주",
		"  제주  ":         "제주",
		"#제주":            "제주",
		"##제주":           "제주",
		"제주   3일":        "제주 3일",
		"제주\t\n3일":       "제주 3일",
		"JEJU":           "jeju",
		"# 제주":           " 제주",
		"#":              "",
		"   ":            "",
		"제주\u3000카페":     "제주 카페",
		"\u00a0제주\u00a0": "제주",
	} {
		if got := normalizeListText(in); got != want {
			t.Errorf("normalizeListText(%q) = %q, want %q", in, got, want)
		}
	}
}

// POST-65 and POST-91: the search runs over every owned post, by the title as listed or any
// tag, and composes with the status as AND.
func TestListNarrowsEveryOwnedPostOnTheServer(t *testing.T) {
	svc, store, _ := newTestService(t)
	seedListed(store, alice, "jeju-trip", "제주 3일", StatusReview, testNow, "여행")
	seedListed(store, alice, "busan", "부산 밥상", StatusDraft, testNow.Add(-time.Minute), "제주", "맛집")
	seedListed(store, alice, "seoul", "서울 카페", StatusReview, testNow.Add(-2*time.Minute), "카페")
	seedListed(store, alice, "untitled", "", StatusDraft, testNow.Add(-3*time.Minute))
	seedListed(store, bob, "bob-jeju", "제주 사진", StatusReview, testNow.Add(time.Hour))
	// The 가제 is blank here, so the list shows the content's title and the search reads it.
	store.posts["untitled"] = func() Post {
		p := store.posts["untitled"]
		p.Content = &PostContent{Title: "JEJU 노을"}
		return p
	}()

	for _, tc := range []struct {
		name  string
		query ListQuery
		want  []string
	}{
		{"title or tag", ListQuery{Query: "제주"}, []string{"jeju-trip", "busan"}},
		{"a hash names a tag", ListQuery{Query: "#맛집"}, []string{"busan"}},
		{"case and whitespace", ListQuery{Query: "  jeju  "}, []string{"untitled"}},
		{"status alone", ListQuery{Status: StatusReview}, []string{"jeju-trip", "seoul"}},
		{"query AND status", ListQuery{Query: "제주", Status: StatusReview}, []string{"jeju-trip"}},
		{"a query that normalizes away narrows nothing", ListQuery{Query: " # "}, []string{"jeju-trip", "busan", "seoul", "untitled"}},
		{"no match", ListQuery{Query: "없는말"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := walk(t, svc, tc.query)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("slugs = %v, want %v", got, tc.want)
			}
		})
	}
}

// POST-91: a match further down than any page is found on the first page asked for, and the
// pages of a narrowed walk still end.
func TestListSearchFindsAMatchPastEveryPage(t *testing.T) {
	svc, store, _ := newTestService(t)
	for i := range 45 {
		seedListed(store, alice, fmt.Sprintf("p%02d", i), "일상", StatusDraft, testNow.Add(time.Duration(i)*time.Minute), "일상")
	}
	seedListed(store, alice, "oldest", "제주 첫 글", StatusDraft, testNow.Add(-time.Hour))
	seedListed(store, alice, "older", "부산", StatusDraft, testNow.Add(-2*time.Hour), "제주")

	page, err := svc.List(context.Background(), alice, ListQuery{PageSize: 1, Query: "제주"})
	if err != nil {
		t.Fatal(err)
	}
	if got := slugsOf(page.Summaries); !reflect.DeepEqual(got, []string{"oldest"}) || page.NextPageToken == "" {
		t.Fatalf("first narrowed page = %v token %q; want [oldest] and a next token", got, page.NextPageToken)
	}
	if got := walk(t, svc, ListQuery{PageSize: 1, Query: "제주"}); !reflect.DeepEqual(got, []string{"oldest", "older"}) {
		t.Fatalf("narrowed walk = %v", got)
	}
}

func TestListRefusesARequestTheBrowserNeverBuilds(t *testing.T) {
	svc, store, _ := newTestService(t)
	seedListed(store, alice, "p", "글", StatusDraft, testNow)
	for name, q := range map[string]ListQuery{
		"negative size":         {PageSize: -1},
		"unknown status":        {Status: "archived"},
		"a token not base64":    {PageToken: "not a token!"},
		"a token with no slug":  {PageToken: encodeListToken(ListCursor{UpdatedAt: "2026-03-01T12:00:00.000000000Z"})},
		"a token of plain text": {PageToken: "YWJj"},
	} {
		if _, err := svc.List(context.Background(), alice, q); !errors.Is(err, ErrInvalidListRequest) {
			t.Errorf("%s: err = %v, want ErrInvalidListRequest", name, err)
		}
	}
}

func TestListTokenRoundTrips(t *testing.T) {
	cursor := ListCursor{UpdatedAt: "2026-03-01T12:00:00.500000000Z", Slug: "20260301-제주-2"}
	got, err := decodeListToken(encodeListToken(cursor))
	if err != nil || got == nil || *got != cursor {
		t.Fatalf("round trip = %+v, %v; want %+v", got, err, cursor)
	}
}

// Only the answered page is decorated: the port for the running job is asked about the rows
// on the page and nothing else.
func TestListResolvesTheActiveJobForThePageOnly(t *testing.T) {
	svc, store, _ := newTestService(t)
	for i := range 5 {
		seedListed(store, alice, fmt.Sprintf("p%d", i), "글", StatusDraft, testNow.Add(time.Duration(i)*time.Minute))
	}
	asked := countingJobs{}
	svc.jobs = asked
	if _, err := svc.List(context.Background(), alice, ListQuery{PageSize: 2}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(asked.slugs(), []string{"p3", "p4"}) {
		t.Fatalf("active job asked for %v, want the page's p4 and p3", asked.slugs())
	}
}

type countingJobs map[string]int

func (c countingJobs) ActiveForPost(_ context.Context, slug string) (*ActiveJob, error) {
	c[slug]++
	return nil, nil
}

func (c countingJobs) slugs() []string {
	var out []string
	for _, slug := range []string{"p0", "p1", "p2", "p3", "p4"} {
		if c[slug] > 0 {
			out = append(out, slug)
		}
	}
	return out
}
