package generation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/llm"
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
	raw, err := encodeRevisionPayloadForLanguage("shorten", LanguageKorean, nil, FrozenGuidelines{}, 7, false)
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

func TestPromptsAskForAtMostTheFrozenTagCountAndPreserveUnrequestedTags(t *testing.T) {
	writeKo, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	writeEn, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	reviseKo, _ := BuildRevisePromptForLanguage(LanguageKorean, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, FrozenGuidelines{})
	reviseEn, _ := BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, FrozenGuidelines{})
	for name, tc := range map[string]struct{ prompt, want string }{
		"write ko":  {writeKo, "최대 7개의 tags"},
		"write en":  {writeEn, "at most 7 tags"},
		"revise ko": {reviseKo, "태그 변경을 요청받은 경우에만 최대 7개"},
		"revise en": {reviseEn, "at most 7 tags"},
	} {
		if !strings.Contains(tc.prompt, tc.want) {
			t.Errorf("%s: prompt lacks %q:\n%s", name, tc.want, tc.prompt)
		}
		if strings.Contains(tc.prompt, "3–6") {
			t.Errorf("%s: the fixed range survived", name)
		}
		if strings.Contains(tc.prompt, "exactly 7 tags") || strings.Contains(tc.prompt, "정확히 7개") {
			t.Errorf("%s still requires padding", name)
		}
	}
}

func tagResponse(t *testing.T, tags []string) string {
	t.Helper()
	raw, err := json.Marshal(contentJSON{Title: "Owner title", Summary: "Summary", Tags: tags, Blocks: []blockJSON{{Type: "TEXT", Content: "Revised full prose"}}})
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestTagUpperBoundsAcceptEveryCountZeroAndFewerWithoutPadding(t *testing.T) {
	for maximum := 1; maximum <= 10; maximum++ {
		for returned := 0; returned <= 12; returned++ {
			t.Run(fmt.Sprintf("cap%d/returned%d", maximum, returned), func(t *testing.T) {
				tags := make([]string, returned)
				for index := range tags {
					tags[index] = fmt.Sprintf("tag%d", index)
				}
				parsed, err := ParseContent(tagResponse(t, tags), maximum)
				if err != nil {
					t.Fatal(err)
				}
				expected := tags[:min(returned, maximum)]
				if !slices.Equal(parsed.Tags, expected) {
					t.Fatalf("tags padded/reordered=%v expected=%v", parsed.Tags, expected)
				}
			})
		}
	}
}
func TestRevisionTagParserPreservesUnchangedExactArraysBeforeNewUpperBound(t *testing.T) {
	old := []string{"  Keep spacing  ", "둘째 태그", "duplicate", "duplicate", "last"}
	for maximum := 1; maximum <= 10; maximum++ {
		t.Run(fmt.Sprint(maximum), func(t *testing.T) {
			current := PostContent{Tags: slices.Clone(old)}
			parsed, err := ParseRevisionContent(tagResponse(t, old), maximum, current)
			if err != nil || !slices.Equal(parsed.Tags, old) {
				t.Fatalf("unchanged tags capped=%v err=%v", parsed, err)
			}
			parsed.Tags[0] = "changed result"
			if current.Tags[0] != old[0] {
				t.Fatal("parsed output aliased current tags")
			}
		})
	}
	for _, current := range []PostContent{{Tags: nil}, {Tags: []string{}}} {
		parsed, err := ParseRevisionContent(tagResponse(t, []string{}), 1, current)
		if err != nil || (parsed.Tags == nil) != (current.Tags == nil) {
			t.Fatalf("unchanged empty representation lost=%+v err=%v", parsed, err)
		}
	}
}
func TestRevisionTagParserCapsChangedArraysAndAcceptsExplicitRemoval(t *testing.T) {
	current := PostContent{Tags: []string{"old-a", "old-b", "old-c"}}
	for maximum := 1; maximum <= 10; maximum++ {
		for count := 0; count <= 12; count++ {
			changed := make([]string, count)
			for index := range changed {
				changed[index] = fmt.Sprintf("new-%d", index)
			}
			parsed, err := ParseRevisionContent(tagResponse(t, changed), maximum, current)
			if err != nil || !slices.Equal(parsed.Tags, changed[:min(count, maximum)]) {
				t.Fatalf("changed tags cap%d count%d=%+v err=%v", maximum, count, parsed, err)
			}
		}
	}
	for _, raw := range []string{`{"title":"t","summary":"s","blocks":[]}`, `{"title":"t","summary":"s","tags":null,"blocks":[]}`, `{"title":"t","summary":"s","tags":"bad","blocks":[]}`} {
		if _, err := ParseRevisionContent(raw, 1, current); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("malformed revision accepted=%s err=%v", raw, err)
		}
	}
}

func TestRevisionHandlerPreservesUnrequestedTagsAndKeepsOneCallAndFreshAttachmentFilter(t *testing.T) {
	for _, change := range []bool{false, true} {
		for maximum := 1; maximum <= 10; maximum++ {
			t.Run(fmt.Sprintf("changed%v/cap%d", change, maximum), func(t *testing.T) {
				original := []string{" first ", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth", "legacy-extra"}
				current := PostContent{Title: "Original", Summary: "Summary", Tags: slices.Clone(original), Blocks: []Block{{Type: BlockText, Content: "Original body"}}}
				posts := &fakePosts{input: PostInput{UserID: "alice", Slug: "post", Content: &current, Images: []Image{{Kind: AttachmentPhoto, Filename: "gone.jpg", Key: "photo"}, {Kind: AttachmentVideo, Filename: "gone.mp4", Key: "video"}}}}
				models := newFakeModels()
				returned := slices.Clone(original)
				instruction := "Only revise the prose, including the phrase tags in this narrative."
				if change {
					returned = []string{"new-a", "new-b", "new-c", "new-d", "new-e", "new-f", "new-g", "new-h", "new-i", "new-j", "new-k"}
					instruction = "Choose a new tag array from the supplied material"
				}
				models.complete = func(_ llm.ModelRef, _ llm.Request) (llm.Response, error) {
					posts.input.Images = nil
					wire := contentJSON{Title: current.Title, Summary: current.Summary, Tags: returned, Blocks: []blockJSON{{Type: "TEXT", Content: "Revised body"}, {Type: "LIST", Items: []string{"first", "second"}}, {Type: "VIDEO", File: "gone.mp4"}, {Type: "IMAGE", File: "gone.jpg"}}}
					raw, _ := json.Marshal(wire)
					return llm.Response{Text: string(raw)}, nil
				}
				payload, err := encodeRevisionPayloadForLanguage(instruction, LanguageKorean, nil, FrozenGuidelines{}, maximum, false)
				if err != nil {
					t.Fatal(err)
				}
				service := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
				if err := service.Revise(context.Background(), RevisionJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: payload}, func(string, int, int) {}); err != nil {
					t.Fatal(err)
				}
				expected := original
				if change {
					expected = returned[:maximum]
				}
				if len(models.calls) != 1 || len(posts.contents) != 1 || !slices.Equal(posts.contents[0].Tags, expected) || len(posts.contents[0].Blocks) != 2 || posts.contents[0].Blocks[1].Type != BlockList {
					t.Fatalf("tag/call/filter regression calls=%d contents=%+v", len(models.calls), posts.contents)
				}
				if models.calls[0].request.Stage != llm.StageNameWrite || models.calls[0].request.MaxTokens != testBudget.Revise(contentChars(&current), nil, false) {
					t.Fatal("revision changed stage or completion budget")
				}
			})
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
