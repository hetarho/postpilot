package media

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// The hook card (CDS-28) and the ending CTA card (CDS-29): the two moments a
// clip speaks for itself rather than about a cut. Both are plated, so neither
// needs brightness sampling (CDS-16), and both draw only the owner's own
// answers and the preset's own phrases — never model text, except the hook
// sentence itself.
type cardLine struct {
	Text  string
	Role  design.TypeRole
	Fill  string
	Alpha float64
	// A chip line is an accent pill with ink text, not a line of type.
	Chip bool
}
type cardLayout struct {
	Kind           string // hook | end
	Region         clip.Region
	Lines          []cardLine
	Bounds         []clip.Region
	StartMS, EndMS int
	Accent         string
}

func (c cardLayout) empty() bool { return len(c.Lines) == 0 }

// relativeTo moves the card's window from the output timeline onto its own
// cut's, which is the clock the filter graph runs on.
func (c cardLayout) relativeTo(offset int) cardLayout {
	c.StartMS, c.EndMS = c.StartMS-offset, c.EndMS-offset
	return c
}

// hookCard stacks the category chip, the hook sentence and the business name
// (CDS-28). Without a hook sentence there is no card: the first frame stays real
// footage either way, and an empty card would say nothing.
func hookCard(ratio, hook, preset, name, accent string, durationMS int) cardLayout {
	hook, name = strings.TrimSpace(hook), strings.TrimSpace(name)
	if hook == "" || name == "" {
		return cardLayout{Kind: "hook"}
	}
	l, ok := design.Layout(ratio)
	if !ok {
		return cardLayout{Kind: "hook"}
	}
	white, muted := design.Color["text_white"], design.Color["text_muted"]
	lines := []cardLine{}
	if label := design.Presets[preset].Label; label != "" && accent != "" {
		lines = append(lines, cardLine{Text: label, Role: design.Type["label"], Fill: design.Color["ink_900"].Hex, Alpha: 1, Chip: true})
	}
	role := design.Type["hook"]
	role.Size, role.Min = l.HookSize, math.Min(role.Min, l.HookSize)
	for _, line := range hookLines(hook) {
		lines = append(lines, cardLine{Text: line, Role: role, Fill: white.Hex, Alpha: white.Alpha})
	}
	lines = append(lines, cardLine{Text: name, Role: design.Type["body"], Fill: muted.Hex, Alpha: muted.Alpha})
	end := int(design.Timing.HookCardS * 1000)
	return cardLayout{Kind: "hook", Lines: lines, StartMS: 0, EndMS: min(end, durationMS), Accent: accent}
}

// hookLines breaks the hook into the at most two lines of nine CDS-28 allows: at
// the last word boundary that leaves a full first line, and mid-word only when
// the hook has no space to break at. A hook the compiler let through is short
// enough to fit; anything longer is truncated here rather than overflowing the
// card, and measureCard still shrinks a line that is wide in its own face.
func hookLines(hook string) []string {
	per := design.Type["hook"].Chars
	if written := strings.SplitN(hook, "\n", 2); len(written) == 2 {
		return []string{strings.TrimSpace(written[0]), strings.TrimSpace(written[1])}
	}
	if design.Chars(hook) <= per {
		return []string{hook}
	}
	runes, count, split, space := []rune(hook), 0, 0, 0
	for i, r := range runes {
		if unicode.IsSpace(r) {
			if count <= per {
				space = i
			}
			continue
		}
		count++
		if count <= per {
			split = i + 1
		}
	}
	if space > 0 {
		return []string{string(runes[:space]), strings.TrimSpace(string(runes[space:]))}
	}
	return []string{string(runes[:split]), string(runes[split:])}
}

// endingCard stacks the business name, the location, one price or signature line
// and the CTA the owner chose (CDS-29). Without a business name there is no
// card: a closing card that cannot say whose place it was has nothing to close.
func endingCard(ratio, preset, cta string, answers map[string]string, accent string, durationMS int) cardLayout {
	name := strings.TrimSpace(answers["상호"])
	if name == "" {
		return cardLayout{Kind: "end"}
	}
	if _, ok := design.Layout(ratio); !ok {
		return cardLayout{Kind: "end"}
	}
	white, muted := design.Color["text_white"], design.Color["text_muted"]
	lines := []cardLine{{Text: name, Role: design.Type["body"], Fill: white.Hex, Alpha: white.Alpha}}
	if location := strings.TrimSpace(answers["위치"]); location != "" {
		lines = append(lines, cardLine{Text: location, Role: design.Type["label"], Fill: muted.Hex, Alpha: muted.Alpha})
	}
	// One price or signature line, and only when the owner gave one.
	for _, label := range []string{"가격", "메뉴"} {
		if value := strings.TrimSpace(answers[label]); value != "" {
			line := cardLine{Text: value, Role: design.Type["caption"], Fill: white.Hex, Alpha: white.Alpha}
			if label == "가격" && design.Presets[preset].PriceNote != "" {
				line.Text = value + " " + design.Presets[preset].PriceNote
			}
			lines = append(lines, line)
			break
		}
	}
	if phrase := design.CTA[design.DefaultCTA(preset, cta)]; phrase != "" && accent != "" {
		lines = append(lines, cardLine{Text: phrase, Role: design.Type["label"], Fill: design.Accent[accent], Alpha: 1})
	}
	start := durationMS - int(design.Timing.EndCardS*1000)
	return cardLayout{Kind: "end", Lines: lines, StartMS: max(0, start), EndMS: durationMS, Accent: accent}
}

// measureCard measures every line in the face its role names and places the card
// at its ratio's own geometry (CDS-28, CDS-29, CDS-47, CDS-48).
func (r *Rendering) measureCard(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, ratio string, card cardLayout) (cardLayout, error) {
	if card.empty() {
		return card, nil
	}
	l, _ := design.Layout(ratio)
	box := l.HookCard
	if card.Kind == "end" {
		box = l.EndCard
	}
	pad, gap := cardPadding, design.Spacing.GapStack
	width, height := box.Width, 2*pad
	inner := width - 2*pad
	bounds := make([]clip.Region, len(card.Lines))
	for i, line := range card.Lines {
		if err := r.checkCopy(line.Text, line.Role); err != nil {
			return card, err
		}
		measured, err := r.measure(ctx, ws, []string{line.Text}, line.Role.Weight, line.Role.Tracking, r.family(line.Role))
		if err != nil {
			return card, err
		}
		b := scaled(measured[line.Text], line.Role.Size/100)
		// A line wider than the card is set at the size that fits it, never past
		// its role's own floor (CDS-19).
		if b.Width > inner && b.Width > 0 {
			size := math.Max(line.Role.Min, math.Floor(line.Role.Size*inner/b.Width))
			card.Lines[i].Role.Size = size
			b = scaled(measured[line.Text], size/100)
		}
		if line.Chip {
			b.Height += 2 * design.Spacing.PadChip.V
			b.Width += 2 * design.Spacing.PadChip.H
		}
		bounds[i] = b
		height += b.Height
		if i > 0 {
			height += gap
		}
	}
	card.Bounds = bounds
	card.Region = clip.Region{X: (float64(canvas.Width) - width) / 2, Y: box.CenterY - math.Ceil(height)/2, Width: width, Height: math.Ceil(height)}
	// CDS-28 states the 9:16 card as x 144–936, which is sixteen pixels wider
	// than CDS-9's safe area on each of two sides. The plate may sit there; its
	// TEXT may not, so what is checked against the safe area is the padded band
	// the lines are set in, and the plate only against the frame.
	band := clip.Region{X: card.Region.X + pad, Y: card.Region.Y + pad, Width: inner, Height: card.Region.Height - 2*pad}
	frame := clip.Region{Width: float64(canvas.Width), Height: float64(canvas.Height)}
	if !inside(band, canvas.Safe) || !inside(card.Region, frame) {
		return card, clip.ErrInvalid
	}
	return card, nil
}

// CDS-28 fixes the hook card's padding; CDS-29's card uses the same.
const cardPadding float64 = 40

// trimmed formats an alpha the way an SVG attribute wants it: no trailing zeros.
func trimmed(alpha float64) string { return strconv.FormatFloat(alpha, 'f', -1, 64) }

func inside(r, safe clip.Region) bool {
	return r.X >= safe.X && r.Y >= safe.Y && r.X+r.Width <= safe.X+safe.Width && r.Y+r.Height <= safe.Y+safe.Height
}

// cardSVG paints the card and its stack. The card is `ink.900s` at α0.88 with
// radius.card and shadow.card (CDS-28); nothing in it animates by itself.
func cardSVG(canvas clip.Canvas, card cardLayout) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, canvas.Width, canvas.Height)
	if card.empty() {
		b.WriteString(`</svg>`)
		return b.String()
	}
	sh := design.Shadow["card"]
	fmt.Fprintf(&b, `<defs><filter id="card" x="-20%%" y="-20%%" width="140%%" height="140%%"><feDropShadow dx="%.3f" dy="%.3f" stdDeviation="%.3f" flood-color="%s" flood-opacity="%s"/></filter></defs>`,
		sh.DX, sh.DY, sh.Blur/2, sh.Hex, trimmed(sh.Alpha))
	plate, alpha := paint("ink_900s")
	p := card.Region
	fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="%s" fill-opacity="%s" filter="url(#card)"/>`,
		p.X, p.Y, p.Width, p.Height, design.Spacing.RadiusCard, plate, alpha)
	top := p.Y + cardPadding
	for i, line := range card.Lines {
		bounds := card.Bounds[i]
		if line.Chip {
			// The category chip is an accent pill with ink text (CDS-28).
			pad := design.Spacing.PadChip
			fmt.Fprintf(&b, `<rect x="%.3f" y="%.3f" width="%.3f" height="%.3f" rx="%.3f" fill="%s"/>`,
				p.X+cardPadding, top, bounds.Width, bounds.Height, bounds.Height/2, design.Accent[card.Accent])
			fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s">%s</text>`,
				p.X+cardPadding+pad.H-bounds.X, top+pad.V-bounds.Y, design.FontFamily(line.Role.Face), line.Role.Size, line.Role.Weight, line.Role.Tracking*line.Role.Size, line.Fill, escaped(line.Text))
		} else {
			fmt.Fprintf(&b, `<text x="%.3f" y="%.3f" xml:space="preserve" font-family="%s" font-size="%.0f" font-weight="%d" letter-spacing="%.4f" fill="%s" fill-opacity="%s">%s</text>`,
				p.X+cardPadding-bounds.X, top-bounds.Y, design.FontFamily(line.Role.Face), line.Role.Size, line.Role.Weight, line.Role.Tracking*line.Role.Size, line.Fill, trimmed(line.Alpha), escaped(line.Text))
		}
		top += bounds.Height + design.Spacing.GapStack
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// cardPlate rasterizes the card, or nothing when the clip has none.
func (r *Rendering) cardPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, card cardLayout, index int) (string, error) {
	if card.empty() {
		return "", nil
	}
	return r.rasterize(ctx, ws, canvas, cardSVG(canvas, card), fmt.Sprintf("card-%04d", index))
}

// Elements records the card and its chip for the verifier. The card's window is
// already on the output timeline.
func (card cardLayout) Elements(cut int) clip.Manifest {
	if card.empty() {
		return nil
	}
	m := clip.Manifest{{
		Cut: cut, Kind: "card", Text: card.Kind, Region: design.Region(card.Region),
		StartMS: card.StartMS, EndMS: card.EndMS,
		Background: design.Color["ink_900s"].Hex,
	}}
	top := card.Region.Y + cardPadding
	for i, line := range card.Lines {
		bounds := card.Bounds[i]
		kind, background := "copy", design.Color["ink_900s"].Hex
		if line.Chip {
			// The category chip is an opaque accent pill, so its ink is read
			// against the accent and not against the card (CDS-28).
			kind, background = "chip-category", design.Accent[card.Accent]
		}
		m = append(m, design.Element{
			Cut: cut, Kind: kind, Text: line.Text, FontSize: line.Role.Size,
			Fill: line.Fill, Background: background,
			Region:  design.Region{X: card.Region.X + cardPadding, Y: top, Width: bounds.Width, Height: bounds.Height},
			StartMS: card.StartMS, EndMS: card.EndMS,
		})
		top += bounds.Height + design.Spacing.GapStack
	}
	return m
}

// CardElements measures the hook and the ending card and returns them placed,
// for a composer that has to keep copy out from under them (CDS-45). Like
// CaptionSize it needs no source pixels and scopes its SVG to its own
// workspace.
func (r *Rendering) CardElements(ctx context.Context, plan clip.EditPlan) (out clip.Manifest, err error) {
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return nil, err
	}
	if len(plan.Cuts) == 0 {
		return nil, nil
	}
	answers := map[string]string{}
	for _, a := range plan.Facts {
		answers[a.Label] = a.Text
	}
	accent := answerAccent(plan)
	cards := map[int]cardLayout{}
	for index, card := range map[int]cardLayout{
		0:                  hookCard(plan.Ratio, plan.Hook, plan.Preset, answers["상호"], accent, plan.DurationMS),
		len(plan.Cuts) - 1: endingCard(plan.Ratio, plan.Preset, plan.CTA, answers, accent, plan.DurationMS),
	} {
		if !card.empty() {
			cards[index] = card
		}
	}
	// A project with no 상호 has no cards to measure, and then this costs no
	// workspace and no subprocess at all.
	if len(cards) == 0 {
		return nil, nil
	}
	err = r.media.WithWorkspace(ctx, "clip-card-elements", func(ws clip.MediaWorkspace) error {
		for index, card := range cards {
			measured, err := r.measureCard(ctx, ws, canvas, plan.Ratio, card)
			if err != nil {
				return err
			}
			out = append(out, measured.Elements(index)...)
		}
		return nil
	})
	return out, err
}
