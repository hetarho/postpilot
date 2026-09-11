package design

import (
	"fmt"
	"slices"
	"unicode"
)

// Chars counts what CDS's constraints count: Korean syllables, excluding spaces
// and punctuation. A Latin or digit run counts one per character, the stricter
// reading of a Korean-first table.
func Chars(text string) int {
	n := 0
	for _, r := range text {
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		n++
	}
	return n
}

// Anchors in vertical order. The step between consecutive cuts is measured on
// this list, which is why it is an order and not a set (CDS-38, V13).
var AnchorOrder = []string{"top", "upper_mid", "lower_mid", "bottom"}

// Element is one thing the renderer places: the text of a line, the plate under
// it, the accent bar or dot, or the highlight behind a keyword. Regions are
// canvas pixels and the window is the OUTPUT timeline, so the verifier needs
// nothing but the manifest to answer every check.
type Element struct {
	Cut              int
	Kind             string // copy | plate | bar | highlight
	Style            string
	Anchor           string
	Text             string
	Region           Region
	StartMS, EndMS   int
	FontSize         float64
	Fill, Background string
	InMS, OutMS      int
	DY               float64
}
type Manifest []Element

// Violation names the failing check. The clip domain turns it into its own
// invalid-plan error, so this package stays free of every other dependency.
type Violation string

func (v Violation) Error() string                { return "clip layout violation: " + string(v) }
func (v Violation) OutputValidationCode() string { return string(v) }

const (
	ViolationSafeArea   Violation = "plan_layout_safe_area"
	ViolationSize       Violation = "plan_layout_size"
	ViolationOverlap    Violation = "plan_layout_overlap"
	ViolationMotion     Violation = "plan_layout_motion"
	ViolationAnchorStep Violation = "plan_layout_anchor_step"
	ViolationFrequency  Violation = "plan_layout_frequency"
)

func overlaps(a, b Region) bool {
	return a.X < b.X+b.Width && b.X < a.X+a.Width && a.Y < b.Y+b.Height && b.Y < a.Y+a.Height
}
func within(r, safe Region) bool {
	return r.X >= safe.X && r.Y >= safe.Y && r.X+r.Width <= safe.X+safe.Width && r.Y+r.Height <= safe.Y+safe.Height
}

// Verify gates a render on the manifest (CDS-52). It runs before FFmpeg and
// before a single source byte is fetched, so a plan that breaks the design
// system costs neither credits nor a download.
//
// V1 safe area · V2 size floors · V5 lines and characters · V7 overlap between
// elements of different cuts whose windows meet · V9 the two permitted motions ·
// V13 one anchor step between consecutive cuts · V14 style frequency.
// V3 contrast needs frame sampling and lands with the brightness sampler; V4,
// V6, V8, V10, V11 and V12 belong to components this manifest does not carry yet.
func Verify(m Manifest, ratio string) error {
	safe, ok := Safe(ratio)
	if !ok {
		return ViolationSafeArea
	}
	lines := map[int]int{}
	for _, e := range m {
		if !slices.Contains([]string{"copy", "plate", "bar", "highlight"}, e.Kind) {
			return ViolationSize
		}
		style, known := Styles[e.Style]
		if !known || e.StartMS < 0 || e.EndMS <= e.StartMS {
			return ViolationSize
		}
		if !within(e.Region, safe) {
			return ViolationSafeArea
		}
		if e.InMS != Motion.InMS || e.OutMS != Motion.OutMS || e.DY != Motion.InDY {
			return ViolationMotion
		}
		if e.Kind != "copy" {
			continue
		}
		if e.FontSize < style.Role().Min || e.FontSize > style.Role().Size {
			return ViolationSize
		}
		if Chars(e.Text) > style.Chars {
			return ViolationSize
		}
		lines[e.Cut]++
	}
	for cut, n := range lines {
		if n > Styles[styleOfCut(m, cut)].Lines {
			return ViolationSize
		}
	}

	for i, a := range m {
		for _, b := range m[i+1:] {
			if a.Cut != b.Cut && a.StartMS < b.EndMS && b.StartMS < a.EndMS && overlaps(a.Region, b.Region) {
				return ViolationOverlap
			}
		}
	}
	return verifySequence(m)
}

func styleOfCut(m Manifest, cut int) string {
	for _, e := range m {
		if e.Cut == cut {
			return e.Style
		}
	}
	return ""
}

// The per-clip rules of CDS-38 and CDS-40, read off the cut order.
func verifySequence(m Manifest) error {
	cuts := []int{}
	style, anchor := map[int]string{}, map[int]string{}
	for _, e := range m {
		if _, seen := style[e.Cut]; !seen {
			cuts = append(cuts, e.Cut)
			style[e.Cut], anchor[e.Cut] = e.Style, e.Anchor
		}
		if style[e.Cut] != e.Style || anchor[e.Cut] != e.Anchor {
			return ViolationFrequency
		}
	}
	slices.Sort(cuts)
	bold, run := 0, 0
	for i, cut := range cuts {
		if style[cut] == "bold" {
			bold++
		}
		if i > 0 && style[cut] == style[cuts[i-1]] {
			run++
		} else {
			run = 1
		}
		// A fourth consecutive use of one style must have alternated (CDS-40).
		if run > 3 || bold > 2 {
			return ViolationFrequency
		}
		if i == 0 {
			continue
		}
		from, to := slices.Index(AnchorOrder, anchor[cuts[i-1]]), slices.Index(AnchorOrder, anchor[cut])
		if from < 0 || to < 0 {
			return ViolationAnchorStep
		}
		if to-from > 1 || from-to > 1 {
			return ViolationAnchorStep
		}
	}
	return nil
}

// String renders the manifest for a failure message; it never carries copy text,
// which may be the owner's own words.
func (m Manifest) String() string {
	return fmt.Sprintf("clip layout manifest: %d elements", len(m))
}
