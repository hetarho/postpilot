package design

import (
	"slices"
	"strings"
	"unicode"
)

// The sentence classes of CDS-39, in the priority the table is read in.
const (
	ClassNum  = "NUM"
	ClassHook = "HOOK"
	ClassFact = "FACT"
	ClassDesc = "DESC"
)

// Classify answers what a sentence IS, by CDS-39 and in its priority order: a
// number with a unit first, then a hook, then a short noun-led fact, then
// everything else. It is the first half of the decision the model no longer
// makes.
func Classify(text string) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ClassDesc
	}
	if numbered(trimmed) {
		return ClassNum
	}
	chars := Chars(trimmed)
	if chars <= Classes.HookMaxChars && hooked(trimmed) {
		return ClassHook
	}
	if chars <= Classes.FactMaxChars && !verbEnded(trimmed) {
		return ClassFact
	}
	return ClassDesc
}

// An Arabic number followed by one of the CDS-39 units, spaces and the usual
// thousands separators aside: "9,900원" and "30 분" both count.
func numbered(text string) bool {
	runes := []rune(text)
	for i, r := range runes {
		if !unicode.IsDigit(r) {
			continue
		}
		for j := i + 1; j < len(runes); j++ {
			c := runes[j]
			if unicode.IsDigit(c) || c == ',' || c == '.' || c == ' ' {
				continue
			}
			if slices.Contains(Classes.NumUnits, string(c)) {
				return true
			}
			break
		}
	}
	return false
}
func hooked(text string) bool {
	if strings.HasSuffix(text, "?") || strings.HasSuffix(text, "!") {
		return true
	}
	for _, marker := range Classes.HookMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

// A FACT is noun-led: it does not close on a verb ending (CDS-39).
func verbEnded(text string) bool {
	stripped := strings.TrimRight(text, ".!? ")
	for _, ending := range Classes.FactVerbEndings {
		if strings.HasSuffix(stripped, ending) {
			return true
		}
	}
	return false
}

// Scene returns a known scene id, or the default for an analysis written before
// scenes existed (CDS-40 has no row for "unknown").
func Scene(scene string) string {
	if _, ok := SceneStyles[scene]; ok {
		return scene
	}
	return Guards.DefaultScene
}

// StyleHistory is what the guards of CDS-40 need to see: the styles already
// chosen for this clip, in cut order.
type StyleHistory []string

func (h StyleHistory) count(style string) int {
	n := 0
	for _, s := range h {
		if s == style {
			n++
		}
	}
	return n
}

// run is how many times the style at the end of the history repeats.
func (h StyleHistory) run() (string, int) {
	if len(h) == 0 {
		return "", 0
	}
	last, n := h[len(h)-1], 1
	for i := len(h) - 2; i >= 0 && h[i] == last; i-- {
		n++
	}
	return last, n
}

// SelectStyle is the CDS-40 table and its guards: the scene and the sentence
// class choose a style, the guards move it when the clip has had enough of it,
// and the template's approved set is the last word — with 깔끔하게 as the
// universal fallback, which is why every approved set must contain it.
//
// keywords is how many numbers or keywords the sentence turns on: 형광펜 needs
// exactly one (CDS-40).
func SelectStyle(scene, class string, allowed []string, history StyleHistory, keywords int) string {
	style := SceneStyles[Scene(scene)][class]
	if style == "" {
		style = Guards.Fallback
	}
	// 형광펜 highlights ONE number or keyword; with any other count it is 메모.
	if Styles[style].Highlight && keywords != 1 {
		style = "memo"
	}
	// 크게 강조 at most twice per clip, and the first one only on cut 1 or 2.
	if style == "bold" {
		cut := len(history) + 1
		if history.count("bold") >= Guards.BoldMax || (history.count("bold") == 0 && cut > Guards.BoldFirstCut) {
			style = Guards.Fallback
		}
	}
	// A fourth consecutive use of one style alternates between the two plated
	// styles instead of repeating.
	if last, n := history.run(); last == style && n >= Guards.RunMax {
		for _, alternate := range Guards.RunAlternate {
			if alternate != style {
				style = alternate
				break
			}
		}
	}
	return approved(style, allowed)
}

// approved keeps the choice inside the template's own set, falling back to
// 깔끔하게 and then to whatever the owner did approve.
func approved(style string, allowed []string) string {
	if len(allowed) == 0 || slices.Contains(allowed, style) {
		return style
	}
	if slices.Contains(allowed, Guards.Fallback) {
		return Guards.Fallback
	}
	return allowed[0]
}

// Candidate is one placement the selector may choose: an anchor and alignment
// with the plate the copy would actually occupy there, already measured.
type Candidate struct {
	Anchor, Align string
	Plate         Region
	Fits          bool
}

// SelectAnchor implements CDS-38 in order. It returns the chosen candidate's
// index, or -1 when the copy cannot be placed at all.
//
//	메모 walks TOP/LEFT then BOTTOM/LEFT; every other style its default anchor
//	then its alternative. A candidate outside the safe area is dropped, and so is
//	one overlapping the badge, a chip or a card — copy always yields to them.
//	With a principal-subject box the highest-ranked candidate covering ≤ 15 % of
//	it wins, else the least covering. Without a box the default. Footage carrying
//	readable text allows only TOP or BOTTOM, whichever is farther from the
//	subject. Consecutive cuts move at most one anchor step, except under that
//	readable-text rule.
func SelectAnchor(candidates []Candidate, subject Region, placed []Element, readableText bool, previous string) int {
	viable, near := []int{}, []int{}
	for i, c := range candidates {
		if !c.Fits {
			continue
		}
		if readableText && !slices.Contains(Guards.MenuAnchors, c.Anchor) {
			continue
		}
		if collides(c.Plate, placed) {
			continue
		}
		viable = append(viable, i)
		if readableText || previous == "" || step(previous, c.Anchor) <= 1 {
			near = append(near, i)
		}
	}
	// The one-step rule is a PREFERENCE, not a veto. CDS-40's own run guard can
	// demand a style whose anchors are three steps from the last cut's — 메모 at
	// TOP to 깔끔하게 at BOTTOM — and dropping the copy to obey the step would
	// lose the sentence to a rule about the eye. A style change is itself a
	// deliberate visual change, which is why V13 measures the step only between
	// consecutive cuts that share a style.
	if len(near) > 0 {
		viable = near
	}
	if len(viable) == 0 {
		return -1
	}
	if subject.Width <= 0 || subject.Height <= 0 {
		if readableText {
			// The farther of TOP and BOTTOM from the subject is undefined without
			// a box, so the first allowed candidate stands.
			return viable[0]
		}
		return viable[0]
	}
	best, least := -1, 2.0
	for _, i := range viable {
		cover := coverage(candidates[i].Plate, subject)
		if cover <= Guards.SubjectCoverMax {
			return i
		}
		if cover < least {
			best, least = i, cover
		}
	}
	return best
}

// step counts how many CDS-12 anchors apart two placements are.
func step(from, to string) int {
	a, b := slices.Index(AnchorOrder, from), slices.Index(AnchorOrder, to)
	if a < 0 || b < 0 {
		return len(AnchorOrder)
	}
	if a > b {
		return a - b
	}
	return b - a
}

// coverage is how much of the subject the plate hides, as a fraction of the
// subject's own area (CDS-38's 15 % rule).
func coverage(plate, subject Region) float64 {
	if subject.Width <= 0 || subject.Height <= 0 {
		return 0
	}
	w := min(plate.X+plate.Width, subject.X+subject.Width) - max(plate.X, subject.X)
	h := min(plate.Y+plate.Height, subject.Y+subject.Height) - max(plate.Y, subject.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h / (subject.Width * subject.Height)
}

// Copy yields to the badge, the chips and the cards; it never displaces them
// (CDS-38, CDS-45).
func collides(plate Region, placed []Element) bool {
	for _, e := range placed {
		if overlaps(plate, e.Region) {
			return true
		}
	}
	return false
}

// Banned reports whether the copy uses a voice CDS-42 refuses: emoji, ㅋㅋ, ㄹㅇ
// or a superlative.
func Banned(text string) bool {
	for _, token := range Voice.Banned {
		if strings.Contains(text, token) {
			return true
		}
	}
	for _, r := range text {
		if r > 0x2000 && !unicode.IsLetter(r) && !unicode.IsDigit(r) && !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// Transitions is CDS-36's whole rule, and the only place a transition is
// chosen: a hard cut joins two cuts of the same scene, a 200 ms fade joins two
// that differ, and at most 40 % of the boundaries may fade. The value at index i
// is the transition INTO cut i, so index 0 is always a hard cut — a clip does
// not fade in from nothing.
//
// When the fades exceed the ratio the EARLIEST ones are kept: they sit where the
// clip is still establishing its scenes, and a viewer who has already been shown
// three places does not need a fourth one softened.
func Transitions(scenes []string) []int {
	out := make([]int, len(scenes))
	if len(scenes) < 2 {
		return out
	}
	// The ratio is of the BOUNDARIES, of which there are one fewer than cuts.
	budget := int(Transition.FadeRatioMax * float64(len(scenes)-1))
	for i := 1; i < len(scenes); i++ {
		if Scene(scenes[i]) == Scene(scenes[i-1]) || budget <= 0 {
			continue
		}
		out[i] = Transition.FadeMS
		budget--
	}
	return out
}

// CutBounds is CDS-37's TARGET range for one cut, in milliseconds: the template
// preset's own range where it names one (CDS-50), the shared 1.2–6.0 s where it
// does not, and a food close-up capped at 4.0 s either way. These are editing
// rhythm, not correctness: the compiler aims at them wherever the approved
// timeline still allows and never refuses a plan for missing them (r3).
func CutBounds(scene, preset string) (int, int) {
	minimum, maximum := Timing.CutMinS, Timing.CutMaxS
	if p, ok := Presets[preset]; ok && p.CutMinS > 0 && p.CutMaxS > 0 {
		minimum, maximum = p.CutMinS, p.CutMaxS
	}
	if Scene(scene) == "food" {
		maximum = min(maximum, Timing.CutMaxFoodS)
	}
	minimum = min(minimum, maximum)
	return int(minimum * 1000), int(maximum * 1000)
}
