package rpc

import (
	"reflect"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

// ARCH-3: every generated surface but UNSPECIFIED is the image of exactly one domain surface,
// and every domain surface maps to a named, non-zero value.
func TestReplacementSurfaceWalksTheGeneratedEnum(t *testing.T) {
	domain := []post.ReplacementSurface{post.ReplacementSurfaceTitle, post.ReplacementSurfaceTag, post.ReplacementSurfaceBody}
	images := map[postpilotv1.ReplacementSurface]post.ReplacementSurface{}
	for _, surface := range domain {
		value, ok := replacementSurfaceToProto(surface)
		if !ok || value == postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_UNSPECIFIED {
			t.Fatalf("%q mapped to %s (ok=%v)", surface, value, ok)
		}
		if previous, taken := images[value]; taken {
			t.Fatalf("%q and %q both map to %s", previous, surface, value)
		}
		images[value] = surface
	}
	for number := range postpilotv1.ReplacementSurface_name {
		value := postpilotv1.ReplacementSurface(number)
		if value == postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_UNSPECIFIED {
			continue
		}
		if _, ok := images[value]; !ok {
			t.Errorf("%s is the image of no domain surface", value)
		}
	}
	if value, ok := replacementSurfaceToProto("summary"); ok || value != postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_UNSPECIFIED {
		t.Fatalf("an unknown surface mapped to %s", value)
	}
}

// The post carries its stored candidates on the wire, in stored order; its nouns never reach it.
func TestToProtoPostCarriesTheReplacementCandidates(t *testing.T) {
	stored := post.Post{
		Slug: "post", Status: post.StatusReview, TargetLanguage: post.LanguageKorean,
		ContentNouns: []string{"성수", "카페"},
		ReplacementCandidates: []post.ReplacementCandidate{
			{Surface: post.ReplacementSurfaceBody, Index: 2, Source: "분위기 좋은", Phrases: []string{"감성 가득한"}},
			{Surface: post.ReplacementSurfaceTitle, Index: 0, Source: "성수 카페", Phrases: []string{"성수동 카페", "성수 핫플"}},
			{Surface: post.ReplacementSurfaceTag, Index: 1, Source: "라떼", Phrases: []string{"카페라떼"}},
		},
	}
	got := toProtoPost(stored).GetReplacementCandidates()
	want := []*postpilotv1.ReplacementCandidate{
		{Surface: postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_BODY, Index: 2, Source: "분위기 좋은", Phrases: []string{"감성 가득한"}},
		{Surface: postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_TITLE, Index: 0, Source: "성수 카페", Phrases: []string{"성수동 카페", "성수 핫플"}},
		{Surface: postpilotv1.ReplacementSurface_REPLACEMENT_SURFACE_TAG, Index: 1, Source: "라떼", Phrases: []string{"카페라떼"}},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].GetSurface() != want[i].GetSurface() || got[i].GetIndex() != want[i].GetIndex() || got[i].GetSource() != want[i].GetSource() || !reflect.DeepEqual(got[i].GetPhrases(), want[i].GetPhrases()) {
			t.Fatalf("candidate %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if none := toProtoPost(post.Post{Slug: "empty", TargetLanguage: post.LanguageKorean}).GetReplacementCandidates(); none != nil {
		t.Fatalf("a post with none carried %+v", none)
	}
}
