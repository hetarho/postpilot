package generation

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func captureBlockValidationLog(t *testing.T, validate func()) string {
	t.Helper()
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
	defer slog.SetDefault(previous)
	validate()
	return output.String()
}

func TestValidateBlocksNormalizesSchemaMandatedLevels(t *testing.T) {
	input := make([]Block, 0, 16)
	for i := 1; i <= 4; i++ {
		level := int32(i)
		input = append(input,
			Block{Type: BlockText, Content: fmt.Sprintf("text %d", i), Level: level},
			Block{Type: BlockImage, File: fmt.Sprintf("IMG_%d.jpg", i), Alt: "alt", Caption: "caption", Level: level},
			Block{Type: BlockQuote, Content: fmt.Sprintf("quote %d", i), Level: level},
			Block{Type: BlockList, Items: []string{fmt.Sprintf("item %d", i)}, Level: level},
		)
	}

	var got []Block
	logs := captureBlockValidationLog(t, func() { got = ValidateBlocks(input) })
	if len(got) != 16 {
		t.Fatalf("kept blocks = %d, want 16: %+v", len(got), got)
	}
	for i, block := range got {
		if block.Level != 0 {
			t.Errorf("block %d level = %d, want 0", i, block.Level)
		}
	}
	if logs != "" {
		t.Fatalf("normalization logged a warning: %s", logs)
	}
}

func TestValidateBlocksKeepsHeadingRules(t *testing.T) {
	input := []Block{
		{Type: BlockHeading, Content: "two", Level: 2},
		{Type: BlockHeading, Content: "three", Level: 3},
		{Type: BlockHeading, Content: "one", Level: 1},
		{Type: BlockHeading, Content: "four", Level: 4},
		{Type: BlockHeading, Content: "zero"},
		{Type: BlockHeading, Level: 2},
	}

	var got []Block
	logs := captureBlockValidationLog(t, func() { got = ValidateBlocks(input) })
	if len(got) != 5 {
		t.Fatalf("kept headings = %d, want 5: %+v", len(got), got)
	}
	wantLevels := []int32{2, 3, 2, 2, 2}
	for i, want := range wantLevels {
		if got[i].Level != want {
			t.Errorf("heading %d level = %d, want %d", i, got[i].Level, want)
		}
	}
	if strings.Count(logs, "dropping invalid generated block") != 1 || !strings.Contains(logs, "field=content") {
		t.Fatalf("empty heading log = %q", logs)
	}
}

func TestValidateBlocksStillDropsRealFieldConfusion(t *testing.T) {
	tests := map[string]struct {
		block Block
		field string
	}{
		"text items":         {block: Block{Type: BlockText, Content: "text", Items: []string{"wrong"}, Level: 2}, field: "items"},
		"image content":      {block: Block{Type: BlockImage, File: "IMG.jpg", Content: "wrong", Level: 2}, field: "content"},
		"image missing file": {block: Block{Type: BlockImage, Level: 2}, field: "file"},
		"list empty item":    {block: Block{Type: BlockList, Items: []string{""}, Level: 2}, field: "items"},
		"unknown type":       {block: Block{Type: "VIDEO", Content: "wrong", Level: 2}, field: "type"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			var got []Block
			logs := captureBlockValidationLog(t, func() { got = ValidateBlocks([]Block{test.block}) })
			if len(got) != 0 {
				t.Fatalf("invalid block was kept: %+v", got)
			}
			if strings.Count(logs, "dropping invalid generated block") != 1 || !strings.Contains(logs, "field="+test.field) {
				t.Fatalf("drop log = %q, want one field=%s warning", logs, test.field)
			}
		})
	}
}
