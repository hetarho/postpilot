package media

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func fragments(t *testing.T, plan clip.EditPlan) []clip.CaptionFragment {
	t.Helper()
	_, r := measured(t)
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 60000, Width: 1920, Height: 1080}}}
	out, err := r.CaptionFragments(t.Context(), plan, sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("the plan's caption produced no fragment")
	}
	return out
}

// CDS-83: the fragment carries no absolute position of its own — one transform
// brings it to the origin, and the box says where the caller puts it back.
func TestCaptionFragmentIsPlacedByOneTransform(t *testing.T) {
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{})
	f := fragments(t, plan)[0]
	if !strings.HasPrefix(f.SVG, `<g transform="translate(`) || !strings.HasSuffix(f.SVG, "</g>") {
		t.Fatalf("the fragment is not one placeable group: %.80s", f.SVG)
	}
	prefix := f.SVG[:strings.Index(f.SVG, ")")+1]
	if !strings.Contains(prefix, coordinate(-f.Box.X)) || !strings.Contains(prefix, coordinate(-f.Box.Y)) {
		t.Fatalf("the transform does not cancel the box it reports: %s vs %v", prefix, f.Box)
	}
	if f.Box.Width <= 0 || f.Box.Height <= 0 || f.FontSize <= 0 {
		t.Fatalf("the fragment has no measured box: %+v", f)
	}
	if f.Sequence {
		t.Fatal("a static style was labelled a representative frame")
	}
}

// Two captions on screen would otherwise share a filter id and the browser
// would paint one of them with the other's.
func TestCaptionFragmentIdsAreItsOwn(t *testing.T) {
	plan := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "neon"})
	plan.CaptionStyles = []string{design.DefaultCaptionStyle, "neon"}
	f := fragments(t, plan)[0]
	if !f.Sequence {
		t.Fatal("a sequence-rendered style was not labelled as one representative frame")
	}
	if !strings.Contains(f.SVG, "<defs>") {
		t.Fatal("the fragment left behind the defs its filters need")
	}
	at := strings.Index(f.SVG, `id="`)
	if at < 0 {
		t.Fatalf("the style declares no id to prefix: %.200s", f.SVG)
	}
	own := idToken(f.InstanceID) + "-"
	if !strings.Contains(f.SVG, `id="`+own) {
		t.Fatalf("an id was not prefixed with the caption's own instance: %.200s", f.SVG[at:])
	}
	if strings.Contains(f.SVG, "url(#") && !strings.Contains(f.SVG, "url(#"+own) {
		t.Fatal("a reference still points at an unprefixed id")
	}
}

// The drawing follows the caption's text and style. Its position does not reach
// the drawing at all: a move changes only the transform the caller supplies.
func TestOnlyTextAndStyleChangeWhatTheFragmentDraws(t *testing.T) {
	body := func(f clip.CaptionFragment) string { return f.SVG[strings.Index(f.SVG, ">")+1:] }
	base := fragments(t, ownerPlacedPlan(t, "vertical", clip.OwnerCaption{}))[0]
	safe, _ := design.Safe("vertical")
	at := clip.CaptionPlacement{X: int(safe.X) + 40, Y: int(safe.Y) + 40}
	moved := fragments(t, ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Position: &at}))[0]
	if base.Box == moved.Box {
		t.Fatal("the fixture did not actually move the caption")
	}
	// The same caption, drawn the same way: only where it is placed changed, and
	// the caller re-places the fragment it already holds rather than asking for
	// another. That the placement is pixel-exact is the smoke contract's own.
	if strings.Count(body(base), "<text") != strings.Count(body(moved), "<text") || base.FontSize != moved.FontSize ||
		base.Box.Width != moved.Box.Width || base.Box.Height != moved.Box.Height || base.Style != moved.Style {
		t.Fatal("moving the caption changed what it draws")
	}
	restyled := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{Style: "neon"})
	restyled.CaptionStyles = []string{design.DefaultCaptionStyle, "neon"}
	if body(fragments(t, restyled)[0]) == body(base) {
		t.Fatal("a style change did not change the drawing")
	}
	reworded := ownerPlacedPlan(t, "vertical", clip.OwnerCaption{})
	reworded.Portable.Elements[0].Resolved.Text = "완전히 다른 문장"
	if body(fragments(t, reworded)[0]) == body(base) {
		t.Fatal("a text change did not change the drawing")
	}
}
