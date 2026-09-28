package media

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"

	"github.com/postpilot/backend/internal/clip/design"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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
//
// A face covers a character only where it DRAWS it: Paperlogy maps all 11,172
// syllables to glyphs but 8,392 of those glyphs have no outline, so a cmap entry
// alone would pass 갂 and the caption would render it blank.
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

// Reading every outline costs about 40 ms a face, and a bundled face is pinned
// by its checksum, so one process reads each file's set once however many
// renderers it builds. The sets are never written after they are stored.
var coverageCache sync.Map

func faceCoverage(fonts map[string]*sfnt.Font) map[string]map[rune]bool {
	out := make(map[string]map[rune]bool, len(fonts))
	for _, f := range bundledFonts {
		font := fonts[f.Key]
		if font == nil {
			continue
		}
		if set, ok := coverageCache.Load(f.SHA256); ok {
			out[f.Key] = set.(map[rune]bool)
			continue
		}
		set, _ := coverageCache.LoadOrStore(f.SHA256, drawnSet(font))
		out[f.Key] = set.(map[rune]bool)
	}
	return out
}

func drawnSet(font *sfnt.Font) map[rune]bool {
	set := map[rune]bool{}
	var buf sfnt.Buffer
	for _, span := range coverageRanges {
		for c := span[0]; c <= span[1]; c++ {
			if draws(font, &buf, c) {
				set[c] = true
			}
		}
	}
	return set
}

// draws answers whether a face paints a character: a glyph for it that has an
// outline, or none needed because the character is a space or a format mark,
// which is drawn by its advance alone.
func draws(font *sfnt.Font, buf *sfnt.Buffer, c rune) bool {
	id, err := font.GlyphIndex(buf, c)
	if err != nil || id == 0 {
		return false
	}
	if unicode.IsSpace(c) || unicode.Is(unicode.Cf, c) {
		return true
	}
	segments, err := font.LoadGlyph(buf, id, fixed.I(100), nil)
	return err == nil && len(segments) > 0
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
	return draws(font, &buf, c)
}

// A caption character its style's face does not draw is set in Wanted Sans
// Variable, at the style's weight (CDS-84).
func substituteRole(role design.TypeRole) design.TypeRole {
	return design.TypeRole{Face: "wantedsans", Weight: role.Weight}
}

// MissingGlyph is the first character of text the face does not draw, or 0
// when the face can set all of it. A control character is not a coverage
// problem — it is invalid input — so this reports only glyph absence.
func (r *Rendering) MissingGlyph(text string, role design.TypeRole) rune {
	for _, c := range text {
		if coverageExempt(c) {
			continue
		}
		if !r.sets(role, c) {
			return c
		}
	}
	return 0
}

// MissingCaptionGlyph is the first character of a caption that neither its
// style's face nor the substitute draws: the one thing that still sends a
// caption to the default style (CDS-84).
func (r *Rendering) MissingCaptionGlyph(text string, role design.TypeRole) rune {
	for _, c := range text {
		if coverageExempt(c) {
			continue
		}
		if !r.sets(role, c) && !r.sets(substituteRole(role), c) {
			return c
		}
	}
	return 0
}

// substitutes is the set of a caption's characters its style's face does not
// draw and Wanted Sans Variable does, or nil when there is none. A face that is
// Wanted Sans itself has nothing to substitute.
func (r *Rendering) substitutes(text string, role design.TypeRole) map[rune]bool {
	if fontFileKey(role.Face, role.Weight) == fontFileKey("wantedsans", role.Weight) {
		return nil
	}
	var out map[rune]bool
	for _, c := range text {
		if coverageExempt(c) || r.sets(role, c) || !r.sets(substituteRole(role), c) {
			continue
		}
		if out == nil {
			out = map[rune]bool{}
		}
		out[c] = true
	}
	return out
}

func coverageExempt(c rune) bool {
	return c == '\n' || c == '\u200d' || c == '\ufe0e' || c == '\ufe0f' || unicode.IsControl(c)
}
