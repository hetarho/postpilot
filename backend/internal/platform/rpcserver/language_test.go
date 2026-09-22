package rpcserver

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func TestContentLanguageMappingCoversGeneratedEnum(t *testing.T) {
	expected := map[postpilotv1.ContentLanguage]string{
		postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN:  "ko",
		postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH: "en",
	}
	if got, want := len(postpilotv1.ContentLanguage_name), len(expected)+1; got != want {
		t.Fatalf("generated languages = %d, want %d; update the closed mapping", got, want)
	}

	for number, name := range postpilotv1.ContentLanguage_name {
		wire := postpilotv1.ContentLanguage(number)
		t.Run(name, func(t *testing.T) {
			tag, ok := ContentLanguageFromProto(wire)
			if wire == postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED {
				if ok || tag != "" {
					t.Fatalf("unspecified mapped to %q, %v", tag, ok)
				}
				return
			}
			want, exists := expected[wire]
			if !exists {
				t.Fatalf("generated enum %s has no domain mapping", name)
			}
			if !ok || tag != want {
				t.Fatalf("mapping = %q, %v; want %q, true", tag, ok, want)
			}
			if roundTrip := ContentLanguageToProto(tag); roundTrip != wire {
				t.Fatalf("round trip = %s, want %s", roundTrip, wire)
			}
		})
	}
}

func TestContentLanguageMappingRejectsUnknownValues(t *testing.T) {
	if tag, ok := ContentLanguageFromProto(postpilotv1.ContentLanguage(999)); ok || tag != "" {
		t.Fatalf("unknown wire value mapped to %q, %v", tag, ok)
	}
	for _, tag := range []string{"", "fr", "KO"} {
		if got := ContentLanguageToProto(tag); got != postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED {
			t.Errorf("tag %q mapped to valid language %s", tag, got)
		}
	}
}
