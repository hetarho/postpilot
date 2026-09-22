package generation_test

import (
	"strings"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/generation"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerationBlockTypesCoverGeneratedEnumAndProtoJSONNames(t *testing.T) {
	domainByName := map[string]generation.BlockType{
		"TEXT":    generation.BlockText,
		"HEADING": generation.BlockHeading,
		"IMAGE":   generation.BlockImage,
		"QUOTE":   generation.BlockQuote,
		"LIST":    generation.BlockList,
		"VIDEO":   generation.BlockVideo,
	}
	if got, want := len(postpilotv1.BlockType_name), len(domainByName)+1; got != want {
		t.Fatalf("generated block types = %d, want %d; update generation's closed mirror", got, want)
	}
	for number, name := range postpilotv1.BlockType_name {
		value := postpilotv1.BlockType(number)
		if value == postpilotv1.BlockType_BLOCK_TYPE_UNSPECIFIED {
			if _, ok := domainByName[name]; ok {
				t.Fatalf("unspecified %q became a valid generation block", name)
			}
			continue
		}
		domain, ok := domainByName[name]
		if !ok || string(domain) != name {
			t.Fatalf("generated block %s has no matching generation value", name)
		}
		data, err := protojson.Marshal(&postpilotv1.Block{Type: value})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"type":"`+value.String()+`"`) {
			t.Errorf("%s serialized as %s", value, data)
		}
	}
	if _, ok := domainByName[postpilotv1.BlockType(999).String()]; ok {
		t.Fatal("unknown generated block became a valid generation value")
	}
}
