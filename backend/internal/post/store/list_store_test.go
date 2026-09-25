package store_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/post"
)

// walkStore follows the store's own cursors a page at a time, the way the service does.
func walkStore(t *testing.T, s interface {
	ListPosts(context.Context, string, post.ListFilter) ([]post.Summary, error)
}, filter post.ListFilter) []string {
	t.Helper()
	var slugs []string
	for range 100 {
		rows, err := s.ListPosts(context.Background(), "alice", filter)
		if err != nil {
			t.Fatalf("ListPosts(%+v): %v", filter, err)
		}
		for _, row := range rows {
			slugs = append(slugs, row.Slug)
		}
		if len(rows) < filter.Limit || len(rows) == 0 {
			return slugs
		}
		last := rows[len(rows)-1].Cursor
		filter.After = &last
	}
	t.Fatal("the walk never ended")
	return nil
}

// POST-90: the keyset seek compares exactly what the ORDER BY compares, so a walk at any page
// size returns every row once — through ties on updated_at, and through rows whose stored
// timestamp predates the pinned fraction width, which sort by their stored string.
func TestListPostsKeysetWalkMatchesTheOrder(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	for i := range 6 {
		seedPost(t, s, fmt.Sprintf("p%d", i), "alice", testNow.Add(time.Duration(i/2)*time.Minute))
	}
	seedPost(t, s, "legacy", "alice", testNow)
	// Written before the width was pinned: no fraction at all. It is the same instant as p0
	// and p1, but "…12:00:00Z" sorts above "…12:00:00.000000000Z" as a string, and the walk
	// has to agree with the ORDER BY rather than with the clock.
	if _, err := handle.Writer.Exec(`UPDATE posts SET updated_at = '2026-03-01T12:00:00Z' WHERE slug = 'legacy'`); err != nil {
		t.Fatal(err)
	}
	seedPost(t, s, "theirs", "bob", testNow.Add(time.Hour))

	whole, err := s.ListPosts(ctx, "alice", post.ListFilter{Limit: -1})
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, row := range whole {
		want = append(want, row.Slug)
	}
	if len(want) != 7 || want[0] != "p5" || want[1] != "p4" {
		t.Fatalf("whole list = %v", want)
	}
	for _, size := range []int{1, 2, 3, 7} {
		if got := walkStore(t, s, post.ListFilter{Limit: size}); !reflect.DeepEqual(got, want) {
			t.Errorf("walk at %d = %v, want %v", size, got, want)
		}
	}
	for _, row := range whole {
		if row.Slug == "legacy" && row.Cursor.UpdatedAt != "2026-03-01T12:00:00Z" {
			t.Errorf("the cursor re-formatted the stored timestamp: %+v", row.Cursor)
		}
	}
}

func TestListPostsFiltersByStatusAndLimits(t *testing.T) {
	ctx := context.Background()
	s, handle := newStoreWithHandle(t)
	for i := range 5 {
		seedPost(t, s, fmt.Sprintf("p%d", i), "alice", testNow.Add(time.Duration(i)*time.Minute))
	}
	if _, err := handle.Writer.Exec(`UPDATE posts SET status = 'review' WHERE slug IN ('p1', 'p3')`); err != nil {
		t.Fatal(err)
	}

	reviewed, err := s.ListPosts(ctx, "alice", post.ListFilter{Status: post.StatusReview, Limit: -1})
	if err != nil {
		t.Fatal(err)
	}
	if got := summarySlugs(reviewed); !reflect.DeepEqual(got, []string{"p3", "p1"}) {
		t.Errorf("review = %v, want [p3 p1]", got)
	}
	limited, err := s.ListPosts(ctx, "alice", post.ListFilter{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if got := summarySlugs(limited); !reflect.DeepEqual(got, []string{"p4", "p3"}) {
		t.Errorf("limit 2 = %v, want [p4 p3]", got)
	}
}

func summarySlugs(rows []post.Summary) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Slug)
	}
	return out
}
