package post

import (
	"encoding/json"
	"errors"
	"fmt"
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
		{Type: BlockGallery, Files: []string{"photo.jpg", "second.jpg"}, Layout: GalleryCollage, Alt: "묶음", Caption: "캡션"},
		{Type: BlockGallery, Files: []string{"second.jpg", "photo.jpg"}, Layout: GallerySlide},
	}}
	if err := ValidateContent(content, []Image{{Filename: "photo.jpg"}, {Filename: "second.jpg"}}, []Video{{Filename: "clip.mp4"}}); err != nil {
		t.Fatal(err)
	}
}

// A photo group holds two to PhotoGroupMax distinct attached photos and a layout, and nothing
// that belongs to another block type; Files and Layout belong to it alone (GEN-77, POST-106).
func TestValidateContentRefusesMalformedPhotoGroups(t *testing.T) {
	photos := []Image{{Filename: "a.jpg"}, {Filename: "b.jpg"}}
	for i := 0; i < PhotoGroupMax; i++ {
		photos = append(photos, Image{Filename: fmt.Sprintf("p%d.jpg", i)})
	}
	tooMany := make([]string, 0, PhotoGroupMax+1)
	for i := 0; i < PhotoGroupMax; i++ {
		tooMany = append(tooMany, fmt.Sprintf("p%d.jpg", i))
	}
	tooMany = append(tooMany, "a.jpg")
	group := func(files ...string) Block { return Block{Type: BlockGallery, Files: files, Layout: GalleryCollage} }
	for name, tc := range map[string]struct {
		block  Block
		reason string
	}{
		"one photo":         {group("a.jpg"), "a photo group holds 2 to 10 photos"},
		"no photo":          {group(), "a photo group holds 2 to 10 photos"},
		"too many":          {group(tooMany...), "a photo group holds 2 to 10 photos"},
		"repeated photo":    {group("a.jpg", "a.jpg"), "photo appears twice in one group"},
		"unattached photo":  {group("a.jpg", "foreign.jpg"), "image is not attached to this post"},
		"video in a group":  {group("a.jpg", "clip.mp4"), "file is a video and belongs in a VIDEO block"},
		"blank name":        {group("a.jpg", " "), "photo group filename is required"},
		"no layout":         {Block{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}}, "photo group layout is required"},
		"unknown layout":    {Block{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: "GRID"}, "photo group layout is required"},
		"stray file":        {Block{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: GallerySlide, File: "a.jpg"}, "contains fields for another block type"},
		"stray content":     {Block{Type: BlockGallery, Files: []string{"a.jpg", "b.jpg"}, Layout: GallerySlide, Content: "문단"}, "contains fields for another block type"},
		"files on an image": {Block{Type: BlockImage, File: "a.jpg", Files: []string{"b.jpg"}}, "contains fields for another block type"},
		"layout on text":    {Block{Type: BlockText, Content: "문단", Layout: GalleryCollage}, "contains fields for another block type"},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *InvalidContentError
			err := ValidateContent(PostContent{Blocks: []Block{tc.block}}, photos, []Video{{Filename: "clip.mp4"}})
			if !errors.As(err, &invalid) || invalid.Reason != "block 1: "+tc.reason {
				t.Fatalf("error=%v, want %q", err, tc.reason)
			}
		})
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
