package composition_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

// A caption bound to a cut resolves on the cut's TRANSFORMED output length, while
// the cut keeps stating the original source range it was taken from (CDS-62).
func TestCutIntervalsResolveOnTheTransformedOutputTimeline(t *testing.T) {
	l := clip.DefaultCompositionLimits()
	d, problem := composition.Parse(`<clip version="1" intro="b" caption="bold" outro="e"><scene id="s"><text id="c" kind="fixed" role="caption" basis="cut">x</text></scene><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`, l)
	if problem != nil {
		t.Fatal(problem)
	}
	// The same 20 s of footage at 1x and at 2x.
	for _, c := range []struct{ rate, duration int }{{0, 20000}, {1000, 20000}, {2000, 10000}, {500, 40000}} {
		cut := composition.Cut{ID: "a", SourceID: "v", SectionID: "s", StartMS: 5000, EndMS: 25000, PlaybackRatePermille: c.rate}
		timeline, problem := composition.Resolve(d, composition.Inputs{Cuts: []composition.Cut{cut}}, l, 100000)
		if problem != nil {
			t.Fatal(c.rate, problem)
		}
		if timeline.DurationMS != c.duration {
			t.Fatalf("rate %d resolved to %d, not %d", c.rate, timeline.DurationMS, c.duration)
		}
		e := timeline.Elements[0]
		// The automatic window is the transformed length less both insets.
		if e.StartMS != l.AutoInsetMS || e.EndMS != c.duration-l.AutoInsetMS {
			t.Fatalf("rate %d placed the caption at %d..%d", c.rate, e.StartMS, e.EndMS)
		}
		// The cut still names the ORIGINAL source milliseconds.
		if cut.StartMS != 5000 || cut.EndMS != 25000 {
			t.Fatal("the source range was rewritten into output time")
		}
	}
}

// A cut whose transformed length is too short for its own transition is refused
// on the transformed value, not on the source span it came from.
func TestTransitionIsMeasuredAgainstTheTransformedLength(t *testing.T) {
	l := clip.DefaultCompositionLimits()
	d, problem := composition.Parse(`<clip version="1" intro="b" caption="bold" outro="e"><scene id="s"><text id="c" kind="fixed" role="caption" basis="cut">x</text></scene><text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`, l)
	if problem != nil {
		t.Fatal(problem)
	}
	cuts := []composition.Cut{
		{ID: "a", SourceID: "v", SectionID: "s", EndMS: 8000},
		// 600 ms of source is 300 ms of output at 2x, shorter than its own fade.
		{ID: "b", SourceID: "v", SectionID: "s", StartMS: 8000, EndMS: 8600, TransitionMS: 400, PlaybackRatePermille: 2000},
	}
	if _, problem := composition.Resolve(d, composition.Inputs{Cuts: cuts}, l, 100000); problem == nil {
		t.Fatal("a transition longer than the transformed cut was accepted")
	}
	cuts[1].PlaybackRatePermille = 500
	if _, problem := composition.Resolve(d, composition.Inputs{Cuts: cuts}, l, 100000); problem != nil {
		t.Fatal("the same source span was refused when it played long enough", problem)
	}
}

func TestTransformedDurationRoundsToTheNearestMillisecond(t *testing.T) {
	for _, c := range []struct {
		span, rate, want int
		ok               bool
	}{
		{1000, composition.RateUnitPermille, 1000, true},
		{20000, 750, 26667, true},
		{3, 2000, 2, true},
		{1, 2000, 1, true},
		{0, 1000, 0, false},
		{1000, 0, 0, false},
		{-5, 1000, 0, false},
	} {
		got, ok := composition.TransformedDurationMS(c.span, c.rate)
		if got != c.want || ok != c.ok {
			t.Fatalf("%d at %d = %d %v", c.span, c.rate, got, ok)
		}
	}
}
