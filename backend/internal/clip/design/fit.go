package design

import (
	"math"
	"strings"
)

// SlotSpec is what a region slot's fit needs to know about its type: the role
// and the size a preset sets it at, the floor it may shrink to, its face and
// tracking, and how many lines it may take (CDS-19, CDS-86).
type SlotSpec struct {
	Role     string
	Size     float64
	Floor    float64
	Face     string
	Weight   int
	Tracking float64
	// 0 takes the role's own allowance: two lines for the large roles, one for
	// the rest. A preset sets 1 for a slot that never wraps.
	Lines int
}

// Fit is one slot's text laid out for its width: the lines it takes, the whole
// pixel size they are set at, and whether it still does not fit.
type Fit struct {
	Lines []string
	Size  float64
	// Still wider than the width at the floor, or holding a character the face
	// lacks: the slot overflows under CDS-77.
	Over bool
}

// The roles a slot may wrap in when shrinking alone would take it below its
// floor (CDS-86).
var wrappingRoles = map[string]bool{"display": true, "headline": true, "hook": true, "title": true}

// MaxLines is how many lines the spec allows.
func (s SlotSpec) MaxLines() int {
	if s.Lines > 0 {
		return s.Lines
	}
	if wrappingRoles[s.Role] {
		return 2
	}
	return 1
}

func (s SlotSpec) width(text string, size float64) float64 {
	return TextWidth(s.Face, s.Weight, s.Tracking, size, text)
}

// FitRegionSlot is CDS-86: text that fits keeps the slot's size; wider text
// shrinks in proportion to its width down to the floor; a slot allowed two
// lines that would need less than its floor splits at the word (어절) boundary
// that best balances the lines and fits again. Sizes are whole pixels, rounded
// down so the drawn line never exceeds the width.
func FitRegionSlot(spec SlotSpec, text string, width float64) Fit {
	if strings.TrimSpace(text) == "" {
		return Fit{Size: spec.Size}
	}
	floor := math.Min(spec.Floor, spec.Size)
	if floor <= 0 {
		floor = spec.Size
	}
	covered := Covers(spec.Face, spec.Weight, text)
	one := fitSize(spec, width, spec.width(text, 100))
	if one >= floor {
		return Fit{Lines: []string{text}, Size: one, Over: !covered}
	}
	if spec.MaxLines() >= 2 {
		if lines, widest, ok := balancedSplit(spec, text); ok {
			two := fitSize(spec, width, widest)
			if two >= floor {
				return Fit{Lines: lines, Size: two, Over: !covered}
			}
			return Fit{Lines: lines, Size: floor, Over: true}
		}
	}
	return Fit{Lines: []string{text}, Size: floor, Over: true}
}

// fitSize is the largest whole-pixel size, no bigger than the slot's own, at
// which a line `at100` wide at size 100 still fits the width.
func fitSize(spec SlotSpec, width, at100 float64) float64 {
	if at100 <= 0 {
		return spec.Size
	}
	return math.Min(spec.Size, math.Floor(100*width/at100+1e-9))
}

// balancedSplit tries every word boundary and keeps the two lines whose wider
// line is narrowest; text of one word has no boundary to split at.
func balancedSplit(spec SlotSpec, text string) ([]string, float64, bool) {
	words := strings.Fields(text)
	if len(words) < 2 {
		return nil, 0, false
	}
	best, widest := []string(nil), math.Inf(1)
	for i := 1; i < len(words); i++ {
		lines := []string{strings.Join(words[:i], " "), strings.Join(words[i:], " ")}
		w := math.Max(spec.width(lines[0], 100), spec.width(lines[1], 100))
		if w < widest {
			best, widest = lines, w
		}
	}
	return best, widest, true
}

// RegionSlotBudget is how many Hangul syllables fit the slot at its floor
// across the lines it allows, the unit CLIP-116 counts in: the writer is told
// it and the answer counter holds to it, while the fit itself stays the judge.
func RegionSlotBudget(spec SlotSpec, width float64) int {
	m, ok := FaceMetricsFor(spec.Face, spec.Weight)
	if !ok {
		return 0
	}
	syllable := m.Hangul
	if syllable <= 0 {
		for text, w := range m.Glyphs {
			if r := []rune(text); len(r) == 1 && isHangulSyllable(r[0]) && w > syllable {
				syllable = w
			}
		}
	}
	floor := math.Min(spec.Floor, spec.Size)
	if floor <= 0 {
		floor = spec.Size
	}
	if syllable <= 0 || floor <= 0 {
		return 0
	}
	track := spec.Tracking * metrics.Units
	perLine := int(math.Floor((width*metrics.Units/floor + track) / (syllable + track)))
	return max(0, perLine) * spec.MaxLines()
}
