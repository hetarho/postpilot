package generation

import "github.com/postpilot/backend/internal/post"

const (
	BadOutputErrorHeadChars     = 200
	RevisionInstructionMaxChars = 500
	// WriteNounsMax bounds the write answer's nouns (GEN-55). The parser enforces it; the
	// schema's maxItems and the prompt's number only ask the provider for the same.
	WriteNounsMax = 40
	// ReplacementSpansMax and ReplacementPhrasesMax bound the write answer's replacement
	// candidates (GEN-54): an offer the author cannot read through is not an offer.
	ReplacementSpansMax   = 20
	ReplacementPhrasesMax = 3
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
