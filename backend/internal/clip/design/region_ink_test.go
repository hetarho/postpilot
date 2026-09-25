package design_test

import (
	"math"
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
	// NanumMyeongjo is set only at 800 in a region (CDS-90, CDS-95).
	for face, name := range map[string]string{"wantedsans": "wantedsans/WantedSansVariable.ttf", "paperlogy": "paperlogy/Paperlogy-8ExtraBold.ttf", "jua": "jua/Jua-Regular.ttf", "nanummyeongjo": "nanummyeongjo/NanumMyeongjo-ExtraBold.ttf"} {
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

// inkBox measures one region line with the bundled face's own glyph boxes, the way
// resvg reports it from --query-all: the pen walks at ppem 100 with the role's
// tracking between glyphs, the ink box is the union of the glyph bounds, and
// both scale to the role's size. The tracking after the last glyph advances the
// pen but is not part of the ink.
// It also returns the ink's left edge from the pen's start: the renderer sets a
// line by its ink box, so the drawn width is width − left.
func inkBox(t *testing.T, faces inkFaces, text string, role design.TypeRole) (left, top, bottom, width float64) {
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
		if l := pen + float64(bounds.Min.X)/64; first || l < left {
			left = l
		}
		pen += float64(advance)/64 + tracking
		first = false
	}
	return left * scale, top * scale, bottom * scale, width * scale
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
// of one block may overlap on any ratio — a frame, plate, pill or ring is the
// one part that holds its text — and every part stays in the safe area.
func TestRegionBlockPartsNeverOverlapOnAnyRatio(t *testing.T) {
	faces := bundledFaces(t)
	containers := map[string]bool{"frame": true, "plate": true, "pill": true, "ring": true}
	for _, kind := range []string{"intro", "outro"} {
		for _, id := range design.RegionIDs(kind) {
			preset, _ := design.Region(kind, id)
			for _, ratio := range []string{"vertical", "horizontal", "square"} {
				t.Run(kind+"."+id+"/"+ratio, func(t *testing.T) {
					var rows []string
					for i := range preset.Slots() {
						spec, width, _ := design.RegionSlotAt(kind, id, ratio, i)
						rows = append(rows, worstRow(spec, width))
					}
					block, err := design.LayoutRegion(kind, id, ratio, rows)
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
							if line.Arc != nil {
								// Set along its ring inside the ring band (CDS-99).
								if line.Width > preset.Stamp.ArcLen {
									t.Fatalf("an arc line of %.1f px is longer than its arc", line.Width)
								}
								continue
							}
							inkLeft, top, bottom, right := inkBox(t, faces, line.Text, slot.Type)
							width := right - inkLeft
							left := line.X - width/2
							if line.Align == "left" {
								left = line.X
							}
							parts = append(parts, part{name: "slot " + slot.Spec.Role, top: line.Baseline + top, bottom: line.Baseline + bottom, left: left, right: left + width})
							if slot.Spec.Place == "centre" {
								// Inside the inner ring, clear of its stroke.
								r := preset.Stamp.RInner - preset.Stamp.StrokeInner
								for _, x := range []float64{left, left + width} {
									for _, y := range []float64{line.Baseline + top, line.Baseline + bottom} {
										if math.Hypot(x-block.Rotate.CX, y-block.Rotate.CY) > r {
											t.Fatalf("the stamp's middle line leaves the inner ring at (%.1f, %.1f)", x, y)
										}
									}
								}
							}
						}
					}
					for _, rule := range block.Rules {
						parts = append(parts, part{name: "rule " + rule.Kind, top: rule.Box.Y, bottom: rule.Box.Y + rule.Box.Height, left: rule.Box.X, right: rule.Box.X + rule.Box.Width})
					}
					safe, _ := design.Safe(ratio)
					for _, shape := range block.Shapes {
						b := shape.Box
						if b.X < safe.X || b.Y < safe.Y || b.X+b.Width > safe.X+safe.Width || b.Y+b.Height > safe.Y+safe.Height {
							t.Fatalf("%s %+v leaves the safe area %+v", shape.Kind, b, safe)
						}
						if !containers[shape.Kind] {
							parts = append(parts, part{name: shape.Kind, top: b.Y, bottom: b.Y + b.Height, left: b.X, right: b.X + b.Width})
						}
					}
					for _, p := range parts {
						if p.top < safe.Y || p.bottom > safe.Y+safe.Height || p.left < safe.X || p.right > safe.X+safe.Width {
							t.Fatalf("%s [%.2f, %.2f] x [%.2f, %.2f] leaves the safe area %+v", p.name, p.top, p.bottom, p.left, p.right, safe)
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
}
