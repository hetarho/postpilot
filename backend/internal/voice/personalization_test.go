package voice

import (
	"encoding/json"
	"strings"
	"testing"
)

// VOICE-24, VOICE-27: the analysis call's schema names the four AI fields, all required, and the
// prompt asks for nothing the product counts.
func TestTheAnalysisAsksOnlyForTheAIPart(t *testing.T) {
	var schema struct {
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(VoiceAnalysisSchema(), &schema); err != nil {
		t.Fatalf("analysis schema is invalid JSON: %v", err)
	}
	if strings.Join(schema.Required, ",") != "impression,tics,signature_phrases,examples" {
		t.Fatalf("required = %v", schema.Required)
	}
	for _, want := range []string{"수치는 다시 세지 말고", "impression", "tics", "signature_phrases", "examples", "없는 문장은 만들지 마세요"} {
		if !strings.Contains(analysisPrompt, want) {
			t.Fatalf("the analysis prompt lacks %q", want)
		}
	}
}

func TestExcerptUsesFirstBoundaryBetweenTargetAndCap(t *testing.T) {
	body := strings.Repeat("가", 510) + "." + strings.Repeat("나", 400)
	got := excerptAroundTarget(body, 500, 800)
	if len([]rune(got)) != 511 || !strings.HasSuffix(got, ".") {
		t.Fatalf("excerpt length=%d suffix=%q", len([]rune(got)), got[len(got)-1:])
	}
	if got := excerptAroundTarget(strings.Repeat("가", 900), 500, 800); len([]rune(got)) != 800 {
		t.Fatalf("hard-capped excerpt length=%d", len([]rune(got)))
	}
}
