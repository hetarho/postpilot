package design

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
)

// Advance metrics for every face and weight region text is set in (CDS-86).
// The table is generated from the bundled fonts by resvg itself — the same
// measurement the renderer shapes with — and committed, so admission, the
// writer's repair, the renderer and the browser all decide a slot's fit from
// one table and can never disagree about it. It deliberately carries no
// kerning: resvg shapes a kerned Latin pair narrower than its two advances, so
// the table can only over-measure, and a line it fits never leaves its width.
//
// Every advance is stated at font-size 100 (`units`).
type FaceMetrics struct {
	// The one advance every Hangul syllable shares, where the face covers all
	// 11,172 of them at a single width; 0 for a face whose syllables differ in
	// width or that covers only some, which lists them in Glyphs instead.
	Hangul float64 `json:"hangul"`
	// The widest advance the face has, taken for any covered character the
	// table does not list.
	Fallback float64 `json:"fallback"`
	// The ink box of the face's Hangul as em ratios above and below the
	// baseline, which the region stack measures its gaps between (CDS-87).
	Ink    Ink                `json:"ink"`
	Glyphs map[string]float64 `json:"glyphs"`
}

type Ink struct {
	Top    float64 `json:"top"`
	Bottom float64 `json:"bottom"`
}

type metricsFile struct {
	Units float64                `json:"units"`
	Faces map[string]FaceMetrics `json:"faces"`
}

//go:embed metrics.json
var metricsData []byte

var metrics = parseMetrics()

func parseMetrics() metricsFile {
	var m metricsFile
	if err := json.Unmarshal(metricsData, &m); err != nil {
		panic(fmt.Errorf("clip design metrics: %w", err))
	}
	if m.Units <= 0 || len(m.Faces) == 0 {
		panic("clip design metrics carry no faces")
	}
	for key, face := range m.Faces {
		if face.Fallback <= 0 || face.Ink.Top <= 0 || len(face.Glyphs) == 0 {
			panic(fmt.Errorf("clip design metrics: face %s is incomplete", key))
		}
	}
	return m
}

// MetricsJSON returns the embedded table's bytes, for the mirror test that keeps
// the frontend copy identical.
func MetricsJSON() []byte { return metricsData }

// MetricsKey names one face at one weight the way the table does.
func MetricsKey(face string, weight int) string { return fmt.Sprintf("%s:%d", face, weight) }

// FaceMetricsFor returns one face's table.
func FaceMetricsFor(face string, weight int) (FaceMetrics, bool) {
	m, ok := metrics.Faces[MetricsKey(face, weight)]
	return m, ok
}

// MetricFaces lists every face the table carries, for the generator and tests.
func MetricFaces() []string {
	keys := make([]string, 0, len(metrics.Faces))
	for key := range metrics.Faces {
		keys = append(keys, key)
	}
	return keys
}

func isHangulSyllable(r rune) bool { return r >= 0xAC00 && r <= 0xD7A3 }

func (m FaceMetrics) advance(r rune) float64 {
	if w, ok := m.Glyphs[string(r)]; ok {
		return w
	}
	if m.Hangul > 0 && isHangulSyllable(r) {
		return m.Hangul
	}
	return m.Fallback
}

func (m FaceMetrics) covers(r rune) bool {
	if _, ok := m.Glyphs[string(r)]; ok {
		return true
	}
	// A face covering every syllable is one of the full faces: a character it
	// lacks outside the table is refused by name when it renders (CDS-84).
	return m.Hangul > 0
}

// TextWidth is one line's advance width at `size`: the advances plus the
// tracking between characters. The letter-spacing resvg adds after the last
// glyph moves the pen but is not part of the line.
func TextWidth(face string, weight int, tracking, size float64, text string) float64 {
	m, ok := FaceMetricsFor(face, weight)
	if !ok {
		return math.Inf(1)
	}
	sum, n := 0.0, 0
	for _, r := range text {
		sum += m.advance(r)
		n++
	}
	if n == 0 {
		return 0
	}
	return size / metrics.Units * (sum + tracking*metrics.Units*float64(n-1))
}

// Covers reports whether a face sets every character of the text: a face
// covering only some syllables (Jua) sets exactly the characters it lists, so
// a region slot set in it overflows on any other (CDS-77, CDS-84).
func Covers(face string, weight int, text string) bool {
	m, ok := FaceMetricsFor(face, weight)
	if !ok {
		return false
	}
	for _, r := range text {
		if !m.covers(r) {
			return false
		}
	}
	return true
}

// InkOf is the face's Hangul ink box as em ratios, falling back to a full em
// for a face the table does not carry.
func InkOf(face string, weight int) Ink {
	if m, ok := FaceMetricsFor(face, weight); ok {
		return m.Ink
	}
	return Ink{Top: 1, Bottom: 0}
}
