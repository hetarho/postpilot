package clip_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
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

func evidenceFor(t *testing.T, analyses []clip.SourceAnalysis, cut clip.Cut) []clip.ObservedEvidence {
	t.Helper()
	evidence, _ := clip.CutEvidence(analyses, cut)
	return evidence
}
