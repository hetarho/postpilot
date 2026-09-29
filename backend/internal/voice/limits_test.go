package voice_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/voice"
)

func TestPersonalizationDefaultsContainNoScheduler(t *testing.T) {
	got := voice.PersonalizationThresholds()
	if got.FewShotMax != 3 || got.FewShotExcerptTargetChars != 500 || got.FewShotExcerptMaxChars != 800 || got.EndingMaxConsecutive != 2 {
		t.Fatalf("voice personalization defaults = %+v", got)
	}
	typeOf := reflect.TypeOf(got)
	for i := 0; i < typeOf.NumField(); i++ {
		name := strings.ToLower(typeOf.Field(i).Name)
		if strings.Contains(name, "interval") || strings.Contains(name, "schedule") || strings.Contains(name, "sweep") {
			t.Fatalf("scheduled personalization config is forbidden: %s", typeOf.Field(i).Name)
		}
	}
}
