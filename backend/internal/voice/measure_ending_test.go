package voice

import "testing"

// VOICE-37: a composed formal ending — 합니다, 입니다, 갑니다 — is 습니다 in the distribution,
// exactly as the diff's ending reader classifies it, never the plain 다 it also ends in.
func TestMeasureCountsComposedFormalEndingsAsFormal(t *testing.T) {
	got := Measure("오늘은 맛있게 먹었습니다. 여기가 제 단골집입니다. 다음에 또 갑니다. 정말 좋았다.")
	ratios := map[string]float64{}
	for _, entry := range got.EndingDistribution {
		ratios[entry.Ending] = entry.Ratio
	}
	if ratios["습니다"] != 0.75 || ratios["다"] != 0.25 {
		t.Fatalf("ending distribution = %+v", got.EndingDistribution)
	}
	for _, sentence := range []string{"여기가 제 단골집입니다.", "다음에 또 갑니다."} {
		if endingOf(sentence) != "습니다" {
			t.Fatalf("diff reads %q as %q", sentence, endingOf(sentence))
		}
	}
}
