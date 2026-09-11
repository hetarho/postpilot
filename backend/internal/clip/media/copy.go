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
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/rivo/uniseg"
	"golang.org/x/image/font/sfnt"
)

const pretendardSHA256 = "3090ccde0442bb347aa7685d9ba8b17436a60682df6e8f92a9a670de14056e22"
const fontFamily = "Pretendard Variable"

type Rendering struct {
	media *Adapter
	cfg   clip.RenderConfig
	font  *sfnt.Font
}

var _ clip.Renderer = (*Rendering)(nil)

func NewRenderer(media *Adapter, cfg clip.RenderConfig) (*Rendering, error) {
	if media == nil || cfg.FadeMS != design.Transition.FadeMS || cfg.FPS != 30 || cfg.MaxCuts <= 0 || cfg.MaxCopyRunes <= 0 || cfg.MinDurationMS != 15000 || cfg.MaxDurationMS != 90000 || !filepath.IsAbs(cfg.ResvgPath) || !filepath.IsAbs(cfg.FontPath) {
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
func copyCandidates(text string, maxLines int) ([][]string, []string, error) {
	if strings.Contains(text, "\n") {
		if maxLines < 2 {
			return nil, nil, clip.ErrCopyTooLong
		}
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
	if maxLines < 2 {
		return candidates, values, nil
	}
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

// Measured at 100 px with the role's tracking expressed in px, so scaling the
// result by size/100 gives exactly the tracking the final SVG asks for.
func measureSVG(values []string, weight int, tracking float64) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="10000" height="500">`)
	for i, text := range values {
		fmt.Fprintf(&b, `<text id="m%d" x="0" y="200" xml:space="preserve" font-family="%s" font-weight="%d" font-size="100" letter-spacing="%.4f">%s</text>`, i, fontFamily, weight, tracking*100, escaped(text))
	}
	b.WriteString(`</svg>`)
	return b.String()
}
func (r *Rendering) resvg(ctx context.Context, ws clip.MediaWorkspace, args ...string) ([]byte, error) {
	return r.media.run(ctx, ws, r.cfg.ResvgPath, append([]string{"--skip-system-fonts", "--use-font-file", r.cfg.FontPath, "--font-family", fontFamily}, args...)...)
}
func (r *Rendering) measure(ctx context.Context, ws clip.MediaWorkspace, values []string, weight int, tracking float64) (map[string]clip.Region, error) {
	path := filepath.Join(ws.Path, "copy-measure.svg")
	data := []byte(measureSVG(values, weight, tracking))
	if err := r.media.capacity(ws, int64(len(data))); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
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

// One laid-out copy: the lines chosen, their scaled ink boxes, the region the
// whole element occupies (plate for a plated style, stroke-inflated text block
// for an unplated one) and where its keyword sits on its own line.
type copyLayout struct {
	Style    design.StyleRule
	Role     design.TypeRole
	Lines    []string
	Bounds   []clip.Region
	Region   clip.Region
	FontSize float64
	Keyword  keywordSpan
}
type keywordSpan struct {
	Line    int
	Text    string
	Offset  float64 // advance of the prefix before it, on its line
	Width   float64
	Present bool
}

// An unplated style has no padding but its stroke is painted half outside the
// glyph outline, so the stroke is what keeps it off the safe area's edge.
func copyInsets(style design.StyleRule) (left, right, vertical float64) {
	inset := style.StrokeWidth() / 2
	left = style.Padding.H
	if style.PadLeft > left {
		left = style.PadLeft
	}
	return left + inset, style.Padding.H + inset, style.Padding.V + inset
}

// Where the keyword starts on the line that holds it, measured rather than
// estimated (CDS-26). The measured string is the line UP TO AND INCLUDING the
// keyword, never the prefix alone: a prefix can be a single space, which has an
// advance but no ink box, and resvg reports boxes, not advances.
func keywordOn(lines []string, keyword string, bounds map[string]clip.Region, factor float64) keywordSpan {
	if keyword == "" {
		return keywordSpan{}
	}
	for i, line := range lines {
		at := strings.Index(line, keyword)
		if at < 0 {
			continue
		}
		through, word, whole := bounds[line[:at+len(keyword)]], bounds[keyword], bounds[line]
		return keywordSpan{
			Line: i, Text: keyword, Present: true,
			Width:  word.Width * factor,
			Offset: (through.X + through.Width - word.Width - whole.X) * factor,
		}
	}
	return keywordSpan{}
}

// The fit loop searches only between the role's nominal size and its minimum
// (CDS-19); below the minimum the copy does not fit and is refused, never shrunk.
func fitCopy(canvas clip.Canvas, c clip.Copy, candidates [][]string, bounds map[string]clip.Region) (copyLayout, error) {
	style := design.Styles[c.Style]
	role := style.Role()
	left, right, vertical := copyInsets(style)
	for size := role.Size; size >= role.Min; size-- {
		best := copyLayout{}
		bestWidth := math.Inf(1)
		for _, lines := range candidates {
			if len(lines) > style.Lines {
				continue
			}
			factor := size / 100
			width, height := 0.0, 0.0
			scaled := make([]clip.Region, 0, len(lines))
			for _, line := range lines {
				b := bounds[line]
				b = clip.Region{X: b.X * factor, Y: b.Y * factor, Width: b.Width * factor, Height: b.Height * factor}
				width = math.Max(width, b.Width)
				height += b.Height
				scaled = append(scaled, b)
			}
			// The gap between lines is the role's own line height (CDS-19).
			height += float64(len(lines)-1) * size * (role.LineHeight - 1)
			width += left + right
			height += 2 * vertical
			region, err := clip.PlaceCopy(canvas, c.Anchor, c.Align, math.Ceil(width), math.Ceil(height))
			if err == nil && width < bestWidth {
				best = copyLayout{style, role, lines, scaled, region, size, keywordOn(lines, c.Keyword, bounds, factor)}
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

func paint(token string) (string, string) {
	c := design.Color[token]
	return c.Hex, strconv.FormatFloat(c.Alpha, 'f', -1, 64)
}

// Every colour, radius, stroke and offset below is a CDS token read from the
// design system; the only arithmetic is placement.
func copySVG(canvas clip.Canvas, c clip.Copy, l copyLayout) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, canvas.Width, canvas.Height)
	p, s := l.Region, l.Style
	accent := design.Accent[c.Accent]
	radius := design.Spacing.RadiusBox
	if s.Shadow != "" {
		sh := design.Shadow[s.Shadow]
		// CSS blur radius is twice a Gaussian standard deviation.
		fmt.Fprintf(&b, `<defs><filter id="shadow" x="-20%%" y="-20%%" width="140%%" height="140%%"><feDropShadow dx="%.3f" dy="%.3f" stdDeviation="%.3f" flood-color="%s" flood-opacity="%s"/></filter></defs>`, sh.DX, sh.DY, sh.Blur/2, sh.Hex, strconv.FormatFloat(sh.Alpha, 'f', -1, 64))
	}
	if s.Plate != "" {
		fill, opacity := paint(s.Plate)
		// The accent bar is clipped to the plate so its corners cannot escape it.
		fmt.Fprintf(&b, `<defs><clipPath id="plate"><rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f"/></clipPath></defs>`, p.X, p.Y, p.Width, p.Height, radius)
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="%s" fill-opacity="%s"/>`, p.X, p.Y, p.Width, p.Height, radius, fill, opacity)
	}
	if accent != "" && s.Bar {
		fmt.Fprintf(&b, `<g clip-path="url(#plate)"><rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s"/></g>`, p.X, p.Y, design.Spacing.BarAccent, p.Height, accent)
	}
	if accent != "" && s.Dot {
		r := design.Spacing.DotAccent / 2
		fmt.Fprintf(&b, `<circle cx="%.3f" cy="%.3f" r="%.3f" fill="%s"/>`, p.X+s.Padding.H+r, p.Y+s.Padding.V+r, r, accent)
	}
	left, right, vertical := copyInsets(s)
	inner := p.Width - left - right
	top := p.Y + vertical
	for i, line := range l.Lines {
		bounds := l.Bounds[i]
		x := p.X + left + (inner-bounds.Width)/2 - bounds.X
		y := top - bounds.Y
		if accent != "" && s.Highlight && l.Keyword.Present && l.Keyword.Line == i {
			u := design.Spacing.UnderlineMark
			// Drawn behind the text, through it: height 0.42em, its bottom edge
			// raised 0.28em above the baseline, extended 6 px each side.
			fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" fill="%s" fill-opacity="0.9"/>`,
				x+bounds.X+l.Keyword.Offset-u.Extend, y-(u.RaiseEM+u.HeightEM)*l.FontSize, l.Keyword.Width+2*u.Extend, u.HeightEM*l.FontSize, accent)
		}
		fill, _ := paint("text_white")
		if s.Plate == "paper_50" {
			fill, _ = paint("text_ink")
		}
		stroke, strokeOpacity := "none", "1"
		if s.Stroke != "" {
			stroke, strokeOpacity = paint("stroke_dark")
		}
		filter := ""
		if s.Shadow != "" {
			filter = ` filter="url(#shadow)"`
		}
		fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s" stroke="%s" stroke-opacity="%s" stroke-width="%.3f" stroke-linejoin="round" paint-order="stroke fill"%s>`,
			x, y, fontFamily, l.FontSize, l.Role.Weight, l.Role.Tracking*l.FontSize, fill, stroke, strokeOpacity, s.StrokeWidth(), filter)
		// 크게 강조 colours one word in place, so the line stays one shaped run.
		if accent != "" && !s.Highlight && s.Stroke != "" && l.Keyword.Present && l.Keyword.Line == i {
			at := strings.Index(line, l.Keyword.Text)
			fmt.Fprintf(&b, `%s<tspan fill="%s">%s</tspan>%s`, escaped(line[:at]), accent, escaped(l.Keyword.Text), escaped(line[at+len(l.Keyword.Text):]))
		} else {
			b.WriteString(escaped(line))
		}
		b.WriteString(`</text>`)
		top += bounds.Height + l.FontSize*(l.Role.LineHeight-1)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// Elements returns what this copy places, for the manifest the verifier reads.
// The window is already on the output timeline.
func (l copyLayout) Elements(cut int, c clip.Copy, startMS, endMS int) clip.Manifest {
	m := clip.Manifest{}
	add := func(kind string, region clip.Region, size float64, fill, background string) {
		m = append(m, design.Element{Cut: cut, Kind: kind, Style: c.Style, Anchor: c.Anchor, Region: design.Region(region),
			StartMS: startMS, EndMS: endMS, FontSize: size, Fill: fill, Background: background,
			InMS: design.Motion.InMS, OutMS: design.Motion.OutMS, DY: design.Motion.InDY})
	}
	p, s := l.Region, l.Style
	if s.Plate != "" {
		add("plate", p, 0, "", design.Color[s.Plate].Hex)
	}
	accent := design.Accent[c.Accent]
	if accent != "" && s.Bar {
		add("bar", clip.Region{X: p.X, Y: p.Y, Width: design.Spacing.BarAccent, Height: p.Height}, 0, accent, "")
	}
	if accent != "" && s.Dot {
		d := design.Spacing.DotAccent
		add("bar", clip.Region{X: p.X + s.Padding.H, Y: p.Y + s.Padding.V, Width: d, Height: d}, 0, accent, "")
	}
	left, right, vertical := copyInsets(s)
	inner := p.Width - left - right
	top := p.Y + vertical
	fill := design.Color["text_white"].Hex
	if s.Plate == "paper_50" {
		fill = design.Color["text_ink"].Hex
	}
	for i, line := range l.Lines {
		bounds := l.Bounds[i]
		x := p.X + left + (inner-bounds.Width)/2
		if accent != "" && s.Highlight && l.Keyword.Present && l.Keyword.Line == i {
			u := design.Spacing.UnderlineMark
			add("highlight", clip.Region{X: x + l.Keyword.Offset - u.Extend, Y: top + (1-u.RaiseEM-u.HeightEM)*l.FontSize, Width: l.Keyword.Width + 2*u.Extend, Height: u.HeightEM * l.FontSize}, 0, accent, "")
		}
		add("copy", clip.Region{X: x, Y: top, Width: bounds.Width, Height: bounds.Height}, l.FontSize, fill, design.Color[s.Plate].Hex)
		_ = line
		top += bounds.Height + l.FontSize*(l.Role.LineHeight-1)
	}
	return m
}

// One measurement pass covers every candidate line and, when the copy names a
// keyword, the prefix that precedes it on each line that holds it: its advance
// is where the highlight starts (CDS-26), never an estimated width.
func (r *Rendering) layoutCopy(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy) (copyLayout, error) {
	if err := r.checkCopy(c.Text); err != nil {
		return copyLayout{}, err
	}
	style := design.Styles[c.Style]
	candidates, values, err := copyCandidates(c.Text, style.Lines)
	if err != nil {
		return copyLayout{}, err
	}
	if c.Keyword != "" {
		seen := map[string]bool{}
		for _, v := range values {
			seen[v] = true
		}
		for _, extra := range keywordValues(candidates, c.Keyword) {
			if !seen[extra] {
				values, seen[extra] = append(values, extra), true
			}
		}
	}
	bounds, err := r.measure(ctx, ws, values, style.Role().Weight, style.Role().Tracking)
	if err != nil {
		return copyLayout{}, err
	}
	return fitCopy(canvas, c, candidates, bounds)
}

// The keyword itself plus, per candidate line that holds it, that line up to and
// including the keyword. Both always carry ink, so both always have a box.
func keywordValues(candidates [][]string, keyword string) []string {
	out := []string{keyword}
	for _, lines := range candidates {
		for _, line := range lines {
			if at := strings.Index(line, keyword); at >= 0 {
				out = append(out, line[:at+len(keyword)])
			}
		}
	}
	return out
}

// The disclosure badge (CDS-31) and the information chips (CDS-30). Both are
// plated, so neither needs brightness sampling (CDS-16), and both are typeset
// from the same font and the same tokens as copy.
type chip struct {
	Label, Value string
	Region       clip.Region
	LabelWidth   float64
}
type furniture struct {
	Badge       clip.Region
	BadgeText   string
	BadgeLines  []string
	Chips       []chip
	BadgeBounds clip.Region
}

// badgeAndChips measures the phrase and every chip's text, then places them.
// The measurement is the only impure half, exactly as it is for copy.
func (r *Rendering) badgeAndChips(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio, phrase string, labels []string, answers map[string]string) (furniture, error) {
	values := []string{phrase}
	seen := map[string]bool{phrase: true}
	for _, label := range labels {
		for _, v := range []string{label, strings.TrimSpace(answers[label])} {
			if v != "" && !seen[v] {
				values, seen[v] = append(values, v), true
			}
		}
	}
	if err := r.checkCopy(strings.Join(values, "") + ellipsis); err != nil {
		return furniture{}, err
	}
	role := design.Type["badge"]
	bounds, err := r.measure(ctx, ws, values, role.Weight, role.Tracking)
	if err != nil {
		return furniture{}, err
	}
	return placeFurniture(canvas, ratio, phrase, labels, answers, bounds)
}

// placeFurniture puts the disclosure badge at its ratio's fixed corner and
// stacks at most two chips from the priority it was given (CDS-30, CDS-31).
func placeFurniture(canvas clip.Canvas, ratio, phrase string, labels []string, answers map[string]string, bounds map[string]clip.Region) (furniture, error) {
	out := furniture{BadgeText: phrase}
	l, ok := design.Layout(ratio)
	if !ok {
		return out, clip.ErrInvalid
	}
	badgeRole, labelRole, valueRole := design.Type["badge"], design.Type["label"], design.Type["caption"]
	b := scaled(bounds[phrase], badgeRole.Size/100)
	out.BadgeBounds = b
	width, height := math.Ceil(b.Width+2*badgePadH), math.Ceil(b.Height+2*badgePadV)
	out.Badge = clip.Region{X: l.Badge.Right - width, Y: l.Badge.Top, Width: width, Height: height}
	if out.Badge.X < canvas.Safe.X || out.Badge.Y+height > canvas.Safe.Y+canvas.Safe.Height {
		return out, clip.ErrInvalid
	}
	pad, gap := design.Spacing.PadChip, design.Spacing.GapStack
	x, y := l.Chip.X, l.Chip.Y
	for _, label := range labels {
		if len(out.Chips) >= maxChips {
			break
		}
		value := strings.TrimSpace(answers[label])
		if value == "" {
			continue
		}
		lb := scaled(bounds[label], labelRole.Size/100)
		vb := scaled(bounds[value], valueRole.Size/100)
		// A chip is at most 600 px wide (CDS-30). A value that does not fit is
		// CUT and given an ellipsis rather than squeezed: the face is fixed
		// (CDS-18), so distorting its glyphs is the worse failure. The cut is
		// proportional to the measured width, and textLength in the SVG is the
		// hard bound that keeps an imperfect estimate inside the pill.
		room := l.Chip.MaxWidth - 2*pad.H - lb.Width - design.Spacing.GapChip
		if vb.Width > room {
			runes := []rune(value)
			keep := int(float64(len(runes)) * room / vb.Width)
			if keep > 0 {
				keep--
			}
			value = string(runes[:keep]) + ellipsis
			vb.Width = room
		}
		w := math.Min(l.Chip.MaxWidth, math.Ceil(lb.Width+design.Spacing.GapChip+vb.Width+2*pad.H))
		h := math.Ceil(math.Max(lb.Height, vb.Height) + 2*pad.V)
		c := chip{Label: label, Value: value, LabelWidth: lb.Width, Region: clip.Region{X: x, Y: y, Width: w, Height: h}}
		if c.Region.X+w > canvas.Safe.X+canvas.Safe.Width || c.Region.Y+h > canvas.Safe.Y+canvas.Safe.Height {
			break
		}
		out.Chips = append(out.Chips, c)
		if l.Chip.Columns > 1 && len(out.Chips) == 1 {
			x += w + gap
			continue
		}
		y += h + gap
	}
	return out, nil
}

// CDS-30 shows at most two chips at once, and CDS-31 fixes the badge's padding.
const maxChips = 2
const badgePadV, badgePadH = 10, 18

// What a cut chip value ends with. Checked against the bundled font like every
// other glyph, so an unsupported ellipsis is an error rather than a blank.
const ellipsis = "…"

func scaled(r clip.Region, factor float64) clip.Region {
	return clip.Region{X: r.X * factor, Y: r.Y * factor, Width: r.Width * factor, Height: r.Height * factor}
}

// furnitureSVG paints the badge and the chips on one full-canvas plate: they
// share a window (the whole clip for the badge) and never animate.
func furnitureSVG(canvas clip.Canvas, f furniture) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, canvas.Width, canvas.Height)
	badge, badgeAlpha := paint("badge_ad")
	white, _ := paint("text_white")
	muted, mutedAlpha := paint("text_muted")
	ink, inkAlpha := paint("ink_900")
	role := design.Type["badge"]
	p := f.Badge
	fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="%s" fill-opacity="%s"/>`, p.X, p.Y, p.Width, p.Height, p.Height/2, badge, badgeAlpha)
	fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s">%s</text>`,
		p.X+badgePadH-f.BadgeBounds.X, p.Y+badgePadV-f.BadgeBounds.Y, fontFamily, role.Size, role.Weight, role.Tracking*role.Size, white, escaped(f.BadgeText))
	label, value := design.Type["label"], design.Type["caption"]
	for _, c := range f.Chips {
		fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="%s" fill-opacity="%s"/>`, c.Region.X, c.Region.Y, c.Region.Width, c.Region.Height, c.Region.Height/2, ink, inkAlpha)
		x := c.Region.X + design.Spacing.PadChip.H
		baseline := c.Region.Y + c.Region.Height - design.Spacing.PadChip.V
		fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s" fill-opacity="%s">%s</text>`,
			x, baseline, fontFamily, label.Size, label.Weight, label.Tracking*label.Size, muted, mutedAlpha, escaped(c.Label))
		// The value is cut to the chip's own width; a chip never grows past it.
		fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s" textLength="%.3f" lengthAdjust="spacingAndGlyphs">%s</text>`,
			x+c.LabelWidth+design.Spacing.GapChip, baseline, fontFamily, value.Size, value.Weight, value.Tracking*value.Size, white,
			math.Max(1, c.Region.Width-2*design.Spacing.PadChip.H-c.LabelWidth-design.Spacing.GapChip), escaped(c.Value))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// Elements places the badge for the whole clip and each chip for its own cut.
func (f furniture) Elements(duration int, chipCut int, chipStart, chipEnd int) clip.Manifest {
	m := clip.Manifest{{
		Kind: "badge", Text: f.BadgeText, FontSize: design.Type["badge"].Size,
		Background: design.Color["badge_ad"].Hex, Fill: design.Color["text_white"].Hex,
		Region: design.Region(f.Badge), StartMS: 0, EndMS: duration,
	}}
	for _, c := range f.Chips {
		m = append(m, design.Element{
			Cut: chipCut, Kind: "chip", Text: c.Label + " " + c.Value,
			FontSize: design.Type["caption"].Size, Background: design.Color["ink_900"].Hex,
			Fill: design.Color["text_white"].Hex, Region: design.Region(c.Region),
			StartMS: chipStart, EndMS: chipEnd,
		})
	}
	return m
}

func (r *Rendering) copyPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy, layout copyLayout, index int) (string, error) {
	if strings.TrimSpace(c.Text) == "" {
		return "", nil
	}
	return r.rasterize(ctx, ws, canvas, copySVG(canvas, c, layout), fmt.Sprintf("copy-%04d", index))
}

// furniturePlate is the fixed layer: the disclosure badge and this cut's chips,
// drawn in CDS-45's order under the copy and never animated.
func (r *Rendering) furniturePlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, f furniture, index int) (string, error) {
	if f.BadgeText == "" {
		return "", nil
	}
	return r.rasterize(ctx, ws, canvas, furnitureSVG(canvas, f), fmt.Sprintf("fixed-%04d", index))
}

func (r *Rendering) rasterize(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, body, name string) (string, error) {
	svg := filepath.Join(ws.Path, name+".svg")
	png := filepath.Join(ws.Path, name+".png")
	data := []byte(body)
	// A full RGBA canvas plus PNG/metadata headroom is small and known before
	// rasterization. Reserve it before the subprocess, not after its write.
	if err := r.media.capacity(ws, int64(len(data))+int64(canvas.Width)*int64(canvas.Height)*5); err != nil {
		return "", err
	}
	if err := os.WriteFile(svg, data, 0600); err != nil {
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
		layout, err := r.layoutCopy(ctx, ws, canvas, c)
		if err != nil {
			return err
		}
		width, height = layout.Region.Width, layout.Region.Height
		return nil
	})
	return
}
