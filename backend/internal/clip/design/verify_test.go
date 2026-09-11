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
	m := design.Manifest{}
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
			m[0].Region.Y = 249
			return m
		}},
		// V2: below the body floor of 48 px.
		"size floor": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[2].FontSize = 47
			return m
		}},
		"size over nominal": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[2].FontSize = 57
			return m
		}},
		// V5: 깔끔하게 takes fourteen characters a line and two lines.
		"characters": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[2].Text = strings.Repeat("가", 15)
			return m
		}},
		// A third line in one cut. Elements of the SAME cut may share pixels, so
		// this breaks the line count and nothing else.
		"lines": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			return append(m, m[2], m[2])
		}},
		"unknown kind": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[1].Kind = "sticker"
			return m
		}},
		"unknown style": {design.ViolationSize, func(m design.Manifest) design.Manifest {
			m[0].Style = "neon"
			return m
		}},
		// V7: two cuts whose windows meet may not share a pixel.
		"overlap": {design.ViolationOverlap, func(m design.Manifest) design.Manifest {
			m[3].StartMS, m[3].EndMS = m[0].StartMS, m[0].EndMS
			m[3].Region = m[0].Region
			return m
		}},
		// V9: the two permitted motions and nothing else.
		"motion in": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			m[0].InMS = 400
			return m
		}},
		"motion settle": {design.ViolationMotion, func(m design.Manifest) design.Manifest {
			m[2].DY = 48
			return m
		}},
		// V13: BOTTOM to TOP is three steps.
		"anchor step": {design.ViolationAnchorStep, func(m design.Manifest) design.Manifest {
			for i := 3; i < 6; i++ {
				m[i].Anchor = "top"
			}
			return m
		}},
		// V14: a third 크게 강조 in one clip.
		"bold frequency": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			out := design.Manifest{}
			for i := range 3 {
				e := m[2]
				e.Cut, e.Style, e.Anchor, e.FontSize, e.Region.Y = i, "bold", "upper_mid", 72, 700
				e.StartMS, e.EndMS = 120+i*7000, 3120+i*7000
				out = append(out, e)
			}
			return out
		}},
		// V14: a fourth consecutive 깔끔하게 must have alternated.
		"style run": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			out := design.Manifest{}
			for i := range 4 {
				e := m[2]
				e.Cut, e.Anchor = i, []string{"bottom", "lower_mid", "bottom", "lower_mid"}[i]
				e.Region.Y = []float64{1292, 1067, 1292, 1067}[i]
				e.StartMS, e.EndMS = 120+i*7000, 3120+i*7000
				out = append(out, e)
			}
			return out
		}},
		// One cut carries one style at one anchor.
		"two styles in a cut": {design.ViolationFrequency, func(m design.Manifest) design.Manifest {
			m[2].Style = "memo"
			m[2].FontSize = 44
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
