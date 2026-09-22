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
