package clip_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

func TestRapidPhraseTimingAndLosslessFallback(t *testing.T) {
	seed := clip.Caption{Text: "오늘은 구로디지털단지에 와보았는데요", Style: "simple", Anchor: "bottom", Align: "center"}
	copies, ok := clip.SplitRapid(seed, 120, 1420)
	if !ok || len(copies) != 3 {
		t.Fatal(copies, ok)
	}
	for i, want := range []struct {
		text       string
		start, end int
	}{{"오늘은", 120, 420}, {"구로디지털단지에", 420, 920}, {"와보았는데요", 920, 1420}} {
		if c := copies[i]; c.Text != want.text || c.StartMS != want.start || c.EndMS != want.end || c.Pace != "rapid" {
			t.Fatal(c)
		}
	}
	compressed, ok := clip.SplitRapid(seed, 0, 1000)
	if !ok || compressed[2].EndMS > 1000 {
		t.Fatal(compressed)
	}
	for _, c := range compressed {
		if c.EndMS-c.StartMS < 300 {
			t.Fatal(c)
		}
	}
	if _, ok := clip.SplitRapid(seed, 0, 899); ok {
		t.Fatal("truncated text to fit")
	}
	if got := clip.RapidPhrases("오늘은 철판 요리를 먹어봤어요"); !reflect.DeepEqual(got, []string{"오늘은", "철판 요리를", "먹어봤어요"}) {
		t.Fatal(got)
	}
	long := strings.Repeat("가", 30)
	if got := clip.RapidPhrases(long); strings.Join(got, "") != long || len(got) != 3 {
		t.Fatal(got)
	}
	canvas, _ := clip.ClipCanvas("vertical")
	fit := func(clip.Caption) (clip.Region, bool, error) {
		return clip.Region{X: 100, Y: 1270, Width: 400, Height: 100}, true, nil
	}
	cut := clip.Cut{ID: "one", EndMS: 1540, Focal: clip.Point{X: .5, Y: .5}}
	got, _, err := clip.Compose(canvas, cut, clip.Written{Text: seed.Text, Pace: "rapid"}, "food", false, clip.Region{}, nil, []string{"clean", "simple"}, "", nil, "", 1540, fit)
	if err != nil || !reflect.DeepEqual(got.Copies, copies) {
		t.Fatal(got, err)
	}
	// No room for every phrase: the existing short grounded wording is used.
	cut.EndMS = 740
	got, _, err = clip.Compose(canvas, cut, clip.Written{Text: seed.Text, ShortText: "오늘은", Pace: "rapid"}, "food", false, clip.Region{}, nil, []string{"clean", "simple"}, "", nil, "", 740, fit)
	if err != nil || len(got.Copies) != 1 || got.FirstCopy().Text != "오늘은" || !got.Rapid() {
		t.Fatal(got, err)
	}
}

func TestRapidCorrectionRoundtripAndBoundaries(t *testing.T) {
	p, draft := correctionFixture(t)
	copies, _ := clip.SplitRapid(clip.Caption{Text: "오늘은 구로디지털단지에 와보았는데요", Style: "clean", Anchor: "bottom", Align: "center"}, 120, 1420)
	draft.Cuts[0].Copies = copies
	cfg := config.ClipRender(&config.Config{})
	next, styles, err := clip.ApplyCorrection(cfg, p, draft)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := clip.EncodeEditPlan(next, styles)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := clip.DecodeEditPlan(raw)
	if err != nil || !reflect.DeepEqual(again.Cuts[0].Copies, copies) {
		t.Fatal(again, err)
	}
	for name, mutate := range map[string]func([]clip.Caption){
		"overlap":   func(c []clip.Caption) { c[1].StartMS = 419 },
		"too short": func(c []clip.Caption) { c[0].EndMS = 419 },
		"too long":  func(c []clip.Caption) { c[2].EndMS = c[2].StartMS + 1001 },
		"implicit":  func(c []clip.Caption) { c[0].StartMS = 0; c[0].EndMS = 0 },
		"mixed":     func(c []clip.Caption) { c[0].Pace = "steady" },
		"unknown":   func(c []clip.Caption) { c[0].Pace = "fast" },
		"empty":     func(c []clip.Caption) { c[0].Text = "" },
		"two lines": func(c []clip.Caption) { c[0].Text = "오늘은\n여기로" },
	} {
		t.Run(name, func(t *testing.T) {
			d := draft
			d.Cuts = append([]clip.CorrectionCut(nil), draft.Cuts...)
			d.Cuts[0].Copies = append([]clip.Caption(nil), copies...)
			mutate(d.Cuts[0].Copies)
			if _, _, err := clip.ApplyCorrection(cfg, p, d); err == nil {
				t.Fatal("invalid rapid cue accepted")
			}
		})
	}
}
