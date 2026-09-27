package media

import (
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// The badge's right edge and top are the ratio's own (CDS-31: 984/80 on 9:16,
// 1824/112 on 16:9 and 1016/112 on 1:1), and the furniture is the badge alone:
// no fact chip exists (CLIP-68).
func TestBadgeGeometryPerRatio(t *testing.T) {
	bounds := clip.Region{X: 2, Y: -70, Width: 300, Height: 90}
	for ratio, badge := range map[string][2]float64{"vertical": {984, 80}, "horizontal": {1824, 112}, "square": {1016, 112}} {
		canvas, _ := clip.ClipCanvas(ratio)
		f, err := placeFurniture(canvas, ratio, design.Disclosure["ad"], bounds)
		if err != nil {
			t.Fatalf("%s: %v", ratio, err)
		}
		if f.Badge.X+f.Badge.Width != badge[0] || f.Badge.Y != badge[1] || f.Badge.Height != 68 {
			t.Fatalf("%s badge at %+v want right %v top %v", ratio, f.Badge, badge[0], badge[1])
		}
		// The badge is on the manifest for the whole clip, and nothing else is.
		m := f.Elements(20000)
		if len(m) != 1 || m[0].Kind != "badge" || m[0].StartMS != 0 || m[0].EndMS != 20000 || m[0].Text != "광고" {
			t.Fatalf("%s furniture elements %+v", ratio, m)
		}
		svg := furnitureSVG(canvas, f)
		if !strings.Contains(svg, "광고") || strings.Count(svg, "<rect") != 1 || strings.Count(svg, "<text") != 1 {
			t.Fatalf("%s furniture drawing: %s", ratio, svg)
		}
	}
}

func TestHiddenDisclosureLeavesNoFurnitureAndExplicitVerification(t *testing.T) {
	bounds := clip.Region{X: 2, Y: -70, Width: 300, Height: 90}
	for _, ratio := range []string{"vertical", "horizontal", "square"} {
		canvas, _ := clip.ClipCanvas(ratio)
		for _, hidden := range []bool{false, true} {
			phrase := design.Disclosure["ad"]
			if hidden {
				phrase = ""
			}
			f, err := placeFurniture(canvas, ratio, phrase, bounds)
			if err != nil {
				t.Fatal(ratio, f, err)
			}
			manifest := f.Elements(15000)
			if err := design.Verify(manifest, ratio, hidden); err != nil {
				t.Fatal(ratio, hidden, err)
			}
			// A drawn badge is refused where the badge is meant to be hidden.
			if err := design.Verify(manifest, ratio, true); !hidden && err == nil {
				t.Fatal("visibility mismatch passed", ratio)
			}
			svg := furnitureSVG(canvas, f)
			want := 1
			if hidden {
				want = 0
			}
			if strings.Count(svg, "<rect") != want || strings.Contains(svg, "광고") == hidden {
				t.Fatal(svg)
			}
		}
	}
}
