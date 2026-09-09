package clip

import (
	"context"
	"errors"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

var ErrCopyTooLong = errors.New("CLIP_COPY_TOO_LONG")

// All persisted time authority is integer milliseconds. Positions and volume are
// normalized values, never arbitrary filter expressions or pixel coordinates.
type Point struct{ X, Y float64 }
type Region struct{ X, Y, Width, Height float64 }
type Caption struct {
	Text, Position, Style, Accent string
	// Relative to the trimmed cut. Both zero preserves the whole-cut default.
	StartMS, EndMS int
}
type Copy = Caption
type Cut struct {
	ID, SourceID, Fingerprint string
	StartMS, EndMS            int
	Focal                     Point
	Copy                      Copy
	Volume                    *float64 // nil keeps original audio; explicit zero mutes it
}
type EditCut = Cut

func (c Cut) CaptionWindow() (int, int) {
	if c.Copy.StartMS == 0 && c.Copy.EndMS == 0 {
		return 0, c.EndMS - c.StartMS
	}
	return c.Copy.StartMS, c.Copy.EndMS
}

func (c EditCut) OriginalVolume() float64 {
	if c.Volume == nil {
		return 1
	}
	return *c.Volume
}

type EditPlan struct {
	Ratio      string
	DurationMS int
	Cuts       []EditCut
}
type RenderSource struct {
	ID, Fingerprint string
	Info            MediaInfo
}

// The consumer downloads only the requested source, then removes it after fn.
type RenderSourceLoader func(context.Context, string, func(MediaSource) error) error
type RenderedVideo struct {
	Path  string
	Info  MediaInfo
	Bytes int64
}
type Renderer interface {
	Render(context.Context, MediaWorkspace, EditPlan, []RenderSource, RenderSourceLoader) (RenderedVideo, error)
}
type RenderConfig struct {
	ResvgPath, FontPath                                              string
	MaxCuts, MaxCopyRunes, FadeMS, FPS, CRF, AudioRate, AudioBitrate int
	MinDurationMS, MaxDurationMS                                     int
}
type Canvas struct {
	Width, Height int
	Safe          Region
}

// Conservative product-owned caption regions; these are not claimed to be
// Naver's exact UI geometry. They leave space for top, right and bottom controls.
func ClipCanvas(ratio string) (Canvas, error) {
	switch ratio {
	case "vertical":
		return Canvas{1080, 1920, Region{86, 192, 778, 1308}}, nil
	case "horizontal":
		return Canvas{1920, 1080, Region{154, 108, 1574, 756}}, nil
	case "square":
		return Canvas{1080, 1080, Region{86, 108, 778, 756}}, nil
	default:
		return Canvas{}, ErrInvalid
	}
}
func normalized(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func ValidCopy(c Copy, maxRunes int) bool {
	return utf8.ValidString(c.Text) && utf8.RuneCountInString(c.Text) <= maxRunes && slices.Contains([]string{"top", "center", "bottom"}, c.Position) && slices.Contains([]string{"clean", "diary", "emphasis"}, c.Style) && slices.Contains([]string{"", "coral", "amber", "lime", "teal", "blue", "violet", "pink"}, c.Accent)
}
func ValidateEditPlan(cfg RenderConfig, plan EditPlan, sources []RenderSource) error {
	if _, err := ClipCanvas(plan.Ratio); err != nil {
		return err
	}
	if len(plan.Cuts) == 0 || len(plan.Cuts) > cfg.MaxCuts || plan.DurationMS < cfg.MinDurationMS || plan.DurationMS > cfg.MaxDurationMS {
		return ErrInvalid
	}
	byID := map[string]RenderSource{}
	for _, s := range sources {
		if s.ID == "" || s.Fingerprint == "" || s.Info.DurationMS <= 0 || s.Info.Width <= 0 || s.Info.Height <= 0 || byID[s.ID].ID != "" {
			return ErrInvalid
		}
		byID[s.ID] = s
	}
	total := 0
	seen := map[string]bool{}
	for _, c := range plan.Cuts {
		if utf8.RuneCountInString(c.Copy.Text) > cfg.MaxCopyRunes {
			return ErrCopyTooLong
		}
		s, ok := byID[c.SourceID]
		if !ok || c.Fingerprint != s.Fingerprint || strings.TrimSpace(c.ID) == "" || seen[c.ID] || c.StartMS < 0 || c.StartMS >= c.EndMS || c.EndMS > s.Info.DurationMS || c.EndMS-c.StartMS <= 2*cfg.FadeMS || !normalized(c.Focal.X) || !normalized(c.Focal.Y) || !normalized(c.OriginalVolume()) || !ValidCopy(c.Copy, cfg.MaxCopyRunes) {
			return ErrInvalid
		}
		seen[c.ID] = true
		captionStart, captionEnd := c.CaptionWindow()
		if captionStart < 0 || captionEnd <= captionStart || captionEnd > c.EndMS-c.StartMS {
			return ErrInvalid
		}
		if c.EndMS-c.StartMS > cfg.MaxDurationMS+cfg.FadeMS*(len(plan.Cuts)-1)-total {
			return ErrInvalid
		}
		total += c.EndMS - c.StartMS
	}
	// Overlap is part of the approved timeline, not extra trimming after approval.
	if total-cfg.FadeMS*(len(plan.Cuts)-1) != plan.DurationMS {
		return ErrInvalid
	}
	return nil
}
func PlaceCopy(canvas Canvas, position string, width, height float64) (Region, error) {
	s := canvas.Safe
	if !slices.Contains([]string{"top", "center", "bottom"}, position) || math.IsNaN(width) || math.IsNaN(height) || width <= 0 || height <= 0 || width > s.Width || height > s.Height {
		return Region{}, ErrInvalid
	}
	y := s.Y
	if position == "center" {
		y += (s.Height - height) / 2
	}
	if position == "bottom" {
		y += s.Height - height
	}
	return Region{s.X + (s.Width-width)/2, y, width, height}, nil
}

// PickCopyPosition maps a normalized output-space avoid region only to the three
// approved positions. Manual placement remains exactly what the owner selected.
func PickCopyPosition(canvas Canvas, preferred string, width, height float64, avoid Region) (string, error) {
	if !normalized(avoid.X) || !normalized(avoid.Y) || !normalized(avoid.Width) || !normalized(avoid.Height) || avoid.X+avoid.Width > 1 || avoid.Y+avoid.Height > 1 {
		return "", ErrInvalid
	}
	if _, err := PlaceCopy(canvas, preferred, width, height); err != nil {
		return "", err
	}
	avoid = Region{avoid.X * float64(canvas.Width), avoid.Y * float64(canvas.Height), avoid.Width * float64(canvas.Width), avoid.Height * float64(canvas.Height)}
	best, area := preferred, math.Inf(1)
	for _, p := range []string{preferred, "bottom", "top", "center"} {
		r, _ := PlaceCopy(canvas, p, width, height)
		overlap := math.Max(0, math.Min(r.X+r.Width, avoid.X+avoid.Width)-math.Max(r.X, avoid.X)) * math.Max(0, math.Min(r.Y+r.Height, avoid.Y+avoid.Height)-math.Max(r.Y, avoid.Y))
		if overlap < area {
			best, area = p, overlap
		}
	}
	return best, nil
}
