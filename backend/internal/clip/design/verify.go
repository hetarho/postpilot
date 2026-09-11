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
// ElementKinds is the whole catalogue a manifest may name (V10): anything else
// is a decoration the design system refuses (CDS-6, CDS-33).
var ElementKinds = []string{"copy", "plate", "bar", "highlight", "badge", "chip"}

type Element struct {
	Cut              int
	Kind             string // one of ElementKinds
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
	ViolationDisclosure Violation = "plan_layout_disclosure"
	ViolationKind       Violation = "plan_layout_kind"
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
		if !slices.Contains(ElementKinds, e.Kind) {
			return ViolationKind
		}
		if e.StartMS < 0 || e.EndMS <= e.StartMS {
			return ViolationSize
		}
		if !within(e.Region, safe) {
			return ViolationSafeArea
		}
		// The badge never moves and a chip does not settle: only a copy's own
		// elements carry the two permitted motions (CDS-31, CDS-30, CDS-4).
		if e.Kind == "badge" || e.Kind == "chip" {
			if e.InMS != 0 || e.OutMS != 0 || e.DY != 0 {
				return ViolationMotion
			}
			continue
		}
		style, known := Styles[e.Style]
		if !known {
			return ViolationSize
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

	// The badge is placed first and never moves (CDS-45), so it is checked first.
	duration := durationOf(m)
	if err := verifyDisclosure(m, duration); err != nil {
		return err
	}
	for i, a := range m {
		for _, b := range m[i+1:] {
			// A badge or chip shares its window with the copy of whatever cut it
			// spans, so overlap is checked between every pair whose windows meet
			// and that do not belong to the same cut's own copy.
			if sameElement(a, b) || !(a.StartMS < b.EndMS && b.StartMS < a.EndMS) {
				continue
			}
			if overlaps(a.Region, b.Region) {
				return ViolationOverlap
			}
		}
	}
	return verifySequence(m)
}

// Elements of one cut's own copy are meant to sit on each other; the badge is
// its own layer and a chip belongs to whichever cut it spans.
func sameElement(a, b Element) bool {
	return a.Kind != "badge" && b.Kind != "badge" && a.Kind != "chip" && b.Kind != "chip" && a.Cut == b.Cut
}
func durationOf(m Manifest) int {
	end := 0
	for _, e := range m {
		if e.EndMS > end {
			end = e.EndMS
		}
	}
	return end
}

// V6: the disclosure phrase is present, covers the first and last 3.0 s, is
// typeset at the badge size and painted on the badge colour. The Fair Trade
// endorsement guideline asks for a size, face and colour clearly distinct from
// the background, shown at a video's start and end (CDS-5, CDS-31).
func verifyDisclosure(m Manifest, duration int) error {
	if len(m) == 0 {
		return nil
	}
	badge := Element{}
	for _, e := range m {
		if e.Kind != "badge" {
			continue
		}
		if badge.Kind != "" {
			return ViolationDisclosure
		}
		badge = e
	}
	phrase := false
	for _, text := range Disclosure {
		phrase = phrase || badge.Text == text
	}
	if !phrase || badge.FontSize != Type["badge"].Size || badge.Background != Color["badge_ad"].Hex {
		return ViolationDisclosure
	}
	// ONE badge covering both of CDS-5's minimum windows — the first 3.0 s and
	// the last 3.0 s — necessarily spans the whole clip, which is also CDS-5's
	// default. A second badge would be a second disclosure and is refused above.
	if badge.StartMS > 0 || badge.EndMS < duration {
		return ViolationDisclosure
	}
	if duration < int(Timing.BadgeMinHeadS*1000) && duration < int(Timing.BadgeMinTailS*1000) {
		return ViolationDisclosure
	}
	return nil
}

// The badge is its own layer and a chip carries no style, so neither answers for
// the cut it happens to sit on.
func styleOfCut(m Manifest, cut int) string {
	for _, e := range m {
		if e.Cut == cut && e.Kind != "badge" && e.Kind != "chip" {
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
		if e.Kind == "badge" || e.Kind == "chip" {
			continue
		}
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
		// Only between cuts that share a style: a style change is a deliberate
		// visual change, and CDS-40 can demand one whose anchors are further
		// apart than a step (see SelectAnchor).
		if style[cut] != style[cuts[i-1]] {
			continue
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
