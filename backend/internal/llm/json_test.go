package llm

import "testing"

func TestJSONCandidateFallback(t *testing.T) {
	for _, raw := range []string{`{"value":"brace } and quote \""}`, "```json\n{\"value\":1}\n```", `Explanation {"value": {"nested":true}} trailing`} {
		if _, ok := JSONCandidate(raw); !ok {
			t.Fatal(raw)
		}
	}
	for _, raw := range []string{"nothing", "{\"partial\":", "[]", "{not JSON}"} {
		if _, ok := JSONCandidate(raw); ok {
			t.Fatal(raw)
		}
	}
	if ResponseParseError(Response{FinishReason: "length"}, nil) != nil {
		t.Fatal("usable length-limited JSON rejected")
	}
}
