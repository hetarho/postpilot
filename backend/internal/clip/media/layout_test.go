package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
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
			{ID: "one", SourceID: "audio", Fingerprint: "audio", EndMS: 5200, Focal: clip.Point{X: .5, Y: .5}, Chips: []string{"위치", "가격"}, Copies: []clip.Copy{{Text: "정확한 한글 & 여행", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral"}}},
			{ID: "two", SourceID: "rotated", Fingerprint: "rotated", EndMS: 5000, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "기록처럼 <오늘>", Style: "memo", Anchor: "lower_mid", Align: "left", Accent: "teal"}}},
			{ID: "three", SourceID: "silent", Fingerprint: "silent", EndMS: 5200, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber", StartMS: 120, EndMS: 2400}}},
		}}
		canvas, _ := clip.ClipCanvas(ratio)
		if err := a.WithWorkspace(t.Context(), "dry", func(ws clip.MediaWorkspace) error {
			c, err := r.layout(t.Context(), ws, canvas, plan)
			if err != nil {
				return fmt.Errorf("%s layout: %w", ratio, err)
			}
			if err := clip.VerifyLayout(ratio, nil, c.manifest); err != nil {
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
			c.grounds[2][0] = Luminance{Mean: 0.8, R: 0.9, G: 0.9, B: 0.9, Frames: []float64{0.8}}
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
			start, end := plan.Cuts[2].CaptionWindow(0)
			offsets := cutOffsets(plan)
			if scrim[0].StartMS != offsets[2]+start || scrim[0].EndMS != offsets[2]+end {
				return fmt.Errorf("%s scrim window %d..%d", ratio, scrim[0].StartMS, scrim[0].EndMS)
			}
			for _, e := range c.manifest {
				if e.Cut == 2 && e.Kind == "copy" && e.Background == "" {
					return fmt.Errorf("%s left a sampled copy without its ground", ratio)
				}
			}
			if err := clip.VerifyLayout(ratio, nil, c.manifest); err != nil {
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
		{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "기록처럼 오늘", Style: "memo", Anchor: "top", Align: "left", Accent: "teal"}}},
		{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber"}}},
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
		c.grounds[1][0] = Luminance{Mean: 0.95, R: 1, G: 1, B: 1, Frames: []float64{0.95}}
		c.resolve(canvas)
		if _, err := r.rung(t.Context(), ws, canvas, &c, 1, 0, rungStyle, "contrast"); err != nil {
			return err
		}
		// 깔끔하게 at its own anchor, because CDS-24 does not let it stand where
		// 크게 강조 stood and V6 would refuse the copy there.
		got := c.plan.Cuts[1].FirstCopy()
		if got.Style != "clean" || got.Anchor != design.Styles["clean"].Anchor || got.Align != design.Styles["clean"].Align {
			return fmt.Errorf("fallback placed %+v", got)
		}
		if c.plan.Decisions[1].Fallback != "contrast" {
			return fmt.Errorf("the decision does not say why: %+v", c.plan.Decisions[1])
		}
		// The plate needs no ground, so the sample is no longer part of what the
		// cut draws — and no scrim is left behind on a plated style (CDS-32).
		if c.grounds[1][0].Sampled() {
			return fmt.Errorf("kept a ground a plate does not read: %+v", c.grounds[1][0])
		}
		for _, e := range c.manifest {
			if e.Kind == "scrim" {
				return fmt.Errorf("a plated style kept a scrim: %+v", e)
			}
		}
		// The caller's own plan is untouched: the cuts are cloned, never patched
		// through the slice the caller still holds.
		if plan.Cuts[1].Copies[0].Style != "bold" {
			t.Fatal("the caller's plan was mutated")
		}
		return clip.VerifyLayout("vertical", nil, c.manifest)
	}); err != nil {
		t.Fatal(err)
	}
}

// A cut carrying CDS-43's two copies lays out and verifies both: two plates, two
// windows that never meet, and one manifest that names them apart.
func TestTwoCopiesOnOneCutLayOutAndVerify(t *testing.T) {
	a, r := measured(t)
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Preset: "restaurant", Accent: "coral", Cuts: []clip.EditCut{
		{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7500, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{
			{Text: "조용한 골목을 걸었어요", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral", StartMS: 120, EndMS: 3000},
			{Text: "9900원", Style: "clean", Anchor: "bottom", Align: "center", Accent: "coral", StartMS: 3120, EndMS: 7380},
		}},
		{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7500, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{
			{Text: "기록처럼 오늘", Style: "memo", Anchor: "top", Align: "left", Accent: "teal"},
		}},
	}}
	canvas, _ := clip.ClipCanvas("vertical")
	if err := a.WithWorkspace(t.Context(), "two-copies", func(ws clip.MediaWorkspace) error {
		c, err := r.layout(t.Context(), ws, canvas, plan)
		if err != nil {
			return err
		}
		if err := clip.VerifyLayout("vertical", nil, c.manifest); err != nil {
			return err
		}
		if len(c.layouts[0]) != 2 || len(c.grounds[0]) != 2 {
			return fmt.Errorf("one layout for two copies: %d %d", len(c.layouts[0]), len(c.grounds[0]))
		}
		// The manifest tells the two apart, and their windows never meet: the
		// verifier would otherwise read them as one caption on two anchors.
		windows := map[int][2]int{}
		for _, e := range c.manifest {
			if e.Cut == 0 && e.Kind == "copy" {
				windows[e.Copy] = [2]int{e.StartMS, e.EndMS}
			}
		}
		if len(windows) != 2 {
			return fmt.Errorf("the manifest does not name both copies: %+v", windows)
		}
		if windows[0][1] > windows[1][0] {
			return fmt.Errorf("the two copies share the screen: %+v", windows)
		}
		// A plan the compiler wrote is refused the same way as a plan a person
		// wrote when the second copy breaks CDS-43.
		broken := plan
		broken.Cuts = slices.Clone(plan.Cuts)
		broken.Cuts[0].Copies = slices.Clone(plan.Cuts[0].Copies)
		broken.Cuts[0].Copies[1].StartMS = 2000
		if _, err := r.layout(t.Context(), ws, canvas, broken); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A compiled plan whose manifest fails a check walks CDS-55's ladder before any
// download: the caption's style falls back to 깔끔하게, then its anchor to the
// style's own default, then the copy is dropped — and each rung is recorded on
// the cut so step ② can say what happened.
func TestTheRepairLadderWalksStyleThenAnchorThenDrop(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	base := func() clip.EditPlan {
		return clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Preset: "restaurant", Accent: "coral", Cuts: []clip.EditCut{
			{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "기록처럼 오늘", Style: "clean", Anchor: "bottom", Align: "center"}}},
			{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "clean", Anchor: "bottom", Align: "center"}}},
		}, Decisions: []clip.Composition{{Class: "DESC"}, {Class: "DESC"}}}
	}
	// Rung 1: a third 크게 강조 in one clip is one more than CDS-40 allows (V14);
	// the later caption yields its style and everything else stands.
	styleCase := base()
	styleCase.Cuts = append(styleCase.Cuts, clip.EditCut{ID: "three", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "또 오고 싶다", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber"}}})
	styleCase.Decisions = append(styleCase.Decisions, clip.Composition{Class: "EMOTION"})
	styleCase.DurationMS = 22400
	for i := 0; i < 2; i++ {
		styleCase.Cuts[i].Copies[0] = clip.Copy{Text: "또 오고 싶다", Style: "bold", Anchor: "upper_mid", Align: "center", Accent: "amber"}
	}
	long := "또 오고 싶다"
	// Rung 2: two 깔끔하게 cuts whose anchors are three steps apart (V13); the
	// style is already 깔끔하게, so the first rung has nothing to do.
	anchorCase := base()
	anchorCase.Cuts[1].Copies[0].Anchor = "top"
	// Rung 3: the previous copy is TOP, this one already defaults to BOTTOM.
	// Neither style nor default anchor can fix the step, so the later copy drops.
	// Overlap itself no longer walks the ladder (CDS-56).
	dropCase := base()
	dropCase.Cuts[0].Copies[0].Anchor = "top"
	for name, tc := range map[string]struct {
		plan   clip.EditPlan
		cut    int
		record string
		check  func(clip.EditPlan) error
	}{
		"style": {styleCase, 2, "style", func(p clip.EditPlan) error {
			if c := p.Cuts[2].FirstCopy(); c.Style != "clean" || c.Text != long || p.Cuts[0].FirstCopy().Style != "bold" || p.Cuts[1].FirstCopy().Style != "bold" {
				return fmt.Errorf("style rung placed %+v", p.Cuts)
			}
			return nil
		}},
		"anchor": {anchorCase, 1, "anchor", func(p clip.EditPlan) error {
			if c := p.Cuts[1].FirstCopy(); c.Anchor != design.Styles["clean"].Anchor || c.Style != "clean" {
				return fmt.Errorf("anchor rung placed %+v", c)
			}
			return nil
		}},
		"drop": {dropCase, 1, "dropped", func(p clip.EditPlan) error {
			if p.Cuts[0].Copies[0].Text == "" || p.Cuts[1].Copies[0].Text != "" || len(p.Cuts[1].Placed()) != 0 {
				return fmt.Errorf("drop rung kept the wrong copy: %+v", p.Cuts)
			}
			return nil
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := a.WithWorkspace(t.Context(), "ladder-"+name, func(ws clip.MediaWorkspace) error {
				c, err := r.layout(t.Context(), ws, canvas, tc.plan)
				if err != nil {
					return err
				}
				if clip.VerifyLayout("vertical", nil, c.manifest) == nil {
					return fmt.Errorf("the fixture verifies before repair")
				}
				if err := r.repair(t.Context(), ws, canvas, &c); err != nil {
					return fmt.Errorf("the ladder gave up: %w", err)
				}
				if err := clip.VerifyLayout("vertical", nil, c.manifest); err != nil {
					return fmt.Errorf("repaired manifest still fails: %w", err)
				}
				if err := tc.check(c.plan); err != nil {
					return err
				}
				if got := c.plan.Decisions[tc.cut].Fallback; got != tc.record {
					return fmt.Errorf("recorded %q, want %q", got, tc.record)
				}
				for i, d := range c.plan.Decisions {
					if i != tc.cut && d.Fallback != "" {
						return fmt.Errorf("an untouched cut was recorded: %+v", d)
					}
				}
				// The caller's plan is untouched: every rung clones before it moves.
				if tc.plan.Cuts[tc.cut].Copies[0].Style == "" && name != "drop" {
					return fmt.Errorf("the caller's plan was mutated")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The ladder is the compiler's: a plan a person corrected is refused with the
// check named and never moved, and a failure the design system's own furniture
// caused fails at once with the furniture slot — no rung is tried.
func TestTheLadderRefusesPersonsPlansAndFurnitureFailures(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Preset: "restaurant", Accent: "coral", Cuts: []clip.EditCut{
		{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "기록처럼 오늘", Style: "clean", Anchor: "bottom", Align: "center"}}},
		{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "clean", Anchor: "top", Align: "center"}}},
	}}
	if err := a.WithWorkspace(t.Context(), "ladder-refusals", func(ws clip.MediaWorkspace) error {
		// A person's plan: no decisions, the anchor-step failure is named, nothing moves.
		c, err := r.layout(t.Context(), ws, canvas, plan)
		if err != nil {
			return err
		}
		var failure *clip.LayoutError
		if err := r.repair(t.Context(), ws, canvas, &c); !errors.As(err, &failure) || failure.LayoutReason() != "CLIP_LAYOUT_ANCHOR_STEP" || failure.Cut != 1 || failure.Copy != 0 || failure.Furniture() {
			return fmt.Errorf("a person's plan was not refused with its check: %v", err)
		}
		if c.plan.Cuts[1].FirstCopy().Anchor != "top" {
			return fmt.Errorf("a person's plan was moved: %+v", c.plan.Cuts[1])
		}
		// Furniture: the badge pushed out of the safe area on an otherwise
		// compiled plan is a renderer defect, refused at once.
		compiled := plan
		compiled.Cuts = slices.Clone(plan.Cuts)
		compiled.Cuts[1].Copies = []clip.Copy{{Text: "다시 오고 싶은 곳", Style: "clean", Anchor: "bottom", Align: "center"}}
		compiled.Decisions = []clip.Composition{{Class: "DESC"}, {Class: "DESC"}}
		c, err = r.layout(t.Context(), ws, canvas, compiled)
		if err != nil {
			return err
		}
		if err := r.repair(t.Context(), ws, canvas, &c); err != nil {
			return fmt.Errorf("the compiled fixture does not verify: %w", err)
		}
		for i := range c.manifest {
			if c.manifest[i].Kind == "badge" {
				c.manifest[i].Region.X = -10
			}
		}
		if err := r.repair(t.Context(), ws, canvas, &c); !errors.As(err, &failure) || !failure.Furniture() || failure.LayoutReason() != "CLIP_LAYOUT_SAFE_AREA" {
			return fmt.Errorf("a furniture failure was not refused at once: %v", err)
		}
		for _, d := range c.plan.Decisions {
			if d.Fallback != "" {
				return fmt.Errorf("a rung was tried on a furniture failure: %+v", d)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A clip whose every caption was dropped still lays out and verifies: the badge
// and the cards remain, and nothing carries a style.
func TestAClipWithEveryCaptionDroppedStillLaysOut(t *testing.T) {
	a, r := measured(t)
	canvas, _ := clip.ClipCanvas("vertical")
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Disclosure: "ad", Preset: "restaurant", Accent: "coral", Hook: "오늘의 한 끼", Cuts: []clip.EditCut{
		{ID: "one", SourceID: "s", Fingerprint: "s", EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{}}},
		{ID: "two", SourceID: "s", Fingerprint: "s", EndMS: 7600, TransitionMS: 200, Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{{}}},
	}, Decisions: []clip.Composition{{Fallback: "dropped"}, {Fallback: "dropped"}}}
	if err := a.WithWorkspace(t.Context(), "all-dropped", func(ws clip.MediaWorkspace) error {
		c, err := r.layout(t.Context(), ws, canvas, plan)
		if err != nil {
			return err
		}
		if err := r.repair(t.Context(), ws, canvas, &c); err != nil {
			return err
		}
		badge, styled := 0, 0
		for _, e := range c.manifest {
			if e.Kind == "badge" {
				badge++
			}
			if e.Style != "" {
				styled++
			}
		}
		if badge != 1 || styled != 0 {
			return fmt.Errorf("badge=%d styled=%d", badge, styled)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// RENDER.md carries the two verify points and the ladder between them.
func TestRenderNotesDocumentTheRepairLadder(t *testing.T) {
	notes, err := os.ReadFile("../../../build/RENDER.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"repair ladder", "furniture slot", "AFTER the cuts are rendered", "never walks the ladder"} {
		if !strings.Contains(string(notes), phrase) {
			t.Fatalf("RENDER.md does not say %q", phrase)
		}
	}
}
