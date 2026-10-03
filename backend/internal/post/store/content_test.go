package store

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/post"
)

// A photo group's files and layout survive the stored JSON, while every other block keeps the
// bytes it had before groups existed, so stored content decodes unchanged (GEN-77).
func TestContentJSONCarriesPhotoGroupsAndKeepsOtherBlocksUnchanged(t *testing.T) {
	content := post.PostContent{Title: "묶음", Blocks: []post.Block{
		{Type: post.BlockImage, File: "a.jpg", Caption: "한 장"},
		{Type: post.BlockGallery, Files: []string{"b.jpg", "c.jpg"}, Layout: post.GallerySlide, Alt: "두 장", Caption: "넘겨 보기"},
	}}
	data, err := marshalContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"type":"IMAGE","file":"a.jpg","caption":"한 장"}`; !strings.Contains(data, want) {
		t.Fatalf("an IMAGE block gained bytes: %s", data)
	}
	if want := `"files":["b.jpg","c.jpg"],"layout":"SLIDE"`; !strings.Contains(data, want) {
		t.Fatalf("stored group = %s", data)
	}
	got, err := unmarshalContent(data)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, content) {
		t.Fatalf("round trip = %+v, want %+v", *got, content)
	}

	legacy, err := unmarshalContent(`{"title":"옛 글","blocks":[{"type":"IMAGE","file":"a.jpg"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if block := legacy.Blocks[0]; block.Files != nil || block.Layout != "" || block.File != "a.jpg" {
		t.Fatalf("legacy block = %+v", block)
	}
}
