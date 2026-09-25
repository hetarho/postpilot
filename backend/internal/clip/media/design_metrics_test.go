package media

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Every face and weight region text is set in (CDS-19, CDS-89..99), with the
// bundled file it comes from.
var metricFaces = []struct {
	Face   string
	Weight int
	File   string
}{
	{"paperlogy", 800, "paperlogy"},
	{"wantedsans", 600, "wantedsans"},
	{"wantedsans", 700, "wantedsans"},
	{"nanummyeongjo", 800, "nanummyeongjo-800"},
	{"jua", 400, "jua"},
}

// The non-Hangul characters a region line is written with: printable ASCII
// and the punctuation templates use.
func metricCharacters() []rune {
	var out []rune
	for r := rune(0x20); r <= 0x7e; r++ {
		out = append(out, r)
	}
	return append(out, []rune("·‧–—…‘’“”₩%℃㎡~×")...)
}

func realMetricsRenderer(t *testing.T) *Rendering {
	t.Helper()
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := renderConfig(t)
	cfg.FontPaths = bundledFontPaths(t)
	r, err := NewRenderer(a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

// lenientMeasure is the renderer's own --query-all measurement at 100 px, except
// that a value resvg reports nothing for is left out rather than failing the
// batch: a face may map a code point to a glyph it never draws.
func lenientMeasure(r *Rendering, ws clip.MediaWorkspace, values []string, weight int, family string) (map[string]float64, error) {
	path := filepath.Join(ws.Path, "metrics-measure.svg")
	if err := os.WriteFile(path, []byte(measureSVG(values, weight, 0, family)), 0o600); err != nil {
		return nil, err
	}
	defer os.Remove(path)
	data, err := r.resvg(context.Background(), ws, "--query-all", path)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.Split(line, ",")
		if len(parts) != 5 || !strings.HasPrefix(parts[0], "m") {
			continue
		}
		i, err := strconv.Atoi(strings.TrimPrefix(parts[0], "m"))
		if err != nil || i < 0 || i >= len(values) {
			continue
		}
		if w, err := strconv.ParseFloat(parts[3], 64); err == nil && w > 0 {
			out[values[i]] = w
		}
	}
	return out, nil
}

type metricsTable struct {
	Units float64                       `json:"units"`
	Faces map[string]design.FaceMetrics `json:"faces"`
}

// TestWriteDesignMetrics regenerates design/metrics.json and its frontend
// mirror from the bundled fonts with resvg itself, the measurement the renderer
// shapes with (CDS-86). Host run:
//
//	CLIP_METRICS_UPDATE=1 CLIP_RESVG_PATH=$(command -v resvg) go test ./internal/clip/media/ -run TestWriteDesignMetrics
func TestWriteDesignMetrics(t *testing.T) {
	if os.Getenv("CLIP_METRICS_UPDATE") != "1" {
		t.Skip("regenerates the committed design metrics")
	}
	r := realMetricsRenderer(t)
	table := metricsTable{Units: 100, Faces: map[string]design.FaceMetrics{}}
	err := r.media.WithWorkspace(t.Context(), "design-metrics", func(ws clip.MediaWorkspace) error {
		for _, f := range metricFaces {
			face := r.fonts[f.File]
			family := design.FontFamily(f.Face)
			var buf sfnt.Buffer
			covered := func(c rune) bool {
				index, err := face.GlyphIndex(&buf, c)
				return err == nil && index != 0
			}
			var syllables, others []string
			for c := rune(0xAC00); c <= 0xD7A3; c++ {
				if covered(c) {
					syllables = append(syllables, string(c))
				}
			}
			for _, c := range metricCharacters() {
				if c != ' ' && covered(c) {
					others = append(others, string(c))
				}
			}
			values := append(append([]string{}, syllables...), others...)
			base := syllables[0]
			values = append(values, base+base, base+" "+base)
			measured := map[string]float64{}
			for start := 0; start < len(values); start += 1000 {
				chunk, err := lenientMeasure(r, ws, values[start:min(start+1000, len(values))], f.Weight, family)
				if err != nil {
					return err
				}
				for k, v := range chunk {
					measured[k] = v
				}
			}
			// A glyph resvg sets with no advance is one the face does not really
			// draw: it stays out of the table, so Covers refuses it.
			syllables = slices.DeleteFunc(syllables, func(s string) bool { return measured[s] <= 0 })
			others = slices.DeleteFunc(others, func(s string) bool { return measured[s] <= 0 })
			m := design.FaceMetrics{Glyphs: map[string]float64{}}
			uniform := len(syllables) == 11172
			first := measured[syllables[0]]
			for _, s := range syllables {
				if math.Abs(measured[s]-first) > 0.005 {
					uniform = false
				}
			}
			if uniform {
				m.Hangul = round3(first)
			} else {
				for _, s := range syllables {
					m.Glyphs[s] = round3(measured[s])
				}
			}
			for _, s := range others {
				m.Glyphs[s] = round3(measured[s])
			}
			m.Glyphs[" "] = round3(measured[base+" "+base] - measured[base+base])
			m.Fallback = m.Hangul
			for _, w := range m.Glyphs {
				m.Fallback = math.Max(m.Fallback, w)
			}
			// The ink box over every covered syllable, from the face's own glyph
			// outlines at 100 ppem: y grows downward, so an ascender is negative.
			top, bottom := 0.0, 0.0
			for _, s := range syllables {
				index, _ := face.GlyphIndex(&buf, []rune(s)[0])
				bounds, _, err := face.GlyphBounds(&buf, index, fixed.I(100), font.HintingNone)
				if err != nil {
					return err
				}
				top = math.Max(top, -float64(bounds.Min.Y)/64/100)
				bottom = math.Max(bottom, float64(bounds.Max.Y)/64/100)
			}
			m.Ink = design.Ink{Top: round3(top), Bottom: round3(bottom)}
			table.Faces[design.MetricsKey(f.Face, f.Weight)] = m
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(table); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../design/metrics.json", "../../../../frontend/src/entities/clip-design/config/clip-metrics.json"} {
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestDesignMetricsMatchResvg holds the committed table to resvg on real text:
// never narrower than the shaped line, within 0.5 % for Hangul, and within 3 %
// where kerned digits and Latin pairs let resvg set the line tighter.
func TestDesignMetricsMatchResvg(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real bundled-font renderer gate")
	}
	r := realMetricsRenderer(t)
	samples := []string{"해미 한우", "연남동 골목에서 30년째 숯불 한우만 굽는 집", "4.8", "1인 한우 모둠 49,000원", "서울 마포구 연남동 · 투뿔 한우 숯불 구이", "매일 11:30 – 22:00"}
	kerned := func(text string) bool {
		return strings.ContainsAny(text, "0123456789.,:–ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	}
	err := r.media.WithWorkspace(t.Context(), "design-metrics-check", func(ws clip.MediaWorkspace) error {
		for _, f := range metricFaces {
			for _, tracking := range []float64{0, -0.02, 0.08} {
				texts := samples
				if f.Face == "jua" {
					texts = []string{"해미 한우", "다시 가고 싶은 불판", "4.8"}
				}
				measured, err := r.measure(t.Context(), ws, texts, f.Weight, tracking, design.FontFamily(f.Face))
				if err != nil {
					return err
				}
				for _, text := range texts {
					// resvg's box ends at the last glyph's advance: the tracking sits
					// between characters only, as the table counts it.
					shaped := measured[text].Width
					table := design.TextWidth(f.Face, f.Weight, tracking, 100, text)
					slack := 1.005
					if kerned(text) {
						slack = 1.03
					}
					if table < shaped-0.05 || table > shaped*slack+0.05 {
						t.Errorf("%s:%d %q tracking %.2f: table %.3f, resvg %.3f", f.Face, f.Weight, text, tracking, table, shaped)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
