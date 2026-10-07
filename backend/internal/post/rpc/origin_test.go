package rpc

import (
	"reflect"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

func assertOriginEnum[T comparable, W ~int32](t *testing.T, generated map[int32]string, mapping map[W]T) {
	t.Helper()
	if len(generated) != len(mapping) {
		t.Fatalf("generated enum/mapping count mismatch: %d/%d", len(generated), len(mapping))
	}
	for number, name := range generated {
		domain, exists := mapping[W(number)]
		if !exists {
			t.Fatalf("enum %s has no explicit mapping", name)
		}
		wire, found := originWire(mapping, domain)
		if !found || int32(wire) != number {
			t.Fatalf("enum %s does not round-trip", name)
		}
	}
	if _, exists := mapping[W(999)]; exists {
		t.Fatal("unknown enum mapped as known")
	}
}

func TestOriginEnumsCoverGeneratedValues(t *testing.T) {
	assertOriginEnum(t, postpilotv1.SemanticOriginCategory_name, originCategories)
	assertOriginEnum(t, postpilotv1.OriginReviewState_name, originReviewStates)
	assertOriginEnum(t, postpilotv1.OriginFieldKind_name, originFieldKinds)
}

func TestOriginProjectionRoundTripPreservesCurrentTextAndReview(t *testing.T) {
	index := 0
	review := &post.OriginReview{Version: post.OriginVersion, Result: post.OriginResultIdentity{ContentRevision: 7, ContentHash: "result-hash"},
		Sources: []post.OriginSource{{ID: "memo:1", Kind: post.OriginSourceMemo, Text: "기억 🍊", Available: true}},
		Spans: []post.OriginSpan{{Field: post.OriginFieldLocator{Kind: post.OriginFieldTag, TagIndex: &index}, Start: 1, End: 2,
			Quote: "🍊", Category: post.OriginAIAdded, SourceRefs: []string{"memo:1"}, ReviewState: post.OriginConfirmed}},
	}
	wire, err := OriginReviewToProto(review)
	if err != nil {
		t.Fatal(err)
	}
	got := OriginReviewFromProto(wire)
	if !reflect.DeepEqual(got, review) {
		t.Fatalf("round trip changed annotations: %#v", got)
	}
	content := post.PostContent{Tags: []string{"맛🍊"}}
	resolved := post.ValidateOriginReview(content, review.Result, got)
	if len(resolved.Issues) != 0 || len(resolved.Review.Spans) != 1 || resolved.Review.Spans[0].Category != post.OriginAIAdded {
		t.Fatalf("confirmation changed semantic category or Unicode range: %+v", resolved)
	}
	got.Spans[0].SourceRefs[0] = "changed"
	if wire.Spans[0].SourceRefs[0] != "memo:1" {
		t.Fatal("mapper aliases transport evidence")
	}
}

func TestOriginProjectionMissingAndUnknownMetadataStayUnconfirmed(t *testing.T) {
	content := post.PostContent{Title: "글"}
	identity := post.OriginResultIdentity{ContentRevision: 1, ContentHash: "hash"}
	if OriginReviewFromProto(nil) != nil {
		t.Fatal("absent legacy annotation reconstructed")
	}
	missing := post.ValidateOriginReview(content, identity, OriginReviewFromProto(nil))
	if len(missing.Review.Spans) != 0 || missing.Issues[0].Code != post.OriginIssueMissing {
		t.Fatalf("legacy result: %+v", missing)
	}
	wire := &postpilotv1.OriginReview{Version: 1, Result: &postpilotv1.OriginResultIdentity{ContentRevision: 1, ContentHash: "hash"},
		Spans: []*postpilotv1.OriginSpan{{Field: &postpilotv1.OriginFieldLocator{Kind: postpilotv1.OriginFieldKind_ORIGIN_FIELD_KIND_TITLE},
			Start: 0, End: 1, Quote: "글", Category: postpilotv1.SemanticOriginCategory(999), ReviewState: postpilotv1.OriginReviewState_ORIGIN_REVIEW_STATE_UNREVIEWED}},
	}
	resolved := post.ValidateOriginReview(content, identity, OriginReviewFromProto(wire))
	if len(resolved.Review.Spans) != 0 || resolved.Issues[0].Code != post.OriginIssueCategory {
		t.Fatalf("unknown metadata: %+v", resolved)
	}
	if content.Title != "글" {
		t.Fatal("annotation damaged canonical content")
	}
}

func TestOriginProjectionRejectsUnsafeIntegerCasts(t *testing.T) {
	review := &post.OriginReview{Version: 1, Result: post.OriginResultIdentity{ContentHash: "hash"},
		Spans: []post.OriginSpan{{Field: post.OriginFieldLocator{Kind: post.OriginFieldTitle}, Start: -1, End: 1,
			Category: post.OriginAIAdded, ReviewState: post.OriginUnreviewed}},
	}
	if _, err := OriginReviewToProto(review); err == nil {
		t.Fatal("negative scalar offset wrapped onto wire")
	}
	negative := -1
	if _, err := originIndexToProto(&negative); err == nil {
		t.Fatal("negative field index wrapped onto wire")
	}
}
