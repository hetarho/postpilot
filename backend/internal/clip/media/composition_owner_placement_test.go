package media

import (
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

const ownerCaptionBody = `<clip version="1"><scene id="scene"><text id="caption" kind="ai" role="caption" basis="cut">Describe.</text></scene></clip>`

func ownerPlacedPlan(t *testing.T, ratio string, owner clip.OwnerCaption) clip.EditPlan {
	t.Helper()
	plan := declaredPlan(t, ownerCaptionBody, ratio)
	plan.Portable.Elements[0].Resolved.Text = "현재 장면"
	plan.Portable.Elements[0].Owner = owner
	plan.Portable.Elements[0].OwnerEdited = true
	return plan
}

// CDS-82: the caption goes exactly where the owner put it, and a placement past
// the edge is moved back inside the safe area rather than resized or refused.
func TestOwnerPlacementIsKeptAndClampedAtEveryRatio(t *testing.T) {
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		safe, _ := design.Safe(ratio)
		t.Run(ratio+"/kept", func(t *testing.T) {
			at := clip.CaptionPlacement{X: int(safe.X) + 20, Y: int(safe.Y) + 30}
			element := measuredDeclared(t, ownerPlacedPlan(t, ratio, clip.OwnerCaption{Position: &at})).elements()[0]
			if element.Region.X != float64(at.X) || element.Region.Y != float64(at.Y) {
				t.Fatalf("owner placement moved: %v want %v", element.Region, at)
			}
			if !element.OwnerPlaced {
				t.Fatal("the manifest does not say the owner placed it")
			}
		})
		t.Run(ratio+"/clamped", func(t *testing.T) {
			at := clip.CaptionPlacement{X: 100000, Y: 100000}
			element := measuredDeclared(t, ownerPlacedPlan(t, ratio, clip.OwnerCaption{Position: &at})).elements()[0]
			r := element.Region
			if r.X+r.Width > safe.X+safe.Width+0.01 || r.Y+r.Height > safe.Y+safe.Height+0.01 || r.X < safe.X || r.Y < safe.Y {
				t.Fatalf("%s left the safe area: %v not inside %v", ratio, r, safe)
			}
		})
	}
}

// CDS-3's floor and V2's ceiling are checked where the size is written, so the
// layout sets exactly the size the owner asked for and never shrinks it to fit.
func TestOwnerSizeIsSetExactlyAsWritten(t *testing.T) {
	role := design.Caption().Role()
	size := int(role.Min) + 2
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Size: size})
	for _, part := range measuredDeclared(t, plan).elements()[0].Parts {
		if part.Kind == "copy" && math.Abs(part.FontSize-float64(size)) > 0.01 {
			t.Fatalf("copy was set at %v, not the owner's %d", part.FontSize, size)
		}
	}
}

// CLIP-142: the styles a caption may take are the project's own selection, and
// an owner style outside it is an authoring error rather than a quiet swap.
func TestOwnerStyleMustBeOneTheProjectAllows(t *testing.T) {
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "film"})
	plan.CaptionStyles = []string{design.DefaultCaptionStyle, "film"}
	layout := measuredDeclared(t, plan)
	if style := layout.elements()[0].Style; style != "film" {
		t.Fatalf("the owner's style was not used: %q", style)
	}
	// V19: the caption is set in the face its STYLE names, not the default one.
	style, _ := design.LookupCaptionStyle("film")
	if face := layout.visuals[0].caption.Caption.Face; face != style.Face || face == design.DefaultCaption().Face {
		t.Fatalf("the owner's style was set in %q, not its own %q", face, style.Face)
	}
	refused := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "film"})
	refused.CaptionStyles = []string{design.DefaultCaptionStyle}
	a, r := measured(t)
	err := a.WithWorkspace(t.Context(), "owner-style", func(ws clip.MediaWorkspace) error {
		_, err := r.layoutComposition(t.Context(), ws, refused)
		return err
	})
	if err == nil {
		t.Fatal("a style outside the allowed set rendered")
	}
}

// CDS-38: an owner placement replaces automatic placement and is never re-run
// over — not even when the footage the caption sits on would move it.
func TestAutomaticPlacementNeverRunsOverAnOwnerPlacement(t *testing.T) {
	safe, _ := design.Safe("vertical")
	at := clip.CaptionPlacement{X: int(safe.X) + 12, Y: int(safe.Y) + 12}
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Position: &at})
	plan.Portable.Observations = []clip.SourceAnalysis{{
		Source:   clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Info: clip.MediaInfo{Width: 1920, Height: 1080}}},
		Segments: []clip.Segment{{EndMS: 15000, Scene: "person", Subject: clip.Region{X: 0, Y: 0, Width: 1, Height: 1}}},
	}}
	element := measuredDeclared(t, plan).elements()[0]
	if element.Region.X != float64(at.X) || element.Region.Y != float64(at.Y) {
		t.Fatalf("a covered subject moved the owner's caption: %v", element.Region)
	}
}
