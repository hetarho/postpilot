package media

import (
	"math"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// cutTimeline is a composition's one frame arithmetic at the output rate: the
// frames each entry contributes, the output frame it starts on, and the frames
// and xfade its leading transition overlaps the entry before it with (CDS-36).
// The render's picture and sound joins and the ground sampler all build from
// it, and the browser draws the frames its fixture records, so a browser and a
// server export of one plan show and sample the same frames (CLIP-192).
type cutTimeline struct {
	fps int
	// frames is how long each entry is and starts its first output frame. The
	// leading transition is in transitions (ms), overlaps (frames) and kinds
	// (the xfade drawing it, empty for a hard cut); the first entry has none,
	// whatever it carries, because it leads in from nothing.
	frames, starts, transitions, overlaps []int
	kinds                                 []string
	total                                 int
}

// newCutTimeline lays a plan's cuts out. Rounding cumulative cut time (not
// every individual cut independently) keeps the final CFR duration within half
// a frame of the millisecond plan. The clock it rounds is the TRANSFORMED output
// timeline, so a cut's frame budget already carries its fixed rate and no
// individually rounded clip can drift away from the captions and transitions
// placed on that same timeline (CDS-62).
func newCutTimeline(fps int, plan clip.EditPlan) cutTimeline {
	frames, transitions := make([]int, len(plan.Cuts)), make([]int, len(plan.Cuts))
	elapsed, previous := 0, 0
	for i, c := range plan.Cuts {
		elapsed += c.OutputDurationMS()
		next := int(math.Round(float64(elapsed*fps) / 1000))
		frames[i], transitions[i] = next-previous, c.TransitionMS
		previous = next
	}
	return joinedTimeline(fps, frames, transitions)
}

// joinedTimeline joins entries whose length is already known — a plan's cuts,
// or the merge tree's branches, each carrying the transition that leads into it.
func joinedTimeline(fps int, frames, transitions []int) cutTimeline {
	t := cutTimeline{fps: fps, frames: frames, transitions: transitions, starts: make([]int, len(frames)), overlaps: make([]int, len(frames)), kinds: make([]string, len(frames))}
	if len(frames) == 0 {
		return t
	}
	elapsed := frames[0]
	for i := 1; i < len(frames); i++ {
		// Every admitted transition is a whole number of frames at 30 fps, so
		// the output timeline stays exactly integral (CDS-36).
		t.overlaps[i] = transitions[i] * fps / 1000
		if t.overlaps[i] > 0 {
			t.kinds[i] = transitionKind(transitions[i])
		}
		t.starts[i] = elapsed - t.overlaps[i]
		elapsed += frames[i] - t.overlaps[i]
	}
	t.total = elapsed
	return t
}

// The xfade CDS-36 names for a transition: a plain dissolve for the 200 ms fade
// a scene change earns, and through black for the 300 ms one a manual plan may
// ask for around the cards.
func transitionKind(ms int) string {
	if ms == design.Transition.BlackMS {
		return "fadeblack"
	}
	return "fade"
}

// footageTake is one cut's own frame, counted from the first frame the cut
// contributes.
type footageTake struct{ cut, local int }

// outputFootage is how the composition makes one output frame: one cut's own
// frame, or inside an overlap the xfade of the outgoing and incoming cuts at
// xfade's own progress, which is 1 on the overlap's first frame.
type outputFootage struct {
	outgoing, incoming footageTake
	kind               string
	progress           float64
}

func (o outputFootage) blended() bool { return o.kind != "" }

// at is the footage of one output frame.
func (t cutTimeline) at(frame int) (outputFootage, bool) {
	if frame < 0 || frame >= t.total {
		return outputFootage{}, false
	}
	for i := len(t.frames) - 1; i >= 0; i-- {
		if frame < t.starts[i] {
			continue
		}
		if frame >= t.starts[i]+t.frames[i] {
			return outputFootage{}, false
		}
		own := footageTake{cut: i, local: frame - t.starts[i]}
		if i > 0 && frame < t.starts[i]+t.overlaps[i] {
			k := frame - t.starts[i]
			return outputFootage{outgoing: footageTake{cut: i - 1, local: frame - t.starts[i-1]}, incoming: own,
				kind: t.kinds[i], progress: 1 - float64(k)/float64(t.overlaps[i])}, true
		}
		return outputFootage{outgoing: footageTake{cut: -1}, incoming: own}, true
	}
	return outputFootage{}, false
}

// weights is how much of the outgoing and the incoming cut's frame xfade mixes
// into this output frame. A dissolve is the plain mix at xfade's progress; a
// fade through black first mixes each side with black on its own smoothstep
// over the phase xfade uses, and the rest of the frame is that black.
func (o outputFootage) weights() (outgoing, incoming float64) {
	switch o.kind {
	case "":
		return 0, 1
	case "fadeblack":
		const phase = 0.2
		return smoothstep(1-phase, 1, o.progress) * o.progress, (1 - smoothstep(phase, 1, o.progress)) * (1 - o.progress)
	}
	return o.progress, 1 - o.progress
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := min(1, max(0, (x-edge0)/(edge1-edge0)))
	return t * t * (3 - 2*t)
}
