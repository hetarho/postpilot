package post

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateContentAcceptsEveryCanonicalBlockType(t *testing.T) {
	content := PostContent{Title: "제목", Blocks: []Block{
		{Type: BlockText, Content: "문단"},
		{Type: BlockHeading, Content: "소제목", Level: 2},
		{Type: BlockQuote, Content: "인용"},
		{Type: BlockList, Items: []string{"하나", "둘"}},
		{Type: BlockImage, File: "photo.jpg", Alt: "대체 텍스트", Caption: "캡션"},
		{Type: BlockVideo, File: "clip.mp4", Alt: "대체 텍스트", Caption: "캡션"},
	}}
	if err := ValidateContent(content, []Image{{Filename: "photo.jpg"}}, []Video{{Filename: "clip.mp4"}}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateContentRejectsCrossTypeAndUnattachedImageFields(t *testing.T) {
	for name, block := range map[string]Block{
		"empty text":       {Type: BlockText},
		"invalid heading":  {Type: BlockHeading, Content: "제목", Level: 7},
		"empty list item":  {Type: BlockList, Items: []string{""}},
		"unattached image": {Type: BlockImage, File: "foreign.jpg"},
		"mixed fields":     {Type: BlockText, Content: "문단", File: "photo.jpg"},
		// A filename is unique across the two kinds, so naming the other kind's file is a
		// wrong block type rather than an unknown file — and either way it is refused.
		"unattached video": {Type: BlockVideo, File: "foreign.mp4"},
		"video in image":   {Type: BlockImage, File: "clip.mp4"},
		"photo in video":   {Type: BlockVideo, File: "photo.jpg"},
		"empty video file": {Type: BlockVideo, File: "  "},
		"video with text":  {Type: BlockVideo, File: "clip.mp4", Content: "문단"},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *InvalidContentError
			if err := ValidateContent(PostContent{Blocks: []Block{block}}, []Image{{Filename: "photo.jpg"}}, []Video{{Filename: "clip.mp4"}}); !errors.As(err, &invalid) {
				t.Fatalf("error=%v, want InvalidContentError", err)
			}
		})
	}
}

func TestValidateContentRejectsCanonicallyDuplicateTags(t *testing.T) {
	content := PostContent{
		Tags:   []string{"여행", " #여행 "},
		Blocks: []Block{{Type: BlockText, Content: "문단"}},
	}
	var invalid *InvalidContentError
	if err := ValidateContent(content, nil, nil); !errors.As(err, &invalid) {
		t.Fatalf("error=%v, want InvalidContentError", err)
	}
}

// The tag identity the browser mirrors, pinned by one fixture both suites run: a whitespace or
// hash rule changed on one side fails the other.
func TestCanonicalTagFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("testdata", "tag_identity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Name      string `json:"name"`
			Tag       string `json:"tag"`
			Canonical string `json:"canonical"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("the fixture has no cases")
	}
	for _, c := range fixture.Cases {
		if got := canonicalTag(c.Tag); got != c.Canonical {
			t.Errorf("%s: canonicalTag(%q) = %q, want %q", c.Name, c.Tag, got, c.Canonical)
		}
	}
}
