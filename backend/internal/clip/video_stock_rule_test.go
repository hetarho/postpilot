package clip_test

import (
	"encoding/json"
	"github.com/postpilot/backend/internal/clip"
	"reflect"
	"testing"
)

func TestVideoGuidelineApplicabilityFreezesWithoutChangingLegacyMaterial(t *testing.T) {
	legacy := clip.VideoGuidelines{Defaults: []string{"literal legacy rule"}, Owner: []string{"owner rule\nexact second line"}}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != `{"Defaults":["literal legacy rule"],"Owner":["owner rule\nexact second line"]}` {
		t.Fatal("retained legacy payload shape changed", string(raw))
	}
	var retained clip.VideoGuidelines
	if err = json.Unmarshal(raw, &retained); err != nil || retained.Stock != nil || !reflect.DeepEqual(legacy, retained) {
		t.Fatal("legacy rule metadata was inferred")
	}
	current := legacy
	current.Stock = []clip.VideoStockRule{{Key: "caption-limit", Text: "literal legacy rule", SourceOrder: 2, Applicability: []clip.VideoRuleApplicability{{Stage: "clip-write", Outputs: []string{"captions"}}}}}
	frozen, _ := json.Marshal(current)
	var decoded clip.VideoGuidelines
	if err = json.Unmarshal(frozen, &decoded); err != nil || !reflect.DeepEqual(current, decoded) || current.Digest() == legacy.Digest() {
		t.Fatal("applicability was not frozen in quote identity")
	}
	current.Stock[0].Applicability[0].Outputs[0] = "narration"
	if current.Digest() == decoded.Digest() || decoded.Stock[0].Applicability[0].Outputs[0] != "captions" {
		t.Fatal("rule-only applicability change escaped quote fence or changed frozen data")
	}
}
