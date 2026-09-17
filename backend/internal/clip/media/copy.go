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
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
	"github.com/rivo/uniseg"
	"golang.org/x/image/font/sfnt"
)

// Every bundled font file (CDS-17, CLIP-13), each pinned by size and checksum:
// the renderer never discovers a font and never substitutes one. `Key` is what
// RenderConfig.FontPaths carries the file under; a face shipping one weight for
// every role keeps its bare face name.
type bundledFont struct {
	Key, Face string
	// 0 where the face ships one file for every weight.
	Weight int
	Bytes  int64
	SHA256 string
}

var bundledFonts = []bundledFont{
	{Key: "pretendard", Face: "pretendard", Bytes: 6739336, SHA256: "3090ccde0442bb347aa7685d9ba8b17436a60682df6e8f92a9a670de14056e22"},
	{Key: "paperlogy", Face: "paperlogy", Bytes: 1304560, SHA256: "fb0324f8ac057e50f4f4632331617e347bfe5a04184f7b0db514be682fb6b25c"},
	{Key: "jua", Face: "jua", Bytes: 2119352, SHA256: "769677aef240bfc3b9965f2b50748075bff885e6c6992fc591a3fb268279f898"},
	{Key: "nanummyeongjo", Face: "nanummyeongjo", Weight: 400, Bytes: 3058408, SHA256: "7ed9e8653a8ed04285d51dc343ffea6eb3d9c73afc27383ea8929ee4ffd03205"},
	{Key: "nanummyeongjo-800", Face: "nanummyeongjo", Weight: 800, Bytes: 3180888, SHA256: "60c0077fce069ba90ae97c0a3679f6eb3712e0ca637bdd0c15b72d335ec46db7"},
}

// The family resvg falls back to for text that names none, which no element in
// this package emits: every text carries its own font-family.
const fontFamily = "Pretendard Variable"

type Rendering struct {
	media *Adapter
	cfg   clip.RenderConfig
	// Every bundled file by its RenderConfig key, and the paths in the order
	// they are handed to resvg, so a render is byte-identical across processes.
	fonts    map[string]*sfnt.Font
	fontArgs []string
	coverage map[string]map[rune]bool
	overlays *overlay.Catalog
}

var _ clip.Renderer = (*Rendering)(nil)
var _ clip.CompositionExecutor = (*Rendering)(nil)

func (r *Rendering) CompositionPlanVersion() int { return clip.CompositionPlanVersion }

func NewRenderer(media *Adapter, cfg clip.RenderConfig) (*Rendering, error) {
	if cfg.OverlayBatchSize <= 0 || cfg.OverlayBatchSize > 16 || cfg.Composition.Cues <= 0 || cfg.MergeInputs < 2 || cfg.MergeInputs > 8 || cfg.SampleBatch < 3 || cfg.SampleBatch > 48 {
		return nil, errors.New("invalid clip composition renderer configuration")
	}
	if media == nil || cfg.FadeMS != design.Transition.FadeMS || cfg.FPS != 30 || cfg.MaxCuts <= 0 || cfg.MaxCopyRunes <= 0 || cfg.MinDurationMS != 15000 || cfg.MaxDurationMS != 90000 || !filepath.IsAbs(cfg.ResvgPath) {
		return nil, errors.New("invalid clip renderer configuration")
	}
	fonts := map[string]*sfnt.Font{}
	args := []string{}
	for _, f := range bundledFonts {
		path := cfg.FontPaths[f.Key]
		if !filepath.IsAbs(path) {
			return nil, errors.New("invalid clip renderer configuration")
		}
		font, err := bundledFace(path, f.Bytes, f.SHA256)
		if err != nil {
			return nil, err
		}
		fonts[f.Key], args = font, append(args, "--use-font-file", path)
	}
	if err := validateFontFamilies(fonts); err != nil {
		return nil, err
	}
	catalog, err := loadOverlays(cfg.OverlayDir)
	if err != nil {
		return nil, err
	}
	return &Rendering{media: media, cfg: cfg, fonts: fonts, fontArgs: args, coverage: faceCoverage(fonts), overlays: catalog}, nil
}

// bundledFace loads one pinned face: the size and the checksum both have to
// match, so a swapped or truncated file fails the constructor rather than the
// render.
func bundledFace(path string, size int64, sum string) (*sfnt.Font, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() != size {
		return nil, errors.New("bundled clip font is missing or changed")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	if hex.EncodeToString(hash[:]) != sum {
		return nil, errors.New("bundled clip font checksum mismatch")
	}
	return sfnt.Parse(data)
}

// family and face answer which bundled file a type role — or a caption style
// wearing one (CDS-18) — is set in. A role naming no bundled face is a
// configuration mistake the constructor already refused, so the lookup falls
// back to the default family rather than to a discovered one.
func (r *Rendering) family(role design.TypeRole) string {
	if family := design.FontFamily(role.Face); family != "" {
		return family
	}
	return fontFamily
}
func (r *Rendering) face(role design.TypeRole) *sfnt.Font {
	return r.fonts[fontFileKey(role.Face, role.Weight)]
}

// fontFileKey picks the file a face sets a given weight in. Only a face that
// ships more than one weight distinguishes them.
func fontFileKey(face string, weight int) string {
	for _, f := range bundledFonts {
		if f.Face == face && f.Weight == weight {
			return f.Key
		}
	}
	for _, f := range bundledFonts {
		if f.Face == face && f.Weight == 0 {
			return f.Key
		}
	}
	return "pretendard"
}

func escaped(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}
func (r *Rendering) checkCopy(text string, role design.TypeRole) error {
	for _, c := range text {
		if c == '\n' || c == '\u200d' || c == '\ufe0e' || c == '\ufe0f' {
			continue
		}
		if unicode.IsControl(c) || c == '\ufffe' || c == '\uffff' {
			return clip.ErrInvalid
		}
	}
	// A face that cannot set a syllable is a style problem the caller resolves
	// by falling back to the default style (CDS-84); reaching the rasteriser
	// with one left is invalid input, because nothing may substitute a glyph.
	if r.MissingGlyph(text, role) != 0 {
		return clip.ErrInvalid
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
func measureSVG(values []string, weight int, tracking float64, family string) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="10000" height="500">`)
	for i, text := range values {
		fmt.Fprintf(&b, `<text id="m%d" x="0" y="200" xml:space="preserve" font-family="%s" font-weight="%d" font-size="100" letter-spacing="%.4f">%s</text>`, i, family, weight, tracking*100, escaped(text))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// Every bundled face is handed to resvg explicitly, and system discovery stays
// off: each text asks for one of their family names by hand (CLIP-13, CDS-84).
func (r *Rendering) resvg(ctx context.Context, ws clip.MediaWorkspace, args ...string) ([]byte, error) {
	head := append([]string{"--skip-system-fonts"}, r.fontArgs...)
	head = append(head, "--font-family", fontFamily)
	return r.media.run(ctx, ws, r.cfg.ResvgPath, append(head, args...)...)
}
func (r *Rendering) measure(ctx context.Context, ws clip.MediaWorkspace, values []string, weight int, tracking float64, family string) (map[string]clip.Region, error) {
	path := filepath.Join(ws.Path, "copy-measure.svg")
	data := []byte(measureSVG(values, weight, tracking, family))
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
	// The approved style this caption was laid out in: the face it is set in,
	// the colour it is painted and the motion it declares all come from here
	// (CDS-80), and Style is the same style's layout contract.
	Caption  design.CaptionStyle
	Style    design.StyleRule
	Role     design.TypeRole
	Lines    []string
	Bounds   []clip.Region
	Region   clip.Region
	FontSize float64
	Keyword  keywordSpan
	// Per line, the words a per-word style needs; empty for every other style.
	Words [][]wordSpan
}

// One measured word on its line, for the styles that light words one at a time.
type wordSpan struct {
	Text   string
	Offset float64 // advance of the line up to this word's left edge
	Width  float64
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

// captionStyle resolves the style a copy names. A style id outside the approved
// set never reaches a rasterisation from a new composition — the layout refuses
// it as an authoring error first (CDS-66). What does reach here is a retired
// name on a stored plan, which renders in the default treatment it already
// rendered in when the set carried one style.
func captionStyle(id string) design.CaptionStyle {
	if style, ok := design.LookupCaptionStyle(id); ok {
		return style
	}
	return design.DefaultCaption()
}

// The fit loop searches only between the role's nominal size and its minimum
// (CDS-19); below the minimum the copy does not fit and is refused, never shrunk.
func fitCopy(canvas clip.Canvas, c clip.Copy, candidates [][]string, bounds map[string]clip.Region) (copyLayout, error) {
	caption := captionStyle(c.Style)
	style := caption.Rule()
	role := caption.Role()
	left, right, vertical := copyInsets(style)
	// The style's per-line character rule is part of the fit, not a verdict on
	// it: a sentence that is one character over on one line has a two-line
	// arrangement that obeys the rule, and preferring the single line that fits
	// the canvas returned a layout the composition then refused (copy_limit).
	// A rapid phrase is one line by construction and carries its own limit.
	lineChars := style.Chars
	if c.Pace == "rapid" {
		lineChars = design.Rapid.MaxChars
	}
	// A two-line arrangement that breaks inside a word reads worse than a wider
	// one that breaks where the writer put a space, so a clean break wins even
	// when a mid-word candidate is narrower. Candidates are every grapheme split,
	// so a clean one exists whenever the sentence has a space to break at.
	cleanBreak := func(lines []string) bool {
		return len(lines) < 2 || strings.HasSuffix(lines[0], " ") || strings.HasPrefix(lines[1], " ")
	}
	// The owner's own size is the ONE size tried: the shrink-to-fit walk exists
	// to find a size nobody chose, and running it over a number the owner set
	// would quietly change it (CDS-82). The floor and the role's own size are
	// checked where the size is written, not here.
	largest, smallest := role.Size, role.Min
	if c.Size > 0 {
		largest, smallest = float64(c.Size), float64(c.Size)
	}
	for size := largest; size >= smallest; size-- {
		best := copyLayout{}
		bestWidth := math.Inf(1)
		bestClean := false
		for _, lines := range candidates {
			if len(lines) > style.Lines || lineChars > 0 && slices.ContainsFunc(lines, func(line string) bool { return design.Chars(line) > lineChars }) {
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
			if c.Placement != nil {
				// An owner placement replaces the anchor's result entirely: the
				// caption's measured bounds go where the owner put them, moved
				// back inside the safe area but never resized (CDS-82).
				region, err = clip.PlaceOwnerCopy(canvas, *c.Placement, math.Ceil(width), math.Ceil(height))
			}
			clean := cleanBreak(lines)
			if err == nil && (clean && !bestClean || clean == bestClean && width < bestWidth) {
				words := [][]wordSpan(nil)
				if caption.PerWord() {
					for _, line := range lines {
						words = append(words, wordsOn(line, bounds, factor))
					}
				}
				best = copyLayout{caption, style, role, lines, scaled, region, size, keywordOn(lines, c.Keyword, bounds, factor), words}
				bestWidth, bestClean = width, clean
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

// Elements returns what this copy places, for the manifest the verifier reads.
// The window is already on the output timeline.
func (l copyLayout) Elements(cut, copy int, c clip.Copy, startMS, endMS int) clip.Manifest {
	m := clip.Manifest{}
	// The style the manifest names is the one the copy carries, not the one the
	// layout resolved it to: a stored plan's retired style name is its own
	// history, and a composition that fell back under CDS-84 already carries
	// the style it fell back to.
	motion := design.CaptionMotion(c.Style, c.Pace)
	add := func(kind string, region clip.Region, size float64, fill, background string) {
		m = append(m, design.Element{Cut: cut, Copy: copy, Kind: kind, Style: c.Style, Anchor: c.Anchor, Pace: c.Pace, Region: design.Bounds(region),
			StartMS: startMS, EndMS: endMS, FontSize: size, Fill: fill, Background: background,
			OwnerPlaced: c.Placement != nil,
			InMS:        motion.InMS, OutMS: motion.OutMS, DY: motion.InDY})
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
	fill := l.Caption.Paint.Fill
	for i, line := range l.Lines {
		bounds := l.Bounds[i]
		x := p.X + left + (inner-bounds.Width)/2
		if accent != "" && s.Highlight && l.Keyword.Present && l.Keyword.Line == i {
			u := design.Spacing.UnderlineMark
			add("highlight", clip.Region{X: x + l.Keyword.Offset - u.Extend, Y: top + (1-u.RaiseEM-u.HeightEM)*l.FontSize, Width: l.Keyword.Width + 2*u.Extend, Height: u.HeightEM * l.FontSize}, 0, accent, "")
		}
		background := design.Color[s.Plate].Hex
		if s.Stroke != "" && l.Caption.DarkStroke() {
			stroke := design.Color["stroke_dark"]
			background, _ = design.Over(stroke.Hex, stroke.Alpha, "#FFFFFF")
		}
		add("copy", clip.Region{X: x, Y: top, Width: bounds.Width, Height: bounds.Height}, l.FontSize, fill, background)
		_ = line
		top += bounds.Height + l.FontSize*(l.Role.LineHeight-1)
	}
	return m
}

// One measurement pass covers every candidate line and, when the copy names a
// keyword, the prefix that precedes it on each line that holds it: its advance
// is where the highlight starts (CDS-26), never an estimated width.
func (r *Rendering) layoutCopy(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy) (copyLayout, error) {
	caption := captionStyle(c.Style)
	style := caption.Rule()
	if err := r.checkCopy(c.Text, caption.Role()); err != nil {
		return copyLayout{}, err
	}
	candidates, values, err := copyCandidates(c.Text, style.Lines)
	if err != nil {
		return copyLayout{}, err
	}
	extras := []string(nil)
	if c.Keyword != "" {
		extras = append(extras, keywordValues(candidates, c.Keyword)...)
	}
	if caption.PerWord() {
		extras = append(extras, wordValues(candidates)...)
	}
	if len(extras) > 0 {
		seen := map[string]bool{}
		for _, v := range values {
			seen[v] = true
		}
		for _, extra := range extras {
			if extra != "" && !seen[extra] {
				values, seen[extra] = append(values, extra), true
			}
		}
	}
	role := caption.Role()
	bounds, err := r.measure(ctx, ws, values, role.Weight, role.Tracking, r.family(role))
	if err != nil {
		return copyLayout{}, err
	}
	return fitCopy(canvas, c, candidates, bounds)
}

// wordsOn measures each word of a line the way keywordOn measures the keyword:
// from the line UP TO AND INCLUDING that word, never from the prefix alone,
// because a prefix can end in a space, which has an advance but no ink box.
func wordsOn(line string, bounds map[string]clip.Region, factor float64) []wordSpan {
	whole := bounds[line]
	out := []wordSpan{}
	at := 0
	for _, word := range strings.Split(line, " ") {
		if word == "" {
			at++
			continue
		}
		through, box := bounds[line[:at+len(word)]], bounds[word]
		out = append(out, wordSpan{Text: word, Width: box.Width * factor,
			Offset: (through.X + through.Width - box.Width - whole.X) * factor})
		at += len(word) + 1
	}
	return out
}

// Every word of every candidate line, and that line up to and including each of
// them, so one measurement pass covers a per-word style's whole layout.
func wordValues(candidates [][]string) []string {
	out := []string{}
	for _, lines := range candidates {
		for _, line := range lines {
			at := 0
			for _, word := range strings.Split(line, " ") {
				if word != "" {
					out = append(out, word, line[:at+len(word)])
				}
				at += len(word) + 1
			}
		}
	}
	return out
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

// The disclosure badge (CDS-31) and legacy information pairs (CDS-30).
// Plans with information are routed through the sampled composition renderer.
type chip struct {
	Label, Value             string
	Region                   clip.Region
	LabelWidth               float64
	LabelBounds, ValueBounds clip.Region
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
	groups := map[string][]string{}
	if phrase != "" {
		groups["badge"] = []string{phrase}
	}
	for _, label := range labels {
		if value := strings.TrimSpace(answers[label]); value != "" {
			groups["label"] = append(groups["label"], label)
			groups["caption"] = append(groups["caption"], value)
		}
	}
	bounds := map[string]clip.Region{}
	for _, name := range []string{"badge", "label", "caption"} {
		values := groups[name]
		if len(values) == 0 {
			continue
		}
		role := design.Type[name]
		if name == "label" {
			role.Tracking = design.Information.LabelTracking
		}
		if err := r.checkCopy(strings.Join(values, ""), role); err != nil {
			return furniture{}, err
		}
		measured, err := r.measure(ctx, ws, values, role.Weight, role.Tracking, r.family(role))
		if err != nil {
			return furniture{}, err
		}
		for text, box := range measured {
			bounds[furnitureKey(name, text)] = box
		}
	}
	return placeFurniture(canvas, ratio, phrase, labels, answers, bounds)
}

func furnitureKey(role, value string) string { return role + "\x00" + value }

// placeFurniture puts the disclosure badge at its ratio's fixed corner and
// stacks at most two chips from the priority it was given (CDS-30, CDS-31).
func placeFurniture(canvas clip.Canvas, ratio, phrase string, labels []string, answers map[string]string, bounds map[string]clip.Region) (furniture, error) {
	out := furniture{BadgeText: phrase}
	l, ok := design.Layout(ratio)
	if !ok {
		return out, clip.ErrInvalid
	}
	badgeRole, labelRole, valueRole := design.Type["badge"], design.Type["label"], design.Type["caption"]
	b := scaled(bounds[furnitureKey("badge", phrase)], badgeRole.Size/100)
	out.BadgeBounds = b
	pad, gap := design.Spacing.PadChip, design.Spacing.GapStack
	badgeHeight := badgeRole.Size + 2*pad.V
	width := math.Ceil(b.Width + 2*pad.H)
	if phrase != "" {
		out.Badge = clip.Region{X: l.Badge.Right - width, Y: l.Badge.Top, Width: width, Height: badgeHeight}
	}
	if phrase != "" && !inside(out.Badge, clip.Region{X: l.Anchor.Left, Y: canvas.Safe.Y, Width: l.Badge.Right - l.Anchor.Left, Height: canvas.Safe.Height}) {
		return out, clip.ErrInvalid
	}
	x, y := l.Anchor.Left, l.Badge.Top
	for _, label := range labels {
		if len(out.Chips) >= maxChips {
			break
		}
		value := strings.TrimSpace(answers[label])
		if value == "" {
			continue
		}
		lb := scaled(bounds[furnitureKey("label", label)], labelRole.Size/100)
		vb := scaled(bounds[furnitureKey("caption", value)], valueRole.Size/100)
		w := math.Ceil(math.Max(lb.Width, vb.Width))
		h := math.Ceil(math.Max(lb.Height, labelRole.Size) + gap + math.Max(vb.Height, valueRole.Size))
		if w > l.CopyMaxWidth {
			return out, clip.ErrCopyTooLong
		}
		right := canvas.Safe.X + canvas.Safe.Width
		if phrase != "" && y < out.Badge.Y+out.Badge.Height {
			right = math.Min(right, out.Badge.X-gap)
		}
		if x+w > right {
			x = l.Anchor.Left
			y += h + gap
		}
		box := clip.Region{X: x, Y: y, Width: w, Height: h}
		if !inside(box, canvas.Safe) {
			return out, clip.ErrCopyTooLong
		}
		out.Chips = append(out.Chips, chip{Label: label, Value: value, LabelWidth: lb.Width, LabelBounds: lb, ValueBounds: vb, Region: box})
		x += w + gap
	}
	rowHeight := badgeHeight
	for _, c := range out.Chips {
		if c.Region.Y == l.Badge.Top {
			rowHeight = math.Max(rowHeight, c.Region.Height)
		}
	}
	if phrase != "" {
		out.Badge.Y += (rowHeight - badgeHeight) / 2
	}

	return out, nil
}

// CDS-30 shows at most two chips at once, and CDS-31 fixes the badge's padding.
const maxChips = 2

func scaled(r clip.Region, factor float64) clip.Region {
	return clip.Region{X: r.X * factor, Y: r.Y * factor, Width: r.Width * factor, Height: r.Height * factor}
}

// Elements places the badge for the whole clip and each chip for its own cut.
func (f furniture) Elements(duration int, chipCut int, chipStart, chipEnd int) clip.Manifest {
	m := clip.Manifest{}
	if f.BadgeText != "" {
		m = append(m, design.Element{
			Kind: "badge", Text: f.BadgeText, FontSize: design.Type["badge"].Size,
			Background: design.Color["badge_ad"].Hex, Fill: design.Color["text_white"].Hex,
			Region: design.Bounds(f.Badge), StartMS: 0, EndMS: duration,
		})
	}
	for _, c := range f.Chips {
		for i, role := range []string{"label", "caption"} {
			text, b, alpha, y := c.Label, c.LabelBounds, design.Color["text_muted"].Alpha, c.Region.Y
			if i == 1 {
				text, b, alpha, y = c.Value, c.ValueBounds, 1, y+math.Max(design.Type["label"].Size, c.LabelBounds.Height)+design.Spacing.GapStack
			}
			m = append(m, design.Element{Cut: chipCut, Kind: "chip", TypeRole: role, Text: text,
				FontSize: design.Type[role].Size, Fill: design.Color["text_white"].Hex, Opacity: alpha,
				Region: design.Bounds{X: c.Region.X + (c.Region.Width-b.Width)/2, Y: y, Width: b.Width, Height: b.Height}, StartMS: chipStart, EndMS: chipEnd})
		}
	}
	return m
}

func (r *Rendering) copyPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c clip.Copy, layout copyLayout, index int, ground Luminance) (string, error) {
	if strings.TrimSpace(c.Text) == "" {
		return "", nil
	}
	svg, err := r.overlays.Render("copy.bold", copyView(canvas, c, layout, ground))
	if err != nil {
		return "", err
	}
	region := copyCrop(canvas, c, layout, ground)
	if c.Pace == "rapid" {
		svg = fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="%.0f %.0f %.0f %.0f">%s</svg>`, region.Width, region.Height, region.X, region.Y, region.Width, region.Height, svg)
		canvas.Width, canvas.Height = int(region.Width), int(region.Height)
	}
	return r.rasterize(ctx, ws, canvas, svg, fmt.Sprintf("copy-%04d", index))
}

// furniturePlate is the fixed layer: the disclosure badge and this cut's chips,
// drawn in CDS-45's order under the copy and never animated.
func (r *Rendering) furniturePlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, f furniture, index int) (string, error) {
	if f.BadgeText == "" && len(f.Chips) == 0 {
		return "", nil
	}
	svg, err := r.overlays.Render("furniture", furnitureView(canvas, f))
	if err != nil {
		return "", err
	}
	return r.rasterize(ctx, ws, canvas, svg, fmt.Sprintf("fixed-%04d", index))
}

func (r *Rendering) rasterize(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, body, name string) (string, error) {
	png := filepath.Join(ws.Path, name+".png")
	box := clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}
	if err := r.rasterizeTo(ctx, ws, box, body, png); err != nil {
		return "", err
	}
	return png, nil
}

// rasterizeTo draws one SVG into one named PNG. The box is the pixels it covers,
// which for a sequence layer is its own crop rather than the whole canvas: the
// reservation CLIP-33 accounts for is the layer's, not the frame's.
func (r *Rendering) rasterizeTo(ctx context.Context, ws clip.MediaWorkspace, box clip.Region, body, png string) error {
	svg := strings.TrimSuffix(png, filepath.Ext(png)) + ".svg"
	data := []byte(body)
	// An RGBA layer plus PNG/metadata headroom is small and known before
	// rasterization. Reserve it before the subprocess, not after its write.
	if err := r.media.capacity(ws, int64(len(data))+int64(box.Width)*int64(box.Height)*5); err != nil {
		return err
	}
	if err := os.WriteFile(svg, data, 0600); err != nil {
		return err
	}
	defer os.Remove(svg)
	if _, err := r.resvg(ctx, ws, svg, png); err != nil {
		return err
	}
	return r.media.workspaceFile(ws, png)
}

// FixedElements places the disclosure badge and a cut's chips and returns them
// as manifest elements, so the composer can keep copy off them (CDS-45). Like
// CaptionSize it needs no source pixels and scopes its SVG to its own workspace.
func (r *Rendering) FixedElements(ctx context.Context, ratio, disclosure string, labels []string, answers []clip.Answer, hideDisclosure ...bool) (out clip.Manifest, err error) {
	canvas, err := clip.ClipCanvas(ratio)
	if err != nil {
		return nil, err
	}
	phrase, ok := design.Disclosure[disclosure]
	if !ok {
		return nil, clip.ErrDisclosureRequired
	}
	if len(hideDisclosure) > 0 && hideDisclosure[0] {
		phrase = ""
	}
	texts := map[string]string{}
	for _, a := range answers {
		texts[a.Label] = a.Text
	}
	err = r.media.WithWorkspace(ctx, "clip-fixed-elements", func(ws clip.MediaWorkspace) error {
		f, err := r.badgeAndChips(ctx, ws, canvas, ratio, phrase, labels, texts)
		if err != nil {
			return err
		}
		// The window is the composer's concern; only the regions matter here.
		out = f.Elements(1, 0, 0, 1)
		return nil
	})
	return out, err
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
	if err := r.checkCopy(c.Text, captionStyle(c.Style).Role()); err != nil {
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
