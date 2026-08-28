package generation_test

import (
	"strings"
	"testing"

	postpilotv1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestBlockTypeProtoJSONNames(t *testing.T) {
	values := []postpilotv1.BlockType{
		postpilotv1.BlockType_TEXT, postpilotv1.BlockType_HEADING, postpilotv1.BlockType_IMAGE,
		postpilotv1.BlockType_QUOTE, postpilotv1.BlockType_LIST,
	}
	for _, value := range values {
		data, err := protojson.Marshal(&postpilotv1.Block{Type: value})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"type":"`+value.String()+`"`) {
			t.Errorf("%s serialized as %s", value, data)
		}
	}
}
