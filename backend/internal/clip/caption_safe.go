package clip

import (
	"math"
	"slices"
)

// CutCaptionSafe projects factual empty space through the same cover crop as
// CutSubject, in canvas pixels. A region must stay empty throughout every scene
// the cut touches; intersecting scenes never invents safety between their boxes.
func CutCaptionSafe(canvas Canvas, cut Cut, analysis SourceAnalysis) []Region {
	if cut.SourceID != analysis.Source.ID || analysis.Source.Info.Width <= 0 || analysis.Source.Info.Height <= 0 {
		return nil
	}
	var safe []Region
	seen := false
	for _, segment := range analysis.Segments {
		if segment.EndMS <= cut.StartMS || segment.StartMS >= cut.EndMS {
			continue
		}
		if len(segment.CaptionSafe) == 0 {
			return nil
		}
		if !seen {
			safe = slices.Clone(segment.CaptionSafe)
			seen = true
			continue
		}
		var common []Region
		for _, a := range safe {
			for _, b := range segment.CaptionSafe {
				if box, ok := intersectCaptionSpace(a, b); ok && !slices.Contains(common, box) {
					common = append(common, box)
				}
			}
		}
		safe = common
		if len(safe) == 0 {
			return nil
		}
	}
	w, h := float64(analysis.Source.Info.Width), float64(analysis.Source.Info.Height)
	cw, ch := float64(canvas.Width), float64(canvas.Height)
	scale := math.Max(cw/w, ch/h)
	w, h = w*scale, h*scale
	x := math.Max(0, math.Min(w-cw, w*cut.Focal.X-cw/2))
	y := math.Max(0, math.Min(h-ch, h*cut.Focal.Y-ch/2))
	var out []Region
	for _, box := range safe {
		if !ValidRegion(box) {
			continue
		}
		projected := Region{X: box.X*w - x, Y: box.Y*h - y, Width: box.Width * w, Height: box.Height * h}
		if visible, ok := intersectCaptionSpace(projected, Region{Width: cw, Height: ch}); ok {
			out = append(out, visible)
		}
	}
	return out
}

func intersectCaptionSpace(a, b Region) (Region, bool) {
	x, y := math.Max(a.X, b.X), math.Max(a.Y, b.Y)
	right, bottom := math.Min(a.X+a.Width, b.X+b.Width), math.Min(a.Y+a.Height, b.Y+b.Height)
	return Region{X: x, Y: y, Width: right - x, Height: bottom - y}, right > x && bottom > y
}
