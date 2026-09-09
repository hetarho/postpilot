package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/rivo/uniseg"
	"golang.org/x/image/font/sfnt"
)

const pretendardSHA256 = "3090ccde0442bb347aa7685d9ba8b17436a60682df6e8f92a9a670de14056e22"
const fontFamily = "Pretendard Variable"

type CopyRecipe struct{ FontSize, MinFontSize, Weight, Padding, Radius int }

// Mirrored by entities/clip-template/model/types.ts; tested against that source.
var copyRecipes = map[string]CopyRecipe{
	"clean": {54, 36, 600, 28, 24}, "diary": {44, 32, 600, 22, 16}, "emphasis": {76, 48, 800, 24, 0},
}
var accentColors = map[string]string{"coral": "#ff6b5f", "amber": "#ffb23f", "lime": "#b8d94a", "teal": "#43c5b5", "blue": "#5b8def", "violet": "#8b6fe8", "pink": "#e96aae"}

type Rendering struct {
	media *Adapter
	cfg   clip.RenderConfig
	font  *sfnt.Font
}

var _ clip.Renderer = (*Rendering)(nil)

func NewRenderer(media *Adapter, cfg clip.RenderConfig) (*Rendering, error) {
	if media == nil || cfg.FadeMS != 200 || cfg.FPS != 30 || cfg.MaxCuts <= 0 || cfg.MaxCopyRunes <= 0 || cfg.MinDurationMS != 15000 || cfg.MaxDurationMS != 90000 || !filepath.IsAbs(cfg.ResvgPath) || !filepath.IsAbs(cfg.FontPath) {
		return nil, errors.New("invalid clip renderer configuration")
	}
	info, err := os.Lstat(cfg.FontPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() != 6739336 {
		return nil, errors.New("bundled clip font is missing or changed")
	}
	data, err := os.ReadFile(cfg.FontPath)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != pretendardSHA256 {
		return nil, errors.New("bundled clip font checksum mismatch")
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return nil, err
	}
	return &Rendering{media, cfg, font}, nil
}

func escaped(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}
func (r *Rendering) checkCopy(text string) error {
	var buf sfnt.Buffer
	for _, c := range text {
		if c == '\n' {
			continue
		}
		if unicode.IsControl(c) || c == '\ufffe' || c == '\uffff' {
			return clip.ErrInvalid
		}
		if c == '\u200d' || c == '\ufe0e' || c == '\ufe0f' {
			continue
		}
		id, err := r.font.GlyphIndex(&buf, c)
		if err != nil || id == 0 {
			return clip.ErrInvalid
		}
	}
	return nil
}

// Candidates split only at Unicode extended grapheme boundaries. Query all
// possible lines once at 100px; resvg supplies the actual shaped glyph bounds.
func copyCandidates(text string) ([][]string, []string, error) {
	if strings.Contains(text, "\n") {
		lines := strings.Split(text, "\n")
		if len(lines) != 2 || strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[1]) == "" {
			return nil, nil, clip.ErrCopyTooLong
		}
		values := lines
		if lines[0] == lines[1] {
			values = lines[:1]
		}
		return [][]string{lines}, values, nil
	}
	candidates, values := [][]string{{text}}, []string{text}
	seen := map[string]bool{text: true}
	g := uniseg.NewGraphemes(text)
	for g.Next() {
		_, end := g.Positions()
		if end == len(text) {
			break
		}
		lines := []string{text[:end], text[end:]}
		if strings.TrimSpace(lines[0]) == "" || strings.TrimSpace(lines[1]) == "" {
			continue
		}
		candidates = append(candidates, lines)
		for _, line := range lines {
			if !seen[line] {
				values = append(values, line)
				seen[line] = true
			}
		}
	}
	return candidates, values, nil
}
func measureSVG(values []string, weight int) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="10000" height="500">`)
	for i, text := range values {
		fmt.Fprintf(&b, `<text id="m%d" x="0" y="200" xml:space="preserve" font-family="%s" font-weight="%d" font-size="100">%s</text>`, i, fontFamily, weight, escaped(text))
	}
	b.WriteString(`</svg>`)
	return b.String()
}
func (r *Rendering) resvg(ctx context.Context, ws clip.MediaWorkspace, args ...string) ([]byte, error) {
	return r.media.run(ctx, ws, r.cfg.ResvgPath, append([]string{"--skip-system-fonts", "--use-font-file", r.cfg.FontPath, "--font-family", fontFamily}, args...)...)
}
func (r *Rendering) measure(ctx context.Context, ws clip.MediaWorkspace, values []string, weight int) (map[string]clip.Region, error) {
	path := filepath.Join(ws.Path, "copy-measure.svg")
	if err := os.WriteFile(path, []byte(measureSVG(values, weight)), 0600); err != nil {
		return nil, err
	}
	defer os.Remove(path)
	data, err := r.resvg(ctx, ws, "--query-all", path)
	if err != nil {
		return nil, err
	}
	result := map[string]clip.Region{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) != 5 || !strings.HasPrefix(parts[0], "m") {
			return nil, errors.New("invalid copy measurement")
		}
		i, e := strconv.Atoi(strings.TrimPrefix(parts[0], "m"))
		if e != nil || i < 0 || i >= len(values) {
			return nil, errors.New("invalid copy measurement id")
		}
		var bounds [4]float64
		for j := range bounds {
			bounds[j], e = strconv.ParseFloat(parts[j+1], 64)
			if e != nil || math.IsNaN(bounds[j]) || math.IsInf(bounds[j], 0) {
				return nil, errors.New("invalid copy glyph bounds")
			}
		}
		if bounds[2] <= 0 || bounds[3] <= 0 {
			return nil, clip.ErrInvalid
		}
		result[values[i]] = clip.Region{X: bounds[0], Y: bounds[1] - 200, Width: bounds[2], Height: bounds[3]}
	}
	if len(result) != len(values) {
		return nil, errors.New("copy measurement omitted glyphs")
	}
	return result, nil
}

type copyLayout struct {
	Lines    []string
	Bounds   []clip.Region
	Region   clip.Region
	FontSize int
	Recipe   CopyRecipe
}

func fitCopy(canvas clip.Canvas, c clip.Copy, candidates [][]string, bounds map[string]clip.Region) (copyLayout, error) {
	recipe := copyRecipes[c.Style]
	for size := recipe.FontSize; size >= recipe.MinFontSize; size-- {
		best := copyLayout{}
		bestWidth := math.Inf(1)
		for _, lines := range candidates {
			width, height := 0.0, 0.0
			scaled := make([]clip.Region, 0, len(lines))
			for _, line := range lines {
				b := bounds[line]
				factor := float64(size) / 100
				b = clip.Region{X: b.X * factor, Y: b.Y * factor, Width: b.Width * factor, Height: b.Height * factor}
				width = math.Max(width, b.Width)
				height += b.Height
				scaled = append(scaled, b)
			}
			height += float64((len(lines)-1)*size) / 4
			width += float64(2 * recipe.Padding)
			height += float64(2 * recipe.Padding)
			region, err := clip.PlaceCopy(canvas, c.Position, math.Ceil(width), math.Ceil(height))
			if err == nil && width < bestWidth {
				best = copyLayout{lines, scaled, region, size, recipe}
				bestWidth = width
			}
			// Prefer a single line whenever it fits at the current size.
			if err == nil && len(lines) == 1 {
				return best, nil
			}
		}
		if len(best.Lines) > 0 {
			return best, nil
		}
	}
	return copyLayout{}, clip.ErrCopyTooLong
}
func copySVG(canvas clip.Canvas, c clip.Copy, l copyLayout) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, canvas.Width, canvas.Height)
	p := l.Region
	if c.Style != "emphasis" {
		fill, opacity := "#19171c", "0.78"
		if c.Style == "diary" {
			fill, opacity = "#fff4d9", "1"
		}
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%d" fill="%s" fill-opacity="%s"/>`, p.X, p.Y, p.Width, p.Height, l.Recipe.Radius, fill, opacity)
	}
	if c.Accent != "" {
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="8" height="%.3f" rx="4" fill="%s"/>`, p.X+6, p.Y+float64(l.Recipe.Padding), p.Height-float64(2*l.Recipe.Padding), accentColors[c.Accent])
	}
	top := p.Y + float64(l.Recipe.Padding)
	for i, line := range l.Lines {
		bounds := l.Bounds[i]
		x := p.X + (p.Width-bounds.Width)/2 - bounds.X
		y := top - bounds.Y
		fill, stroke, strokeWidth := "#ffffff", "none", 0
		if c.Style == "diary" {
			fill = "#19171c"
		}
		if c.Style == "emphasis" {
			stroke = "#19171c"
			strokeWidth = 8
		}
		fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%d" font-weight="%d" fill="%s" stroke="%s" stroke-width="%d" paint-order="stroke fill">%s</text>`, x, y, fontFamily, l.FontSize, l.Recipe.Weight, fill, stroke, strokeWidth, escaped(line))
		top += bounds.Height + float64(l.FontSize)/4
	}
	b.WriteString(`</svg>`)
	return b.String()
}
func (r *Rendering) copyPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy, index int) (string, error) {
	if strings.TrimSpace(c.Text) == "" {
		return "", nil
	}
	if err := r.checkCopy(c.Text); err != nil {
		return "", err
	}
	candidates, values, err := copyCandidates(c.Text)
	if err != nil {
		return "", err
	}
	bounds, err := r.measure(ctx, ws, values, copyRecipes[c.Style].Weight)
	if err != nil {
		return "", err
	}
	layout, err := fitCopy(canvas, c, candidates, bounds)
	if err != nil {
		return "", err
	}
	svg := filepath.Join(ws.Path, fmt.Sprintf("copy-%04d.svg", index))
	png := filepath.Join(ws.Path, fmt.Sprintf("copy-%04d.png", index))
	if err := os.WriteFile(svg, []byte(copySVG(canvas, c, layout)), 0600); err != nil {
		return "", err
	}
	defer os.Remove(svg)
	if _, err := r.resvg(ctx, ws, svg, png); err != nil {
		return "", err
	}
	if err := r.media.sourcePath(ws, png); err != nil {
		return "", err
	}
	return png, nil
}

// CaptionSize uses precisely the same shaping and fit as the final PNG. It needs
// no source pixels, and its temporary SVG is scoped to a private workspace.
func (r *Rendering) CaptionSize(ctx context.Context, ratio string, c clip.Caption) (width, height float64, err error) {
	canvas, err := clip.ClipCanvas(ratio)
	if err != nil || !clip.ValidCopy(c, r.cfg.MaxCopyRunes) {
		return 0, 0, clip.ErrInvalid
	}
	if strings.TrimSpace(c.Text) == "" {
		return 0, 0, nil
	}
	if err := r.checkCopy(c.Text); err != nil {
		return 0, 0, err
	}
	err = r.media.WithWorkspace(ctx, "clip-caption-size", func(ws clip.MediaWorkspace) error {
		candidates, values, err := copyCandidates(c.Text)
		if err != nil {
			return err
		}
		bounds, err := r.measure(ctx, ws, values, copyRecipes[c.Style].Weight)
		if err != nil {
			return err
		}
		layout, err := fitCopy(canvas, c, candidates, bounds)
		if err != nil {
			return err
		}
		width, height = layout.Region.Width, layout.Region.Height
		return nil
	})
	return
}
