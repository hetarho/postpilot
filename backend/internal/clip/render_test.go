package clip_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/platform/config"
)

func validPlan() (clip.EditPlan, []clip.RenderSource) {
	s := clip.RenderSource{ID: "source", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 20000, Width: 1920, Height: 1080}}
	c := clip.EditCut{ID: "one", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "Hello", Anchor: "bottom", Align: "center", Style: "clean"}}
	d := c
	d.ID = "two"
	// The scene changes between the two, so the second leads in with CDS-36's
	// fade and the clip is 200 ms shorter than the sum of its cuts.
	d.TransitionMS = 200
	return clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.EditCut{c, d}}, []clip.RenderSource{s}
}
func TestValidateEditPlan(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	p, s := validPlan()
	if err := clip.ValidateEditPlan(cfg, p, s); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*clip.EditPlan){
		"unknown ratio": func(p *clip.EditPlan) { p.Ratio = "2:3" }, "too short": func(p *clip.EditPlan) { p.DurationMS = 14999 }, "too long": func(p *clip.EditPlan) { p.DurationMS = 90001 }, "empty": func(p *clip.EditPlan) { p.Cuts = nil },
		"missing fade accounting": func(p *clip.EditPlan) { p.DurationMS = 15200 }, "foreign source": func(p *clip.EditPlan) { p.Cuts[0].SourceID = "foreign" }, "changed bytes": func(p *clip.EditPlan) { p.Cuts[0].Fingerprint = "changed" }, "duplicate cut": func(p *clip.EditPlan) { p.Cuts[1].ID = p.Cuts[0].ID },
		"before source": func(p *clip.EditPlan) { p.Cuts[0].StartMS = -1 }, "past source": func(p *clip.EditPlan) { p.Cuts[0].EndMS = 20001 }, "short fade": func(p *clip.EditPlan) { p.Cuts[0].EndMS = 200 }, "NaN volume": func(p *clip.EditPlan) { p.Cuts[0].Volume = volume(math.NaN()) }, "loud": func(p *clip.EditPlan) { p.Cuts[0].Volume = volume(1.001) }, "negative volume": func(p *clip.EditPlan) { p.Cuts[0].Volume = volume(-.1) },
		"NaN focal": func(p *clip.EditPlan) { p.Cuts[0].Focal.X = math.NaN() }, "outside focal": func(p *clip.EditPlan) { p.Cuts[0].Focal.Y = 1.1 }, "free anchor": func(p *clip.EditPlan) { p.Cuts[0].Copy.Anchor = "x=10" }, "free align": func(p *clip.EditPlan) { p.Cuts[0].Copy.Align = "justify" }, "free style": func(p *clip.EditPlan) { p.Cuts[0].Copy.Style = "animated" }, "free accent": func(p *clip.EditPlan) { p.Cuts[0].Copy.Accent = "#123456" },
		// CDS-36 admits three transitions and the first cut takes none.
		"invented transition": func(p *clip.EditPlan) { p.Cuts[1].TransitionMS = 150 }, "fades in from nothing": func(p *clip.EditPlan) { p.Cuts[0].TransitionMS = 200 },
	} {
		t.Run(name, func(t *testing.T) {
			p, s := validPlan()
			mutate(&p)
			if err := clip.ValidateEditPlan(cfg, p, s); err == nil {
				t.Fatal("accepted invalid plan")
			}
		})
	}
	p, s = validPlan()
	p.Cuts[0].Copy.Text = strings.Repeat("가", 501)
	if err := clip.ValidateEditPlan(cfg, p, s); err != clip.ErrCopyTooLong {
		t.Fatal(err)
	}
}

// CDS-20 and CDS-23..26 bound the lines and characters per style, and CDS-41 the
// exposure a copy of that length earns. Each case names the code it must return.
func TestCopyLimitsAndExposurePerStyle(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	for name, tc := range map[string]struct {
		style, text string
		start, end  int
		code        string
	}{
		// 깔끔하게 takes two lines of fourteen, 메모 exactly one of eighteen,
		// 크게 강조 two of eleven and 형광펜 one of sixteen.
		"clean two lines":      {"clean", strings.Repeat("가", 14) + "\n" + strings.Repeat("나", 14), 0, 7600, ""},
		"clean third line":     {"clean", "가\n나\n다", 0, 7600, "plan_copy_lines"},
		"clean fifteenth char": {"clean", strings.Repeat("가", 15), 0, 7600, "plan_copy_chars"},
		"memo eighteen":        {"memo", strings.Repeat("가", 18), 0, 7600, ""},
		"memo second line":     {"memo", "가\n나", 0, 7600, "plan_copy_lines"},
		"memo nineteen":        {"memo", strings.Repeat("가", 19), 0, 7600, "plan_copy_chars"},
		"bold eleven":          {"bold", strings.Repeat("가", 11), 0, 7600, ""},
		"bold twelfth char":    {"bold", strings.Repeat("가", 12), 0, 7600, "plan_copy_chars"},
		"mark sixteen":         {"mark", strings.Repeat("가", 16), 0, 7600, ""},
		"mark seventeenth":     {"mark", strings.Repeat("가", 17), 0, 7600, "plan_copy_chars"},
		"mark second line":     {"mark", "가\n나", 0, 7600, "plan_copy_lines"},
		// Five characters earn 900 + 5 × 90 ms, and neither the space nor the
		// punctuation counts toward either the limit or the exposure.
		"exposure met":     {"clean", "여섯 글자다", 0, 1350, ""},
		"exposure short":   {"clean", "여섯 글자다", 0, 1349, "plan_copy_exposure"},
		"punctuation free": {"clean", "여섯 글자다!!!!!!!!", 0, 1350, ""},
		// An empty copy is a cut with no text, not a copy that breaks the limits.
		"no copy": {"clean", "", 0, 0, ""},
	} {
		t.Run(name, func(t *testing.T) {
			p, s := validPlan()
			p.Cuts[0].Copy.Style, p.Cuts[0].Copy.Text = tc.style, tc.text
			p.Cuts[0].Copy.StartMS, p.Cuts[0].Copy.EndMS = tc.start, tc.end
			err := clip.ValidateEditPlan(cfg, p, s)
			if tc.code == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var diagnostic interface{ OutputValidationCode() string }
			if !errors.As(err, &diagnostic) || diagnostic.OutputValidationCode() != tc.code {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
	if clip.MinExposureMS("여섯 글자다") != 900+90*5 || clip.CopyChars("여섯 글자다 !?.,") != 5 {
		t.Fatal("CDS-41 counts spaces or punctuation")
	}
}

// Every anchor × alignment on every ratio, each resolved by CDS-12 (9:16),
// CDS-47 (16:9) and CDS-48 (1:1) and each inside its own safe area.
func TestSafeCopyPlacement(t *testing.T) {
	for ratio, size := range map[string][2]int{"vertical": {1080, 1920}, "horizontal": {1920, 1080}, "square": {1080, 1080}} {
		canvas, err := clip.ClipCanvas(ratio)
		if err != nil {
			t.Fatal(err)
		}
		if canvas.Width != size[0] || canvas.Height != size[1] {
			t.Fatal(canvas)
		}
		safe, _ := design.Safe(ratio)
		if canvas.Safe != clip.Region(safe) {
			t.Fatalf("%s safe area %+v", ratio, canvas.Safe)
		}
		a := canvas.Anchor
		const w, h = 600.0, 150.0
		for _, anchor := range clip.CopyAnchors {
			for _, align := range clip.CopyAligns {
				r, err := clip.PlaceCopy(canvas, anchor, align, w, h)
				if err != nil {
					t.Fatalf("%s %s/%s: %v", ratio, anchor, align, err)
				}
				if r.X < canvas.Safe.X || r.Y < canvas.Safe.Y || r.X+r.Width > canvas.Safe.X+canvas.Safe.Width || r.Y+r.Height > canvas.Safe.Y+canvas.Safe.Height {
					t.Fatalf("%s %s/%s left the safe area: %+v", ratio, anchor, align, r)
				}
				// TOP is the plate's top edge, BOTTOM its bottom, the MIDs its centre.
				want := map[string]float64{"top": r.Y, "bottom": r.Y + h, "upper_mid": r.Y + h/2, "lower_mid": r.Y + h/2}[anchor]
				at := map[string]float64{"top": a.Top, "bottom": a.Bottom, "upper_mid": a.UpperMid, "lower_mid": a.LowerMid}[anchor]
				if want != at {
					t.Fatalf("%s %s: plate at %v, anchor at %v", ratio, anchor, want, at)
				}
				// LEFT starts at the anchor, RIGHT ends at it, CENTER centres on it.
				wantX := map[string]float64{"left": r.X, "center": r.X + w/2, "right": r.X + w}[align]
				atX := map[string]float64{"left": a.Left, "center": a.Center, "right": a.Right}[align]
				if wantX != atX {
					t.Fatalf("%s %s: plate at %v, anchor at %v", ratio, align, wantX, atX)
				}
			}
		}
		for _, args := range []struct {
			anchor, align string
			w, h          float64
		}{
			{"free", "center", 1, 1}, {"top", "justify", 1, 1}, {"center", "center", 1, 1},
			{"top", "left", canvas.Safe.Width + 1, 20}, {"top", "center", 50, math.NaN()},
			{"top", "center", 50, math.Inf(1)}, {"top", "center", 0, 20},
			// Nothing may leave the safe area by one pixel (CDS-2), and 9:16's is
			// deliberately off-centre, so a full-width centred plate misses it.
			{"bottom", "center", canvas.Safe.Width, 20},
		} {
			if ratio != "vertical" && args.w == canvas.Safe.Width {
				continue // Only 9:16's safe area is asymmetric around the canvas centre.
			}
			if _, err := clip.PlaceCopy(canvas, args.anchor, args.align, args.w, args.h); err == nil {
				t.Fatalf("%s %s/%s %vx%v: unsafe placement", ratio, args.anchor, args.align, args.w, args.h)
			}
		}
		anchor, err := clip.PickCopyAnchor(canvas, "bottom", "center", w, h, clip.Region{X: 0, Y: .6, Width: 1, Height: .4})
		if err != nil || anchor != "upper_mid" {
			t.Fatalf("anchor=%s err=%v", anchor, err)
		}
	}
}
func volume(v float64) *float64 { return &v }
func TestOriginalVolumeDefaultsAndExplicitMute(t *testing.T) {
	if (clip.EditCut{}).OriginalVolume() != 1 || (clip.EditCut{Volume: volume(0)}).OriginalVolume() != 0 {
		t.Fatal("original audio default or explicit mute changed")
	}
}

// The plan's duration is the footage it selected less what its transitions
// overlap — per cut, not one fade times the boundaries (CDS-36).
func TestDurationArithmeticWithMixedTransitions(t *testing.T) {
	cfg := config.ClipRender(&config.Config{})
	for name, transitions := range map[string][]int{
		"every boundary hard":  {0, 0, 0},
		"every boundary fades": {0, 200, 200},
		"mixed":                {0, 0, 200},
		"through black":        {0, 300, 0},
	} {
		t.Run(name, func(t *testing.T) {
			p, s := validPlan()
			third := p.Cuts[1]
			third.ID = "three"
			p.Cuts = append(p.Cuts, third)
			total := 0
			for i := range p.Cuts {
				p.Cuts[i].TransitionMS = transitions[i]
				total += p.Cuts[i].EndMS - p.Cuts[i].StartMS
			}
			if p.TransitionTotal() != transitions[1]+transitions[2] {
				t.Fatal("transition total", p.TransitionTotal())
			}
			p.DurationMS = total - p.TransitionTotal()
			if err := clip.ValidateEditPlan(cfg, p, s); err != nil {
				t.Fatal(err)
			}
			// One millisecond either way is a timeline the renderer cannot make.
			for _, off := range []int{-1, 1} {
				wrong := p
				wrong.DurationMS += off
				if err := clip.ValidateEditPlan(cfg, wrong, s); err == nil {
					t.Fatal("accepted a duration the cuts do not add up to")
				}
			}
		})
	}
}
