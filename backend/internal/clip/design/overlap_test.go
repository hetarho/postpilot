package design_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func TestOverlapDoesNotHideOtherDeliveryFailures(t *testing.T) {
	for _, mode := range []string{"overlap only", "anchor step", "size", "disclosure", "contrast"} {
		t.Run(mode, func(t *testing.T) {
			m := conformant()
			// Identical chips on successive cuts share their crossfade window.
			chip := design.Element{Kind: "chip", Cut: 0, Text: "위치 서울", Region: design.Region{X: 96, Y: 380, Width: 300, Height: 70}, StartMS: 0, EndMS: 7400}
			m = append(m, chip)
			chip.Cut, chip.StartMS, chip.EndMS = 1, 7200, 10120
			m = append(m, chip)
			if err := design.Verify(m, "vertical"); !errors.Is(err, design.ViolationOverlap) {
				t.Fatalf("diagnostic must find the furniture overlap: %v", err)
			}
			var want error
			switch mode {
			case "anchor step":
				for i := 4; i < 7; i++ {
					m[i].Anchor = "top"
				}
				want = design.ViolationAnchorStep
				// This check runs after V7, so swallowing the first error is unsafe.
				if err := design.Verify(m, "vertical"); !errors.Is(err, design.ViolationOverlap) {
					t.Fatalf("fixture no longer masks its sequence failure with overlap: %v", err)
				}
			case "size":
				m[3].FontSize = 1
				want = design.ViolationSize
			case "disclosure":
				m[0].Text = ""
				want = design.ViolationDisclosure
			case "contrast":
				m[3].Fill, m[3].Background = "#FFFFFF", "#FFFFFF"
				want = design.ViolationContrast
			}
			if err := design.VerifyRenderable(m, "vertical", nil); !errors.Is(err, want) {
				t.Fatalf("delivery check = %v, want %v", err, want)
			}
		})
	}
}
