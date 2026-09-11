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
// is a decoration the design system refuses (CDS-6, CDS-33). `card` is a hook or
// ending plate, `chip-category` the preset label on the hook card, and `scrim`
// the one full-bleed wash CDS-32 allows.
var ElementKinds = []string{"copy", "plate", "bar", "highlight", "badge", "chip", "card", "chip-category", "scrim"}

type Element struct {
	Cut int
	// Which of the cut's copies this element belongs to (CDS-43): 0 for the one
	// copy a cut usually carries, and for every piece of furniture.
	Copy             int
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
	ViolationContrast   Violation = "plan_layout_contrast"
)

// An element belongs to a caption exactly when it carries a copy style: the
// line, the plate, the bar or dot, the highlight and the scrim the compiler
// placed for one written sentence. The badge, the chips and the two cards are
// the design system's own furniture — they carry no style, never move (CDS-31,
// CDS-30, CDS-28) and are not measured against a style's limits.
func caption(e Element) bool { return e.Style != "" }

// MinTypeSize is the smallest size anything in the type scale may be set at:
// the floor of its smallest role (CDS-19).
func MinTypeSize() float64 {
	smallest := 0.0
	for _, role := range Type {
		if smallest == 0 || role.Min < smallest {
			smallest = role.Min
		}
	}
	return smallest
}

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
// V1 safe area · V2 size floors · V3 contrast against the effective background ·
// V5 lines and characters · V7 overlap between elements of different cuts whose
// windows meet · V9 the two permitted motions · V13 one anchor step between
// consecutive cuts · V14 style frequency.
// V4, V8, V11 and V12 belong to components this manifest does not carry yet.
func Verify(m Manifest, ratio string) error { return VerifyApproved(m, ratio, nil) }

// VerifyApproved is Verify for a clip whose template approved only some styles.
// CDS-40's run rule asks a fourth consecutive use to alternate between 깔끔하게
// and 메모; a template that approved only one of the two left the composer no
// alternate, so the run is the owner's choice, not a defect. nil approves
// every style.
func VerifyApproved(m Manifest, ratio string, approved []string) error {
	safe, ok := Safe(ratio)
	if !ok {
		return ViolationSafeArea
	}
	// A cut may carry two copies (CDS-43), so every per-caption rule is keyed by
	// the copy, not by the cut it sits on.
	lines := map[slot]int{}
	for _, e := range m {
		if !slices.Contains(ElementKinds, e.Kind) {
			return ViolationKind
		}
		if e.StartMS < 0 || e.EndMS <= e.StartMS {
			return ViolationSize
		}
		// Two elements are not bound by the safe area, and only two: the scrim,
		// which CDS-32 places full-bleed across the frame's own edge, and the
		// card plate, whose 9:16 width CDS-28 states as x 144–936 — sixteen
		// pixels past CDS-9's x ≤ 920. What the safe area exists to protect is
		// text, and a card's own lines sit 40 px inside its plate, so they are
		// checked here like every other text.
		if e.Kind != "scrim" && e.Kind != "card" && !within(e.Region, safe) {
			return ViolationSafeArea
		}
		if err := verifyContrast(e); err != nil {
			return err
		}
		// The badge never moves, a chip does not settle and a card only fades:
		// only a caption's own elements carry the two permitted motions (CDS-31,
		// CDS-30, CDS-28, CDS-4).
		if !caption(e) {
			if e.InMS != 0 || e.OutMS != 0 || e.DY != 0 {
				return ViolationMotion
			}
			// A card's own lines are typeset by the design system at a role's
			// nominal size, so what V2 has to hold them to is the scale's own
			// floor: no text in a clip is ever smaller than the smallest role
			// CDS-19 defines (CDS-2).
			if (e.Kind == "copy" || e.Kind == "chip-category") && e.FontSize < MinTypeSize() {
				return ViolationSize
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
		// CDS-32: a scrim appears only under an UNPLATED style and never lies on
		// a plate. A wash under ink at α ≥ 0.72 would darken nothing and dim the
		// footage for no reason.
		if e.Kind == "scrim" && style.Plate != "" {
			return ViolationKind
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
		lines[slotOf(e)]++
	}
	for at, n := range lines {
		if n > Styles[styleOf(m, at)].Lines {
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
			// A scrim lies under everything by definition (CDS-45): it is the
			// ground the badge, the chips and the copy are read against, not a
			// thing they can collide with.
			if a.Kind == "scrim" || b.Kind == "scrim" {
				continue
			}
			if sameElement(a, b) || !(a.StartMS < b.EndMS && b.StartMS < a.EndMS) {
				continue
			}
			if overlaps(a.Region, b.Region) {
				return ViolationOverlap
			}
		}
	}
	return verifySequence(m, approved)
}

// V3: every text is read against its effective background — the plate or card
// under it, or, for an unplated style, the scrim and the sampled footage the
// renderer resolved (CDS-44) — and no pairing may fall under 4.5:1 (CDS-16).
// An element whose background is unknown carries no pairing to check: an
// unplated caption before the sampler has run, and every element the design
// system already certified against its token (CDS-16's plated α ≥ 0.72).
func verifyContrast(e Element) error {
	if e.Kind != "copy" && e.Kind != "chip-category" {
		return nil
	}
	if e.Fill == "" || e.Background == "" {
		return nil
	}
	ratio, ok := Contrast(e.Fill, e.Background)
	if !ok || ratio < Luma.ContrastMin {
		return ViolationContrast
	}
	return nil
}

// Legible reports whether every pairing in these elements clears V3's floor: the
// same check Verify runs, for a renderer deciding whether to fall back (CDS-44).
func Legible(m Manifest) bool {
	for _, e := range m {
		if verifyContrast(e) != nil {
			return false
		}
	}
	return true
}

// The elements of one caption are meant to sit on each other, and so are a
// card's plate and its own lines. Every other pair that shares a window is
// checked: the badge against a chip, a chip against another chip, and a card
// against any caption — which is what keeps copy and chips out from under a
// card (CDS-45, V7).
func sameElement(a, b Element) bool {
	if a.Cut != b.Cut {
		return false
	}
	// Two copies of ONE cut are two elements, not one: CDS-43 keeps them apart
	// in time, and if they ever shared a window they would have to be checked
	// against each other like anything else.
	if caption(a) && caption(b) {
		return a.Copy == b.Copy
	}
	return cardPart(a) && cardPart(b)
}

// slot names one caption of one cut, which is the scope every per-caption rule
// is read on (CDS-43).
type slot struct{ Cut, Copy int }

func slotOf(e Element) slot { return slot{e.Cut, e.Copy} }

// cardPart is the card plate itself, its category chip or one of its own lines.
func cardPart(e Element) bool {
	return !caption(e) && (e.Kind == "card" || e.Kind == "chip-category" || e.Kind == "copy")
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
func styleOf(m Manifest, at slot) string {
	for _, e := range m {
		if slotOf(e) == at && caption(e) {
			return e.Style
		}
	}
	return ""
}

// The per-clip rules of CDS-38 and CDS-40, read off the cut order.
func verifySequence(m Manifest, approved []string) error {
	alternates := canAlternate(approved)
	cuts := []slot{}
	style, anchor := map[slot]string{}, map[slot]string{}
	for _, e := range m {
		if !caption(e) {
			continue
		}
		at := slotOf(e)
		if _, seen := style[at]; !seen {
			cuts = append(cuts, at)
			style[at], anchor[at] = e.Style, e.Anchor
		}
		if style[at] != e.Style || anchor[at] != e.Anchor {
			return ViolationFrequency
		}
	}
	// In clip order: a cut's second copy follows its first, and CDS-40's run and
	// CDS-38's step are read over that sequence.
	slices.SortFunc(cuts, func(a, b slot) int {
		if a.Cut != b.Cut {
			return a.Cut - b.Cut
		}
		return a.Copy - b.Copy
	})
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
		if (run > Guards.RunMax && alternates) || bold > Guards.BoldMax {
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

// canAlternate reports whether the approved set holds both styles CDS-40
// alternates between, which is what makes a long run avoidable.
func canAlternate(approved []string) bool {
	if approved == nil {
		return true
	}
	n := 0
	for _, style := range Guards.RunAlternate {
		if slices.Contains(approved, style) {
			n++
		}
	}
	return n >= 2
}
