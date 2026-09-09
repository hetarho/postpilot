package clip

import (
	"math"
	"strings"
)

// These records contain facts in original, display-oriented source coordinates.
// No source bytes or signed URL survives in an analysis or an edit plan.
type AnalysisSource struct {
	RenderSource
	Filename string
}
type Segment struct {
	StartMS, EndMS         int
	Event, Speech, Quality string
	Subjects               []string
	Focal                  Point
	Avoid                  Region
}
type SourceAnalysis struct {
	Source   AnalysisSource
	Segments []Segment
}
type ChunkInput struct {
	Source                      AnalysisSource
	Index, OffsetMS, DurationMS int
	URL                         string
}
type ChunkAnalysis struct {
	SourceID, Fingerprint       string
	Index, OffsetMS, DurationMS int
	Segments                    []Segment
}
type AnalysisLimits struct {
	ChunkMS, MaxSources, MaxSourceDurationMS, MaxSegments, MaxTextRunes, MaxSubjects int
}
type PlanningInput struct {
	Template         Recipe
	Answers          []Answer
	Ratio            string
	TargetDurationMS int
	Analyses         []SourceAnalysis
}

func ValidRegion(r Region) bool {
	return normalized(r.X) && normalized(r.Y) && normalized(r.Width) && normalized(r.Height) && r.X+r.Width <= 1 && r.Y+r.Height <= 1
}
func ValidateAnalysisSources(l AnalysisLimits, sources []AnalysisSource) error {
	if len(sources) == 0 || len(sources) > l.MaxSources || l.ChunkMS <= 0 {
		return ErrInvalid
	}
	seen, fingerprints, total := map[string]bool{}, map[string]bool{}, 0
	for _, source := range sources {
		if strings.TrimSpace(source.ID) == "" || source.Fingerprint == "" || strings.TrimSpace(source.Filename) == "" || seen[source.ID] || fingerprints[source.Fingerprint] || source.Info.DurationMS <= 0 || source.Info.DurationMS > l.MaxSourceDurationMS-total || source.Info.Width <= 0 || source.Info.Height <= 0 {
			return ErrInvalid
		}
		seen[source.ID], fingerprints[source.Fingerprint] = true, true
		total += source.Info.DurationMS
	}
	return nil
}
func ValidateChunkInput(l AnalysisLimits, in ChunkInput) error {
	if err := ValidateAnalysisSources(l, []AnalysisSource{in.Source}); err != nil {
		return err
	}
	// Validate bounds before multiplication so a forged index cannot overflow.
	if in.Index < 0 || in.Index > (in.Source.Info.DurationMS-1)/l.ChunkMS || in.OffsetMS != in.Index*l.ChunkMS || in.DurationMS != min(l.ChunkMS, in.Source.Info.DurationMS-in.OffsetMS) {
		return ErrInvalid
	}
	return nil
}
func ValidateSegments(l AnalysisLimits, segments []Segment, start, end int) error {
	if len(segments) == 0 || len(segments) > l.MaxSegments {
		return ErrInvalid
	}
	previous := start
	for _, s := range segments {
		if s.StartMS < previous || s.EndMS <= s.StartMS || s.EndMS > end || !normalized(s.Focal.X) || !normalized(s.Focal.Y) || !ValidRegion(s.Avoid) || !bounded(s.Event, 0, l.MaxTextRunes) || !bounded(s.Speech, 0, l.MaxTextRunes) || !bounded(s.Quality, 1, l.MaxTextRunes) || strings.TrimSpace(s.Quality) == "" || len(s.Subjects) > l.MaxSubjects {
			return ErrInvalid
		}
		description := strings.TrimSpace(s.Event) != "" || strings.TrimSpace(s.Speech) != ""
		for _, subject := range s.Subjects {
			if !bounded(subject, 1, l.MaxTextRunes) || strings.TrimSpace(subject) == "" {
				return ErrInvalid
			}
			description = true
		}
		if !description {
			return ErrInvalid
		}
		previous = s.EndMS
	}
	return nil
}

// MergeAnalyses accepts exactly one complete, ordered chunk sequence per frozen
// source. It never sorts or silently drops a hallucinated, duplicated or missing id.
func MergeAnalyses(l AnalysisLimits, sources []AnalysisSource, chunks []ChunkAnalysis) ([]SourceAnalysis, error) {
	if err := ValidateAnalysisSources(l, sources); err != nil {
		return nil, err
	}
	out, cursor := make([]SourceAnalysis, 0, len(sources)), 0
	for _, source := range sources {
		analysis := SourceAnalysis{Source: source}
		for offset, index := 0, 0; offset < source.Info.DurationMS; offset, index = offset+l.ChunkMS, index+1 {
			if cursor >= len(chunks) {
				return nil, ErrInvalid
			}
			c := chunks[cursor]
			duration := min(l.ChunkMS, source.Info.DurationMS-offset)
			if c.SourceID != source.ID || c.Fingerprint != source.Fingerprint || c.Index != index || c.OffsetMS != offset || c.DurationMS != duration {
				return nil, ErrInvalid
			}
			if err := ValidateSegments(l, c.Segments, offset, offset+duration); err != nil {
				return nil, err
			}
			analysis.Segments = append(analysis.Segments, c.Segments...)
			cursor++
		}
		out = append(out, analysis)
	}
	if cursor != len(chunks) {
		return nil, ErrInvalid
	}
	return out, nil
}

// CaptionAvoid projects every relevant source-space subject box through the same
// cover crop as the renderer, conservatively joining the visible intersections.
func CaptionAvoid(canvas Canvas, cut Cut, analysis SourceAnalysis) Region {
	w, h := float64(analysis.Source.Info.Width), float64(analysis.Source.Info.Height)
	scale := math.Max(float64(canvas.Width)/w, float64(canvas.Height)/h)
	w, h = w*scale, h*scale
	x := math.Max(0, math.Min(w-float64(canvas.Width), w*cut.Focal.X-float64(canvas.Width)/2))
	y := math.Max(0, math.Min(h-float64(canvas.Height), h*cut.Focal.Y-float64(canvas.Height)/2))
	left, top, right, bottom := 1.0, 1.0, 0.0, 0.0
	for _, s := range analysis.Segments {
		if s.EndMS <= cut.StartMS || s.StartMS >= cut.EndMS || s.Avoid.Width == 0 || s.Avoid.Height == 0 {
			continue
		}
		l := math.Max(0, math.Min(1, (s.Avoid.X*w-x)/float64(canvas.Width)))
		r := math.Max(0, math.Min(1, ((s.Avoid.X+s.Avoid.Width)*w-x)/float64(canvas.Width)))
		t := math.Max(0, math.Min(1, (s.Avoid.Y*h-y)/float64(canvas.Height)))
		b := math.Max(0, math.Min(1, ((s.Avoid.Y+s.Avoid.Height)*h-y)/float64(canvas.Height)))
		if l >= r || t >= b {
			continue
		}
		left, top, right, bottom = math.Min(left, l), math.Min(top, t), math.Max(right, r), math.Max(bottom, b)
	}
	if left >= right || top >= bottom {
		return Region{}
	}
	return Region{X: left, Y: top, Width: right - left, Height: bottom - top}
}
