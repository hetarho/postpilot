package generation

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
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
	if !strings.HasPrefix(guide.Text, input.AdvertiserGuide.Text) || !strings.Contains(guide.Text, "필수 키워드: 성수 카페, 크림 라떼") || strings.Contains(guide.Text, "필수 해시태그") {
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
	// GEN-80: the guide's meaning keeps its advertiser-request origin, while its requests count
	// as supplied facts (GEN-88), so the clause no longer calls it something other than experience.
	if strings.Contains(advertiserOriginClause, "not the owner's experience") || !strings.Contains(advertiserOriginClause, "store information, emphasis or introduction") {
		t.Fatalf("origin clause = %q", advertiserOriginClause)
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

// GEN-91: an unedited observation holding no stored spans still supplies its scene, objects and
// visible text (and a video its events and speech) as that attachment's visual observation, and
// never its mood.
func TestObservationWithoutSpansSuppliesVisualFieldSources(t *testing.T) {
	input := WritePromptInput{Photos: []string{"a.jpg"}, Videos: []string{"v.mp4"}, Observations: []Observation{
		{File: "a.jpg", Scene: "가게 입구", Mood: "밝음", VisibleText: "OPEN", Objects: []string{"간판", "화분"}, Events: []string{"사진엔 없는 사건"}, Speech: "사진엔 없는 말"},
		{File: "v.mp4", Scene: "주방", Mood: "분주함", Objects: []string{"냄비"}, Events: []string{"국물을 붓는다"}, Speech: "주문 나왔습니다"},
	}}
	sources := WritingOriginSources(input, false, map[string]string{"a.jpg": "img-1", "v.mp4": "vid-1"})
	byFile := map[string]map[string]post.OriginSource{"a.jpg": {}, "v.mp4": {}}
	for _, source := range sources {
		if source.Kind != post.OriginSourceVisualObservation || !source.Available {
			t.Fatalf("source %+v is not available visual observation", source)
		}
		want := map[string]string{"a.jpg": "img-1", "v.mp4": "vid-1"}[source.AttachmentFilename]
		if want == "" || source.AttachmentID != want {
			t.Fatalf("source %+v is not bound to its attachment", source)
		}
		byFile[source.AttachmentFilename][source.Text] = source
	}
	for file, want := range map[string][]string{
		"a.jpg": {"가게 입구", "OPEN", "간판", "화분"},
		"v.mp4": {"주방", "냄비", "국물을 붓는다", "주문 나왔습니다"},
	} {
		if len(byFile[file]) != len(want) {
			t.Fatalf("%s sources = %+v, want %v", file, byFile[file], want)
		}
		for _, text := range want {
			if _, ok := byFile[file][text]; !ok {
				t.Fatalf("%s lacks %q: %+v", file, text, byFile[file])
			}
		}
	}
	if got := byFile["a.jpg"]["간판"].ID; got != "current.observation.0.objects.0" {
		t.Fatalf("id scheme changed: %q", got)
	}
}

// GEN-91, GEN-84: an observation with stored spans keeps its span-derived sources and gains no
// field sources.
func TestObservationWithSpansKeepsSpanSources(t *testing.T) {
	observation := Observation{File: "a.jpg", Scene: "visible cup", Mood: "따뜻함", Objects: []string{"cup"}}
	observation.OriginCandidates = []ObservationOriginCandidate{{Field: "scene", Quote: "visible cup", Category: post.OriginPhotoInterpretation, SourceRefs: []string{"photo"}}}
	observation.Origins = ValidateObservationOrigins(observation, originSources(), AttachmentPhoto)
	if len(ValidateStoredObservationOrigins(observation, observation.Origins, AttachmentPhoto).Spans) != 1 {
		t.Fatal("fixture capture did not validate")
	}
	sources := WritingOriginSources(WritePromptInput{Photos: []string{"a.jpg"}, Observations: []Observation{observation}}, false)
	if len(sources) != 1 || sources[0].ID != "current.visual.0.0" || sources[0].Text != "visible cup" {
		t.Fatalf("sources = %+v, want only the span-derived one", sources)
	}
}

// GEN-91, GEN-84: a caption quoting the scene of an observation with no stored spans resolves as
// photo_interpretation against the fallback source.
func TestCaptionOfSpanlessObservationResolvesAsPhotoInterpretation(t *testing.T) {
	input := WritePromptInput{Photos: []string{"a.jpg"}, Observations: []Observation{{File: "a.jpg", Scene: "푸른 벽 앞의 카페 입구", Mood: "따뜻함"}}}
	sources := WritingOriginSources(input, false, map[string]string{"a.jpg": "img-1"})
	content := PostContent{Title: "카페", Blocks: []Block{{Type: BlockImage, File: "a.jpg", Alt: "입구", Caption: "푸른 벽 앞의 카페 입구"}}}
	candidates := []post.OriginCandidate{originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockCaption, BlockIndex: originInt(0)}, "푸른 벽 앞의 카페 입구", post.OriginPhotoInterpretation, "current.observation.0.scene")}
	got := FinalizeWriteOrigins(WriteAnswer{Content: content, OriginCandidates: candidates}, []string{"a.jpg"}, nil, nil, sources)
	if got.Origins == nil || len(got.Origins.Spans) != 1 || got.Origins.Spans[0].Category != post.OriginPhotoInterpretation {
		t.Fatalf("caption origin = %+v", got.Origins)
	}
	mood := originCandidate(post.OriginFieldLocator{Kind: post.OriginFieldBlockAlt, BlockIndex: originInt(0)}, "입구", post.OriginPhotoInterpretation, "current.observation.0.mood")
	if len(ResolveWriteOriginCandidates(got.Content, sources, []post.OriginCandidate{mood}).Review.Spans) != 0 {
		t.Fatal("mood supported a photo interpretation")
	}
}

// GEN-44, GEN-80: a caption splits the memo's name (owner input) from what the frame shows, in
// the common contract every scoped contract inherits.
func TestOriginContractSplitsCaptionNameFromFrame(t *testing.T) {
	const clause = "In alt and caption, a subject name the memo or a template field supplies is owner_input and what the frame shows is photo_interpretation; split them within the caption."
	if !strings.Contains(originResponseContract, clause+" For post fields,") {
		t.Fatal("the origin contract lost its caption clause")
	}
	for _, mode := range []string{"", "frozen-storyline", "storyline-create"} {
		request := llm.Request{}
		if mode != "" {
			request.Composition = &llm.RequestComposition{Mode: mode}
		}
		contract := scopedOriginContract(request)
		if !strings.Contains(contract, clause) {
			t.Errorf("mode %q drops the caption clause", mode)
		}
		if strings.Contains(contract, "For post fields,") {
			t.Errorf("mode %q keeps the unscoped field sentence", mode)
		}
	}
}
