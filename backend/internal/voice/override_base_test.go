package voice

import "testing"

// VOICE-28: an override remembers the value its field held, a second override of the same
// field keeps the first base, and stripping returns every field — an axis included — to it.
func TestStrippingOverridesReturnsEachFieldToItsBase(t *testing.T) {
	two, unknownAxis := 2, (*int)(nil)
	profile := StructuredProfile{Axes: AxesProfile{Humor: &two, Narrativity: unknownAxis}}
	profile.Syntax.ConnectiveStyle = VoiceValue{Value: "그래서", Source: SourceAnalyzed}
	for _, step := range []struct {
		layer        RuleLayer
		field, value string
	}{
		{LayerSyntax, "connective_style", "하지만"},
		{LayerSyntax, "connective_style", "그런데"},
		{LayerAxes, "humor", "-1"},
		{LayerAxes, "narrativity", "3"},
	} {
		if err := applyOverride(&profile, step.layer, step.field, step.value); err != nil {
			t.Fatal(err)
		}
	}
	if profile.Syntax.ConnectiveStyle.Value != "그런데" || *profile.Axes.Humor != -1 || *profile.Axes.Narrativity != 3 {
		t.Fatalf("overrides did not apply: %+v %+v", profile.Syntax, profile.Axes)
	}
	if err := stripOverrides(&profile); err != nil {
		t.Fatal(err)
	}
	if got := profile.Syntax.ConnectiveStyle; got.Value != "그래서" || got.Source != SourceAnalyzed {
		t.Fatalf("connective style = %+v, want its first base", got)
	}
	if profile.Axes.Humor == nil || *profile.Axes.Humor != 2 || profile.Axes.Narrativity != nil {
		t.Fatalf("axes = %+v, want humor 2 and narrativity unknown", profile.Axes)
	}
	if profile.OverrideBase != nil {
		t.Fatalf("stripped profile still holds bases: %+v", profile.OverrideBase)
	}
}
