package rpc

import (
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func TestPostReadReturnsOnlyCurrentAlignedOriginReview(t *testing.T) {
	content := post.PostContent{Title: "가😀끝"}
	identity := post.ContentOriginIdentity(content, 7)
	resolved := post.ResolveOriginCandidates(content, identity, nil, []post.OriginCandidate{{
		Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: "😀", Category: post.OriginAIAdded,
	}})
	p := post.Post{Content: &content, ContentRevision: 7, ContentOrigins: &resolved.Review}
	wire := toProtoPost(p)
	if wire.ContentHash != identity.ContentHash || wire.ContentOrigins == nil || wire.ContentOrigins.Result.ContentRevision != 7 || len(wire.ContentOrigins.Spans) != 1 {
		t.Fatalf("current result identity/review lost: %+v", wire)
	}
	if span := wire.ContentOrigins.Spans[0]; span.Start != 1 || span.End != 2 || span.Quote != "😀" {
		t.Fatalf("Unicode scalar range changed: %+v", span)
	}
	resolved.Review.Result.ContentRevision++
	stale := toProtoPost(p)
	if stale.Content == nil || stale.Content.Title != content.Title || stale.ContentHash != identity.ContentHash || stale.ContentOrigins != nil {
		t.Fatalf("stale metadata changed usable canonical response: %+v", stale)
	}
	p.ContentOrigins = nil
	legacy := toProtoPost(p)
	if legacy.ContentOrigins != nil || legacy.ContentHash != identity.ContentHash || legacy.ContentRevision != 7 {
		t.Fatal("legacy origin history was invented")
	}
}

func TestPostReadDropsOnlyInvalidCurrentOriginSpans(t *testing.T) {
	content := post.PostContent{Title: "first second"}
	identity := post.ContentOriginIdentity(content, 2)
	resolved := post.ResolveOriginCandidates(content, identity, nil, []post.OriginCandidate{
		{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: "first", Category: post.OriginAIAdded},
		{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Quote: "second", Category: post.OriginAIAdded},
	})
	resolved.Review.Spans[1].Quote = "invalid"
	wire := toProtoPost(post.Post{Content: &content, ContentRevision: 2, ContentOrigins: &resolved.Review})
	if wire.Content == nil || wire.ContentOrigins == nil || len(wire.ContentOrigins.Spans) != 1 || wire.ContentOrigins.Spans[0].Quote != "first" {
		t.Fatalf("invalid annotation discarded valid canonical/evidence: %+v", wire)
	}
}
