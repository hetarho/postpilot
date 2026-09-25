package rpc

import (
	"context"
	"reflect"
	"testing"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/auth"
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

// POST-79: the taken indices are spent in the same save, the answer carries what is left, and a
// malformed list refuses the save as malformed content.
func TestSavePostContentSpendsTheTakenCandidates(t *testing.T) {
	h, svc := rpcServiceWith(t)
	ctx := auth.WithUser(context.Background(), "alice")
	created, err := h.SavePostDraft(ctx, draftRequest("", "제주", nil))
	if err != nil {
		t.Fatal(err)
	}
	slug := created.Msg.GetPost().GetSlug()
	content := post.PostContent{
		Title: "비 온 뒤의 제주", Tags: []string{"제주 산책"},
		Blocks: []post.Block{{Type: post.BlockText, Content: "비가 그치기를 기다렸다."}},
	}
	candidates := []post.ReplacementCandidate{
		{Surface: post.ReplacementSurfaceTitle, Index: 0, Source: "제주", Phrases: []string{"제주도"}},
		{Surface: post.ReplacementSurfaceTag, Index: 0, Source: "산책", Phrases: []string{"걷기"}},
		{Surface: post.ReplacementSurfaceBody, Index: 0, Source: "기다렸다", Phrases: []string{"기다리고 있었다"}},
	}
	if err := svc.SetGeneratedContent(ctx, "alice", slug, content, post.LanguageKorean, &post.WriteAnnotations{Candidates: candidates}); err != nil {
		t.Fatal(err)
	}
	read := func() *postpilotv1.Post {
		got, err := h.GetPost(ctx, connect.NewRequest(&postpilotv1.GetPostRequest{Slug: slug}))
		if err != nil {
			t.Fatal(err)
		}
		return got.Msg.GetPost()
	}
	sources := func(p *postpilotv1.Post) []string {
		var out []string
		for _, candidate := range p.GetReplacementCandidates() {
			out = append(out, candidate.GetSource())
		}
		return out
	}
	save := func(title string, taken ...int32) (*postpilotv1.Post, error) {
		edited := toProtoContent(&content)
		edited.Title = title
		saved, err := h.SavePostContent(ctx, connect.NewRequest(&postpilotv1.SavePostContentRequest{
			Slug: slug, Content: edited, ExpectedRevision: read().GetContentRevision(), TakenCandidates: taken,
		}))
		if err != nil {
			return nil, err
		}
		return saved.Msg.GetPost(), nil
	}

	answered, err := save("비 온 뒤의 제주도", 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := sources(answered); !reflect.DeepEqual(got, []string{"산책"}) {
		t.Fatalf("answered candidates = %q", got)
	}
	if got := sources(read()); !reflect.DeepEqual(got, []string{"산책"}) {
		t.Fatalf("GetPost candidates = %q", got)
	}

	for name, taken := range map[string][]int32{"an index past the list": {5}, "a repeated index": {0, 0}} {
		before := read()
		_, err := save("비 온 뒤의 제주도 산책", taken...)
		if connect.CodeOf(err) != connect.CodeInvalidArgument || postAppErrorDetail(t, err).GetReason() != "POST_CONTENT_INVALID" {
			t.Errorf("%s = %v, want InvalidArgument POST_CONTENT_INVALID", name, err)
		}
		if after := read(); after.GetContentRevision() != before.GetContentRevision() || !reflect.DeepEqual(sources(after), sources(before)) {
			t.Errorf("%s changed the post", name)
		}
	}

	kept, err := save("비 온 뒤의 제주도 산책")
	if err != nil {
		t.Fatal(err)
	}
	if got := sources(kept); !reflect.DeepEqual(got, []string{"산책"}) {
		t.Fatalf("a save with no takes changed the list: %q", got)
	}
}
