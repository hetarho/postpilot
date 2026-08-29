package post

import (
	"fmt"
	"strings"
)

// ValidateContent is the pure canonical validator shared by manual and machine
// persistence. Manual input is rejected as a whole; model callers may still pre-filter
// malformed blocks before reaching this boundary.
func ValidateContent(content PostContent, attached []Image) error {
	if len(content.Blocks) == 0 {
		return &InvalidContentError{Reason: "at least one block is required"}
	}
	files := make(map[string]struct{}, len(attached))
	for _, image := range attached {
		files[image.Filename] = struct{}{}
	}
	for i, block := range content.Blocks {
		bad := func(reason string) error {
			return &InvalidContentError{Reason: fmt.Sprintf("block %d: %s", i+1, reason)}
		}
		switch block.Type {
		case BlockText, BlockQuote:
			if strings.TrimSpace(block.Content) == "" {
				return bad("content is required")
			}
			if block.File != "" || block.Level != 0 || len(block.Items) != 0 {
				return bad("contains fields for another block type")
			}
		case BlockHeading:
			if strings.TrimSpace(block.Content) == "" {
				return bad("heading content is required")
			}
			if block.Level < 1 || block.Level > 6 {
				return bad("heading level must be 1 through 6")
			}
			if block.File != "" || len(block.Items) != 0 {
				return bad("contains fields for another block type")
			}
		case BlockImage:
			if strings.TrimSpace(block.File) == "" {
				return bad("image filename is required")
			}
			if _, ok := files[block.File]; !ok {
				return bad("image is not attached to this post")
			}
			if block.Content != "" || block.Level != 0 || len(block.Items) != 0 {
				return bad("contains fields for another block type")
			}
		case BlockList:
			if len(block.Items) == 0 {
				return bad("list items are required")
			}
			for _, item := range block.Items {
				if strings.TrimSpace(item) == "" {
					return bad("list item cannot be empty")
				}
			}
			if block.Content != "" || block.File != "" || block.Level != 0 {
				return bad("contains fields for another block type")
			}
		default:
			return bad("unknown block type")
		}
	}
	return nil
}
