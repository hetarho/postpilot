package design

import (
	"fmt"
	"math"
	"strings"
)

// The sequence-rendered styles (CDS-80, CDS-81). Each draws one layer per output
// frame, so its motion lives in the drawing rather than in a filter over one
// looped plate: that is exactly what the overlay chain cannot express and what
// the extra rasterisation per frame buys.
//
// Every filter here declares `color-interpolation-filters="sRGB"`. resvg
// computes filters in linearRGB by default and the colour then differs from what
// a browser draws, which CDS-83 forbids: the owner places a caption by what the
// preview shows. `feDisplacementMap` is admitted to no style, because resvg
// 0.48.1 places its result at the wrong offset.
//
// DrawCaptionFrame returns the <defs> and the body for one frame of one
// sequence style. A static style is not drawn here: it keeps the bundled
// overlay template and one rasterisation (CDS-81).
func DrawCaptionFrame(f CaptionFrame) (defs, body string, ok bool) {
	draw, ok := captionPainters[f.Style.ID]
	if !ok {
		return "", "", false
	}
	defs, body = draw(f)
	return defs, body, true
}

type captionPainter func(CaptionFrame) (string, string)

var captionPainters = map[string]captionPainter{
	"word-pop":   drawWordPop,
	"blur-in":    drawBlurIn,
	"ambient":    drawAmbient,
	"neon":       drawNeon,
	"iridescent": drawIridescent,
	"glitch":     drawGlitch,
	"ember":      drawEmber,
	"stack":      drawStack,
	"outline":    drawOutline,
	"pop":        drawPop,
	"sticker":    drawSticker,
	"bubble":     drawBubble,
	"serif":      drawSerif,
}

// SequenceStylesAreDrawable reports whether every sequence-rendered style in the
// approved set has a painter. A style the set carries and nothing can draw would
// fail at the rasterisation, long after a project selected it.
func SequenceStylesAreDrawable() bool {
	for _, style := range captionStyles {
		if !style.Static() && captionPainters[style.ID] == nil {
			return false
		}
	}
	return true
}

// text emits one line in the frame's own face, size and tracking. Only the paint
// and the extra attributes differ between styles.
func (f CaptionFrame) text(l CaptionLine, x, y float64, extra string) string {
	return fmt.Sprintf(`<text x="%s" y="%s" xml:space="preserve" font-family="%s" font-size="%s" font-weight="%d" letter-spacing="%s"%s>%s</text>`,
		num(x), num(y), f.Family, num(f.Size), f.Style.Weight, num(f.Tracking*f.Size), extra, esc(l.Text))
}

// stroke is the style's own outline at the width CDS-21's token fixes.
func (f CaptionFrame) stroke() string {
	if f.Style.Paint.Stroke == "" {
		return ""
	}
	return fmt.Sprintf(` stroke="%s" stroke-width="%s" stroke-linejoin="round" paint-order="stroke"`, f.Style.Paint.Stroke, num(f.Style.Rule().StrokeWidth()))
}

// accentOr falls back to the style's fill where the project chose no accent, so
// a style never invents a colour CDS-15 did not approve.
func (f CaptionFrame) accentOr(fallback string) string {
	if f.Accent != "" {
		return f.Accent
	}
	return fallback
}

// group wraps a body in the opacity and offset the style's declared motion puts
// on this frame.
func group(op, dy float64, body string) string {
	return fmt.Sprintf(`<g opacity="%s" transform="translate(0,%s)">%s</g>`, num(op), num(dy), body)
}

// softShadow is the drop shadow several styles put under their text, built from
// the style's own ShadowPaint so the registry stays the one place it is stated.
func softShadow(id string, s ShadowPaint) string {
	return fmt.Sprintf(`<filter id="%s" x="-30%%" y="-30%%" width="160%%" height="180%%" color-interpolation-filters="sRGB">`+
		`<feOffset in="SourceAlpha" dx="%s" dy="%s" result="o"/><feGaussianBlur in="o" stdDeviation="%s" result="b"/>`+
		`<feFlood flood-color="%s" flood-opacity="%s"/><feComposite in2="b" operator="in" result="s"/>`+
		`<feMerge><feMergeNode in="s"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`,
		id, num(s.DX), num(s.DY), num(s.Blur/2), s.Hex, num(s.Alpha))
}

// 워드 팝 — the word being spoken lives and the rest recede. The spoken word
// takes the accent where the project chose one (CDS-15).
func drawWordPop(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	words := 0
	for _, l := range fr.Lines {
		words += len(l.Words)
	}
	active := int(clamp01((fr.Progress-0.10)/0.74) * float64(words))
	var b strings.Builder
	seen := 0
	for _, l := range fr.Lines {
		for _, w := range l.Words {
			fill, alpha, scale := fr.Style.Paint.Fill, 0.22, 1.0
			switch {
			case seen < active:
				alpha = 0.62
			case seen == active:
				fill, alpha, scale = fr.accentOr(fr.Style.Paint.Fill), 1.0, 1.07
			}
			one := CaptionLine{Text: w.Text}
			g := fr.text(one, w.X, l.Y, fmt.Sprintf(` fill="%s" fill-opacity="%s"%s stroke-opacity="%s"`, fill, num(alpha), fr.stroke(), num(alpha*0.9)))
			if scale != 1 {
				cx, cy := w.X+w.Width/2, l.Y-fr.Size*0.3
				g = fmt.Sprintf(`<g transform="translate(%s,%s) scale(%s) translate(%s,%s)">%s</g>`, num(cx), num(cy), num(scale), num(-cx), num(-cy), g)
			}
			b.WriteString(g)
			seen++
		}
	}
	return "", group(op, 0, b.String())
}

// 블러 인 — out of blur into focus, the entrance every vlog opens on.
func drawBlurIn(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	p := easeOutCubic(fr.settled())
	deviation, scale := 26*(1-p), 1.05-0.05*p
	cx, cy := fr.Region.X+fr.Region.Width/2, fr.Region.Y+fr.Region.Height/2
	defs := fmt.Sprintf(`<filter id="cbi" x="-30%%" y="-40%%" width="160%%" height="180%%" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="%s"/></filter>`, num(math.Max(deviation, 0.01)))
	var b strings.Builder
	fmt.Fprintf(&b, `<g transform="translate(%s,%s) scale(%s) translate(%s,%s)">`, num(cx), num(cy), num(scale), num(-cx), num(-cy))
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s" filter="url(#cbi)"`, fr.Style.Paint.Fill)))
	}
	b.WriteString(`</g>`)
	return defs, group(op, 0, b.String())
}

// 소프트 앰비언트 — the colour breathes behind the letters and never touches
// them. The light is drawn as gradients rather than pushed through a filter: a
// blur wide enough to read as a glow loses its alpha and disappears.
func drawAmbient(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	breath := 0.86 + 0.14*math.Sin(fr.Progress*2*math.Pi*0.9)
	cx := fr.Region.X + fr.Region.Width/2
	cy := fr.Region.Y + fr.Region.Height/2 - fr.Size*0.25
	rx, ry := fr.Region.Width*0.62, fr.Size*1.15
	defs := `<radialGradient id="cambA" cx="50%" cy="50%" r="50%">` +
		`<stop offset="0" stop-color="#8B5CF6" stop-opacity="0.95"/><stop offset="0.55" stop-color="#6D3BF5" stop-opacity="0.42"/>` +
		`<stop offset="1" stop-color="#4C1D95" stop-opacity="0"/></radialGradient>` +
		`<radialGradient id="cambB" cx="50%" cy="50%" r="50%">` +
		`<stop offset="0" stop-color="#4AD9E8" stop-opacity="0.9"/><stop offset="0.5" stop-color="#22B8CF" stop-opacity="0.34"/>` +
		`<stop offset="1" stop-color="#0E7490" stop-opacity="0"/></radialGradient>` +
		`<filter id="cambsoft" x="-40%" y="-60%" width="180%" height="220%" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="26"/></filter>` +
		softShadow("cambtext", fr.Style.Paint.Shadow)
	var b strings.Builder
	fmt.Fprintf(&b, `<g filter="url(#cambsoft)"><ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="url(#cambA)"/><ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="url(#cambB)"/></g>`,
		num(cx-fr.Region.Width*0.18), num(cy), num(rx*breath), num(ry*breath),
		num(cx+fr.Region.Width*0.22), num(cy+fr.Size*0.18), num(rx*0.72*breath), num(ry*0.86*breath))
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s" filter="url(#cambtext)"`, fr.Style.Paint.Fill)))
	}
	return defs, group(op, dy, b.String())
}

// 네온 사인 — one colour with its core kept, pulsing, and flickering on a beat
// the frame index alone decides so two runs agree.
func drawNeon(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	pulse := 0.80 + 0.20*math.Sin(fr.Progress*2*math.Pi*3)
	// The flicker is a function of the progress, never of a random source: a
	// render has to be reproducible frame for frame (CDS-7).
	if fr.Progress > 0.12 && math.Mod(math.Floor(fr.Progress*97), 19) == 0 {
		pulse *= 0.42
	}
	defs := fmt.Sprintf(`<filter id="cneon" x="-60%%" y="-70%%" width="220%%" height="240%%" color-interpolation-filters="sRGB">`+
		`<feGaussianBlur in="SourceAlpha" stdDeviation="%s" result="b1"/>`+
		`<feFlood flood-color="#22D3EE" flood-opacity="%s"/><feComposite in2="b1" operator="in" result="n1"/>`+
		`<feGaussianBlur in="SourceAlpha" stdDeviation="%s" result="b2"/>`+
		`<feFlood flood-color="#7DF9FF" flood-opacity="%s"/><feComposite in2="b2" operator="in" result="n2"/>`+
		`<feMerge><feMergeNode in="n1"/><feMergeNode in="n1"/><feMergeNode in="n2"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`,
		num(26*pulse), num(0.9*pulse), num(8*pulse), num(0.95*pulse))
	var b strings.Builder
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s" filter="url(#cneon)"`, fr.Style.Paint.Fill)))
	}
	return defs, group(op, dy, b.String())
}

// 이리데센트 — a hologram rather than a gold bevel: the colour flows along the
// letters and the angle turns with it.
func drawIridescent(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	shift := math.Mod(fr.Progress*0.9, 1.0)
	stops := ""
	for _, s := range []struct {
		at    float64
		color string
	}{{-0.45, "#5EEAD4"}, {-0.18, "#818CF8"}, {0.1, "#F472B6"}, {0.38, "#FDBA74"}, {0.66, "#5EEAD4"}, {0.95, "#818CF8"}} {
		stops += fmt.Sprintf(`<stop offset="%s" stop-color="%s"/>`, num(clamp01(s.at+shift)), s.color)
	}
	defs := fmt.Sprintf(`<linearGradient id="cirid" x1="0" y1="0.15" x2="1" y2="0.85">%s</linearGradient>`, stops) +
		`<filter id="ciridsh" x="-30%" y="-30%" width="160%" height="170%" color-interpolation-filters="sRGB">` +
		`<feGaussianBlur in="SourceAlpha" stdDeviation="16" result="b"/>` +
		`<feFlood flood-color="#7C3AED" flood-opacity="0.55"/><feComposite in2="b" operator="in" result="g"/>` +
		`<feMerge><feMergeNode in="g"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`
	var b strings.Builder
	b.WriteString(`<g filter="url(#ciridsh)">`)
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, ` fill="url(#cirid)" stroke-opacity="0.55"`+fr.stroke()))
	}
	b.WriteString(`</g>`)
	return defs, group(op, dy, b.String())
}

// 글리치 — red and cyan pull apart and horizontal bands jump. The break differs
// frame by frame but is derived from the frame's own progress, never sampled.
func drawGlitch(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	step := math.Floor(fr.Progress * 900)
	burst := 0.5
	if fr.Progress < 0.14 || math.Mod(step, 5) < 2 {
		burst = 1.0
	}
	offsetX := (7 + math.Mod(step*7, 12)) * burst
	if math.Mod(step, 2) == 1 {
		offsetX = -offsetX
	}
	offsetY := (math.Mod(step*3, 6) - 3) * burst
	var defs, b strings.Builder
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X-offsetX, l.Y+offsetY, ` fill="#FF2D55" opacity="0.9"`))
		b.WriteString(fr.text(l, l.X+offsetX, l.Y-offsetY, ` fill="#00E5FF" opacity="0.9"`))
	}
	for i, l := range fr.Lines {
		height := l.Height / 6
		for band := 0; band < 6; band++ {
			y := l.Top + float64(band)*height
			shift := 0.0
			if math.Mod(step+float64(band*13+i*7), 3) != 0 {
				shift = (math.Mod(step*11+float64(band*29), 72) - 36) * burst
			}
			id := fmt.Sprintf("cgs%d-%d", i, band)
			fmt.Fprintf(&defs, `<clipPath id="%s"><rect x="0" y="%s" width="%d" height="%s"/></clipPath>`, id, num(y), fr.Canvas.Width, num(height+1))
			fmt.Fprintf(&b, `<g clip-path="url(#%s)">%s</g>`, id, fr.text(l, l.X+shift, l.Y, fmt.Sprintf(` fill="%s"`, fr.Style.Paint.Fill)))
		}
	}
	return defs.String(), group(op, 0, b.String())
}

// 엠버 글로우 — the flame rises from inside the letters and wraps them; laid on
// top alone it floats like a candle. The tongues and sparks are placed by index
// rather than by a random source, so two renders agree (CDS-7).
func drawEmber(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	defs := `<radialGradient id="cembt" cx="50%" cy="88%" r="68%">` +
		`<stop offset="0" stop-color="#FFF6DC"/><stop offset="0.28" stop-color="#FFB443"/>` +
		`<stop offset="0.62" stop-color="#FF6A2B" stop-opacity="0.85"/><stop offset="1" stop-color="#E0431B" stop-opacity="0"/></radialGradient>` +
		`<filter id="cembsoft" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="12"/></filter>` +
		`<filter id="cembsharp" color-interpolation-filters="sRGB"><feGaussianBlur stdDeviation="4.5"/></filter>` +
		`<filter id="cembglow" x="-60%" y="-70%" width="220%" height="250%" color-interpolation-filters="sRGB">` +
		`<feGaussianBlur in="SourceAlpha" stdDeviation="30" result="b0"/>` +
		`<feFlood flood-color="#FF5A1F" flood-opacity="0.8"/><feComposite in2="b0" operator="in" result="g0"/>` +
		`<feGaussianBlur in="SourceAlpha" stdDeviation="9" result="b1"/>` +
		`<feFlood flood-color="#FFB443" flood-opacity="0.8"/><feComposite in2="b1" operator="in" result="g1"/>` +
		`<feMerge><feMergeNode in="g0"/><feMergeNode in="g0"/><feMergeNode in="g1"/><feMergeNode in="SourceGraphic"/></feMerge></filter>`
	tongue := func(px, base, height, width, phase float64) string {
		h := height * (1 + 0.30*math.Sin(fr.Progress*2*math.Pi*2.4+phase))
		w := width * (1 + 0.16*math.Cos(fr.Progress*2*math.Pi*1.5+phase))
		lean := math.Sin(fr.Progress*2*math.Pi*1.8+phase) * h * 0.17
		return fmt.Sprintf(`<path d="M%s %s C %s %s %s %s %s %s C %s %s %s %s %s %s Z" fill="url(#cembt)"/>`,
			num(px), num(base), num(px-w), num(base-h*0.32), num(px-w*0.5+lean), num(base-h*0.7), num(px+lean), num(base-h),
			num(px+w*0.6+lean), num(base-h*0.68), num(px+w), num(base-h*0.3), num(px), num(base))
	}
	box := fr.Region
	mid := box.Y + box.Height*0.58
	var back, front, sparks strings.Builder
	n := int(math.Max(9, box.Width/88))
	for i := 0; i < n; i++ {
		x := box.X - 16 + (box.Width+32)*(float64(i)+0.5)/float64(n)
		back.WriteString(tongue(x, mid+box.Height*0.26, 210+math.Mod(float64(i)*47, 110), 44+math.Mod(float64(i)*13, 24), float64(i)*0.9))
	}
	nf := n * 8 / 5
	for i := 0; i < nf; i++ {
		x := box.X - 22 + (box.Width+44)*(float64(i)+0.5)/float64(nf)
		front.WriteString(tongue(x, mid, 105+math.Mod(float64(i)*37, 95), 19+math.Mod(float64(i)*7, 15), float64(i)*0.6))
	}
	for i := 0; i < 16; i++ {
		seed := float64(i)
		// An integer multiple of one has no fractional part, so the spread comes
		// from an irrational step: every spark would otherwise start at the same
		// x and the flame would rise in a single column.
		x := box.X + math.Mod(seed*0.6180339887, 1)*box.Width
		life := math.Mod(fr.Progress*1.3+math.Mod(seed*0.37, 1), 1.0)
		radius := 4.6 * (1 - life) * (0.5 + math.Mod(seed*0.21, 0.6))
		if radius <= 0.6 {
			continue
		}
		fmt.Fprintf(&sparks, `<circle cx="%s" cy="%s" r="%s" fill="#FFC978" opacity="%s"/>`,
			num(x+math.Sin(life*7+seed)*20), num(box.Y-30-life*250), num(radius), num((1-life)*0.85))
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<g filter="url(#cembsoft)" opacity="0.9">%s</g>`, back.String())
	b.WriteString(`<g filter="url(#cembglow)">`)
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"%s`, fr.Style.Paint.Fill, fr.stroke())))
	}
	b.WriteString(`</g>`)
	fmt.Fprintf(&b, `<g filter="url(#cembsharp)" opacity="0.86">%s</g>%s`, front.String(), sparks.String())
	return defs, group(op, dy, b.String())
}

// 스택 블록 — each line has its own block and slides in after the one above.
func drawStack(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	var b strings.Builder
	for i, l := range fr.Lines {
		p := easeOutCubic(clamp01((fr.Progress - 0.06 - float64(i)*0.12) / 0.34))
		plate, ink := fr.Style.Paint.Plate, fr.Style.Paint.Fill
		if i%2 == 1 {
			plate, ink = fr.accentOr("#FFFFFF"), "#10130A"
		}
		height := l.Height + 30
		fmt.Fprintf(&b, `<g opacity="%s" transform="translate(%s,0)">`, num(p), num((1-p)*-70))
		fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="10" fill="%s" fill-opacity="0.94"/>`,
			num(l.X-26), num(l.Top-15), num(l.Width+52), num(height), plate)
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"`, ink)))
		b.WriteString(`</g>`)
	}
	return "", group(op, 0, b.String())
}

// 아웃라인 — the outline alone, and only the accent word fills in.
func drawOutline(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	fill := easeOutCubic(clamp01((fr.Progress - 0.28) / 0.28))
	var defs, b strings.Builder
	for i, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="none" stroke="%s" stroke-width="%s" stroke-linejoin="round"`, fr.Style.Paint.Stroke, num(fr.Style.Rule().StrokeWidth()))))
		if l.Keyword == "" {
			continue
		}
		id := fmt.Sprintf("colc%d", i)
		fmt.Fprintf(&defs, `<clipPath id="%s"><rect x="%s" y="%s" width="%s" height="%s"/></clipPath>`,
			id, num(l.KeywordX-6), num(l.Y-fr.Size), num((l.KeywordWidth+12)*fill), num(fr.Size*1.6))
		fmt.Fprintf(&b, `<g clip-path="url(#%s)">%s</g>`, id, fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"`, fr.accentOr("#FFFFFF"))))
	}
	return defs.String(), group(op, dy, b.String())
}

// 팝 바운스 — the words pop out one at a time.
func drawPop(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	var b strings.Builder
	index := 0
	for _, l := range fr.Lines {
		for _, w := range l.Words {
			p := clamp01((fr.Progress - (0.05 + float64(index)*0.13)) / 0.30)
			scale := 0.001
			if p > 0 {
				scale = math.Max(easeOutBack(p), 0.001)
			}
			rotation := (1 - p) * 7
			if index%2 == 1 {
				rotation = -rotation
			}
			cx, cy := w.X+w.Width/2, l.Y-fr.Size*0.32
			one := CaptionLine{Text: w.Text}
			fmt.Fprintf(&b, `<g transform="translate(%s,%s) rotate(%s) scale(%s) translate(%s,%s)" opacity="%s">%s%s</g>`,
				num(cx), num(cy), num(rotation), num(scale), num(-cx), num(-cy), num(math.Min(1, p*2.2)),
				fr.text(one, w.X, l.Y, fmt.Sprintf(` fill="%s"%s`, fr.Style.Paint.Fill, fr.stroke())),
				fr.text(one, w.X, l.Y, fmt.Sprintf(` fill="%s"`, fr.Style.Paint.Fill)))
			index++
		}
	}
	return "", group(op, 0, b.String())
}

// 스티커 — stuck on and peeled off: it pops out and sits slightly askew.
func drawSticker(fr CaptionFrame) (string, string) {
	op, _ := fr.entrance()
	p := fr.settled()
	scale := math.Max(easeOutBack(p), 0.001)
	rotation := -2.4 + (1-p)*8
	box := fr.Region
	pw, ph := box.Width+72, box.Height+46
	px, py := box.X-36, box.Y-23
	cx, cy := box.X+box.Width/2, box.Y+box.Height/2
	defs := softShadow("cstksh", ShadowPaint{Hex: "#000000", Alpha: 0.38, Blur: 18, DY: 8})
	var b strings.Builder
	fmt.Fprintf(&b, `<g filter="url(#cstksh)" transform="translate(%s,%s) rotate(%s) scale(%s) translate(%s,%s)">`,
		num(cx), num(cy), num(rotation), num(scale), num(-cx), num(-cy))
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`, num(px), num(py), num(pw), num(ph), num(ph*0.34), fr.Style.Paint.Plate)
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="none" stroke="%s" stroke-width="3" stroke-opacity="0.85"/>`,
		num(px+6), num(py+6), num(pw-12), num(ph-12), num(ph*0.29), fr.accentOr("#FF3B6B"))
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"`, fr.Style.Paint.Fill)))
	}
	b.WriteString(`</g>`)
	return defs, group(op, 0, b.String())
}

// 버블 챗 — a tailed speech bubble rising from below.
func drawBubble(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	box := fr.Region
	pw, ph := box.Width+64, box.Height+40
	px, py := box.X-32, box.Y-20
	defs := softShadow("cbbsh", ShadowPaint{Hex: "#000000", Alpha: 0.42, Blur: 20, DY: 6})
	var b strings.Builder
	b.WriteString(`<g filter="url(#cbbsh)">`)
	fmt.Fprintf(&b, `<path d="M%s %s L%s %s L%s %s Z" fill="%s"/>`,
		num(px+34), num(py+ph), num(px+30), num(py+ph+26), num(px+72), num(py+ph), fr.Style.Paint.Plate)
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="%s" rx="%s" fill="%s"/>`, num(px), num(py), num(pw), num(ph), num(ph*0.42), fr.Style.Paint.Plate)
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"`, fr.Style.Paint.Fill)))
	}
	b.WriteString(`</g>`)
	return defs, group(op, dy, b.String())
}

// 세리프 미니멀 — a light serif set large and tracked open, with two rules for
// its only decoration.
func drawSerif(fr CaptionFrame) (string, string) {
	op, dy := fr.entrance()
	width := (fr.Region.Width + 40) * easeOutCubic(clamp01((fr.Progress-0.2)/0.42))
	x := fr.Region.X + fr.Region.Width/2 - width/2
	defs := softShadow("csmsh", fr.Style.Paint.Shadow)
	var b strings.Builder
	b.WriteString(`<g filter="url(#csmsh)">`)
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="1.4" fill="#E9E3D6" opacity="0.72"/>`, num(x), num(fr.Region.Y-26), num(width))
	for _, l := range fr.Lines {
		b.WriteString(fr.text(l, l.X, l.Y, fmt.Sprintf(` fill="%s"`, fr.Style.Paint.Fill)))
	}
	fmt.Fprintf(&b, `<rect x="%s" y="%s" width="%s" height="1.4" fill="#E9E3D6" opacity="0.72"/>`, num(x), num(fr.Region.Y+fr.Region.Height+24), num(width))
	b.WriteString(`</g>`)
	return defs, group(op, dy, b.String())
}
