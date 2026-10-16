package main

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestClipLinkedPostsReadsOwnedCanonicalPostAndHidesForeignIdentity(t *testing.T) {
	posts, voices := publishedPostService(t)
	slug := publishedSlug(t, posts, voices)
	adapter := clipLinkedPosts{service: posts}
	p, err := adapter.Get(t.Context(), "alice", slug)
	if err != nil || !p.ContentReady || p.Slug != slug || p.Title != "제주 3일" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal(p, err)
	}
	for _, tc := range [][2]string{{"bob", slug}, {"alice", "missing"}} {
		if _, err := adapter.Get(t.Context(), tc[0], tc[1]); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal("missing and foreign posts must be indistinguishable", err)
		}
	}
}
