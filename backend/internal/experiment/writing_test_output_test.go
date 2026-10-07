package experiment

import (
	"bytes"
	"errors"
	"reflect"
	"testing"
)

func TestWritingTestOutputRetainsCompleteCanonicalAnswer(t *testing.T) {
	for _, language := range []string{"ko", "en"} {
		t.Run(language, func(t *testing.T) {
			want := TestOutput{
				ContentLanguage: language,
				Content: TestOutputContent{Title: "제목", Summary: "summary", Tags: []string{"tag"}, Blocks: []TestOutputBlock{
					{Type: "TEXT", Content: "본문"},
					{Type: "HEADING", Content: "heading", Level: 3},
					{Type: "IMAGE", File: "photo.jpg", Alt: "alternative", Caption: "caption"},
					{Type: "QUOTE", Content: "quotation"},
					{Type: "LIST", Items: []string{"first", "second"}},
					{Type: "VIDEO", File: "video.mp4", Alt: "video alternative", Caption: "video caption"},
					{Type: "GALLERY", Files: []string{"first.jpg", "second.jpg"}, Layout: "SLIDE", Alt: "group alternative", Caption: "group caption"},
				}},
				Storyline: &TestOutputStoryline{Paragraphs: []TestOutputParagraph{{Text: "plan", Files: []string{"photo.jpg"}}}, MadeWith: []string{"photo.jpg", "video.mp4"}},
				Nouns:     []string{"명사", "noun"},
			}
			raw, err := EncodeTestOutput(want)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeTestOutput(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("answer changed: got %#v, want %#v", got, want)
			}
			// Decoding owns its slices and does not alias the stored private payload.
			got.Content.Tags[0] = "edited"
			again, err := DecodeTestOutput(raw)
			if err != nil || again.Content.Tags[0] != "tag" {
				t.Fatalf("decoded edit changed stored answer: %v, %#v", err, again)
			}
		})
	}
}

func TestWritingTestOutputPreservesMissingAndEmptyStoryline(t *testing.T) {
	for _, storyline := range []*TestOutputStoryline{nil, {}, {Paragraphs: []TestOutputParagraph{}, MadeWith: []string{}}} {
		want := TestOutput{ContentLanguage: "ko", Content: TestOutputContent{Title: "accepted", Blocks: []TestOutputBlock{}, Tags: []string{}}, Nouns: []string{}, Storyline: storyline}
		raw, err := EncodeTestOutput(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeTestOutput(raw)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("accepted paid answer changed: %v, %#v", err, got)
		}
	}
}

func TestWritingTestOutputRejectsUnversionedCorruptAndPrivateFields(t *testing.T) {
	valid, err := EncodeTestOutput(TestOutput{ContentLanguage: "en", Content: TestOutputContent{Blocks: []TestOutputBlock{{Type: "TEXT", Content: "valid"}}}})
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string][]byte{
		"empty":            nil,
		"null":             []byte("null"),
		"unversioned":      bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":0`), 1),
		"future version":   bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":2`), 1),
		"unknown language": bytes.Replace(valid, []byte(`"en"`), []byte(`"fr"`), 1),
		"unknown block":    bytes.Replace(valid, []byte(`"TEXT"`), []byte(`"UNKNOWN"`), 1),
		"supplier cost":    bytes.Replace(valid, []byte(`"version":1`), []byte(`"version":1,"cost_microusd":123`), 1),
		"block credential": bytes.Replace(valid, []byte(`"type":"TEXT"`), []byte(`"type":"TEXT","api_key":"private"`), 1),
		"trailing object":  append(append([]byte(nil), valid...), []byte(`{}`)...),
		"oversized":        bytes.Repeat([]byte(" "), maxTestOutputBytes+1),
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeTestOutput(raw); !errors.Is(err, ErrTestOutputIncompatible) {
				t.Fatalf("invalid output accepted: %v", err)
			}
		})
	}
	if _, err := EncodeTestOutput(TestOutput{ContentLanguage: "fr"}); !errors.Is(err, ErrTestOutputIncompatible) {
		t.Fatalf("invalid provenance accepted: %v", err)
	}
}
