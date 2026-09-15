package design

import (
	"slices"
	"strings"
	"unicode"
)

// Scene identifiers describe observed footage independently of typography.
var Scenes = []string{"food", "interior", "product", "exterior", "menu", "person", "scenery"}

const DefaultScene = "scenery"
const SubjectCoverMax = 0.15

func Scene(scene string) string {
	if slices.Contains(Scenes, scene) {
		return scene
	}
	return DefaultScene
}

// Candidate is one placement the selector may choose: an anchor and alignment
// with the plate the copy would actually occupy there, already measured.
type Candidate struct {
	Anchor, Align string
	Plate         Bounds
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
//	Observed empty-space containment ranks the remaining candidates without
//	overriding their subject-coverage preference or the one-step restriction.
func SelectAnchor(candidates []Candidate, subject Bounds, placed []Element, readableText bool, previous string, captionSafe []Bounds) int {
	viable, near := []int{}, []int{}
	for i, c := range candidates {
		if !c.Fits {
			continue
		}
		if readableText && c.Anchor != "top" && c.Anchor != "bottom" {
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
	if len(captionSafe) > 0 {
		inside := func(i int) bool {
			plate := candidates[i].Plate
			return slices.ContainsFunc(captionSafe, func(box Bounds) bool {
				return box.Width > 0 && box.Height > 0 && plate.X >= box.X && plate.Y >= box.Y && plate.X+plate.Width <= box.X+box.Width && plate.Y+plate.Height <= box.Y+box.Height
			})
		}
		slices.SortStableFunc(viable, func(a, b int) int {
			left, right := inside(a), inside(b)
			if left == right {
				return 0
			}
			if left {
				return -1
			}
			return 1
		})
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
		if cover <= SubjectCoverMax {
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
func coverage(plate, subject Bounds) float64 {
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
func collides(plate Bounds, placed []Element) bool {
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
