package media

import (
	"errors"
	"math"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-100, CLIP-191: the owner's new style redraws the caption and nothing else
// about it moves — the words, the output interval, the size the owner set and
// the place they put it, which the style's own measured bounds keep inside the
// safe area.
func TestAnOwnerStyleChangeKeepsTheWordsIntervalSizeAndPlace(t *testing.T) {
	safe, _ := design.Safe("vertical")
	at := clip.CaptionPlacement{X: int(safe.X) + 20, Y: int(safe.Y) + 40}
	// 72 is inside both styles' roles: the default's ceiling and keynote's floor.
	const size = 72
	before := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Position: &at, Size: size})
	after := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Position: &at, Size: size, Style: "keynote"})
	after.CaptionStyles = []string{design.DefaultCaptionStyle}
	was, now := measuredDeclared(t, before).elements()[0], measuredDeclared(t, after).elements()[0]
	if now.Style != "keynote" || was.Style == now.Style {
		t.Fatalf("the caption was not redrawn in the owner's style: %q → %q", was.Style, now.Style)
	}
	if now.Text != was.Text || now.StartMS != was.StartMS || now.EndMS != was.EndMS {
		t.Fatalf("the style change moved the words or the interval: %+v → %+v", was, now)
	}
	if now.Region.X != float64(at.X) || now.Region.Y != float64(at.Y) || !now.OwnerPlaced {
		t.Fatalf("the owner's place was not kept: %v", now.Region)
	}
	for _, part := range now.Parts {
		if part.Kind == "copy" && math.Abs(part.FontSize-size) > 0.01 {
			t.Fatalf("the owner's size was not kept: %v", part.FontSize)
		}
	}
	// Placed at the far corner, the new style's own bounds are what the safe area
	// holds (CDS-82).
	corner := clip.CaptionPlacement{X: 100000, Y: 100000}
	edge := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Position: &corner, Style: "keynote"})
	r := measuredDeclared(t, edge).elements()[0].Region
	if r.X+r.Width > safe.X+safe.Width+0.01 || r.Y+r.Height > safe.Y+safe.Height+0.01 {
		t.Fatalf("the new style's bounds left the safe area: %v", r)
	}
}

// CDS-100, CDS-64: words the owner's style cannot hold are refused on that
// caption by name. No shorter alternative stands in for them, whatever the
// writer offered, because the owner's style is the owner's to correct.
func TestWordsTheOwnersStyleCannotHoldAreRefusedByName(t *testing.T) {
	// Twenty syllables: two lines of ten, which the default style's eleven a
	// line holds and keynote's nine does not.
	words := "가나다라마바사아자차 카타파하가나다라마바"
	fits := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: design.DefaultCaptionStyle})
	fits.Portable.Elements[0].Resolved.Text = words
	if got := measuredDeclared(t, fits).elements()[0].Text; got != words {
		t.Fatalf("the default style did not hold the words: %q", got)
	}
	over := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "keynote"})
	over.Portable.Elements[0].Resolved.Text = words
	over.Portable.Elements[0].Alternatives = []clip.CopyAlternative{{Text: "짧은 문구"}}
	a, r := measured(t)
	err := a.WithWorkspace(t.Context(), "owner-overflow", func(ws clip.MediaWorkspace) error {
		_, err := r.layoutComposition(t.Context(), ws, over)
		return err
	})
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.Reason != "copy_limit" || problem.ElementID != over.Portable.Elements[0].Resolved.Element.ID {
		t.Fatalf("the overflow was not refused on its caption: %v", err)
	}
}

// CLIP-145, CDS-81: a sequence style the owner chose outside a static-only AI
// set is drawn frame by frame, and the server serves its frames for a browser
// render; going back to a static style serves the one raster again.
func TestAnOwnerSequenceStyleOutsideTheSetIsDrawnFrameByFrame(t *testing.T) {
	_, r := measured(t)
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	preview := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	plan, _ := framesPlan(t, design.DefaultCaptionStyle)
	plan.Portable.Elements[0].Owner.Style = "word-pop"
	if visual := captionVisual(t, measuredDeclared(t, plan)); visual.caption.Caption.Static() || visual.manifest.Style != "word-pop" {
		t.Fatalf("the owner's sequence style was not the one laid out: %q", visual.manifest.Style)
	}
	frames, err := r.PrepareCaptionFrames(t.Context(), plan, sources, "caption/cut", 0, preview)
	if err != nil || frames.Cells <= 1 || len(frames.Sheet) == 0 {
		t.Fatalf("the owner's sequence style has no frames: %+v %v", frames, err)
	}
	back, _ := framesPlan(t, "word-pop")
	back.Portable.Elements[0].Owner.Style = design.DefaultCaptionStyle
	if visual := captionVisual(t, measuredDeclared(t, back)); !visual.caption.Caption.Static() {
		t.Fatal("going back to a static style still drew frame by frame")
	}
	if _, err := r.PrepareCaptionFrames(t.Context(), back, sources, "caption/cut", 0, preview); !errors.Is(err, clip.ErrInvalid) {
		t.Fatalf("a static style was served frames: %v", err)
	}
}
