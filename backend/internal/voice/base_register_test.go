package voice

import (
	"strings"
	"testing"
	"time"
)

// VOICE-26/27: the measured ending register wins. A model that leaves base_register empty
// adds nothing, so the measured register stands instead of turning into 알 수 없음; the model
// fills the register only where measurement had none.
func TestAnEmptyModelRegisterLeavesTheMeasuredOne(t *testing.T) {
	unknown := func(v string) VoiceValue {
		if strings.TrimSpace(v) == "" {
			return VoiceValue{Unknown: true, Source: SourceUnknown}
		}
		return VoiceValue{Value: strings.TrimSpace(v), Source: SourceAnalyzed}
	}
	profile := MeasuredProfile("오늘은 맛있게 먹었어요. 다음에 또 갈 거예요. 정말 좋았어요.", time.Now)
	measured := profile.Endings.BaseRegister
	if measured.Unknown || measured.Value == "" {
		t.Fatalf("the fixture measured no register: %+v", measured)
	}
	mergeQualitativeProfile(&profile, qualitativeJSON{}, unknown)
	if profile.Endings.BaseRegister != measured {
		t.Fatalf("an empty model register replaced the measured %+v with %+v", measured, profile.Endings.BaseRegister)
	}
	unmeasured := StructuredProfile{}
	unmeasured.Endings.BaseRegister = VoiceValue{Unknown: true, Source: SourceUnknown}
	mergeQualitativeProfile(&unmeasured, qualitativeJSON{BaseRegister: "해요체"}, unknown)
	if unmeasured.Endings.BaseRegister.Value != "해요체" || unmeasured.Endings.BaseRegister.Unknown {
		t.Fatalf("the model did not fill a register measurement lacked: %+v", unmeasured.Endings.BaseRegister)
	}
}
