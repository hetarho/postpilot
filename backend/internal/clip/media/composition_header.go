package media

import (
	"math"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func visibleTogether(a, b clip.CompositionElement) bool {
	return a.StartMS < b.EndMS && b.StartMS < a.EndMS
}

// Header slots belong to visible declarations. A disconnected time interval
// starts a fresh row, with no reservation for a badge that is absent there.
func arrangeDeclaredHeader(ratio string, visuals []declaredVisual) {
	l, _ := design.Layout(ratio)
	indices := []int{}
	for i, v := range visuals {
		if v.manifest.Position == "header" && (v.manifest.Role == "badge" || v.manifest.Role == "info") {
			indices = append(indices, i)
		}
	}
	slices.SortStableFunc(indices, func(a, b int) int { return visuals[a].manifest.StartMS - visuals[b].manifest.StartMS })
	for start := 0; start < len(indices); {
		end, until := start+1, visuals[indices[start]].manifest.EndMS
		for end < len(indices) && visuals[indices[end]].manifest.StartMS < until {
			until = max(until, visuals[indices[end]].manifest.EndMS)
			end++
		}
		group := indices[start:end]
		height := 0.0
		for _, i := range group {
			height = math.Max(height, visuals[i].manifest.Region.Height)
		}
		placed := []int{}
		for _, i := range group {
			if visuals[i].manifest.Role == "badge" {
				resizeHeader(&visuals[i], visuals[i].manifest.Region.X, l.Badge.Top, height)
			}
		}
		for _, i := range group {
			v := &visuals[i]
			if v.manifest.Role != "info" {
				continue
			}
			for row := 0; ; row++ {
				y, right := l.Badge.Top+float64(row)*(height+design.Spacing.GapStack), l.Badge.Right
				if row == 0 {
					for _, j := range group {
						b := visuals[j].manifest
						if b.Role == "badge" && visibleTogether(v.manifest, b) {
							right = math.Min(right, b.Region.X-design.Spacing.GapChip)
						}
					}
				}
				x, count := l.Chip.X, 0
				for _, j := range placed {
					p := visuals[j].manifest
					if p.Region.Y == y && visibleTogether(v.manifest, p) {
						x = math.Max(x, p.Region.X+p.Region.Width+design.Spacing.GapChip)
						count++
					}
				}
				if count < 2 && x+v.manifest.Region.Width <= right {
					resizeHeader(v, x, y, height)
					placed = append(placed, i)
					break
				}
				// A too-wide authored pill is rejected by the shared safe-area
				// verifier; never truncate it or loop while looking for a slot.
				if row > len(group) {
					resizeHeader(v, x, y, height)
					break
				}
			}
		}
		start = end
	}
}

func resizeHeader(v *declaredVisual, x, y, height float64) {
	old := v.manifest.Region
	dx, dy := x-old.X, y-old.Y
	v.manifest.Region.X, v.manifest.Region.Y, v.manifest.Region.Height = x, y, height
	for i := range v.manifest.Parts {
		p := &v.manifest.Parts[i]
		p.Region.X, p.Region.Y = p.Region.X+dx, p.Region.Y+dy
		if p.Kind == "plate" || p.Kind == "badge" {
			p.Region.Height = height
		} else {
			p.Region.Y += (height - old.Height) / 2
		}
	}
	if v.manifest.Role == "badge" {
		v.furniture.Badge = v.manifest.Region
	} else {
		v.info.Plate.X, v.info.Plate.Y, v.info.Plate.Height, v.info.Plate.Radius = x, y, height, height/2
		for i := range v.info.Lines {
			v.info.Lines[i].X += dx
			v.info.Lines[i].Y += dy + (height-old.Height)/2
		}
	}
}
