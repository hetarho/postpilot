package rpc

import (
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/post"
)

func TestBlockTypeMappingCoversGeneratedEnum(t *testing.T) {
	expected := map[postpilotv1.BlockType]post.BlockType{
		postpilotv1.BlockType_TEXT:    post.BlockText,
		postpilotv1.BlockType_HEADING: post.BlockHeading,
		postpilotv1.BlockType_IMAGE:   post.BlockImage,
		postpilotv1.BlockType_QUOTE:   post.BlockQuote,
		postpilotv1.BlockType_LIST:    post.BlockList,
		postpilotv1.BlockType_VIDEO:   post.BlockVideo,
		postpilotv1.BlockType_GALLERY: post.BlockGallery,
	}
	if got, want := len(postpilotv1.BlockType_name), len(expected)+1; got != want {
		t.Fatalf("generated block types = %d, want %d; update the closed mapping", got, want)
	}

	for number, name := range postpilotv1.BlockType_name {
		wire := postpilotv1.BlockType(number)
		t.Run(name, func(t *testing.T) {
			if wire == postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED {
				if got := fromProtoBlockType(wire); got != "" {
					t.Fatalf("unspecified mapped to valid domain value %q", got)
				}
				return
			}
			want, ok := expected[wire]
			if !ok {
				t.Fatalf("generated enum %s has no domain mapping", name)
			}
			if got := fromProtoBlockType(wire); got != want {
				t.Fatalf("from proto = %q, want %q", got, want)
			}
			if got := toProtoBlockType(want); got != wire {
				t.Fatalf("to proto = %s, want %s", got, wire)
			}
		})
	}
}

func TestBlockTypeMappingRejectsUnknownValues(t *testing.T) {
	if got := fromProtoBlockType(postpilotv1.BlockType(999)); got != "" {
		t.Fatalf("unknown wire value mapped to valid domain value %q", got)
	}
	for _, value := range []post.BlockType{"", "TABLE"} {
		if got := toProtoBlockType(value); got != postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED {
			t.Errorf("domain value %q mapped to valid wire value %s", value, got)
		}
	}
}

// A photo group's layout maps both ways, UNSPECIFIED reading as no layout, and its files ride
// the content unchanged in both directions (GEN-77).
func TestGalleryLayoutAndFilesRoundTrip(t *testing.T) {
	for wire, domain := range map[postpilotv1.GalleryLayout]post.GalleryLayout{
		postpilotv1.GalleryLayout_GALLERY_LAYOUT_UNSPECIFIED: "",
		postpilotv1.GalleryLayout_GALLERY_LAYOUT_COLLAGE:     post.GalleryCollage,
		postpilotv1.GalleryLayout_GALLERY_LAYOUT_SLIDE:       post.GallerySlide,
	} {
		if got := fromProtoGalleryLayout(wire); got != domain {
			t.Errorf("from proto %s = %q, want %q", wire, got, domain)
		}
		if got := toProtoGalleryLayout(domain); got != wire {
			t.Errorf("to proto %q = %s, want %s", domain, got, wire)
		}
	}
	if got := len(postpilotv1.GalleryLayout_name); got != 3 {
		t.Fatalf("generated layouts = %d, want 3; update the closed mapping", got)
	}
	content, err := fromProtoContent(&postpilotv1.PostContent{Blocks: []*postpilotv1.Block{{
		Type: postpilotv1.BlockType_GALLERY, Files: []string{"a.jpg", "b.jpg"},
		Layout: postpilotv1.GalleryLayout_GALLERY_LAYOUT_SLIDE, Alt: "묶음", Caption: "캡션",
	}}})
	if err != nil {
		t.Fatal(err)
	}
	block := content.Blocks[0]
	if block.Type != post.BlockGallery || block.Layout != post.GallerySlide || len(block.Files) != 2 || block.Files[1] != "b.jpg" || block.Caption != "캡션" {
		t.Fatalf("from proto = %+v", block)
	}
	back := toProtoContent(&content).GetBlocks()[0]
	if back.GetType() != postpilotv1.BlockType_GALLERY || back.GetLayout() != postpilotv1.GalleryLayout_GALLERY_LAYOUT_SLIDE || len(back.GetFiles()) != 2 || back.GetAlt() != "묶음" {
		t.Fatalf("to proto = %+v", back)
	}
}
