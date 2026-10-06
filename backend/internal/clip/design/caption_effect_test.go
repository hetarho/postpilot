package design_test

import (
	"github.com/postpilot/backend/internal/clip/design"
	"strings"
	"testing"
)

// Future native palette edits must update the exported browser paint together.
func TestCaptionEffectPaletteMatchesNativePainter(t *testing.T) {
	var visit func(any, string)
	visit = func(value any, svg string) {
		switch v := value.(type) {
		case string:
			if strings.HasPrefix(v, "#") && !strings.Contains(svg, v) {
				t.Errorf("exported effect colour %s absent from native painter", v)
			}
		case map[string]any:
			for _, value := range v {
				visit(value, svg)
			}
		case []map[string]any:
			for _, value := range v {
				visit(value, svg)
			}
		}
	}
	for _, id := range []string{"ambient", "neon", "iridescent", "glitch"} {
		style, ok := design.LookupCaptionStyle(id)
		if !ok {
			t.Fatal(id)
		}
		defs, body, ok := design.DrawCaptionFrame(sequenceFrame(style, .5))
		if !ok {
			t.Fatal(id)
		}
		visit(design.CaptionEffectPaint()[id], defs+body)
	}
}
