package generation

import (
	"errors"
	"testing"
)

func TestCompleteRevisionEncodingPreservesKnownWireBytes(t *testing.T) {
	options := revisionOptions{Instruction: "edit", Language: LanguageEnglish, TagCount: 7, WriteNativeEffort: true,
		Guidelines: FrozenGuidelines{Owner: []string{"owner"}, Defaults: []string{"default"}}}
	legacy := `{"instruction":"edit","content_language":"en","guidelines":["owner"],"default_guidelines":["default"],"tag_count":7,"write_native_effort":true}`
	for _, version := range []int{0, OriginProtocolVersion} {
		options.OriginProtocolVersion = version
		want := legacy
		if version != 0 {
			options.CompletionTokens = 8192
			want = `{"origin_protocol_version":1,"completion_tokens":8192,` + legacy[1:]
		}
		encoded, err := encodeRevisionOptions(options)
		if err != nil || string(encoded) != want {
			t.Fatalf("protocol %d bytes=%s err=%v want=%s", version, encoded, err, want)
		}
		decoded, err := parseRevisionPayload(encoded)
		if err != nil || decoded.OriginProtocolVersion != version || decoded.CompletionTokens != options.CompletionTokens {
			t.Fatalf("protocol/cap round trip=%+v err=%v", decoded, err)
		}
	}
	legacyEncoded, err := encodeRevisionPayloadForLanguage("edit", LanguageEnglish, nil, options.Guidelines, 7, true)
	if err != nil || string(legacyEncoded) != legacy {
		t.Fatalf("legacy helper bytes=%s err=%v", legacyEncoded, err)
	}
}

func TestCompleteRevisionEncodingRejectsMissingLanguageBeforePayloadCreation(t *testing.T) {
	if value, err := encodeRevisionOptions(revisionOptions{Instruction: "edit"}); value != nil || !errors.Is(err, ErrContentLanguageRequired) {
		t.Fatalf("bytes=%s err=%v", value, err)
	}
}
