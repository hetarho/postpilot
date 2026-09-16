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
		// The fixture carries no status, so it is a legacy record and the prose
		// heuristic still reads it (CLIP-93).
		{"legacy_uncertain_prose", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			a[0].Segments[0].Quality = "uncertain identity"
		}, "", "item_uncertain"},
		// A v2 record states its own status, so the same prose no longer decides.
		{"v2_certain_beats_prose", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			a[0].Segments[0].Quality = "uncertain identity"
			for i := range a[0].Segments {
				a[0].Segments[i].Certainty, a[0].Segments[i].Usability = clip.CertaintyCertain, clip.UsabilityUsable
			}
		}, "sea", ""},
		{"v2_uncertain", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			for i := range a[0].Segments {
				a[0].Segments[i].Certainty, a[0].Segments[i].Usability = clip.CertaintyUncertain, clip.UsabilityUsable
			}
		}, "", "item_uncertain"},
		{"v2_unknown", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			for i := range a[0].Segments {
				a[0].Segments[i].Certainty, a[0].Segments[i].Usability = clip.CertaintyUnknown, clip.UsabilityUsable
			}
		}, "", "item_uncertain"},
		{"v2_unusable", func(_ *clip.CompositionInputs, a []clip.SourceAnalysis) {
			for i := range a[0].Segments {
				a[0].Segments[i].Certainty, a[0].Segments[i].Usability = clip.CertaintyCertain, clip.UsabilityUnusable
			}
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
			// No instruction: every case below is the rule exactly as it
			// stands today (CLIP-64).
			reason := clip.GroundScopedText(tc.text, []composition.Fact{{Value: tc.fact}}, clip.CompositionInputs{}, clip.ItemBinding{}, "scene", false)
			if (reason == "") != tc.allowed {
				t.Fatalf("allowed %v reason %s", tc.allowed, reason)
			}
		})
	}
	if reason := clip.GroundScopedText("1234원", []composition.Fact{{Value: "12"}, {Value: "34원"}}, clip.CompositionInputs{}, clip.ItemBinding{}, "scene", false); reason == "" {
		t.Fatal("concatenated distinct facts")
	}
}

func TestFactScopeRequiresCorrectItemOrDeclaredContext(t *testing.T) {
	doc, problem := composition.Parse(`<clip version="1" intro="b" caption="bold" outro="e"><field id="price" label="입장료"/><group id="menu"><field id="price" label="가격"/></group><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`, config.ClipCompositionLimits())
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
		if reason := clip.GroundScopedText(text, []composition.Fact{{Value: "12,000원"}}, inputs, clip.ItemBinding{}, "context", false); reason == "" {
			t.Fatalf("context labeled item: %s", text)
		}
	}
}

// An owner instruction is owner-authored fact in CDS-42's sense: where one is
// present the writer may state the experiential claims it asks for without the
// instruction repeating the words, while every figure still needs a referenced
// fact and every other rule stands (CLIP-122).
func TestInstructionAdmitsExperienceButNeverAFigure(t *testing.T) {
	for _, tc := range []struct {
		name               string
		text, fact         string
		instructed, absent bool
	}{
		{"experience with an instruction", "직접 먹어보니 고소했어요", "삼겹살", true, false},
		{"the same sentence without one", "직접 먹어보니 고소했어요", "삼겹살", false, true},
		{"english experience with an instruction", "I tried it and loved it", "pork belly", true, false},
		{"english experience without one", "I tried it and loved it", "pork belly", false, true},
		{"a figure with an instruction", "직접 먹어보니 12,000원", "삼겹살", true, true},
		{"a figure without one", "직접 먹어보니 12,000원", "삼겹살", false, true},
		{"a supported figure with an instruction", "먹어보니 12,000원", "12,000원", true, false},
		{"a description needs no instruction", "노릇하게 구워진 고기", "삼겹살", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason := clip.GroundScopedText(tc.text, []composition.Fact{{Value: tc.fact}}, clip.CompositionInputs{}, clip.ItemBinding{}, "scene", tc.instructed)
			if (reason != "") != tc.absent {
				t.Fatalf("refused %v (%s)", tc.absent, reason)
			}
		})
	}
	// The instruction lifts only the appearance rule: a cross-item identity and
	// a context claim about the depicted item are refused with one present.
	inputs, _, _ := bindingFixture()
	if reason := clip.GroundScopedText("치즈라면도 맛있어요", nil, inputs, clip.ItemBinding{GroupID: "menu", ItemID: "sea"}, "scene", true); reason != "cross_item_identity" {
		t.Fatalf("an instruction admitted another item's identity: %s", reason)
	}
	if reason := clip.GroundScopedText("이 메뉴 12,000원", []composition.Fact{{Value: "12,000원"}}, inputs, clip.ItemBinding{}, "context", true); reason != "context_item_claim" {
		t.Fatalf("an instruction admitted a context item claim: %s", reason)
	}
}

// A whole-source binding made before generation reaches every cut taken from
// that source, whether observation names no subject or several, while a source
// the owner left unbound keeps today's automatic behaviour (CLIP-123, CLIP-62).
func TestWholeSourceBindingReachesEveryCutAndLeavesOthersAutomatic(t *testing.T) {
	inputs, analyses, cut := bindingFixture()
	whole := clip.SourceAssociation{GroupID: "menu", ItemID: "cheese", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 10000}

	// Unbound, this footage names 해물라면, so the automatic rule settles on
	// that item. The owner's binding is what has to outrank it.
	if binding := clip.BindCutItem(inputs, evidenceFor(t, analyses, cut), cut, ""); binding.ItemID != "sea" || binding.Owner {
		t.Fatalf("automatic association changed: %+v", binding)
	}
	bound := inputs
	bound.Associations = []clip.SourceAssociation{whole}
	for _, c := range []clip.Cut{
		cut,
		{ID: "early", SourceID: "source", Fingerprint: "fp", StartMS: 0, EndMS: 5000},
		{ID: "late", SourceID: "source", Fingerprint: "fp", StartMS: 5000, EndMS: 10000},
	} {
		binding := clip.BindCutItem(bound, evidenceFor(t, analyses, c), c, "")
		if binding.ItemID != "cheese" || binding.GroupID != "menu" || !binding.Owner {
			t.Fatalf("cut %s did not inherit the bound item: %+v", c.ID, binding)
		}
	}
	// Another source is untouched by the binding and stays automatic.
	other := clip.Cut{ID: "other", SourceID: "second", Fingerprint: "fp2", StartMS: 0, EndMS: 5000}
	second := []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "second", Fingerprint: "fp2"}}, Segments: []clip.Segment{{StartMS: 0, EndMS: 5000, Subjects: []string{"치즈라면"}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}}}
	if binding := clip.BindCutItem(bound, evidenceFor(t, second, other), other, ""); binding.ItemID != "cheese" || binding.Owner {
		t.Fatalf("an unbound source lost its automatic association: %+v", binding)
	}
}

func evidenceFor(t *testing.T, analyses []clip.SourceAnalysis, cut clip.Cut) []clip.ObservedEvidence {
	t.Helper()
	evidence, _ := clip.CutEvidence(analyses, cut)
	return evidence
}
