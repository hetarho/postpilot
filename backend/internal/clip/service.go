package clip

import (
	"fmt"
	"strings"
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

// RequiredAnswers is the generation gate, separate from saving an unfinished form.
func RequiredAnswers(t VideoTemplate, p Project, limits ...composition.Limits) error {
	if t.CompositionBody != "" && !t.CompositionLegacy {
		if len(limits) != 1 {
			return ErrInvalid
		}
		_, err := GenerationComposition(t, p, limits[0])
		return err
	}
	answers := map[string]string{}
	for _, a := range p.Answers {
		answers[a.Label] = a.Text
	}
	for _, f := range t.InformationFields {
		if strings.TrimSpace(answers[f.Label]) == "" {
			return fmt.Errorf("%w: missing template answer", ErrInvalid)
		}
	}
	return nil
}

// These identifiers are retained for legacy project conversion only. Authored
// composition owns the visible text; setup and admission do not require a campaign.
func ValidDisclosure(s string) bool {
	_, ok := design.Disclosure[s]
	return ok
}
func ValidCTA(s string) bool {
	if s == "" {
		return true
	}
	_, ok := design.CTA[s]
	return ok
}
func ValidPreset(s string) bool {
	_, ok := design.Presets[s]
	return ok
}

// MissingFactsError names the reserved labels a clip still needs, so the refusal
// can say which ones rather than that something is missing.
type MissingFactsError struct{ Labels []string }

func (e *MissingFactsError) Error() string {
	return "clip needs more on-screen facts: " + strings.Join(e.Labels, ", ")
}
