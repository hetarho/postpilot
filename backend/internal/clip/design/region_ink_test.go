package design_test

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"github.com/postpilot/backend/internal/clip/design"
)

const inkPPEM = 100

type inkFaces map[string]*sfnt.Font

func bundledFaces(t *testing.T) inkFaces {
	t.Helper()
	faces := inkFaces{}
	for face, name := range map[string]string{"wantedsans": "wantedsans/WantedSansVariable.ttf", "paperlogy": "paperlogy/Paperlogy-8ExtraBold.ttf"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "assets", "fonts", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := sfnt.Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		faces[face] = parsed
	}
	return faces
}

// ink measures one region line with the bundled face's own glyph boxes, the way
// resvg reports it from --query-all: the pen walks at ppem 100 with the role's
// tracking between glyphs, the ink box is the union of the glyph bounds, and
// both scale to the role's size. The tracking after the last glyph advances the
// pen but is not part of the ink.
func ink(t *testing.T, faces inkFaces, text string, role design.TypeRole) (top, bottom, width float64) {
	t.Helper()
	f, ok := faces[role.Face]
	if !ok {
		t.Fatal("unbundled face", role.Face)
	}
	var buf sfnt.Buffer
	scale, tracking := role.Size/inkPPEM, role.Tracking*inkPPEM
	pen, first := 0., true
	for _, r := range text {
		index, err := f.GlyphIndex(&buf, r)
		if err != nil || index == 0 {
			t.Fatalf("%q is not in %s: %v", r, role.Face, err)
		}
		bounds, _, err := f.GlyphBounds(&buf, index, fixed.I(inkPPEM), font.HintingNone)
		if err != nil {
			t.Fatal(err)
		}
		advance, err := f.GlyphAdvance(&buf, index, fixed.I(inkPPEM), font.HintingNone)
		if err != nil {
			t.Fatal(err)
		}
		// Y grows downward from the baseline, so an ascender is negative.
		high, low := float64(bounds.Min.Y)/64, float64(bounds.Max.Y)/64
		if first || high < top {
			top = high
		}
		if first || low > bottom {
			bottom = low
		}
		if right := pen + float64(bounds.Max.X)/64; first || right > width {
			width = right
		}
		pen += float64(advance)/64 + tracking
		first = false
	}
	return top * scale, bottom * scale, width * scale
}

// worstRow is the longest text of repeated words a slot still fits (CDS-86):
// shrunk to its floor and, for the large roles, wrapped into two lines.
func worstRow(spec design.SlotSpec, width float64) string {
	text := "한글빵"
	for {
		next := text + " 한글빵"
		if design.FitRegionSlot(spec, next, width).Over {
			return text
		}
		text = next
	}
}

// CDS-79 and CDS-87 regression: the 1:1 outro E block once collapsed into
// itself. Measured with real ink at each slot's fitted worst case, no two parts
// of one block may overlap on any ratio, and every part stays in the safe area.
func TestRegionBlockPartsNeverOverlapOnAnyRatio(t *testing.T) {
	faces := bundledFaces(t)
	for _, choice := range []struct{ kind, id string }{{"intro", "a"}, {"intro", "b"}, {"outro", "b"}, {"outro", "e"}} {
		preset, _ := design.Region(choice.kind, choice.id)
		for _, ratio := range []string{"vertical", "horizontal", "square"} {
			t.Run(choice.kind+"."+choice.id+"/"+ratio, func(t *testing.T) {
				layout, _ := design.Layout(ratio)
				var rows []string
				for _, slot := range preset.Slots() {
					rows = append(rows, worstRow(slot.Spec(ratio), layout.CopyMaxWidth))
				}
				block, err := design.LayoutRegion(choice.kind, choice.id, ratio, rows)
				if err != nil || block.Over {
					t.Fatal(err, block.Over)
				}
				type part struct {
					name                     string
					top, bottom, left, right float64
				}
				var parts []part
				for _, slot := range block.Slots {
					for _, line := range slot.Lines {
						top, bottom, width := ink(t, faces, line.Text, slot.Type)
						parts = append(parts, part{name: "slot " + slot.Spec.Role, top: line.Baseline + top, bottom: line.Baseline + bottom, left: block.AnchorX - width/2, right: block.AnchorX + width/2})
					}
				}
				for _, rule := range block.Rules {
					parts = append(parts, part{name: "rule " + rule.Kind, top: rule.Box.Y, bottom: rule.Box.Y + rule.Box.Height, left: rule.Box.X, right: rule.Box.X + rule.Box.Width})
				}
				safe, _ := design.Safe(ratio)
				for _, p := range parts {
					if p.top < safe.Y || p.bottom > safe.Y+safe.Height || p.left < safe.X || p.right > safe.X+safe.Width {
						t.Fatalf("%s [%.2f, %.2f] leaves the safe area %+v", p.name, p.top, p.bottom, safe)
					}
				}
				for i, a := range parts {
					for _, b := range parts[i+1:] {
						if a.top < b.bottom && b.top < a.bottom && a.left < b.right && b.left < a.right {
							t.Fatalf("%s [%.2f, %.2f] overlaps %s [%.2f, %.2f]", a.name, a.top, a.bottom, b.name, b.top, b.bottom)
						}
					}
				}
			})
		}
	}
}
