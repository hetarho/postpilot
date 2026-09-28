package ai

import (
	"slices"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip/composition"
)

func sentenceKey(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			return -1
		}
		return unicode.ToLower(r)
	}, text)
}

func generatesText(e composition.Element) bool {
	if len(e.Rows) == 0 {
		return e.Kind == "ai"
	}
	return slices.ContainsFunc(e.Rows, func(row composition.Row) bool { return composition.RowKind(e, row) == "ai" })
}
