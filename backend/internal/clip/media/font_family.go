package media

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip/design"
	"golang.org/x/image/font/sfnt"
)

// ErrFontFamily identifies V19 failures before a renderer can substitute a face.
var ErrFontFamily = errors.New("clip font family does not resolve")

func validateFontFamilies(fonts map[string]*sfnt.Font) error {
	for key, family := range design.Faces {
		if !resolvesFontFamily(fonts[key], family) {
			return fmt.Errorf("%w: faces.%s=%q", ErrFontFamily, key, family)
		}
	}
	return nil
}

// SVG emits these names as unquoted CSS identifiers separated by spaces. A
// display name containing a numeric component is not that CSS family, even
// when the font's legacy name table happens to contain the same display name.
func resolvesFontFamily(font *sfnt.Font, family string) bool {
	if font == nil || strings.TrimSpace(family) == "" {
		return false
	}
	for _, component := range strings.Fields(family) {
		component = strings.TrimPrefix(component, "-")
		if component == "" {
			return false
		}
		for i, r := range component {
			if !(unicode.IsLetter(r) || r == '_' || i > 0 && (unicode.IsDigit(r) || r == '-')) {
				return false
			}
		}
	}
	for _, id := range []sfnt.NameID{sfnt.NameIDFamily, sfnt.NameIDTypographicFamily} {
		name, err := font.Name(nil, id)
		if err == nil && name == family {
			return true
		}
	}
	return false
}
