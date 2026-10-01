package template

import (
	"context"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

type fakeEstimator struct {
	credits int
	ok      bool
	asked   [2]int64
}

func (f *fakeEstimator) CallCredits(_ context.Context, _ llm.ModelInfo, prompt, completion int64) (int, bool) {
	f.asked = [2]int64{prompt, completion}
	return f.credits, f.ok
}

// QUOTA-67: one request is the catalog estimate of one call on the write model; a free write
// registration is 무료, and a model with no bounded price has no figure.
func TestEstimateRequest(t *testing.T) {
	paid := writerInfo()
	paid.Levels = map[string]string{llm.StageNameWrite: "value"}
	free := writerInfo()
	free.Levels = map[string]string{llm.StageNameWrite: "free"}
	cases := []struct {
		name      string
		info      llm.ModelInfo
		known     bool
		ref       string
		estimator fakeEstimator
		want      RequestEstimate
	}{
		{"priced", paid, true, "p/m", fakeEstimator{credits: 3, ok: true}, RequestEstimate{Credits: 3, Available: true}},
		{"free", free, true, "p/m", fakeEstimator{credits: 9, ok: true}, RequestEstimate{Free: true, Available: true}},
		{"unpriced", paid, true, "p/m", fakeEstimator{}, RequestEstimate{}},
		{"unclassified", writerInfo(), true, "p/m", fakeEstimator{credits: 3, ok: true}, RequestEstimate{}},
		{"unknown", paid, false, "p/m", fakeEstimator{credits: 3, ok: true}, RequestEstimate{}},
		{"no ref", paid, true, "", fakeEstimator{credits: 3, ok: true}, RequestEstimate{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRequestHarness(t)
			h.models.info, h.models.known = tc.info, tc.known
			estimator := tc.estimator
			h.service.ConfigureEstimate(&estimator)
			got, err := h.service.EstimateRequest(context.Background(), tc.ref)
			if err != nil || got != tc.want {
				t.Fatalf("estimate = %+v, %v; want %+v", got, err, tc.want)
			}
			if tc.name == "priced" && estimator.asked != [2]int64{requestEstimatePromptTokens, requestEstimateCompletionTokens} {
				t.Fatalf("priced %v", estimator.asked)
			}
		})
	}
}
