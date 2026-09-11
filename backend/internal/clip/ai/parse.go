package ai

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type pointJSON struct {
	X *float64 `json:"x"`
	Y *float64 `json:"y"`
}

func (p *pointJSON) domain() (clip.Point, bool) {
	if p == nil || p.X == nil || p.Y == nil {
		return clip.Point{}, false
	}
	return clip.Point{X: *p.X, Y: *p.Y}, true
}

type regionJSON struct {
	X      *float64 `json:"x"`
	Y      *float64 `json:"y"`
	Width  *float64 `json:"width"`
	Height *float64 `json:"height"`
}

func (r *regionJSON) domain() (clip.Region, bool) {
	if r == nil || r.X == nil || r.Y == nil || r.Width == nil || r.Height == nil {
		return clip.Region{}, false
	}
	return clip.Region{X: *r.X, Y: *r.Y, Width: *r.Width, Height: *r.Height}, true
}

type segmentJSON struct {
	Start    *int        `json:"start_ms"`
	End      *int        `json:"end_ms"`
	Event    *string     `json:"event"`
	Subjects *[]string   `json:"subjects"`
	Speech   *string     `json:"speech"`
	Quality  *string     `json:"quality"`
	Focal    *pointJSON  `json:"focal"`
	Avoid    *regionJSON `json:"avoid"`
}
type chunkJSON struct {
	SourceID *string        `json:"source_id"`
	Index    *int           `json:"chunk_index"`
	Segments *[]segmentJSON `json:"segments"`
}
type captionJSON struct {
	Text     *string `json:"text"`
	Start    *int    `json:"start_ms"`
	End      *int    `json:"end_ms"`
	Position *string `json:"position"`
	Style    *string `json:"style"`
	Accent   *string `json:"accent"`
}
type cutJSON struct {
	ID       *string         `json:"id"`
	SourceID *string         `json:"source_id"`
	Start    *int            `json:"start_ms"`
	End      *int            `json:"end_ms"`
	Focal    *pointJSON      `json:"focal"`
	Caption  *captionJSON    `json:"caption"`
	Volume   json.RawMessage `json:"volume"`
}
type planJSON struct {
	Ratio    *string    `json:"ratio"`
	Duration *int       `json:"duration_ms"`
	Cuts     *[]cutJSON `json:"cuts"`
}

// The embedded closed contract also guards exact key spelling and nulls: Go's
// struct decoder alone accepts case-insensitive keys and null primitive values.
type shape struct {
	Type       string            `json:"type"`
	Properties map[string]*shape `json:"properties"`
	Required   []string          `json:"required"`
	Items      *shape            `json:"items"`
}

func readShape(data []byte) *shape {
	var s shape
	if err := json.Unmarshal(data, &s); err != nil {
		panic(err)
	}
	return &s
}

var chunkShape = readShape(chunkSchema)
var planShape = readShape(planSchema)

func (s *shape) accepts(value any) bool {
	if value == nil {
		return false
	}
	switch s.Type {
	case "object":
		v, ok := value.(map[string]any)
		if !ok {
			return false
		}
		for _, key := range s.Required {
			if _, ok := v[key]; !ok {
				return false
			}
		}
		for key, item := range v {
			child, ok := s.Properties[key]
			if !ok || !child.accepts(item) {
				return false
			}
		}
	case "array":
		v, ok := value.([]any)
		if !ok || s.Items == nil {
			return false
		}
		for _, item := range v {
			if !s.Items.accepts(item) {
				return false
			}
		}
	case "string":
		if _, ok := value.(string); !ok {
			return false
		}
	case "integer", "number":
		if _, ok := value.(json.Number); !ok {
			return false
		}
	default:
		return false
	}
	return true
}
func decode(raw string, maxBytes int, contract *shape, out any) error {
	if len(raw) > maxBytes || !utf8.ValidString(raw) {
		return outputError("output_encoding_or_size")
	}
	candidate, ok := llm.JSONCandidate(raw)
	if !ok {
		return outputError("output_json")
	}
	var value any
	structure := json.NewDecoder(strings.NewReader(candidate))
	structure.UseNumber()
	if structure.Decode(&value) != nil || !contract.accepts(value) {
		return outputError("output_shape")
	}
	d := json.NewDecoder(strings.NewReader(candidate))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return outputError("output_field_type")
	}
	return nil
}
func parseChunk(cfg Config, input clip.ChunkInput, raw string) (clip.ChunkAnalysis, error) {
	var wire chunkJSON
	if err := decode(raw, cfg.MaxResponseBytes, chunkShape, &wire); err != nil {
		return clip.ChunkAnalysis{}, err
	}
	if wire.SourceID == nil || *wire.SourceID != input.Source.ID || wire.Index == nil || *wire.Index != input.Index || wire.Segments == nil {
		return clip.ChunkAnalysis{}, llm.ErrBadOutput
	}
	result := clip.ChunkAnalysis{SourceID: input.Source.ID, Fingerprint: input.Source.Fingerprint, Index: input.Index, OffsetMS: input.OffsetMS, DurationMS: input.DurationMS}
	for _, s := range *wire.Segments {
		focal, focalOK := s.Focal.domain()
		avoid, avoidOK := s.Avoid.domain()
		if s.Start == nil || s.End == nil || *s.Start >= *s.End || s.Event == nil || s.Subjects == nil || s.Speech == nil || s.Quality == nil || !focalOK || !avoidOK {
			return clip.ChunkAnalysis{}, llm.ErrBadOutput
		}
		if !input.Source.Info.HasAudio && strings.TrimSpace(*s.Speech) != "" {
			return clip.ChunkAnalysis{}, llm.ErrBadOutput
		}
		// Clamp locally BEFORE adding the authoritative source offset.
		start, end := max(0, min(input.DurationMS, *s.Start)), max(0, min(input.DurationMS, *s.End))
		result.Segments = append(result.Segments, clip.Segment{StartMS: input.OffsetMS + start, EndMS: input.OffsetMS + end, Event: *s.Event, Subjects: *s.Subjects, Speech: *s.Speech, Quality: *s.Quality, Focal: focal, Avoid: avoid})
	}
	if err := clip.ValidateSegments(cfg.Analysis, result.Segments, input.OffsetMS, input.OffsetMS+input.DurationMS); err != nil {
		return clip.ChunkAnalysis{}, llm.ErrBadOutput
	}
	return result, nil
}
func parsePlan(cfg Config, input clip.PlanningInput, raw string) (clip.EditPlan, error) {
	var wire planJSON
	if err := decode(raw, cfg.MaxResponseBytes, planShape, &wire); err != nil {
		return clip.EditPlan{}, err
	}
	if wire.Ratio == nil || wire.Duration == nil || wire.Cuts == nil {
		return clip.EditPlan{}, outputError("plan_required")
	}
	byID := map[string]clip.AnalysisSource{}
	for _, analysis := range input.Analyses {
		byID[analysis.Source.ID] = analysis.Source
	}
	result := clip.EditPlan{Ratio: *wire.Ratio, DurationMS: *wire.Duration}
	for _, c := range *wire.Cuts {
		focal, ok := c.Focal.domain()
		if !ok || c.ID == nil || utf8.RuneCountInString(*c.ID) > cfg.MaxCutIDRunes || c.SourceID == nil || c.Start == nil || c.End == nil || c.Caption == nil {
			return clip.EditPlan{}, outputError("plan_cut_fields")
		}
		source, exists := byID[*c.SourceID]
		p := c.Caption
		if !exists {
			return clip.EditPlan{}, outputError("plan_source")
		}
		if p.Text == nil || p.Start == nil || p.End == nil || p.Position == nil || p.Style == nil || p.Accent == nil {
			return clip.EditPlan{}, outputError("plan_caption_fields")
		}
		if *p.End <= *p.Start {
			return clip.EditPlan{}, outputError("plan_caption_time")
		}
		volume := 1.0
		if len(c.Volume) != 0 {
			if string(c.Volume) == "null" || json.Unmarshal(c.Volume, &volume) != nil {
				return clip.EditPlan{}, outputError("plan_volume")
			}
		}
		result.Cuts = append(result.Cuts, clip.Cut{ID: *c.ID, SourceID: source.ID, Fingerprint: source.Fingerprint, StartMS: *c.Start, EndMS: *c.End, Focal: focal, Volume: &volume, Copy: clip.Caption{Text: *p.Text, StartMS: *p.Start, EndMS: *p.End, Position: *p.Position, Style: *p.Style, Accent: *p.Accent}})
	}
	if err := validatePlan(cfg, input, result); err != nil {
		return clip.EditPlan{}, err
	}
	return result, nil
}
