package design_test

import (
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

// CDS-80: every approved style names its own face, type role, colour treatment
// and motion, and says whether one rasterisation covers its whole interval.
func TestApprovedCaptionStyleSet(t *testing.T) {
	set := design.CaptionStyles()
	if len(set) < 2 {
		t.Fatal("the approved set carries no alternative to the default")
	}
	if set[0].ID != design.DefaultCaptionStyle || set[0].Name != "크게 강조" || !set[0].Static() {
		t.Fatalf("크게 강조 is not the first, default, static style: %+v", set[0])
	}
	seen := map[string]bool{}
	faces, static, sequence := map[string]bool{}, 0, 0
	for _, style := range set {
		if seen[style.ID] || style.ID == "" || style.Name == "" {
			t.Fatalf("%q is unnamed or registered twice", style.ID)
		}
		seen[style.ID] = true
		if design.FontFamily(style.Face) == "" {
			t.Fatalf("%s names the unbundled face %q", style.ID, style.Face)
		}
		faces[style.Face] = true
		if style.Weight < 100 || style.Weight > 900 {
			t.Fatalf("%s asks for weight %d", style.ID, style.Weight)
		}
		role := style.Role()
		if role.Size <= 0 || role.Min <= 0 || role.Chars <= 0 || role.Face != style.Face || role.Weight != style.Weight {
			t.Fatalf("%s has no usable type role: %+v", style.ID, role)
		}
		// CDS-3's floor holds whatever the style is: nothing in the set may be
		// typeset smaller than the scale's own smallest role.
		if role.Min < design.MinTypeSize() {
			t.Fatalf("%s may be typeset at %v, under the %v floor", style.ID, role.Min, design.MinTypeSize())
		}
		// CDS-4: a style declares its motion, and a style declaring none would
		// make V9 meaningless.
		if style.Motion.InMS <= 0 || style.Motion.OutMS <= 0 {
			t.Fatalf("%s declares no motion: %+v", style.ID, style.Motion)
		}
		if style.Paint.Fill == "" && style.Paint.Stroke == "" {
			t.Fatalf("%s paints neither a fill nor a stroke", style.ID)
		}
		// A style names a stroke colour exactly when its rule gives it a width,
		// because CDS-21 owns the width and the style owns only the colour.
		if (style.Paint.Stroke != "") != (style.Rule().StrokeWidth() > 0) {
			t.Fatalf("%s disagrees with its rule about its stroke", style.ID)
		}
		if (style.Paint.Shadow.Hex != "") != (style.Rule().Shadow != "") {
			t.Fatalf("%s disagrees with its rule about its shadow", style.ID)
		}
		switch style.Rendering {
		case design.StaticCaption:
			static++
		case design.SequenceCaption:
			sequence++
		default:
			t.Fatalf("%s is neither static nor sequence-rendered: %q", style.ID, style.Rendering)
		}
	}
	// CDS-17 bundles four caption faces; a face nothing sets is dead weight in
	// the image and a face nothing bundles cannot be set at all.
	for _, face := range []string{"wantedsans", "paperlogy", "jua", "nanummyeongjo"} {
		if !faces[face] {
			t.Fatalf("no approved style is set in %q", face)
		}
	}
	// CDS-81 quotes the two kinds apart, which needs both to exist.
	if static == 0 || sequence == 0 {
		t.Fatalf("static=%d sequence=%d", static, sequence)
	}
}

func TestCaptionStyleLookupRefusesWhatTheSetDoesNotCarry(t *testing.T) {
	if _, ok := design.LookupCaptionStyle("sparkle"); ok {
		t.Fatal("an unapproved style resolved")
	}
	if _, ok := design.CaptionRule("sparkle"); ok {
		t.Fatal("an unapproved style has a layout rule")
	}
	if rule, ok := design.CaptionRule(design.DefaultCaptionStyle); !ok || rule != design.Caption() {
		t.Fatal("the default style's rule is not the default treatment")
	}
	if design.DefaultCaption().ID != design.DefaultCaptionStyle {
		t.Fatal("the fallback style is not the default one")
	}
}

// CDS-4: the motion is the style's own, except that a rapid phrase carries none
// whatever style it wears.
func TestCaptionMotionIsTheStylesOwnUnlessRapid(t *testing.T) {
	for _, style := range design.CaptionStyles() {
		if got := design.CaptionMotion(style.ID, "steady"); got != style.Motion {
			t.Fatalf("%s: %+v", style.ID, got)
		}
		if got := design.CaptionMotion(style.ID, "rapid"); got != (design.MotionTokens{}) {
			t.Fatalf("%s carried motion into a rapid phrase: %+v", style.ID, got)
		}
	}
	// A retired name on a stored plan keeps the motion it rendered with.
	for _, retired := range []string{"clean", "memo", "mark", "simple"} {
		if got := design.CaptionMotion(retired, "steady"); got != design.Motion {
			t.Fatalf("%s: %+v", retired, got)
		}
	}
}

// V9 is per style (CDS-52, CDS-80): a caption carrying the DEFAULT style's
// motion while naming another one is refused, which is what makes a style's
// declared motion a promise rather than a comment.
func TestV9ChecksTheMotionOfTheStyleTheCaptionNames(t *testing.T) {
	for _, style := range design.CaptionStyles() {
		motion := design.CaptionMotion(style.ID, "steady")
		caption := design.Element{
			Cut: 0, Copy: 0, Kind: "copy", Style: style.ID, Anchor: "upper_mid", Pace: "steady",
			Region: design.Bounds{X: 112, Y: 650, Width: 856, Height: 100}, StartMS: 0, EndMS: 3000,
			FontSize: style.Role().Size, Fill: style.Paint.Fill,
			InMS: motion.InMS, OutMS: motion.OutMS, DY: motion.InDY,
		}
		if err := design.VerifyRenderable(design.Manifest{caption}, "vertical", true); err != nil {
			t.Fatalf("%s: a caption in its own declared motion was refused: %v", style.ID, err)
		}
		wrong := caption
		wrong.InMS, wrong.OutMS, wrong.DY = design.Motion.InMS+40, design.Motion.OutMS+40, design.Motion.InDY+8
		if err := design.VerifyRenderable(design.Manifest{wrong}, "vertical", true); err == nil {
			t.Fatalf("%s: a caption moving unlike its style was accepted", style.ID)
		}
	}
}
