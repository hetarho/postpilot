package generation

import (
	"strings"
	"testing"
)

// GEN-46: the frozen per-post tag count reaches every prompt, survives every payload, and
// trims rather than fails.

func TestParseContentTrimsSurplusTagsAndAcceptsFewer(t *testing.T) {
	body := `,"blocks":[{"type":"TEXT","content":"본문"}]}`
	cases := map[string]struct {
		tags string
		want []string
	}{
		"eight to four":  {`["a","b","c","d","e","f","g","h"]`, []string{"a", "b", "c", "d"}},
		"two stay two":   {`["a","b"]`, []string{"a", "b"}},
		"none stay none": {`[]`, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			content, err := ParseContent(`{"title":"제목","summary":"요약","tags":`+tc.tags+body, 4)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(content.Tags, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("tags = %v, want %v", content.Tags, tc.want)
			}
		})
	}
	if _, err := ParseContent(`{"title":"제목","summary":"요약"`+body, 4); err == nil {
		t.Fatal("a missing tags field must still be bad output")
	}
}

func TestGenerationPayloadFreezesTheTagCount(t *testing.T) {
	raw, err := encodeGenerationPayload(generationOptions{TargetLanguage: LanguageKorean, TagCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeGenerationPayload(raw)
	if err != nil || decoded.TagCount != 7 {
		t.Fatalf("decoded = %+v err=%v", decoded, err)
	}
	for name, legacy := range map[string][]byte{
		"before the member": []byte(`{"target_language":"ko"}`),
		"empty payload":     nil,
	} {
		decoded, err := decodeGenerationPayload(legacy)
		if err != nil || decoded.TagCount != 4 {
			t.Fatalf("%s: decoded = %+v err=%v, want the default 4", name, decoded, err)
		}
	}
}

func TestRevisionPayloadFreezesTheTagCount(t *testing.T) {
	raw, err := encodeRevisionPayloadForLanguage("shorten", false, LanguageKorean, nil, nil, 7, false)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := parseRevisionPayload(raw)
	if err != nil || payload.TagCount != 7 {
		t.Fatalf("payload = %+v err=%v", payload, err)
	}
	legacy, err := parseRevisionPayload([]byte(`{"instruction":"shorten","content_language":"ko"}`))
	if err != nil || resolveTagCount(legacy.TagCount) != 4 {
		t.Fatalf("legacy payload = %+v err=%v, want the default 4", legacy, err)
	}
}

func TestPromptsAskForExactlyTheFrozenTagCount(t *testing.T) {
	writeKo, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	writeEn, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	reviseKo, _ := BuildRevisePromptForLanguage(LanguageKorean, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, nil)
	reviseEn, _ := BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, nil)
	for name, tc := range map[string]struct{ prompt, want string }{
		"write ko":  {writeKo, "정확히 7개의 tags"},
		"write en":  {writeEn, "exactly 7 tags"},
		"revise ko": {reviseKo, "태그를 바꾸라는 요청이면 정확히 7개로 유지하세요."},
		"revise en": {reviseEn, "A requested tag change keeps exactly 7 tags."},
	} {
		if !strings.Contains(tc.prompt, tc.want) {
			t.Errorf("%s: prompt lacks %q:\n%s", name, tc.want, tc.prompt)
		}
		if strings.Contains(tc.prompt, "3–6") {
			t.Errorf("%s: the fixed range survived", name)
		}
	}
}

func TestWriteSnapshotCarriesTheTagCount(t *testing.T) {
	four, _ := encodeWriteSnapshot(writeSnapshot{TargetLanguage: LanguageKorean, Post: PostInput{TargetLanguage: LanguageKorean, TagCount: 4}})
	seven, _ := encodeWriteSnapshot(writeSnapshot{TargetLanguage: LanguageKorean, Post: PostInput{TargetLanguage: LanguageKorean, TagCount: 7}})
	if string(four) == string(seven) {
		t.Fatal("two tag counts froze to the same snapshot bytes, so their hashes would collide")
	}
	if !strings.Contains(string(seven), `"tag_count":7`) {
		t.Fatalf("snapshot does not carry the count: %s", seven)
	}
	decoded, err := decodeWriteSnapshot([]byte(`{"kind":"write","target_language":"ko","post":{"target_language":"ko"}}`))
	if err != nil || decoded.Post.TagCount != 4 {
		t.Fatalf("legacy snapshot = %+v err=%v, want the default 4", decoded.Post, err)
	}
	decodedSeven, err := decodeWriteSnapshot(seven)
	if err != nil || decodedSeven.Post.TagCount != 7 {
		t.Fatalf("snapshot round trip = %+v err=%v", decodedSeven.Post, err)
	}
}
