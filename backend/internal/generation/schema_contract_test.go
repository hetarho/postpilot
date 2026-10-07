package generation

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestCanonicalPostSchemasShareAllBlockFieldsAndPermitSparseTagArrays(t *testing.T) {
	schemas := [][]byte{PostContentSchema(), WriteAnswerSchema(), WriteAlongStorylineAnswerSchema()}
	var canonical map[string]any
	for _, raw := range schemas {
		var schema map[string]any
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		properties := schema["properties"].(map[string]any)
		tags := properties["tags"].(map[string]any)
		if tags["type"] != "array" || tags["minItems"] != nil || tags["maxItems"] != nil || tags["items"].(map[string]any)["type"] != "string" {
			t.Fatalf("canonical tags require fixed/padded count=%+v", tags)
		}
		block := properties["blocks"].(map[string]any)["items"].(map[string]any)
		if canonical == nil {
			canonical = block
		} else if !reflect.DeepEqual(block, canonical) {
			t.Fatalf("post block vocabulary diverged=%+v", block)
		}
		fields := block["properties"].(map[string]any)
		kinds := fields["type"].(map[string]any)["enum"].([]any)
		var names []string
		for _, kind := range kinds {
			names = append(names, kind.(string))
		}
		slices.Sort(names)
		want := []string{"GALLERY", "HEADING", "IMAGE", "LIST", "QUOTE", "TEXT", "VIDEO"}
		if !slices.Equal(names, want) || len(fields) != 9 {
			t.Fatalf("canonical kinds/fields=%v/%+v", names, fields)
		}
		for name, wantType := range map[string]string{"content": "string", "level": "integer", "file": "string", "files": "array", "layout": "string", "alt": "string", "caption": "string", "items": "array"} {
			if fields[name].(map[string]any)["type"] != wantType {
				t.Fatalf("%s field disagrees with canonical block vocabulary", name)
			}
		}
		required := block["required"].([]any)
		if len(required) != len(fields) {
			t.Fatal("canonical per-block fields no longer required")
		}
		for _, name := range required {
			if fields[name.(string)] == nil {
				t.Fatalf("unrecognized required block field=%s", name)
			}
		}
	}
	blocks := []Block{{Type: BlockText, Content: "Full text"}, {Type: BlockHeading, Content: "Heading", Level: 2}, {Type: BlockHeading, Content: "Subheading", Level: 3}, {Type: BlockQuote, Content: "Exact quotation"}, {Type: BlockList, Items: []string{"first", "second"}}, {Type: BlockImage, File: "photo.jpg", Alt: "Photo", Caption: "Photo caption"}, {Type: BlockVideo, File: "clip.mp4", Alt: "Video", Caption: "Video caption"}, {Type: BlockGallery, Files: []string{"photo.jpg", "other.jpg"}, Layout: "COLLAGE", Alt: "Group", Caption: "Group caption"}}
	valid := ValidateBlocks(blocks)
	if len(valid) != len(blocks) {
		t.Fatalf("canonical field combinations dropped=%+v", valid)
	}
	for _, level := range []int32{0, 1, 4, 5, 6, 99} {
		heading := ValidateBlocks([]Block{{Type: BlockHeading, Content: "Heading", Level: level}})
		if len(heading) != 1 || heading[0].Level != 2 {
			t.Fatalf("existing heading clamp changed=%+v", heading)
		}
	}
}
