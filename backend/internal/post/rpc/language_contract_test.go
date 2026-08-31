package rpc

import (
	"errors"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
	"google.golang.org/protobuf/proto"
)

func TestLanguageProtoMappingIsClosedAndOptionalAbsenceIsDistinct(t *testing.T) {
	for _, test := range []struct {
		name  string
		value postpilotv1.ContentLanguage
		want  post.Language
		ok    bool
	}{
		{name: "Korean", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN, want: post.LanguageKorean, ok: true},
		{name: "English", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH, want: post.LanguageEnglish, ok: true},
		{name: "unspecified", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
		{name: "unknown", value: postpilotv1.ContentLanguage(99)},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := languageFromProto(test.value)
			if test.ok {
				if err != nil || got != test.want {
					t.Fatalf("mapped = %q, %v; want %q", got, err, test.want)
				}
				return
			}
			if !errors.Is(err, post.ErrLanguageRequired) {
				t.Fatalf("error = %v, want ErrLanguageRequired", err)
			}
		})
	}

	got, err := optionalLanguageFromProto(nil)
	if err != nil || got != nil {
		t.Fatalf("absent optional = %v, %v", got, err)
	}
	unspecified := postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	if _, err := optionalLanguageFromProto(&unspecified); !errors.Is(err, post.ErrLanguageRequired) {
		t.Fatalf("present unspecified = %v, want ErrLanguageRequired", err)
	}
}

func TestUnknownLanguageEnumSurvivesProtobufRoundTrip(t *testing.T) {
	unknown := postpilotv1.ContentLanguage(99)
	input := &postpilotv1.SavePostDraftRequest{Slug: "post", TargetLanguage: &unknown}
	wire, err := proto.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output postpilotv1.SavePostDraftRequest
	if err := proto.Unmarshal(wire, &output); err != nil {
		t.Fatal(err)
	}
	if output.TargetLanguage == nil || *output.TargetLanguage != unknown {
		t.Fatalf("round-trip target = %v, want numeric enum %d", output.TargetLanguage, unknown)
	}
	if _, err := optionalLanguageFromProto(output.TargetLanguage); !errors.Is(err, post.ErrLanguageRequired) {
		t.Fatalf("unknown enum accepted at domain boundary: %v", err)
	}
}
