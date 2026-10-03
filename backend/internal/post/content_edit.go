package post

import (
	"fmt"
	"strings"
)

// ValidateContent is the pure canonical validator shared by manual and machine
// persistence. Manual input is rejected as a whole; model callers may still pre-filter
// malformed blocks before reaching this boundary.
// Photos and videos arrive as separate lists because a block names one KIND of
// attachment: a filename is unique across both, so an IMAGE block naming a video is the
// wrong block type rather than an unknown file, and the message has to be able to say so.
// canonicalTag is a tag's identity: runs of whitespace collapsed to one space, leading '#'s
// dropped, case kept. Whitespace is unicode.IsSpace (what Fields and TrimSpace use), and
// testdata/tag_identity/cases.json pins the rule.
func canonicalTag(tag string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.Join(strings.Fields(tag), " "), "#"))
}

func ValidateContent(content PostContent, attached []Image, videos []Video) error {
	if len(content.Blocks) == 0 {
		return &InvalidContentError{Reason: "at least one block is required"}
	}
	tags := make(map[string]struct{}, len(content.Tags))
	for _, tag := range content.Tags {
		canonical := canonicalTag(tag)
		if canonical == "" {
			return &InvalidContentError{Reason: "tag cannot be empty"}
		}
		if _, duplicate := tags[canonical]; duplicate {
			return &InvalidContentError{Reason: "tags must be unique"}
		}
		tags[canonical] = struct{}{}
	}
	files := make(map[string]struct{}, len(attached))
	for _, image := range attached {
		files[image.Filename] = struct{}{}
	}
	clips := make(map[string]struct{}, len(videos))
	for _, video := range videos {
		clips[video.Filename] = struct{}{}
	}
	for i, block := range content.Blocks {
		bad := func(reason string) error {
			return &InvalidContentError{Reason: fmt.Sprintf("block %d: %s", i+1, reason)}
		}
		// Files and Layout are a photo group's alone (GEN-77), so on any other block they are
		// fields for another block type exactly as a stray file is.
		if block.Type != BlockGallery && (len(block.Files) != 0 || block.Layout != "") {
			return bad("contains fields for another block type")
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
				if _, isVideo := clips[block.File]; isVideo {
					return bad("file is a video and belongs in a VIDEO block")
				}
				return bad("image is not attached to this post")
			}
			if block.Content != "" || block.Level != 0 || len(block.Items) != 0 {
				return bad("contains fields for another block type")
			}
		case BlockGallery:
			if err := validateGallery(block, files, clips); err != "" {
				return bad(err)
			}
		case BlockVideo:
			if strings.TrimSpace(block.File) == "" {
				return bad("video filename is required")
			}
			if _, ok := clips[block.File]; !ok {
				if _, isPhoto := files[block.File]; isPhoto {
					return bad("file is a photo and belongs in an IMAGE block")
				}
				return bad("video is not attached to this post")
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

// validateGallery is a photo group's shape (GEN-77, POST-106): two to PhotoGroupMax distinct
// attached photos, a layout, and none of the fields that belong to other block types. It
// returns the refusal reason, or "" when the group is valid. Unlike the model path, which
// repairs a group photo by photo (GEN-78), an edit is refused whole: the editor never sends a
// group it would not accept.
func validateGallery(block Block, photos, videos map[string]struct{}) string {
	if len(block.Files) < 2 || len(block.Files) > PhotoGroupMax {
		return fmt.Sprintf("a photo group holds 2 to %d photos", PhotoGroupMax)
	}
	seen := make(map[string]struct{}, len(block.Files))
	for _, file := range block.Files {
		if strings.TrimSpace(file) == "" {
			return "photo group filename is required"
		}
		if _, ok := photos[file]; !ok {
			if _, isVideo := videos[file]; isVideo {
				return "file is a video and belongs in a VIDEO block"
			}
			return "image is not attached to this post"
		}
		if _, repeated := seen[file]; repeated {
			return "photo appears twice in one group"
		}
		seen[file] = struct{}{}
	}
	if block.Layout != GalleryCollage && block.Layout != GallerySlide {
		return "photo group layout is required"
	}
	if block.File != "" || block.Content != "" || block.Level != 0 || len(block.Items) != 0 {
		return "contains fields for another block type"
	}
	return ""
}
