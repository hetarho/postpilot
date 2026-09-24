package post

// TagCountRange is the per-post tag count (POST-63): what a post never saved
// with one reads as, and the range SavePostGenerationOptions accepts. The
// generation context reads the same default — for a queued payload or a
// write-experiment snapshot frozen before the member existed. The frontend
// mirrors the three as POST_TAG_COUNT_DEFAULT / _MIN / _MAX.
var TagCountRange = TagCount{Default: 4, Min: 1, Max: 10}

// TagCount is a tag-count range with the value an unset post reads as.
type TagCount struct {
	Default, Min, Max int
}

// Allows reports whether an explicitly requested tag count is in range.
func (t TagCount) Allows(count int) bool { return count >= t.Min && count <= t.Max }

// PublishedURLMaxChars bounds a pasted Naver Blog address, in Unicode scalar values like every
// other *MaxChars bound. The frontend mirrors it for its pre-check, and the shared fixture's
// maxChars (testdata/published_url/cases.json) pins the two equal.
const PublishedURLMaxChars = 2048
