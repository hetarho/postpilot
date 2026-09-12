package clip_test

import (
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/platform/config"
)

func bindingFixture() (clip.CompositionInputs, []clip.SourceAnalysis, clip.Cut) {
	inputs := clip.CompositionInputs{Items: map[string][]composition.Item{"menu": {
		{ID: "sea", Values: map[string]string{"name": "해물라면", "price": "12,000원"}},
		{ID: "cheese", Values: map[string]string{"name": "치즈라면", "price": "$12 per serving"}},
	}}}
	analyses := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp"}, Filename: "치즈라면 $12.mp4"}, Segments: []clip.Segment{{StartMS: 0, EndMS: 5000, Event: "해물라면을 담는다"}, {StartMS: 5000, EndMS: 10000, Subjects: []string{"해물라면"}}}}}
	return inputs, analyses, clip.Cut{ID: "cut", SourceID: "source", Fingerprint: "fp", StartMS: 1000, EndMS: 7000}
}

func TestCutEvidenceRequiresFullObservedCoverageAndFingerprint(t *testing.T) {
	_, analyses, cut := bindingFixture()
	evidence, covered := clip.CutEvidence(analyses, cut)
	if !covered || len(evidence) != 2 || evidence[0].Source.StartMS != 1000 || evidence[1].Source.EndMS != 7000 {
		t.Fatalf("%+v %v", evidence, covered)
	}
	analyses[0].Segments[1].StartMS = 5001
	if _, covered = clip.CutEvidence(analyses, cut); covered {
		t.Fatal("bridged source gap")
	}
	analyses[0].Segments[1].StartMS = 5000
	cut.Fingerprint = "changed"
	if _, covered = clip.CutEvidence(analyses, cut); covered {
		t.Fatal("accepted different original")
	}
}

func TestItemBindingUsesAllObservationsOrOwnerAssociation(t *testing.T) {
	for _, tc := range []struct {
		name         string
		change       func(*clip.CompositionInputs, []clip.SourceAnalysis)
		item, reason string
	}{
		{"unique", func(*clip.CompositionInputs, []clip.SourceAnalysis) {}, "sea", ""},
		{"reordered", func(in *clip.CompositionInputs, _ []clip.SourceAnalysis) { slices.Reverse(in.Items["menu"]) }, "sea", ""},
		{"generic_filename_and_numbers", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) { a[0].Segments[0].Event = "음식 접시, 12" }, "", "item_unassigned"},
		{"conflicting_adjacent_item", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			a[0].Segments[1].Subjects = []string{"치즈라면"}
		}, "", "item_binding_conflict"},
		{"uncertain", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			a[0].Segments[0].Quality = "uncertain identity"
		}, "", "item_uncertain"},
		{"shared_alias", func(in *clip.CompositionInputs, _ []clip.SourceAnalysis) {
			in.Items["menu"][1].Values["alias"] = "해물라면"
		}, "", "item_unassigned"},
		{"not_a_word", func(in *clip.CompositionInputs, a []clip.SourceAnalysis) {
			in.Items["menu"][0].Values["name"] = "면"
			a[0].Segments[0].Event = "화면이 보인다"
		}, "", "item_unassigned"},
		{"owner_overrides_appearance", func(in *clip.CompositionInputs, _ []clip.SourceAnalysis) {
			in.Associations = []clip.SourceAssociation{{GroupID: "menu", ItemID: "cheese", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 10000}}
		}, "cheese", ""},
		{"partial_owner", func(in *clip.CompositionInputs, _ []clip.SourceAnalysis) {
			in.Associations = []clip.SourceAssociation{{GroupID: "menu", ItemID: "cheese", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 5000}}
		}, "", "item_binding_conflict"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in, analyses, cut := bindingFixture()
			tc.change(&in, analyses)
			evidence, ok := clip.CutEvidence(analyses, cut)
			if !ok {
				t.Fatal("fixture")
			}
			got := clip.BindCutItem(in, evidence, cut, "menu")
			if got.ItemID != tc.item || got.Reason != tc.reason {
				t.Fatalf("%+v", got)
			}
		})
	}
}

func TestScopedNumbersCurrencyUnitBasisAndExperience(t *testing.T) {
	for _, tc := range []struct {
		text, fact string
		allowed    bool
	}{
		{"12,000원", "12000원", true}, {"12원", "12000원", false},
		{"$12 per serving", "$12 per serving", true}, {"€12 per serving", "$12 per serving", false},
		{"$12", "$12 per serving", false}, {"$12 per night", "$12 per serving", false},
		{"1박 12000원", "1박 12,000원", true}, {"12000원", "1박 12000원", false}, {"2인 12000원", "1인 12000원", false},
		{"15g", "1,5g", false}, {"1.5g", "1.5g", true}, {"10m", "10mm", false}, {"-10%", "10%", false},
		{"10", "10 million", false}, {"10 million", "10 million", true},
		{"맛있어요", "보기 좋다", false}, {"맛있어요", "맛있어요", true}, {"맛있어요", "맛있어요라는 말은 할 수 없다", false},
		{"I tried it", "색상이 밝다", false}, {"I tried it", "I tried it", true},
	} {
		t.Run(tc.text+"/"+tc.fact, func(t *testing.T) {
			reason := clip.GroundScopedText(tc.text, []composition.Fact{{Value: tc.fact}}, clip.CompositionInputs{}, clip.ItemBinding{}, "scene")
			if (reason == "") != tc.allowed {
				t.Fatalf("allowed %v reason %s", tc.allowed, reason)
			}
		})
	}
	if reason := clip.GroundScopedText("1234원", []composition.Fact{{Value: "12"}, {Value: "34원"}}, clip.CompositionInputs{}, clip.ItemBinding{}, "scene"); reason == "" {
		t.Fatal("concatenated distinct facts")
	}
}

func TestFactScopeRequiresCorrectItemOrDeclaredContext(t *testing.T) {
	doc, problem := composition.Parse(`<clip version="1"><field id="price" label="입장료"/><group id="menu"><field id="price" label="가격"/></group></clip>`, config.ClipCompositionLimits())
	if problem != nil {
		t.Fatal(problem)
	}
	inputs, _, _ := bindingFixture()
	inputs.Values = map[string]string{"price": "12,000원"}
	binding := clip.ItemBinding{GroupID: "menu", ItemID: "sea"}
	for _, tc := range []struct {
		ref   clip.FactReference
		scope string
		valid bool
	}{
		{clip.FactReference{FieldID: "price", GroupID: "menu", ItemID: "sea"}, "item", true},
		{clip.FactReference{FieldID: "price", GroupID: "menu", ItemID: "cheese"}, "item", false},
		{clip.FactReference{FieldID: "price"}, "item", false},
		{clip.FactReference{FieldID: "price"}, "context", true},
		{clip.FactReference{FieldID: "price", GroupID: "menu", ItemID: "sea"}, "context", false},
	} {
		_, valid := clip.ScopedFact(doc, inputs, tc.ref, tc.scope, binding)
		if valid != tc.valid {
			t.Fatalf("%+v", tc)
		}
	}
	for _, text := range []string{"치즈라면", "이 메뉴 12,000원"} {
		if reason := clip.GroundScopedText(text, []composition.Fact{{Value: "12,000원"}}, inputs, clip.ItemBinding{}, "context"); reason == "" {
			t.Fatalf("context labeled item: %s", text)
		}
	}
}
