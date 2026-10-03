package post

import "fmt"

// TagCountRange is the per-post tag count (POST-63): what a post never saved
// with one reads as, and the range SavePostGenerationOptions accepts. The
// generation context reads the same default — for a queued payload or a
// write-experiment snapshot frozen before the member existed. The frontend
// mirrors the three as POST_TAG_COUNT_DEFAULT / _MIN / _MAX.
var TagCountRange = TagCount{Default: 4, Min: 1, Max: 10}

// TargetLengthMin and TargetLengthMax bound the 목표 글자 수 an option save accepts: one range on
// both sides and for a template (POST-20, TMPL-6). The frontend mirrors them as
// POST_TARGET_LENGTH_MIN / _MAX. Template reads the floor too, for the length a template may
// seed (TMPL-47).
const (
	TargetLengthMin = 100
	TargetLengthMax = 10_000
)

// PhotoGroupMax is how many photos one photo group may hold (GEN-77); a group holds at least two.
// The generation context splits a longer model-written group at this bound, and the frontend
// mirrors it as PHOTO_GROUP_MAX.
const PhotoGroupMax = 10

// TargetLengthError is a 목표 글자 수 outside TargetLengthMin … TargetLengthMax.
type TargetLengthError struct{ Min, Max int }

func (e *TargetLengthError) Error() string {
	return fmt.Sprintf("target length must be between %d and %d", e.Min, e.Max)
}

// PhotoMissingError refuses a finalize while IMAGE blocks or photo groups name photos no longer
// attached to the post; Count is how many such photo places remain (POST-13).
type PhotoMissingError struct{ Count int }

func (e *PhotoMissingError) Error() string {
	return fmt.Sprintf("%d photo places name photos no longer attached", e.Count)
}

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
