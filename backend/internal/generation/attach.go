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
//
// portrait names every attached photo whose orientation is portrait (GEN-77); a photo it does
// not name is landscape, so a nil map treats every photo as one orientation.
func FilterAttachments(content PostContent, photos, videos []string, portrait map[string]bool) PostContent {
	attachedPhotos := nameSet(photos)
	attachedVideos := nameSet(videos)
	blocks := make([]Block, 0, len(content.Blocks))
	for _, block := range content.Blocks {
		if block.Type == BlockGallery {
			blocks = append(blocks, filterGallery(block, attachedPhotos, portrait)...)
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
// as far as its photos do. Unattached names are dropped and a repeat keeps its first place; the
// rest split by orientation — the first photo's orientation first, order kept — and each
// orientation's photos into the fewest even parts of at most post.PhotoGroupMax, so no placed
// photo is lost and no group mixes portrait with landscape. A part of one photo stands alone as
// an IMAGE. Every part carries a caption: the first the written one, each later part the alt (or
// the caption again without one); a written group with an empty caption takes its alt. An absent
// or unknown layout reads as a collage.
func filterGallery(block Block, attached map[string]struct{}, portrait map[string]bool) []Block {
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
	if len(files) == 0 {
		slog.Warn("dropping generated photo group with no attached photo")
		return nil
	}
	layout := strings.ToUpper(strings.TrimSpace(block.Layout))
	if layout != GallerySlide {
		layout = GalleryCollage
	}
	caption := strings.TrimSpace(block.Caption)
	if caption == "" {
		caption = block.Alt
	}
	later := block.Alt
	if strings.TrimSpace(later) == "" {
		later = caption
	}
	first, other := splitByOrientation(files, portrait)
	var out []Block
	for _, run := range [][]string{first, other} {
		for _, part := range evenParts(run, post.PhotoGroupMax) {
			text := later
			if len(out) == 0 {
				text = caption
			}
			if len(part) == 1 {
				out = append(out, Block{Type: BlockImage, File: part[0], Alt: block.Alt, Caption: text})
				continue
			}
			out = append(out, Block{Type: BlockGallery, Files: part, Layout: layout, Alt: block.Alt, Caption: text})
		}
	}
	return out
}

// splitByOrientation keeps the photos of the first photo's orientation, in order, apart from the
// others, also in order.
func splitByOrientation(files []string, portrait map[string]bool) (first, other []string) {
	lead := portrait[files[0]]
	for _, file := range files {
		if portrait[file] == lead {
			first = append(first, file)
		} else {
			other = append(other, file)
		}
	}
	return first, other
}

// evenParts cuts files into the fewest parts of at most max, sizes differing by at most one and
// the larger first: four become two and two, seven three, two and two.
func evenParts(files []string, max int) [][]string {
	if len(files) == 0 {
		return nil
	}
	count := (len(files) + max - 1) / max
	parts := make([][]string, 0, count)
	start := 0
	for i := 0; i < count; i++ {
		size := len(files) / count
		if i < len(files)%count {
			size++
		}
		parts = append(parts, files[start:start+size])
		start += size
	}
	return parts
}

// PhotoPortraits names every attached photo that stands portrait — taller than wide on record —
// for the orientation rule (GEN-77). A square photo, or one with no dimensions, is landscape.
// It is the one place a photo's orientation is decided.
func PhotoPortraits(images []Image) map[string]bool {
	portrait := make(map[string]bool, len(images))
	for _, image := range images {
		if image.Kind == AttachmentVideo {
			continue
		}
		if image.Height > image.Width {
			portrait[image.Filename] = true
		}
	}
	return portrait
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
