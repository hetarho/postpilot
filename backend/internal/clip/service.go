package clip

import (
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

func BoundedText(s string, min, max int) bool {
	n := utf8.RuneCountInString(s)
	return utf8.ValidString(s) && n >= min && n <= max
}

// Empty is neutral; the seven others are the CDS-15 palette, one per project.
func ValidAccent(s string) bool {
	if s == "" {
		return true
	}
	_, ok := design.Accent[s]
	return ok
}

// RequiredAnswers is the generation gate, separate from saving an unfinished form:
// the outline's required values and items must be present (CLIP-5, CLIP-102).
func RequiredAnswers(t VideoTemplate, p Project, limits composition.Limits) error {
	_, err := GenerationComposition(t, p, limits)
	return err
}

// The campaign type the disclosure badge names (CDS-31).
func ValidDisclosure(s string) bool {
	_, ok := design.Disclosure[s]
	return ok
}
