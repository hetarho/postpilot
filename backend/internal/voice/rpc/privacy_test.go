package rpc

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/postpilot/backend/internal/voice"
)

func TestLearningAndValidationProjectionsHideFrozenInputsModelIdentityAndLegacyErrors(t *testing.T) {
	failure := &voice.Failure{Reason: voice.FailureReasonUnknown, Params: map[string]string{"safe": "value"}, TechnicalDetail: "structured detail"}
	event := toProtoLearningEvent(voice.LearningEvent{ID: "event", PostSlug: "post", ModelRef: "secret-provider/secret-model", BaselineJSON: "private baseline", FinalJSON: "private final", InputHash: "private hash", Error: "legacy event error", Failure: failure, CreatedAt: time.Now()})
	comparison := toProtoComparison(voice.RuleComparison{ID: "comparison", InputSnapshot: "private comparison input", ModelRef: "secret-provider/secret-model", RuleOnSide: "right", CreatedAt: time.Now(), Candidates: []voice.ComparisonCandidate{{ID: "left", DisplaySide: "left", Status: "failed", Error: "legacy candidate error", Failure: failure}}})
	validation := toProtoValidation(voice.ProfileValidation{ID: "validation", AnalyzeModelRef: "secret-provider/analyze", WriteModelRef: "secret-provider/write", CreatedAt: time.Now(), Items: []voice.ValidationItem{{ID: "item", Status: "failed", Error: "legacy item error", Failure: failure}}})
	for name, message := range map[string]proto.Message{"event": event, "comparison": comparison, "validation": validation} {
		encoded, err := protojson.Marshal(message)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{"secret-provider", "secret-model", "private baseline", "private final", "private hash", "private comparison input", "right", "legacy event error", "legacy candidate error", "legacy item error"} {
			if strings.Contains(string(encoded), secret) {
				t.Fatalf("%s projection leaked %q: %s", name, secret, encoded)
			}
		}
	}
}
