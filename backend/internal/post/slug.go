package post

import (
	"fmt"
	"strings"
	"unicode"
)

// untitledSlugBody is what a post with no title gets. A date alone would collide with
// every other untitled post from the same day and read as an error in the URL.
const untitledSlugBody = "untitled"

// maxSlugBodyRunes caps the title part. Object keys embed the slug, and a pasted
// paragraph as a title would otherwise produce a key nothing can work with.
const maxSlugBodyRunes = 60

// MintSlug builds `YYYYMMDD-<title>`, appending `-2`, `-3`, … until exists says no.
//
// It is minted once and never revised, because the slug is the primary key AND part of
// every object key for the post's photos — renaming it would orphan the photos. That
// is also why sanitizing is aggressive: the result has to survive being a URL path
// segment and an S3 key.
//
// Korean (and any other letter) is kept. A slug is not required to be ASCII; URL
// encoding handles it, and stripping Hangul would make almost every real title
// "untitled".
func MintSlug(date string, title string, exists func(string) bool) string {
	candidate := date + "-" + slugBody(title)
	if !exists(candidate) {
		return candidate
	}
	// The PRD (§7) specifies a serial suffix for "same title, same day". Starting at 2
	// makes the first duplicate `-2`, which reads as "the second one".
	for n := 2; ; n++ {
		next := fmt.Sprintf("%s-%d", candidate, n)
		if !exists(next) {
			return next
		}
	}
}

// slugBody sanitizes a title into one path segment.
func slugBody(title string) string {
	var b strings.Builder
	lastWasSeparator := false

	for _, r := range strings.TrimSpace(title) {
		switch {
		case unicode.IsSpace(r) || r == '-' || r == '_':
			// Collapse runs of whitespace and existing separators into one hyphen, so
			// "제주  3일차" and "제주-3일차" mint the same shape.
			lastWasSeparator = true
		case unicode.IsControl(r), isUnsafeInPath(r):
			// Dropped, not replaced: a title of "a/b" should read "ab", not "a-b".
		default:
			if lastWasSeparator && b.Len() > 0 {
				b.WriteRune('-')
			}
			lastWasSeparator = false
			b.WriteRune(unicode.ToLower(r))
		}
	}

	body := truncateRunes(b.String(), maxSlugBodyRunes)
	// Truncation can land on a hyphen, and a trailing one looks like a mistake.
	body = strings.Trim(body, "-")
	if body == "" {
		return untitledSlugBody
	}
	return body
}

// isUnsafeInPath reports characters that would break a URL path segment or an S3 key.
func isUnsafeInPath(r rune) bool {
	switch r {
	case '/', '\\', '?', '#', '%', ':', '*', '"', '\'', '<', '>', '|', '&', '+', '.':
		return true
	}
	return false
}

func truncateRunes(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit])
}
