package generation

import "github.com/postpilot/backend/internal/post"

const (
	BadOutputErrorHeadChars     = 200
	RevisionInstructionMaxChars = 500
	// WriteNounsMax bounds the write answer's nouns (GEN-55). The parser enforces it; the
	// schema's maxItems and the prompt's number only ask the provider for the same.
	WriteNounsMax = 40
	// StorylineParagraphMax and StorylineTextMaxChars bound the write answer's storyline
	// (GEN-67): at most this many paragraphs are kept, and a paragraph's text is cut at this
	// many runes. The parser enforces both; nothing about the storyline fails a paid write.
	StorylineParagraphMax = 30
	StorylineTextMaxChars = 1000
)

// resolveTagCount is the one place an absent tag count becomes the default (GEN-46): a
// generate or revise payload queued before the member existed, and a write-experiment
// snapshot frozen before it, all decode to 0 and must prompt for the same count a post
// never saved with one reads as. Three decode sites, one rule.
func resolveTagCount(n int) int {
	if n <= 0 {
		return post.TagCountRange.Default
	}
	return n
}
