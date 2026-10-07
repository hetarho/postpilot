package generation

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

func TestOriginSidecarCodecsPreserveScalarSpansAndDetachMutableSlices(t *testing.T) {
	index := 1
	result := post.OriginResultIdentity{ContentRevision: 2, ContentHash: "current-content"}
	sources := []post.OriginSource{{ID: "memo", Kind: post.OriginSourceMemo, Text: "카페😀", Available: true}}
	content := &post.OriginReview{Version: post.OriginVersion, Result: result, Sources: sources, Spans: []post.OriginSpan{{Field: post.OriginFieldLocator{Kind: post.OriginFieldBlockItem, BlockIndex: &index, ItemIndex: &index}, Start: 0, End: 3, Quote: "카페😀", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}, ReviewState: post.OriginUnreviewed}}}
	plan := &PlanOriginReview{Version: post.OriginVersion, Result: result, Sources: sources, Spans: []PlanOriginSpan{{ParagraphIndex: 1, Start: 0, End: 3, Quote: "카페😀", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}, ReviewState: post.OriginUnreviewed}}}
	observation := &ObservationOriginReview{Version: post.OriginVersion, Result: result, Sources: sources, Spans: []ObservationOriginSpan{{Field: "object", ItemIndex: &index, Start: 0, End: 3, Quote: "카페😀", Category: post.OriginOwnerInput, SourceRefs: []string{"memo"}, ReviewState: post.OriginUnreviewed}}}
	for name, test := range map[string]struct{ input, decoded any }{
		"content":     {content, decodeOriginReview(encodeOriginReview(content))},
		"plan":        {plan, decodePlanOrigins(encodePlanOrigins(plan))},
		"observation": {observation, decodeObservationOrigins(encodeObservationOrigins(observation))},
	} {
		if !reflect.DeepEqual(test.input, test.decoded) {
			t.Errorf("%s codec lost result-local origin fields: %#v", name, test.decoded)
		}
	}
	contentCopy := cloneOriginReview(content)
	planCopy := clonePlanOrigins(plan)
	observationCopy := cloneObservationOrigins(observation)
	*contentCopy.Spans[0].Field.BlockIndex = 9
	*observationCopy.Spans[0].ItemIndex = 9
	contentCopy.Spans[0].SourceRefs[0] = "other"
	planCopy.Sources[0].Text = "changed"
	observationCopy.Spans[0].SourceRefs[0] = "other"
	if *content.Spans[0].Field.BlockIndex != 1 || content.Spans[0].SourceRefs[0] != "memo" || plan.Sources[0].Text != "카페😀" || *observation.Spans[0].ItemIndex != 1 || observation.Spans[0].SourceRefs[0] != "memo" {
		t.Fatal("origin clone aliases caller-owned evidence")
	}
}

func TestMalformedOriginSidecarsNeverRejectOtherwiseValidCanonicalEnvelope(t *testing.T) {
	type envelope struct {
		Canonical   string                       `json:"canonical"`
		Content     *originReviewJSON            `json:"content_origins,omitempty"`
		Plan        *planOriginReviewJSON        `json:"plan_origins,omitempty"`
		Observation *observationOriginReviewJSON `json:"observation_origins,omitempty"`
	}
	for _, malformed := range []string{`"wrong"`, `7`, `[]`, `{"version":999}`, `{"version":1,"sources":"wrong"}`, `{"version":1,"spans":[{"start":"wrong"}]}`} {
		var decoded envelope
		raw := `{"canonical":"usable","content_origins":` + malformed + `,"plan_origins":` + malformed + `,"observation_origins":` + malformed + `}`
		if err := json.Unmarshal([]byte(raw), &decoded); err != nil || decoded.Canonical != "usable" || decodeOriginReview(decoded.Content) != nil || decodePlanOrigins(decoded.Plan) != nil || decodeObservationOrigins(decoded.Observation) != nil {
			t.Fatalf("optional malformed sidecar rejected/created canonical evidence: %+v %v", decoded, err)
		}
	}
	legacy, err := json.Marshal(envelope{Canonical: "usable"})
	if err != nil || string(legacy) != `{"canonical":"usable"}` {
		t.Fatalf("nil origin metadata changed exact legacy envelope bytes: %s %v", legacy, err)
	}
}
