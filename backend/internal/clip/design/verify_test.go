package design_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// A CDS-conformant manifest: two cuts, one anchor step apart, each a plated
// 깔끔하게 line inside the 9:16 safe area at the body size.
func conformant() design.Manifest {
	// Every clip carries its disclosure badge for the whole clip (CDS-5), at the
	// ratio's badge box with its right edge at x 888 and its top at y 270.
	m := design.Manifest{{
		Kind: "badge", Text: design.Disclosure["ad"], FontSize: design.Type["badge"].Size,
		Background: design.Color["badge_ad"].Hex, Region: design.Region{X: 768, Y: 270, Width: 120, Height: 60},
		StartMS: 0, EndMS: 10120, // the clip's own end: cut 1 runs 7120..10120
	}}
	for i, anchor := range []string{"bottom", "lower_mid"} {
		y := 1270.0
		if anchor == "lower_mid" {
			y = 1045
		}
		start := 120 + i*7000
		for _, e := range []design.Element{
			{Kind: "plate", Region: design.Region{X: 300, Y: y, Width: 400, Height: 110}},
			{Kind: "bar", Region: design.Region{X: 300, Y: y, Width: 8, Height: 110}},
			{Kind: "copy", Region: design.Region{X: 340, Y: y + 22, Width: 320, Height: 66}, FontSize: 56, Text: "열네 글자까지"},
		} {
			e.Cut, e.Style, e.Anchor = i, "clean", anchor
			e.StartMS, e.EndMS = start, start+3000
			e.InMS, e.OutMS, e.DY = design.Motion.InMS, design.Motion.OutMS, design.Motion.InDY
			m = append(m, e)
		}
	}
	return m
}

// The hook card over the first cut: its plate, its category chip and its two
// lines, none of which carries a copy style — the design system typesets them
// itself (CDS-28).
func card() design.Manifest {
	m := design.Manifest{{
		Cut: 0, Kind: "card", Text: "hook", Region: design.Region{X: 144, Y: 640, Width: 792, Height: 300},
		StartMS: 0, EndMS: 1500, Background: design.Color["ink_900s"].Hex,
	}}
	for i, e := range []design.Element{
		{Kind: "chip-category", Text: "카페", FontSize: design.Type["label"].Size, Fill: design.Color["ink_900"].Hex, Background: design.Accent["coral"]},
		{Kind: "copy", Text: "갓 구운 빵", FontSize: design.Type["hook"].Size, Fill: design.Color["text_white"].Hex, Background: design.Color["ink_900s"].Hex},
		{Kind: "copy", Text: "해람 베이커리", FontSize: design.Type["body"].Size, Fill: design.Color["text_muted"].Hex, Background: design.Color["ink_900s"].Hex},
	} {
		e.Cut, e.StartMS, e.EndMS = 0, 0, 1500
		e.Region = design.Region{X: 184, Y: 680 + float64(i)*80, Width: 400, Height: 60}
		m = append(m, e)
	}
	return m
}

// The scrim under an unplated copy: full-bleed by CDS-32, riding the copy's own
// window and motion, and lying under everything (CDS-45).
func scrim() design.Element {
	l, _ := design.Layout("vertical")
	return design.Element{
		Cut: 0, Kind: "scrim", Style: "mark", Anchor: "bottom", Region: design.Region(l.ScrimBottom),
		StartMS: 120, EndMS: 3120, Background: design.Scrim["bottom"].Hex,
		InMS: design.Motion.InMS, OutMS: design.Motion.OutMS, DY: design.Motion.InDY,
	}
}

func TestVerifyAcceptsAConformantManifest(t *testing.T) {
	if err := design.Verify(conformant(), "vertical"); err != nil {
		t.Fatal(err)
	}
	if err := design.Verify(design.Manifest{}, "vertical"); err != nil {
		t.Fatal("an empty manifest is a clip with no copy, not a violation:", err)
	}
	if err := design.Verify(conformant(), "2:3"); !errors.Is(err, design.ViolationSafeArea) {
		t.Fatal("unknown ratio", err)
	}
	// The cards join the same manifest: their own lines answer to the type
	// scale's floor rather than to a copy style's table.
	full := append(conformant(), card()...)
	if err := design.Verify(full, "vertical"); err != nil {
		t.Fatal(err)
	}
	if !design.Legible(full) {
		t.Fatal("every pairing in a conformant manifest clears V3")
	}
}

// The scrim: the one element allowed outside the safe area and under everything
// else, and only ever under an unplated style (CDS-32, CDS-45).
func TestVerifyAcceptsAScrimOnlyUnderAnUnplatedStyle(t *testing.T) {
	badge := conformant()[0]
	badge.EndMS = 3120
	m := design.Manifest{badge, {
		Cut: 0, Kind: "copy", Style: "mark", Anchor: "bottom", Text: "열여섯 글자까지",
		Region: design.Region{X: 300, Y: 1310, Width: 400, Height: 70}, FontSize: design.Type["mark"].Size,
		StartMS: 120, EndMS: 3120, Fill: design.Color["text_white"].Hex, Background: "#737373",
		InMS: design.Motion.InMS, OutMS: design.Motion.OutMS, DY: design.Motion.InDY,
	}, scrim()}
	if err := design.Verify(m, "vertical"); err != nil {
		t.Fatal(err)
	}
	// The scrim runs full-bleed to y 1420, past the badge and every chip, and
	// collides with none of them: it is the ground they are read against.
	if l, _ := design.Layout("vertical"); float64(l.ScrimBottom.X) != 0 {
		t.Fatal("CDS-32's scrim is full-bleed")
	}
	// A scrim under a plate would darken nothing (CDS-32).
	plated := append(design.Manifest{}, m...)
	plated[2].Style = "clean"
	if err := design.Verify(plated, "vertical"); !errors.Is(err, design.ViolationKind) {
		t.Fatal("a scrim under a plated style is not an element CDS allows:", err)
	}
	// And the copy it protects still has to clear V3 against the washed ground.
	dim := append(design.Manifest{}, m...)
	dim[1].Background = "#B9B9B9"
	if err := design.Verify(dim, "vertical"); !errors.Is(err, design.ViolationContrast) {
		t.Fatal("a scrim does not excuse an unreadable pairing:", err)
	}
}

// One fixture per check, each breaking exactly one rule of CDS-52.
func TestVerifyRejectsOneFixturePerCheck(t *testing.T) {
	for name, tc := range map[string]struct {
		want   design.Violation
		mutate func(design.Manifest) design.Manifest
	}{
		// V1: one pixel above the 9:16 safe area's y 250 is a breach.
		"safe area": {design.ViolationSafeArea, func(m design.Manifest) design.Manifest {
			m[1].Region.Y = 249
			return m
		}},
		// V2: below the body floor of 48 px.
		"size floor": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[3].FontSize = 47
			return m
		}},
		"size over nominal": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[3].FontSize = 57
			return m
		}},
		// V5: 깔끔하게 takes fourteen characters a line and two lines.
		"characters": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[3].Text = strings.Repeat("가", 15)
			return m
		}},
		// A third line in one cut. Elements of the SAME cut may share pixels, so
		// this breaks the line count and nothing else.
		"lines": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			return append(m, m[3], m[3])
		}},
		// V10: a kind outside the catalogue is a decoration CDS refuses.
		"unknown kind": {design.ViolationKind, func(m design.Manifest) design.Manifest {
			m[2].Kind = "sticker"
			return m
		}},
		// V6: the disclosure badge, its phrase, its size, its colour and its span.
		"no badge": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			return m[1:]
		}},
		"two badges": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			return append(m, m[0])
		}},
		"edited phrase": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			m[0].Text = "광고입니다"
			return m
		}},
		"badge shrunk": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			m[0].FontSize = 34
			return m
		}},
		"badge decorated": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			m[0].Background = design.Accent["coral"]
			return m
		}},
		"badge leaves early": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			m[0].EndMS = 3000
			return m
		}},
		"badge arrives late": {design.ViolationDisclosure, func(m design.Manifest) design.Manifest {
			m[0].StartMS = 120
			return m
		}},
		// The badge never moves and a chip does not settle (CDS-31, CDS-4).
		"badge moves": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			m[0].DY = design.Motion.InDY
			return m
		}},
		"unknown style": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[1].Style = "neon"
			return m
		}},
		// V7: two cuts whose windows meet may not share a pixel.
		"overlap": {design.ViolationOverlap, func(m design.Manifest) design.Manifest {
			m[4].StartMS, m[4].EndMS = m[1].StartMS, m[1].EndMS
			m[4].Region = m[1].Region
			return m
		}},
		// V9: the two permitted motions and nothing else.
		"motion in": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			m[1].InMS = 400
			return m
		}},
		"motion settle": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			m[3].DY = 48
			return m
		}},
		// V13: BOTTOM to TOP is three steps.
		"anchor step": {design.ViolationAnchorStep, func(m design.Manifest) design.Manifest {
			for i := 4; i < 7; i++ {
				m[i].Anchor = "top"
			}
			return m
		}},
		// V14: a third 크게 강조 in one clip.
		"bold frequency": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			out := design.Manifest{m[0]}
			out[0].EndMS = 3120 + 2*7000
			for i := range 3 {
				e := m[3]
				e.Cut, e.Style, e.Anchor, e.FontSize, e.Region.Y = i, "bold", "upper_mid", 72, 700
				e.StartMS, e.EndMS = 120+i*7000, 3120+i*7000
				out = append(out, e)
			}
			return out
		}},
		// V14: a fourth consecutive 깔끔하게 must have alternated.
		"style run": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			out := design.Manifest{m[0]}
			out[0].EndMS = 3120 + 3*7000
			for i := range 4 {
				e := m[3]
				e.Cut, e.Anchor = i, []string{"bottom", "lower_mid", "bottom", "lower_mid"}[i]
				e.Region.Y = []float64{1292, 1067, 1292, 1067}[i]
				e.StartMS, e.EndMS = 120+i*7000, 3120+i*7000
				out = append(out, e)
			}
			return out
		}},
		// V3: white on a ground the scrim could not darken enough. This is the
		// one check that needs the footage, so it is the sampled background that
		// fails it, never a token pairing (CDS-44).
		"contrast": {design.ViolationContrast, func(m design.Manifest) design.Manifest {
			m[3].Fill, m[3].Background = design.Color["text_white"].Hex, "#B9B9B9"
			return m
		}},
		"contrast on a card": {design.ViolationContrast, func(m design.Manifest) design.Manifest {
			c := card()
			c[1].Background = "#3A3A3A" // an accent that dark would hide its ink
			return append(m, c...)
		}},
		// A card covers the footage, so nothing of another layer may show under
		// it: not a caption (CDS-28's "no copy under it") and not a chip.
		"copy under a card": {design.ViolationOverlap, func(m design.Manifest) design.Manifest {
			c := card()
			c[0].Region = m[1].Region
			c[0].StartMS, c[0].EndMS = m[1].StartMS, m[1].EndMS
			return append(m, c[0])
		}},
		// A scrim is a wash, not a plate: it may not move on its own, and its
		// kind is the only one exempt from the safe area.
		"scrim moves on its own": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			e := scrim()
			e.Style, e.DY = "", 48
			return append(m, e)
		}},
		// One cut carries one style at one anchor.
		"two styles in a cut": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			m[3].Style = "memo"
			m[3].FontSize = 44
			return m
		}},
	} {
		t.Run(name, func(t *testing.T) {
			got := design.Verify(tc.mutate(conformant()), "vertical")
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			// A violation names its check and never the owner's own words.
			if got.Error() == "" || got.(design.Violation).OutputValidationCode() != string(tc.want) {
				t.Fatalf("violation does not name its check: %v", got)
			}
		})
	}
}
