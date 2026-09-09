package clip_test

import (
	"math"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func validPlan() (clip.EditPlan, []clip.RenderSource) {
	s := clip.RenderSource{ID: "source", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 20000, Width: 1920, Height: 1080}}
	c := clip.EditCut{ID: "one", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: 7600, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Text: "Hello", Position: "bottom", Style: "clean"}}
	d := c
	d.ID = "two"
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
		"NaN focal": func(p *clip.EditPlan) { p.Cuts[0].Focal.X = math.NaN() }, "outside focal": func(p *clip.EditPlan) { p.Cuts[0].Focal.Y = 1.1 }, "free position": func(p *clip.EditPlan) { p.Cuts[0].Copy.Position = "x=10" }, "free style": func(p *clip.EditPlan) { p.Cuts[0].Copy.Style = "animated" }, "free accent": func(p *clip.EditPlan) { p.Cuts[0].Copy.Accent = "#123456" },
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
func TestSafeCopyPlacement(t *testing.T) {
	for ratio, size := range map[string][2]int{"vertical": {1080, 1920}, "horizontal": {1920, 1080}, "square": {1080, 1080}} {
		canvas, err := clip.ClipCanvas(ratio)
		if err != nil {
			t.Fatal(err)
		}
		if canvas.Width != size[0] || canvas.Height != size[1] {
			t.Fatal(canvas)
		}
		for _, position := range []string{"top", "center", "bottom"} {
			r, err := clip.PlaceCopy(canvas, position, 600, 150)
			if err != nil {
				t.Fatal(err)
			}
			if r.X < canvas.Safe.X || r.Y < canvas.Safe.Y || r.Y+r.Height > canvas.Safe.Y+canvas.Safe.Height {
				t.Fatal(r)
			}
		}
		for _, args := range []struct {
			p    string
			w, h float64
		}{{"free", 1, 1}, {"top", canvas.Safe.Width + 1, 20}, {"top", 50, math.NaN()}, {"top", 50, math.Inf(1)}, {"top", 0, 20}} {
			if _, err := clip.PlaceCopy(canvas, args.p, args.w, args.h); err == nil {
				t.Fatal("unsafe placement")
			}
		}
		pos, err := clip.PickCopyPosition(canvas, "bottom", 600, 150, clip.Region{X: 0, Y: .6, Width: 1, Height: .4})
		if err != nil || pos != "top" {
			t.Fatalf("position=%s err=%v", pos, err)
		}
	}
}
func volume(v float64) *float64 { return &v }
func TestOriginalVolumeDefaultsAndExplicitMute(t *testing.T) {
	if (clip.EditCut{}).OriginalVolume() != 1 || (clip.EditCut{Volume: volume(0)}).OriginalVolume() != 0 {
		t.Fatal("original audio default or explicit mute changed")
	}
}
