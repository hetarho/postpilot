package generation

import (
	"log/slog"
	"strings"
)

// ValidateBlocks is the only field-combination validator for model-produced blocks.
// It drops one invalid block without disturbing its valid neighbours.
func ValidateBlocks(blocks []Block) []Block {
	valid := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		field := invalidField(block)
		if field != "" {
			slog.Warn("dropping invalid generated block", "type", block.Type, "field", field)
			continue
		}
		if block.Type == BlockHeading && block.Level != 2 && block.Level != 3 {
			block.Level = 2
		} else if block.Type != BlockHeading {
			// The structured-output schema requires level on every block. It has no
			// meaning outside headings, so a schema-obedient value is normalized away.
			block.Level = 0
		}
		if block.Type == BlockGallery {
			// The files list is the group; a model that also filled `file` mixed the two
			// shapes, and dropping the whole group over it would lose every photo in it.
			block.File = ""
		} else {
			// Files and layout are schema-required on every block too (GEN-77), so like level
			// they are normalized away wherever they mean nothing.
			block.Files, block.Layout = nil, ""
		}
		valid = append(valid, block)
	}
	return valid
}

func invalidField(block Block) string {
	hasContent := strings.TrimSpace(block.Content) != ""
	hasItems := len(block.Items) > 0
	switch block.Type {
	case BlockText:
		if !hasContent {
			return "content"
		}
		return firstPopulated(block.File, block.Alt, block.Caption, hasItems)
	case BlockHeading:
		if !hasContent {
			return "content"
		}
		return firstPopulated(block.File, block.Alt, block.Caption, hasItems)
	case BlockImage, BlockVideo:
		// A video block carries the IMAGE fields and none of its own (VIDEO-2), so the field
		// rules are the same ones — only which attachment list its file must name differs,
		// and that is the attachment filter's question, not this one's.
		if strings.TrimSpace(block.File) == "" {
			return "file"
		}
		if hasContent {
			return "content"
		}
		if hasItems {
			return "items"
		}
		return ""
	case BlockGallery:
		// Whether each photo is attached, repeated or one too many is the attachment filter's
		// question (GEN-78); here a group needs at least one name to be a group at all.
		named := false
		for _, file := range block.Files {
			if strings.TrimSpace(file) != "" {
				named = true
				break
			}
		}
		if !named {
			return "files"
		}
		if hasContent {
			return "content"
		}
		if hasItems {
			return "items"
		}
		return ""
	case BlockQuote:
		if !hasContent {
			return "content"
		}
		return firstPopulated(block.File, block.Alt, block.Caption, hasItems)
	case BlockList:
		if !hasItems {
			return "items"
		}
		for _, item := range block.Items {
			if strings.TrimSpace(item) == "" {
				return "items"
			}
		}
		if hasContent {
			return "content"
		}
		return firstPopulated(block.File, block.Alt, block.Caption, false)
	default:
		return "type"
	}
}

func firstPopulated(file, alt, caption string, items bool) string {
	if file != "" {
		return "file"
	}
	if alt != "" {
		return "alt"
	}
	if caption != "" {
		return "caption"
	}
	if items {
		return "items"
	}
	return ""
}
