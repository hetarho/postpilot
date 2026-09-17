package design_test

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// V1 counts an owner placement like any other element: the owner may put a
// caption anywhere INSIDE the safe area, and nowhere outside it (CDS-82).
func TestSafeAreaStillHoldsAnOwnerPlacement(t *testing.T) {
	m := conformant()
	for i := range m {
		if m[i].Kind == "copy" && m[i].Cut == 0 {
			m[i].OwnerPlaced = true
			m[i].Region.X = -40
		}
	}
	if err := design.Verify(m, "vertical"); !errors.Is(err, design.ViolationSafeArea) {
		t.Fatalf("an owner placement outside the safe area passed V1: %v", err)
	}
}

// V3: a shortfall under an owner placement is delivered with a notice rather
// than refused — the owner chose the spot (CDS-52).
func TestContrastShortfallUnderAnOwnerPlacementIsNotAFailure(t *testing.T) {
	shortfall := func(owner bool) design.Manifest {
		m := conformant()
		for i := range m {
			if m[i].Kind == "copy" && m[i].Cut == 0 {
				m[i].Fill, m[i].Background, m[i].OwnerPlaced = "#FFFFFF", "#FFFFFF", owner
			}
		}
		return m
	}
	if err := design.Verify(shortfall(false), "vertical"); !errors.Is(err, design.ViolationContrast) {
		t.Fatalf("an automatic shortfall stopped being a failure: %v", err)
	}
	if err := design.Verify(shortfall(true), "vertical"); err != nil {
		t.Fatalf("an owner-placed shortfall blocked the render: %v", err)
	}
	if design.Legible(shortfall(true)) {
		t.Fatal("the shortfall stopped being measurable, so no notice could name it")
	}
}

// V13 walks the anchors AUTOMATIC placement chose. An owner-placed caption is
// not part of that walk, whatever anchor its manifest records (CDS-38).
func TestAnchorStepSkipsAnOwnerPlacedCaption(t *testing.T) {
	jump := func(owner bool) design.Manifest {
		m := conformant()
		for i := range m {
			if m[i].Cut == 1 && m[i].Style != "" {
				m[i].Anchor, m[i].OwnerPlaced = "top", owner
				m[i].Region.Y = 300
			}
		}
		return m
	}
	if err := design.Verify(jump(false), "vertical"); !errors.Is(err, design.ViolationAnchorStep) {
		t.Fatalf("a three-step automatic jump stopped failing V13: %v", err)
	}
	if err := design.Verify(jump(true), "vertical"); err != nil {
		t.Fatalf("V13 measured an owner placement: %v", err)
	}
}
