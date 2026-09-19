package app

import (
	"encoding/json"
	"testing"

	"github.com/postpilot/backend/internal/clip"
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
		var p clip.GenerationPayload
		err := json.Unmarshal([]byte(tc.raw), &p)
		if (err == nil) != tc.valid || tc.valid && p.Language != tc.language {
			t.Fatalf("%s: %+v %v", tc.raw, p, err)
		}
	}
	ko := clip.GenerationPayload{Language: "ko"}
	en := ko
	en.Language = "en"
	if planRecoveryDigest(ko) == planRecoveryDigest(en) {
		t.Fatal("a language change reused the plan")
	}
	if clip.QuoteInputDigest(clip.Project{Language: "ko"}, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) == clip.QuoteInputDigest(clip.Project{Language: "en"}, clip.VideoTemplate{}, clip.SourceBatch{}, clip.GenerationPricing{}) {
		t.Fatal("a language change reused the quote")
	}
}
