package media

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// measured is a renderer whose face reports one box per queried text: 40 px a
// character at the 100 px measuring size, close enough to the real face that a
// layout's arithmetic can be checked without resvg.
func measured(t *testing.T) (*Adapter, *Rendering) {
	t.Helper()
	a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		data, err := os.ReadFile(c.Args[len(c.Args)-1])
		if err != nil {
			return nil, err
		}
		out := ""
		for i, part := range strings.Split(string(data), `id="m`)[1:] {
			text := part[strings.Index(part, ">")+1 : strings.Index(part, "</text>")]
			out += fmt.Sprintf("m%d,1,320,%d,100\n", i, 40*len([]rune(text)))
		}
		return []byte(out), nil
	}})
	return a, testRenderer(t, a)
}

// The plan the Docker smoke renders, laid out and verified on every ratio. The
// two cards, the badge, the chips and three styles are on screen together, and
// the last cut's copy leaves before the ending card arrives (CDS-45).
func TestTheRenderedPlanVerifiesOnEveryRatio(t *testing.T) {
	a, r := measured(t)
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		// Fifteen-two, not fifteen: the second cut joins with a hard cut and only
		// the third fades, so the transitions take 200 ms off the sum and not
		// 400 (CDS-36).
		plan := clip.EditPlan{Ratio: ratio, DurationMS: 15200, Disclosure: "ad", Preset: "restaurant", Hook: "정확한 한글", Accent: "coral", Facts: []clip.Answer{
			{Label: "상호", Text: "연남 김밥"}, {Label: "위치", Text: "서울 연남동"}, {Label: "가격", Text: "9,900원"},
		}, Cuts: []clip.EditCut{
			{ID: "one", SourceID: "audio", Fingerprint: "audio", EndMS: 5200, Focal: clip.Point{X: .5, Y: .5}, Chips: []string{"위치", "가격"}, Copy: clip.Copy{Text: "정확한 한글 & 여행", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral"}},
			{ID: "two", SourceID: "rotated", Fingerprint: "rotated", EndMS: 5000, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "기록처럼 <오늘>", Style: "memo", Anchor: "lower_mid", Align: "left", Accent: "teal"}},
			{ID: "three", SourceID: "silent", Fingerprint: "silent", EndMS: 5200, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber", StartMS: 120, EndMS: 2400}},
		}}
		canvas, _ := clip.ClipCanvas(ratio)
		if err := a.WithWorkspace(t.Context(), "dry", func(ws clip.MediaWorkspace) error {
			c, err := r.layout(t.Context(), ws, canvas, plan)
			if err != nil {
				return fmt.Errorf("%s layout: %w", ratio, err)
			}
			if err := clip.VerifyLayout(ratio, c.manifest); err != nil {
				for _, e := range c.manifest {
					t.Logf("%s cut%d %-13s %-6s %+v %d..%d", ratio, e.Cut, e.Kind, e.Style, e.Region, e.StartMS, e.EndMS)
				}
				return fmt.Errorf("%s verify: %w", ratio, err)
			}
			// Both cards are in the manifest, and nothing of another layer
			// shows under either.
			cards := 0
			for _, e := range c.manifest {
				if e.Kind == "card" {
					cards++
				}
			}
			if cards != 2 {
				return fmt.Errorf("%s placed %d cards", ratio, cards)
			}
			// A bright ground on the unplated cut adds CDS-32's scrim, turns the
			// accent word white and still verifies — the second pass the render
			// makes after the cuts are drawn (CDS-44, CDS-52).
			l, _ := design.Layout(ratio)
			c.grounds[2] = Luminance{Mean: 0.8, R: 0.9, G: 0.9, B: 0.9, Frames: []float64{0.8}}
			c.resolve(canvas)
			scrim := clip.Manifest{}
			for _, e := range c.manifest {
				if e.Kind == "scrim" {
					scrim = append(scrim, e)
				}
			}
			if len(scrim) != 1 || scrim[0].Cut != 2 || scrim[0].Region != l.ScrimTop {
				return fmt.Errorf("%s scrim %+v", ratio, scrim)
			}
			// It shares the copy's own window, and the copy is now read against
			// the washed ground rather than against nothing.
			start, end := plan.Cuts[2].CaptionWindow()
			offsets := cutOffsets(plan)
			if scrim[0].StartMS != offsets[2]+start || scrim[0].EndMS != offsets[2]+end {
				return fmt.Errorf("%s scrim window %d..%d", ratio, scrim[0].StartMS, scrim[0].EndMS)
			}
			for _, e := range c.manifest {
				if e.Cut == 2 && e.Kind == "copy" && e.Background == "" {
					return fmt.Errorf("%s left a sampled copy without its ground", ratio)
				}
			}
			if err := clip.VerifyLayout(ratio, c.manifest); err != nil {
				return fmt.Errorf("%s after sampling: %w", ratio, err)
			}
			// Resolving twice is the same manifest: a fallback re-measures the
			// plan and replays every ground it already had.
			before := len(c.manifest)
			c.resolve(canvas)
			if len(c.manifest) != before {
				return fmt.Errorf("%s resolve is not idempotent: %d then %d", ratio, before, len(c.manifest))
			}
			return nil
		}); err != nil {
			t.Error(err)
		}
	}
}

// The fallback CDS-44 names, exercised on its own: with today's tokens a stroked
// style always clears V3 (see RENDER.md), so the mechanism is tested here rather
// than through a ground that cannot fail it.
func TestAContrastFallbackPutsTheSentenceBackOnAPlate(t *testing.T) {
	a, r := measured(t)
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Preset: "restaurant", Accent: "coral", Cuts: []clip.EditCut{
		{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "기록처럼 오늘", Style: "memo", Anchor: "top", Align: "left", Accent: "teal"}},
		{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber"}},
	}, Decisions: []clip.Composition{{Class: "EMOTION"}, {Class: "EMOTION"}}}
	canvas, _ := clip.ClipCanvas("vertical")
	if !plan.Compiled() {
		t.Fatal("a plan carrying one decision per cut came from the compiler")
	}
	stored := plan
	stored.Decisions = nil
	if stored.Compiled() {
		t.Fatal("a plan decoded from storage carries no decisions, so a person owns its styles")
	}
	if err := a.WithWorkspace(t.Context(), "fallback", func(ws clip.MediaWorkspace) error {
		c, err := r.layout(t.Context(), ws, canvas, plan)
		if err != nil {
			return err
		}
		c.grounds[1] = Luminance{Mean: 0.95, R: 1, G: 1, B: 1, Frames: []float64{0.95}}
		c.resolve(canvas)
		if err := r.fallback(t.Context(), ws, canvas, &c, 1); err != nil {
			return err
		}
		// 깔끔하게 at its own anchor, because CDS-24 does not let it stand where
		// 크게 강조 stood and V6 would refuse the copy there.
		got := c.plan.Cuts[1].Copy
		if got.Style != "clean" || got.Anchor != design.Styles["clean"].Anchor || got.Align != design.Styles["clean"].Align {
			return fmt.Errorf("fallback placed %+v", got)
		}
		if c.plan.Decisions[1].Fallback != "contrast" {
			return fmt.Errorf("the decision does not say why: %+v", c.plan.Decisions[1])
		}
		// The plate needs no ground, so the sample is no longer part of what the
		// cut draws — and no scrim is left behind on a plated style (CDS-32).
		if c.grounds[1].Sampled() {
			return fmt.Errorf("kept a ground a plate does not read: %+v", c.grounds[1])
		}
		for _, e := range c.manifest {
			if e.Kind == "scrim" {
				return fmt.Errorf("a plated style kept a scrim: %+v", e)
			}
		}
		// The caller's own plan is untouched: the cuts are cloned, never patched
		// through the slice the caller still holds.
		if plan.Cuts[1].Copy.Style != "bold" {
			t.Fatal("the caller's plan was mutated")
		}
		return clip.VerifyLayout("vertical", c.manifest)
	}); err != nil {
		t.Fatal(err)
	}
}
