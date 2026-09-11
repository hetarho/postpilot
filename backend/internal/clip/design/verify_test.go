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
