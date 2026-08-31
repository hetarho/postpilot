package rpc

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/proto"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice"
)

func TestVoiceLanguageProtoMapping(t *testing.T) {
	for _, test := range []struct {
		name  string
		value postpilotv1.ContentLanguage
		want  voice.Language
		ok    bool
	}{
		{name: "Korean", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_KOREAN, want: voice.LanguageKorean, ok: true},
		{name: "English", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_ENGLISH, want: voice.LanguageEnglish, ok: true},
		{name: "unspecified", value: postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED},
		{name: "unknown", value: postpilotv1.ContentLanguage(99)},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := languageFromProto(test.value)
			if test.ok {
				if err != nil || got != test.want || languageToProto(got) != test.value {
					t.Fatalf("mapping = %q, %v", got, err)
				}
				return
			}
			want := voice.ErrLanguageRequired
			if test.name == "unknown" {
				want = voice.ErrLanguageUnsupported
			}
			if !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
		})
	}
}

func TestCreateVoiceSourceLanguagePresenceAndUnknownRoundTrip(t *testing.T) {
	if (&postpilotv1.CreateVoiceRequest{}).SourceLanguage != nil {
		t.Fatal("missing source language unexpectedly has presence")
	}
	unspecified := postpilotv1.ContentLanguage_CONTENT_LANGUAGE_UNSPECIFIED
	if request := (&postpilotv1.CreateVoiceRequest{SourceLanguage: &unspecified}); request.SourceLanguage == nil || request.GetSourceLanguage() != unspecified {
		t.Fatal("explicit unspecified source language lost presence")
	}
	unknown := postpilotv1.ContentLanguage(777)
	wire, err := proto.Marshal(&postpilotv1.CreateVoiceRequest{Name: "future", SourceLanguage: &unknown})
	if err != nil {
		t.Fatal(err)
	}
	var decoded postpilotv1.CreateVoiceRequest
	if err := proto.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SourceLanguage == nil || decoded.GetSourceLanguage() != unknown {
		t.Fatalf("unknown enum round trip = %#v", decoded.SourceLanguage)
	}
}
