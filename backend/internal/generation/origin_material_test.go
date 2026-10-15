package generation

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

// GEN-80, GEN-88: a frozen guide is one catalog source of the advertiser's kind, which supports
// an advertiser request and never owner input; the origin contract names the category only
// when that source is supplied.
func TestTheGuideIsAnAdvertiserRequestSource(t *testing.T) {
	input := advertiserGoldenInput()
	sources := WritingOriginSources(input, false)
	var guide *post.OriginSource
	for i := range sources {
		if sources[i].ID == advertiserGuideSourceID {
			guide = &sources[i]
		}
	}
	if guide == nil || guide.Kind != post.OriginSourceAdvertiserGuide || !guide.Available || guide.Text != advertiserGuideSourceText(input.AdvertiserGuide) {
		t.Fatalf("guide source = %+v", guide)
	}
	if !strings.HasPrefix(guide.Text, input.AdvertiserGuide.Text) || !strings.Contains(guide.Text, "필수 해시태그: #성수맛집") {
		t.Fatalf("guide source text = %q", guide.Text)
	}

	content := PostContent{Title: "성수 카페의 크림 라떼"}
	title := post.OriginFieldLocator{Kind: post.OriginFieldTitle}
	advertiser := ResolveWriteOriginCandidates(content, sources, []post.OriginCandidate{originCandidate(title, "크림 라떼", post.OriginAdvertiserRequest, advertiserGuideSourceID)})
	owner := ResolveWriteOriginCandidates(content, sources, []post.OriginCandidate{originCandidate(title, "크림 라떼", post.OriginOwnerInput, advertiserGuideSourceID)})
	if len(advertiser.Review.Spans) != 1 || len(owner.Review.Spans) != 0 {
		t.Fatalf("advertiser %+v, owner %+v", advertiser.Review.Spans, owner.Review.Spans)
	}

	withGuide := appendOriginRequest(ComposeWriteRequest(input), sources, nil)
	plain := input
	plain.AdvertiserGuide = nil
	plainSources := WritingOriginSources(plain, false)
	without := appendOriginRequest(ComposeWriteRequest(plain), plainSources, nil)
	if !strings.Contains(withGuide.System, advertiserOriginClause) || strings.Contains(without.System, "advertiser_request") {
		t.Fatal("the advertiser clause does not follow the guide source")
	}
	for _, source := range plainSources {
		if source.Kind == post.OriginSourceAdvertiserGuide {
			t.Fatal("a run without a guide catalogued one")
		}
	}
}
