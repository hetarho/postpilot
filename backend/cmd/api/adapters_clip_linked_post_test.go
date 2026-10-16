package main

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/post"
	"reflect"
)

func TestClipLinkedPostsReadsOwnedCanonicalPostAndHidesForeignIdentity(t *testing.T) {
	posts, voices := publishedPostService(t)
	slug := publishedSlug(t, posts, voices)
	adapter := clipLinkedPosts{service: posts}
	p, err := adapter.Get(t.Context(), "alice", slug)
	if err != nil || !p.ContentReady || p.Slug != slug || p.Title != "제주 3일" || p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal(p, err)
	}
	doc, err := adapter.Text(t.Context(), "alice", slug)
	if err != nil || doc.Text == nil || !doc.CreatedAt.Equal(p.CreatedAt) || doc.Text.Slug != slug {
		t.Fatal("text and identity must come from one owned canonical read", doc, err)
	}
	for _, tc := range [][2]string{{"bob", slug}, {"alice", "missing"}} {
		if _, err := adapter.Text(t.Context(), tc[0], tc[1]); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal("foreign text identity leaked", err)
		}
		if _, err := adapter.Get(t.Context(), tc[0], tc[1]); !errors.Is(err, clip.ErrNotFound) {
			t.Fatal("missing and foreign posts must be indistinguishable", err)
		}
	}
}

func TestLinkedPostTextKeepsCanonicalSectionsAndExcludesMediaAndMetadata(t *testing.T) {
	content := &post.PostContent{Title: "Canonical title", Summary: "DO NOT SEND SUMMARY", Tags: []string{"DO NOT SEND TAG"}, Blocks: []post.Block{
		{Type: post.BlockText, Content: "preface"}, {Type: post.BlockQuote, Content: "quoted preface"},
		{Type: post.BlockHeading, Content: "첫 경험"}, {Type: post.BlockText, Content: "honest detail"},
		{Type: post.BlockImage, Alt: "hidden alt", Caption: "hidden photo caption"},
		{Type: post.BlockGallery, Caption: "hidden gallery caption"},
		{Type: post.BlockVideo, Caption: "hidden video caption"},
		{Type: post.BlockList, Items: []string{"one", "two"}},
		{Type: post.BlockHeading, Content: "결론"}, {Type: post.BlockQuote, Content: "owner verdict"},
	}}
	got := linkedPostCanonicalText("owned", content)
	want := &clip.LinkedPostText{Slug: "owned", Title: "Canonical title", Sections: []clip.LinkedPostSection{{Text: "preface\nquoted preface"}, {Heading: "첫 경험", Text: "honest detail\none\ntwo"}, {Heading: "결론", Text: "owner verdict"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical text changed: %+v", got)
	}
	if linkedPostCanonicalText("owned", nil) != nil || linkedPostCanonicalText("owned", &post.PostContent{}) != nil {
		t.Fatal("a post without canonical content supplied material")
	}
}
