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
		return firstPopulated(block.Level != 0, block.File, block.Alt, block.Caption, hasItems)
	case BlockHeading:
		if !hasContent {
			return "content"
		}
		return firstPopulated(false, block.File, block.Alt, block.Caption, hasItems)
	case BlockImage:
		if strings.TrimSpace(block.File) == "" {
			return "file"
		}
		if hasContent {
			return "content"
		}
		if block.Level != 0 {
			return "level"
		}
		if hasItems {
			return "items"
		}
		return ""
	case BlockQuote:
		if !hasContent {
			return "content"
		}
		return firstPopulated(block.Level != 0, block.File, block.Alt, block.Caption, hasItems)
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
		return firstPopulated(block.Level != 0, block.File, block.Alt, block.Caption, false)
	default:
		return "type"
	}
}

func firstPopulated(level bool, file, alt, caption string, items bool) string {
	if level {
		return "level"
	}
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
