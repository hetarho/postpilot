package generation

import "log/slog"

// FilterAttachments removes invented IMAGE references using exact, case-sensitive names.
func FilterAttachments(content PostContent, filenames []string) PostContent {
	attached := make(map[string]struct{}, len(filenames))
	for _, filename := range filenames {
		attached[filename] = struct{}{}
	}
	blocks := make([]Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Type == BlockImage {
			if _, ok := attached[block.File]; !ok {
				slog.Warn("dropping unattached generated image", "file", block.File)
				continue
			}
		}
		blocks = append(blocks, block)
	}
	content.Blocks = blocks
	return content
}
