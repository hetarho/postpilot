package clip

import (
	"encoding/json"
	"testing"
)

func TestGenerationPayloadFreezesLanguageAndReadsLegacyKorean(t *testing.T) {
	for _, tc := range []struct {
		raw, language string
		valid         bool
	}{
		{`{"Version":3}`, "ko", true}, {`{"Version":4}`, "ko", true},
		{`{"Version":5,"Language":"ko"}`, "ko", true}, {`{"Version":5,"Language":"en"}`, "en", true},
		{`{"Version":5}`, "", false}, {`{"Version":5,"Language":"ja"}`, "", false},
		{`{"Version":5,"Language":"en","unexpected":true}`, "", false},
	} {
		var p generationPayload
		err := json.Unmarshal([]byte(tc.raw), &p)
		if (err == nil) != tc.valid || tc.valid && p.Language != tc.language {
			t.Fatalf("%s: %+v %v", tc.raw, p, err)
		}
	}
	ko := generationPayload{Language: "ko"}
	en := ko
	en.Language = "en"
	if planRecoveryDigest(ko) == planRecoveryDigest(en) {
		t.Fatal("a language change reused the plan")
	}
	if QuoteInputDigest(Project{Language: "ko"}, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) == QuoteInputDigest(Project{Language: "en"}, VideoTemplate{}, SourceBatch{}, GenerationPricing{}) {
		t.Fatal("a language change reused the quote")
	}
}
