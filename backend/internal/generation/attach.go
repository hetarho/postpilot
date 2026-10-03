package generation

import (
	"log/slog"
	"strings"

	"github.com/postpilot/backend/internal/post"
)

// FilterAttachments removes invented IMAGE and VIDEO references using exact, case-sensitive
// names, and repairs a photo group photo by photo (GEN-78).
//
// The two lists are separate because a block names one KIND of attachment: a filename is
// unique across both, so an IMAGE block naming a video is a block the writer got wrong, and
// keeping it would put an <img> where the post has a clip.
func FilterAttachments(content PostContent, photos, videos []string) PostContent {
	attachedPhotos := nameSet(photos)
	attachedVideos := nameSet(videos)
	blocks := make([]Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Type == BlockGallery {
			blocks = append(blocks, filterGallery(block, attachedPhotos)...)
			continue
		}
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

// filterGallery is GEN-78 for one model-written photo group: the place the writer chose survives
// as far as its photos do. Unattached names are dropped and a repeat keeps its first place; one
// photo left stands alone as an IMAGE with the group's alt and caption, none drops the block, and
// a group past post.PhotoGroupMax continues as the next group of the same layout with no caption
// rather than losing a placed photo. An absent or unknown layout reads as a collage.
func filterGallery(block Block, attached map[string]struct{}) []Block {
	files := make([]string, 0, len(block.Files))
	seen := make(map[string]struct{}, len(block.Files))
	for _, file := range block.Files {
		file = strings.TrimSpace(file)
		if _, ok := attached[file]; !ok {
			slog.Warn("dropping unattached generated group photo", "file", file)
			continue
		}
		if _, repeated := seen[file]; repeated {
			continue
		}
		seen[file] = struct{}{}
		files = append(files, file)
	}
	layout := strings.ToUpper(strings.TrimSpace(block.Layout))
	if layout != GallerySlide {
		layout = GalleryCollage
	}
	var out []Block
	for start := 0; start < len(files); start += post.PhotoGroupMax {
		chunk := files[start:min(start+post.PhotoGroupMax, len(files))]
		caption := block.Caption
		if start > 0 {
			caption = ""
		}
		if len(chunk) == 1 {
			out = append(out, Block{Type: BlockImage, File: chunk[0], Alt: block.Alt, Caption: caption})
			continue
		}
		out = append(out, Block{Type: BlockGallery, Files: chunk, Layout: layout, Alt: block.Alt, Caption: caption})
	}
	if len(out) == 0 {
		slog.Warn("dropping generated photo group with no attached photo")
	}
	return out
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
