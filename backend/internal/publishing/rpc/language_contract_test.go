package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/publishing"
)

func TestPublishingLanguageProtoMapping(t *testing.T) {
	for _, test := range []struct {
		name  string
		value publishing.Language
		want  postpilotv1.ContentLanguage
	}{
		{name: "Korean", value: publishing.LanguageKorean, want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN},
		{name: "English", value: publishing.LanguageEnglish, want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH},
		{name: "empty", want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
		{name: "unknown", value: publishing.Language("fr"), want: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := toProtoLanguage(test.value); got != test.want {
				t.Fatalf("mapped = %s, want %s", got, test.want)
			}
		})
	}
}
