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

// GEN-90, GEN-80: an owner-edited observation offers each field the owner changed as owner
// material that supports owner_input, while the fields the owner left stay visual observation.
func TestOwnerEditedObservationFieldsAreOwnerMaterial(t *testing.T) {
	input := WritePromptInput{Photos: []string{"a.jpg"}, Observations: []Observation{{
		File: "a.jpg", Scene: "가게 입구", Mood: "밝음", Objects: []string{"간판", "화분"},
		OwnerEdited: true, OwnerFields: []string{post.ObservationFieldScene, post.ObservationFieldObjects},
	}}}
	sources := WritingOriginSources(input, false, map[string]string{"a.jpg": "img-1"})
	catalog := map[string]post.OriginSource{}
	kinds := map[string]post.OriginSourceKind{}
	for _, source := range sources {
		catalog[source.ID] = source
		kinds[source.Text] = source.Kind
		if source.AttachmentFilename != "a.jpg" || source.AttachmentID != "img-1" || !source.Available {
			t.Fatalf("source %+v is not bound to its attachment", source)
		}
	}
	want := map[string]post.OriginSourceKind{
		"가게 입구": post.OriginSourceOwnerObservation, "간판": post.OriginSourceOwnerObservation, "화분": post.OriginSourceOwnerObservation,
		"밝음": post.OriginSourceVisualObservation,
	}
	if len(kinds) != len(want) {
		t.Fatalf("sources = %+v", sources)
	}
	for text, kind := range want {
		if kinds[text] != kind {
			t.Fatalf("%q catalogued as %q, want %q", text, kinds[text], kind)
		}
	}
	for id, source := range catalog {
		owner := source.Kind == post.OriginSourceOwnerObservation
		if originSourceCategoryCompatible(post.OriginOwnerInput, []string{id}, catalog) != owner {
			t.Fatalf("%q owner_input compatibility = %v", source.Text, !owner)
		}
		if originSourceCategoryCompatible(post.OriginPhotoInterpretation, []string{id}, catalog) == owner {
			t.Fatalf("%q photo_interpretation compatibility = %v", source.Text, owner)
		}
	}
	// Deleting the attachment withdraws the owner's material about it too (POST-114).
	for _, source := range OriginSourcesWithAttachments(sources, nil, nil) {
		if source.Available {
			t.Fatalf("source %+v outlived its attachment", source)
		}
	}
}
