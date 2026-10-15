package clip

import (
	"errors"
	"github.com/postpilot/backend/internal/clip/composition"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUploadTextBoundsWhatEachChannelActuallyCopies(t *testing.T) {
	for _, channel := range []string{ChannelNaver, ChannelInstagram} {
		for _, language := range []string{"ko", "en"} {
			for _, on := range []bool{false, true} {
				in := UploadTextInput{Channel: channel, Language: language, Campaign: on, AdvertiserGuide: &AdvertiserGuide{Hashtags: []string{"#필수 하나", "필수둘", "셋", "넷", "다섯"}}}
				text, e := BoundGeneratedUploadText(in, strings.Repeat("가🚀", 2200), []string{"optional", "OPTIONAL"})
				if e != nil {
					t.Fatal(e)
				}
				if !utf8.ValidString(text.Body) || len(text.Tags) > UploadTextMaxTags {
					t.Fatal(text)
				}
				if on && (len(text.Tags) != 5 || text.Tags[0] != "필수하나") {
					t.Fatal("required tags lost", text.Tags)
				}
				body := text.Body
				if on {
					body = UploadTextDisclosure(language) + "\n\n" + body
				}
				limits, _ := UploadLimits(channel)
				if limits.TagsInCaption && len(text.Tags) > 0 {
					body += "\n\n#" + strings.Join(text.Tags, " #")
				}
				if utf8.RuneCountInString(body) > limits.MaxChars {
					t.Fatal("copied caption exceeded limit", channel, language, on)
				}
				room, e := UploadTextBodyLimit(channel, language, on, text.Tags)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = ValidateUploadTextEdit(channel, language, on, strings.Repeat("가", room), text.Tags); e != nil {
					t.Fatal(e)
				}
				if _, e = ValidateUploadTextEdit(channel, language, on, strings.Repeat("가", room+1), text.Tags); !errors.Is(e, ErrUploadTextLimit) {
					t.Fatal("owner edit was silently cut", e)
				}
			}
		}
	}
}
func TestUploadTextKeepsRequiredTagsBeforeOptionalOnes(t *testing.T) {
	in := UploadTextInput{Channel: ChannelInstagram, Language: "ko", Campaign: true, AdvertiserGuide: &AdvertiserGuide{Hashtags: []string{"#제주"}}}
	got, e := BoundGeneratedUploadText(in, "직접 쓴 말", []string{"제 주", "JEJU", "c", "d", "e", "discarded"})
	if e != nil || len(got.Tags) != 5 || got.Tags[0] != "제주" {
		t.Fatal(got, e)
	}
	in.AdvertiserGuide.Hashtags = []string{"a", "b", "c", "d", "e", "f"}
	if _, e = BoundGeneratedUploadText(in, "text", nil); !errors.Is(e, ErrUploadTextLimit) {
		t.Fatal(e)
	}
}

func TestUploadTextUsesEditedRowsAndPhrasesInsteadOfObsoleteParentText(t *testing.T) {
	elements := []PortableText{
		{Resolved: composition.ResolvedElement{InstanceID: "intro", Element: composition.Element{Role: "hook"}, Text: "obsolete", Rows: []composition.ResolvedRow{{Text: "current first"}, {Text: "current second"}, {Text: "not drawn"}}}},
		{Resolved: composition.ResolvedElement{InstanceID: "body", Text: "old words"}, Phrases: []EditablePhrase{{Text: "edited phrase"}, {Text: "next phrase"}}},
	}
	got := uploadPortableCaptions(elements, composition.UnchosenDesign())
	want := []string{"current first", "current second", "edited phrase", "next phrase"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("captions = %#v, want %#v", got, want)
	}
}
