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

// Coverage is read once per face at construction so the per-caption check is a
// set lookup (CDS-84). Only the ranges Korean captions are written in are
// enumerated; anything outside them is rare enough to ask the face directly.
var coverageRanges = [][2]rune{
	{0x0020, 0x007E}, // ASCII punctuation, digits and Latin
	{0x00A0, 0x00FF}, // Latin-1 punctuation and symbols
	{0x1100, 0x11FF}, // Hangul jamo
	{0x2000, 0x206F}, // general punctuation: … — ‘ ’ “ ”
	{0x20A0, 0x20BF}, // currency signs
	{0x2190, 0x21FF}, // arrows
	{0x3000, 0x303F}, // CJK symbols and punctuation: · 「 」 〈 〉
	{0x3130, 0x318F}, // Hangul compatibility jamo
	{0xAC00, 0xD7A3}, // Hangul syllables
	{0xFF00, 0xFFEF}, // fullwidth forms
}

func faceCoverage(fonts map[string]*sfnt.Font) map[string]map[rune]bool {
	out := make(map[string]map[rune]bool, len(fonts))
	for key, font := range fonts {
		set := map[rune]bool{}
		var buf sfnt.Buffer
		for _, span := range coverageRanges {
			for c := span[0]; c <= span[1]; c++ {
				if id, err := font.GlyphIndex(&buf, c); err == nil && id != 0 {
					set[c] = true
				}
			}
		}
		out[key] = set
	}
	return out
}

// sets answers whether one face can set one rune: the cached set first, and the
// face itself only for a rune outside the enumerated ranges.
func (r *Rendering) sets(role design.TypeRole, c rune) bool {
	key := fontFileKey(role.Face, role.Weight)
	if set, ok := r.coverage[key]; ok {
		if set[c] {
			return true
		}
		for _, span := range coverageRanges {
			if c >= span[0] && c <= span[1] {
				return false
			}
		}
	}
	font := r.fonts[key]
	if font == nil {
		return false
	}
	var buf sfnt.Buffer
	id, err := font.GlyphIndex(&buf, c)
	return err == nil && id != 0
}

// MissingGlyph is the first character of text the face has no glyph for, or 0
// when the face can set all of it. A control character is not a coverage
// problem — it is invalid input — so this reports only glyph absence.
func (r *Rendering) MissingGlyph(text string, role design.TypeRole) rune {
	for _, c := range text {
		if c == '\n' || c == '‍' || c == '︎' || c == '️' || unicode.IsControl(c) {
			continue
		}
		if !r.sets(role, c) {
			return c
		}
	}
	return 0
}
