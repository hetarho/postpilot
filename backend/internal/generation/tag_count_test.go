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
	"github.com/postpilot/backend/internal/post"
)

// GEN-46: the frozen per-post tag count is a minimum every prompt asks for and every payload
// carries; the parser keeps what the model returns up to post.TagOutputMax and never fails a
// write over a shorter list.

func TestParseContentKeepsEveryTagAndAcceptsFewer(t *testing.T) {
	body := `,"blocks":[{"type":"TEXT","content":"본문"}]}`
	cases := map[string]struct {
		tags string
		want []string
	}{
		"eight stay eight": {`["a","b","c","d","e","f","g","h"]`, []string{"a", "b", "c", "d", "e", "f", "g", "h"}},
		"two stay two":     {`["a","b"]`, []string{"a", "b"}},
		"none stay none":   {`[]`, nil},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			content, err := ParseContent(`{"title":"제목","summary":"요약","tags":` + tc.tags + body)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(content.Tags, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("tags = %v, want %v", content.Tags, tc.want)
			}
		})
	}
	if _, err := ParseContent(`{"title":"제목","summary":"요약"` + body); err == nil {
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

func TestPromptsAskForAtLeastTheFrozenTagCountAndPreserveUnrequestedTags(t *testing.T) {
	writeKo, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageKorean, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	writeEn, _ := BuildWritePromptForLanguage(WritePromptInput{Language: LanguageEnglish, Profile: goldenProfile(), Memo: "memo", Title: "title", TagCount: 7})
	reviseKo, _ := BuildRevisePromptForLanguage(LanguageKorean, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, FrozenGuidelines{})
	reviseEn, _ := BuildRevisePromptForLanguage(LanguageEnglish, goldenProfile(), goldenContent(), nil, "shorten", nil, 7, nil, FrozenGuidelines{})
	for name, tc := range map[string]struct{ prompt, want string }{
		"write ko":  {writeKo, "7개 이상의 tags"},
		"write en":  {writeEn, "at least 7 tags"},
		"revise ko": {reviseKo, "태그를 고쳐 달라는 요청이 있으면 tags를 7개 이상으로 다시 고르고, 없으면 현재 tags를 순서와 띄어쓰기까지 그대로"},
		"revise en": {reviseEn, "reselect at least 7; otherwise return the current tags unchanged"},
	} {
		if !strings.Contains(tc.prompt, tc.want) {
			t.Errorf("%s: prompt lacks %q:\n%s", name, tc.want, tc.prompt)
		}
		for _, stale := range []string{"3–6", "최대 7", "0개여도", "태그를 덧붙이지", "at most 7", "upper bound", "never pad", "exactly 7 tags", "정확히 7개"} {
			if strings.Contains(tc.prompt, stale) {
				t.Errorf("%s: a cap-era tag instruction survived: %q", name, stale)
			}
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

func TestTagParserKeepsEveryReturnedTagUpToTheOutputMax(t *testing.T) {
	for returned := 0; returned <= post.TagOutputMax+5; returned++ {
		t.Run(fmt.Sprintf("returned%d", returned), func(t *testing.T) {
			tags := make([]string, returned)
			for index := range tags {
				tags[index] = fmt.Sprintf("tag%d", index)
			}
			parsed, err := ParseContent(tagResponse(t, tags))
			if err != nil {
				t.Fatal(err)
			}
			expected := tags[:min(returned, post.TagOutputMax)]
			if !slices.Equal(parsed.Tags, expected) {
				t.Fatalf("tags padded/reordered/cut=%v expected=%v", parsed.Tags, expected)
			}
		})
	}
}
func TestRevisionTagParserPreservesUnchangedExactArrays(t *testing.T) {
	long := make([]string, post.TagOutputMax+3)
	for index := range long {
		long[index] = fmt.Sprintf("legacy-%d", index)
	}
	for name, old := range map[string][]string{
		"spacing and duplicates": {"  Keep spacing  ", "둘째 태그", "duplicate", "duplicate", "last"},
		"past the output max":    long,
	} {
		t.Run(name, func(t *testing.T) {
			current := PostContent{Tags: slices.Clone(old)}
			parsed, err := ParseRevisionContent(tagResponse(t, old), current)
			if err != nil || !slices.Equal(parsed.Tags, old) {
				t.Fatalf("unchanged tags altered=%v err=%v", parsed, err)
			}
			parsed.Tags[0] = "changed result"
			if current.Tags[0] != old[0] {
				t.Fatal("parsed output aliased current tags")
			}
		})
	}
	for _, current := range []PostContent{{Tags: nil}, {Tags: []string{}}} {
		parsed, err := ParseRevisionContent(tagResponse(t, []string{}), current)
		if err != nil || (parsed.Tags == nil) != (current.Tags == nil) {
			t.Fatalf("unchanged empty representation lost=%+v err=%v", parsed, err)
		}
	}
}
func TestRevisionTagParserCutsChangedArraysOnlyAtTheOutputMaxAndAcceptsRemoval(t *testing.T) {
	current := PostContent{Tags: []string{"old-a", "old-b", "old-c"}}
	for count := 0; count <= post.TagOutputMax+5; count++ {
		changed := make([]string, count)
		for index := range changed {
			changed[index] = fmt.Sprintf("new-%d", index)
		}
		parsed, err := ParseRevisionContent(tagResponse(t, changed), current)
		if err != nil || !slices.Equal(parsed.Tags, changed[:min(count, post.TagOutputMax)]) {
			t.Fatalf("changed tags count%d=%+v err=%v", count, parsed, err)
		}
	}
	for _, raw := range []string{`{"title":"t","summary":"s","blocks":[]}`, `{"title":"t","summary":"s","tags":null,"blocks":[]}`, `{"title":"t","summary":"s","tags":"bad","blocks":[]}`} {
		if _, err := ParseRevisionContent(raw, current); !errors.Is(err, llm.ErrBadOutput) {
			t.Fatalf("malformed revision accepted=%s err=%v", raw, err)
		}
	}
}

func TestRevisionHandlerPreservesUnrequestedTagsAndKeepsOneCallAndFreshAttachmentFilter(t *testing.T) {
	for _, change := range []bool{false, true} {
		for minimum := 1; minimum <= 10; minimum++ {
			t.Run(fmt.Sprintf("changed%v/min%d", change, minimum), func(t *testing.T) {
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
				payload, err := encodeRevisionPayloadForLanguage(instruction, LanguageKorean, nil, FrozenGuidelines{}, minimum, false)
				if err != nil {
					t.Fatal(err)
				}
				service := NewService(posts, fakeProfiles{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
				if err := service.Revise(context.Background(), RevisionJob{UserID: "alice", PostSlug: "post", WriteModel: writeRef.String(), Payload: payload}, func(string, int, int) {}); err != nil {
					t.Fatal(err)
				}
				// A requested change keeps every returned tag, past the minimum too (GEN-46).
				expected := original
				if change {
					expected = returned
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
