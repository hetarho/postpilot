package generation

import "github.com/postpilot/backend/internal/platform/config"

const (
	BadOutputErrorHeadChars     = 200
	RevisionInstructionMaxChars = 500
)

// resolveTagCount is the one place an absent tag count becomes the default (GEN-46): a
// generate or revise payload queued before the member existed, and a write-experiment
// snapshot frozen before it, all decode to 0 and must prompt for the same count a post
// never saved with one reads as. Three decode sites, one rule.
func resolveTagCount(n int) int {
	if n <= 0 {
		return config.PostTagCountDefault
	}
	return n
}
