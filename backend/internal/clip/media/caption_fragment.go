package media

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

var _ clip.CaptionFragmenter = (*Rendering)(nil)

// A representative frame is the caption's own MIDDLE: a style's entrance has
// finished and its exit has not begun there, whatever the interval, so the
// preview shows the caption as it is read rather than as it arrives (CDS-83).
const representativeProgress = 0.5

// CaptionFragments draws every caption of a plan the way the render will, at the
// origin. It measures with the bundled faces and loads no footage, which is why
// no caption carries a scrim here: the wash CDS-44 adds belongs to the sampled
// ground, and the ground is sampled only where the footage is read.
func (r *Rendering) CaptionFragments(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) ([]clip.CaptionFragment, error) {
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return nil, err
	}
	resolved, err := clip.ResolvePortableIntervals(plan, r.cfg.Composition)
	if err != nil {
		return nil, err
	}
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(resolved), sources); err != nil {
		return nil, err
	}
	out := []clip.CaptionFragment{}
	err = r.media.WithWorkspace(ctx, "clip-caption-fragment", func(ws clip.MediaWorkspace) error {
		layout, err := r.layoutComposition(ctx, ws, resolved)
		if err != nil {
			return err
		}
		for _, visual := range layout.visuals {
			if visual.manifest.Role != "caption" {
				continue
			}
			fragment, err := r.captionFragment(canvas, visual)
			if err != nil {
				return err
			}
			out = append(out, fragment)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// captionFragment is one caption's own drawing. A rapid caption is drawn from
// its FIRST phrase: the phrases share one placement and one style, and the
// editor moves the caption, not a phrase of it (CDS-59).
func (r *Rendering) captionFragment(canvas clip.Canvas, visual declaredVisual) (clip.CaptionFragment, error) {
	drawn := visual
	if len(visual.cues) > 0 {
		drawn = visual.cues[0]
	}
	l := drawn.caption
	if len(l.Lines) == 0 {
		return clip.CaptionFragment{}, elementProblem(visual.text, "invalid_design")
	}
	document, err := r.captionDocument(canvas, drawn)
	if err != nil {
		return clip.CaptionFragment{}, err
	}
	body := innerSVG(document)
	// The body keeps the renderer's OWN coordinates, verbatim. The fragment is
	// brought to the origin by one transform and the caller puts it back with
	// another, and the box it is given is the same rounded number the transform
	// carries — so the two cancel exactly and the fragment rasterises to the
	// render's own pixels rather than to something a rounding apart (CDS-83).
	// This document reaches a BROWSER, which the render's own never does. The
	// static path is validated as XML by the overlay catalog on its way out; the
	// drawn one is not, so it is checked here — one place, both kinds.
	if err := wellFormed(document); err != nil {
		return clip.CaptionFragment{}, err
	}
	id := visual.manifest.InstanceID
	box := clip.Region{X: rounded(l.Region.X), Y: rounded(l.Region.Y), Width: rounded(l.Region.Width), Height: rounded(l.Region.Height)}
	return clip.CaptionFragment{
		InstanceID: id, Style: drawn.manifest.Style, Box: box, FontSize: l.FontSize,
		Sequence: !l.Caption.Static(),
		SVG: fmt.Sprintf(`<g transform="translate(%s,%s)">%s</g>`,
			coordinate(-box.X), coordinate(-box.Y), prefixIDs(id, body)),
	}, nil
}

// captionDocument is the renderer's OWN drawing of one caption at its measured
// place on the whole canvas: the bundled template a static style keeps and one
// representative frame of a sequence-rendered one. The fragment is this document
// with its root taken off, which is what makes CDS-83 a contract rather than an
// intention — there is one drawing, and the preview shows it.
func (r *Rendering) captionDocument(canvas clip.Canvas, drawn declaredVisual) (string, error) {
	l := drawn.caption
	if l.Caption.Static() {
		return r.declaredSVG(canvas, drawn)
	}
	frame := captionFrame(canvas, drawn.copy, l, drawn.manifest.EndMS-drawn.manifest.StartMS, representativeProgress)
	defs, body, ok := design.DrawCaptionFrame(frame)
	if !ok {
		return "", elementProblem(drawn.text, "invalid_design")
	}
	out := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d">`, canvas.Width, canvas.Height)
	if defs != "" {
		out += "<defs>" + defs + "</defs>"
	}
	return out + body + "</svg>", nil
}

// wellFormed refuses a fragment no browser could parse rather than sending it.
func wellFormed(document string) error {
	d := xml.NewDecoder(strings.NewReader(document))
	for {
		_, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return clip.ErrInvalid
		}
	}
}

func coordinate(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// rounded is the coordinate as the fragment WRITES it, so the box the caller
// translates by and the transform the fragment carries are the same number.
func rounded(v float64) float64 {
	out, _ := strconv.ParseFloat(coordinate(v), 64)
	return out
}

// innerSVG drops the one root element every bundled template and every painter
// writes, leaving what the caller places. The documents are machine-written by
// this package, so the root tag ends at the first `>`.
func innerSVG(document string) string {
	open := strings.Index(document, ">")
	close := strings.LastIndex(document, "</svg>")
	if open < 0 || close < open {
		return ""
	}
	return document[open+1 : close]
}

var svgID = regexp.MustCompile(`id="([^"]*)"`)
var svgRef = regexp.MustCompile(`(url\(#|href="#)([^)"]*)`)

// prefixIDs makes every id inside one fragment that caption's own. Two captions
// on screen at once otherwise share `filter`, `clipPath` and gradient ids, and
// the browser resolves both to whichever was parsed last.
func prefixIDs(instance, body string) string {
	prefix := idToken(instance) + "-"
	body = svgID.ReplaceAllString(body, `id="`+prefix+`$1"`)
	return svgRef.ReplaceAllString(body, `${1}`+prefix+`$2`)
}

// idToken is the instance id reduced to what an XML id may hold. An instance id
// is server-minted and already tame; this keeps a stored one that is not from
// producing a document no browser will parse.
func idToken(instance string) string {
	var b strings.Builder
	for _, c := range instance {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "caption"
	}
	return b.String()
}
