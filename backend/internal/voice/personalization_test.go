package voice

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMeasureKoreanEndingsAndUnicodeLength(t *testing.T) {
	got := Measure("오늘은 좋아요. 정말 좋습니다! 다음에도 온다.\n짧아요.")
	if len(got.Sentences) != 4 || got.AverageSentenceChars <= 0 {
		t.Fatalf("measure = %+v", got)
	}
	ratios := map[string]float64{}
	for _, item := range got.EndingDistribution {
		ratios[item.Ending] = item.Ratio
	}
	if ratios["해요"] != .5 || ratios["습니다"] != .25 || ratios["다"] != .25 {
		t.Fatalf("ending ratios = %+v", ratios)
	}
}

// A voice is Korean (VOICE-10): the analysis attaches the one embedded schema, which is valid
// JSON, and asks about a Korean corpus.
func TestTheAnalysisIsKoreanOnly(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal(VoiceAnalysisSchema(), &schema); err != nil {
		t.Fatalf("analysis schema is invalid JSON: %v", err)
	}
	if !strings.Contains(structuredAnalysisPrompt, "Korean authored corpus") {
		t.Fatalf("analysis prompt = %q", structuredAnalysisPrompt)
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
