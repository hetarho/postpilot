package generation

import "log/slog"

// FilterAttachments removes invented IMAGE and VIDEO references using exact, case-sensitive
// names.
//
// The two lists are separate because a block names one KIND of attachment: a filename is
// unique across both, so an IMAGE block naming a video is a block the writer got wrong, and
// keeping it would put an <img> where the post has a clip.
func FilterAttachments(content PostContent, photos, videos []string) PostContent {
	attachedPhotos := nameSet(photos)
	attachedVideos := nameSet(videos)
	blocks := make([]Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		switch block.Type {
		case BlockImage:
			if _, ok := attachedPhotos[block.File]; !ok {
				slog.Warn("dropping unattached generated image", "file", block.File)
				continue
			}
		case BlockVideo:
			if _, ok := attachedVideos[block.File]; !ok {
				slog.Warn("dropping unattached generated video", "file", block.File)
				continue
			}
		}
		blocks = append(blocks, block)
	}
	content.Blocks = blocks
	return content
}

func nameSet(filenames []string) map[string]struct{} {
	out := make(map[string]struct{}, len(filenames))
	for _, filename := range filenames {
		out[filename] = struct{}{}
	}
	return out
}

// AttachmentNames splits an attachment list into the two name sets the filter takes.
func AttachmentNames(images []Image) (photos, videos []string) {
	photos = make([]string, 0, len(images))
	videos = make([]string, 0, len(images))
	for _, image := range images {
		if image.Kind == AttachmentVideo {
			videos = append(videos, image.Filename)
			continue
		}
		photos = append(photos, image.Filename)
	}
	return photos, videos
}
