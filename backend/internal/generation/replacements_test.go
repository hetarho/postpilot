package generation

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/llm"
)

var testPhrases = []string{"솔직 후기", "분위기 좋은 카페", "디저트 맛집", "주차 가능한 카페"}

// replacementContent is a final content with one of every block the rules name.
func replacementContent() PostContent {
	return PostContent{
		Title: "성수동 카페 후기",
		Tags:  []string{"성수 카페", "디저트 맛집 추천"},
		Blocks: []Block{
			{Type: BlockText, Content: "성수동 카페에서 케이크를 먹었다."},                          // 0
			{Type: BlockImage, File: "IMG_1.jpg", Caption: "케이크"},                     // 1
			{Type: BlockHeading, Content: "성수동 카페 메뉴", Level: 2},                      // 2
			{Type: BlockQuote, Content: "케이크가 촉촉했다"},                                  // 3
			{Type: BlockList, Items: []string{"라떼 6,000원", "케이크 7,000원"}},             // 4
			{Type: BlockText, Content: "{{slot:1}}", Slot: &BlockSlot{Kind: "place"}}, // 5
			{Type: BlockVideo, File: "clip.mp4"},                                      // 6
		},
	}
}

func TestValidateReplacementsDropsEveryInvalidEntry(t *testing.T) {
	content := replacementContent()
	keep := func(surface ReplacementSurface, index int, source string) Replacement {
		return Replacement{Surface: surface, Index: index, Source: source, Phrases: []string{"분위기 좋은 카페"}}
	}
	for name, test := range map[string]struct {
		candidate Replacement
		kept      bool
	}{
		"a title containing its source":             {keep(ReplacementTitle, 7, "카페 후기"), true},
		"a title not containing it":                 {keep(ReplacementTitle, 0, "라떼 맛집"), false},
		"a tag containing it":                       {keep(ReplacementTag, 0, "성수 카페"), true},
		"a tag past the list":                       {keep(ReplacementTag, 2, "디저트"), false},
		"a negative tag index":                      {keep(ReplacementTag, -1, "디저트"), false},
		"a tag not containing it":                   {keep(ReplacementTag, 1, "성수 카페"), false},
		"a TEXT block containing it":                {keep(ReplacementBody, 0, "성수동 카페"), true},
		"a HEADING containing it":                   {keep(ReplacementBody, 2, "성수동 카페"), true},
		"a QUOTE containing it":                     {keep(ReplacementBody, 3, "촉촉했다"), true},
		"a LIST with an item containing it":         {keep(ReplacementBody, 4, "케이크 7,000원"), true},
		"a LIST with no item containing it":         {keep(ReplacementBody, 4, "마카롱"), false},
		"an IMAGE block":                            {keep(ReplacementBody, 1, "케이크"), false},
		"an unfilled slot":                          {keep(ReplacementBody, 5, "{{slot:1}}"), false},
		"a VIDEO block":                             {keep(ReplacementBody, 6, "clip"), false},
		"a block past the content":                  {keep(ReplacementBody, 7, "케이크"), false},
		"a body block not containing it":            {keep(ReplacementBody, 0, "마카롱"), false},
		"an unknown surface":                        {keep("summary", 0, "카페"), false},
		"a blank source":                            {keep(ReplacementBody, 0, "   "), false},
		"only an unlisted phrase":                   {Replacement{Surface: ReplacementBody, Index: 0, Source: "성수동 카페", Phrases: []string{"성수동 핫플"}}, false},
		"only the source itself as a phrase":        {Replacement{Surface: ReplacementTag, Index: 1, Source: "디저트 맛집", Phrases: []string{"디저트 맛집"}}, false},
		"no phrase at all":                          {Replacement{Surface: ReplacementBody, Index: 0, Source: "성수동 카페"}, false},
		"a source containing exact case and spaces": {keep(ReplacementTitle, 0, "성수동  카페"), false},
	} {
		got := ValidateReplacements([]Replacement{test.candidate}, content, testPhrases)
		if kept := len(got) == 1; kept != test.kept {
			t.Errorf("%s: kept = %v, want %v (%+v)", name, kept, test.kept, got)
		}
	}

	// The title is always index 0; a trimmed source is compared as trimmed.
	got := ValidateReplacements([]Replacement{keep(ReplacementTitle, 7, "  카페 후기 ")}, content, testPhrases)
	if len(got) != 1 || got[0].Index != 0 || got[0].Source != "카페 후기" {
		t.Fatalf("title candidate = %+v", got)
	}

	// Within one entry: an unlisted phrase, the source itself and a repeat go; the rest stay.
	mixed := Replacement{Surface: ReplacementTag, Index: 1, Source: "디저트", Phrases: []string{"디저트 맛집", "성수동 핫플", "디저트", " 디저트 맛집 ", "분위기 좋은 카페"}}
	got = ValidateReplacements([]Replacement{mixed}, content, testPhrases)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Phrases, []string{"디저트 맛집", "분위기 좋은 카페"}) {
		t.Fatalf("phrases = %+v", got)
	}

	// A second entry for the same surface, index and source is a duplicate.
	first := keep(ReplacementBody, 0, "성수동 카페")
	second := Replacement{Surface: ReplacementBody, Index: 0, Source: "성수동 카페", Phrases: []string{"주차 가능한 카페"}}
	if got := ValidateReplacements([]Replacement{first, second}, content, testPhrases); len(got) != 1 || !reflect.DeepEqual(got[0], first) {
		t.Fatalf("duplicate = %+v", got)
	}
}

func TestValidateReplacementsCapsSpansAndPhrasesAfterDrops(t *testing.T) {
	content := PostContent{Title: "카페"}
	for i := 0; i < ReplacementSpansMax+5; i++ {
		content.Blocks = append(content.Blocks, Block{Type: BlockText, Content: fmt.Sprintf("카페 %d번째 문단", i)})
	}
	var candidates []Replacement
	// Invalid entries first: they must not spend any of the cap.
	for i := 0; i < 5; i++ {
		candidates = append(candidates, Replacement{Surface: ReplacementBody, Index: i, Source: "마카롱", Phrases: []string{"분위기 좋은 카페"}})
	}
	for i := 0; i < ReplacementSpansMax+5; i++ {
		candidates = append(candidates, Replacement{
			Surface: ReplacementBody, Index: i, Source: fmt.Sprintf("카페 %d번째", i),
			Phrases: []string{"성수동 핫플", "분위기 좋은 카페", "디저트 맛집", "주차 가능한 카페", "솔직 후기"},
		})
	}
	got := ValidateReplacements(candidates, content, testPhrases)
	if len(got) != ReplacementSpansMax {
		t.Fatalf("kept %d spans, want %d", len(got), ReplacementSpansMax)
	}
	for i, candidate := range got {
		if candidate.Index != i {
			t.Fatalf("span %d has index %d: model order was not kept", i, candidate.Index)
		}
		// The unlisted first phrase is dropped before the cap, so three listed ones remain.
		if want := []string{"분위기 좋은 카페", "디저트 맛집", "주차 가능한 카페"}; !reflect.DeepEqual(candidate.Phrases, want) {
			t.Fatalf("span %d phrases = %q, want %q", i, candidate.Phrases, want)
		}
	}
}

// The candidates are judged against what is stored, after the attachment filter and the slot
// pass have moved blocks around: an index is never re-pointed to follow its block (GEN-53).
func TestReplacementIndexesResolveAgainstTheFinalContent(t *testing.T) {
	run := func(t *testing.T, post PostInput, answer string) []Replacement {
		t.Helper()
		models := newFakeModels()
		models.complete = func(llm.ModelRef, llm.Request) (llm.Response, error) { return llm.Response{Text: answer}, nil }
		svc := NewService(&fakePosts{}, fakeProfiles{}, &fakeRules{}, models, fakeImages{}, &fakeJobs{}, 4, testReasoningPolicy, testBudget, testDeps())
		written, _, err := svc.writeCandidate(context.Background(), post, Profile{}, nil, writeRef)
		if err != nil {
			t.Fatal(err)
		}
		return written.Replacements
	}

	t.Run("an invented photo is dropped and the later blocks move up", func(t *testing.T) {
		post := PostInput{UserID: "alice", TargetLanguage: LanguageKorean, FieldPhrases: testPhrases,
			Images: []Image{{Filename: "IMG_1.jpg", Key: "k", Kind: AttachmentPhoto}}}
		answer := `{"title":"성수 카페","summary":"요약","tags":[],"nouns":[],"blocks":[
			{"type":"TEXT","content":"성수동 카페에 갔다."},
			{"type":"IMAGE","file":"invented.jpg","alt":"없는 사진"},
			{"type":"TEXT","content":"케이크 솔직 후기를 남긴다."}],
			"replacements":[
			{"surface":"body","index":2,"source":"솔직 후기","phrases":["솔직 후기","디저트 맛집"]},
			{"surface":"body","index":1,"source":"솔직 후기","phrases":["디저트 맛집"]},
			{"surface":"body","index":1,"source":"성수동 카페","phrases":["분위기 좋은 카페"]}]}`
		got := run(t, post, answer)
		want := []Replacement{{Surface: ReplacementBody, Index: 1, Source: "솔직 후기", Phrases: []string{"디저트 맛집"}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("replacements = %+v, want %+v", got, want)
		}
	})

	t.Run("an omitted slot is inserted and the later blocks move down", func(t *testing.T) {
		brief := &TemplateBrief{
			Name:  "카페 리뷰",
			Body:  "<write>인트로</write>\n{{slot:1}}\n{{slot:2}}\n<write>마무리</write>",
			Slots: []TemplateSlot{{Kind: "place", Label: "지도"}, {Kind: "link", Label: "링크"}},
		}
		post := PostInput{UserID: "alice", TargetLanguage: LanguageKorean, FieldPhrases: testPhrases, Template: brief}
		// The model omitted {{slot:1}}: it is inserted before slot 2, so the closing paragraph the
		// model called 2 is now 3, and 2 is the second slot.
		answer := `{"title":"성수 카페","summary":"요약","tags":[],"nouns":[],"blocks":[
			{"type":"TEXT","content":"성수동 카페 솔직 후기"},
			{"type":"TEXT","content":"{{slot:2}}"},
			{"type":"TEXT","content":"다음에도 솔직 후기를 쓰겠다."}],
			"replacements":[
			{"surface":"body","index":2,"source":"솔직 후기","phrases":["디저트 맛집"]},
			{"surface":"body","index":3,"source":"솔직 후기","phrases":["분위기 좋은 카페"]},
			{"surface":"body","index":0,"source":"성수동 카페","phrases":["분위기 좋은 카페"]}]}`
		got := run(t, post, answer)
		want := []Replacement{
			{Surface: ReplacementBody, Index: 3, Source: "솔직 후기", Phrases: []string{"분위기 좋은 카페"}},
			{Surface: ReplacementBody, Index: 0, Source: "성수동 카페", Phrases: []string{"분위기 좋은 카페"}},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("replacements = %+v, want %+v", got, want)
		}
	})
}
