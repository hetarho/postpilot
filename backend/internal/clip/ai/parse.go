package ai

import (
	"encoding/json"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
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
	Scene    *string     `json:"scene"`
	Readable *bool       `json:"readable_text"`
	Subject  *regionJSON `json:"subject"`
}
type chunkJSON struct {
	SourceID *string        `json:"source_id"`
	Index    *int           `json:"chunk_index"`
	Segments *[]segmentJSON `json:"segments"`
}

func optional(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

type captionJSON struct {
	Text  *string `json:"text"`
	Start *int    `json:"start_ms"`
	End   *int    `json:"end_ms"`
	// The same fact in 14 characters or fewer, and the one word the sentence
	// turns on. The model no longer names a position, a style or an accent:
	// those are the design system's, not a judgement (CDS-7).
	ShortText *string `json:"short_text"`
	Keyword   *string `json:"keyword"`
}
type cutJSON struct {
	ID       *string         `json:"id"`
	SourceID *string         `json:"source_id"`
	Start    *int            `json:"start_ms"`
	End      *int            `json:"end_ms"`
	Focal    *pointJSON      `json:"focal"`
	Caption  *captionJSON    `json:"caption"`
	Chips    *[]string       `json:"chips"`
	Volume   json.RawMessage `json:"volume"`
}
type planJSON struct {
	Ratio    *string    `json:"ratio"`
	Duration *int       `json:"duration_ms"`
	Hook     *string    `json:"hook"`
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
	case "boolean":
		if _, ok := value.(bool); !ok {
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
		subject, subjectOK := s.Subject.domain()
		if s.Start == nil || s.End == nil || *s.Start >= *s.End || s.Event == nil || s.Subjects == nil || s.Speech == nil || s.Quality == nil || !focalOK || !subjectOK {
			return clip.ChunkAnalysis{}, llm.ErrBadOutput
		}
		// The model is told the seven scene ids, so an eighth is bad output, not
		// something to map away. The tolerance for a scene-less segment belongs
		// to STORED analyses written before scenes existed, and lives in
		// design.Scene where the domain reads them.
		scene, readable := design.Guards.DefaultScene, false
		if s.Scene != nil {
			if _, known := design.SceneStyles[*s.Scene]; !known {
				return clip.ChunkAnalysis{}, llm.ErrBadOutput
			}
			scene = *s.Scene
		}
		if s.Readable != nil {
			readable = *s.Readable
		}
		if !input.Source.Info.HasAudio && strings.TrimSpace(*s.Speech) != "" {
			return clip.ChunkAnalysis{}, llm.ErrBadOutput
		}
		// Clamp locally BEFORE adding the authoritative source offset.
		start, end := max(0, min(input.DurationMS, *s.Start)), max(0, min(input.DurationMS, *s.End))
		result.Segments = append(result.Segments, clip.Segment{StartMS: input.OffsetMS + start, EndMS: input.OffsetMS + end, Event: *s.Event, Subjects: *s.Subjects, Speech: *s.Speech, Quality: *s.Quality, Focal: focal, Scene: scene, ReadableText: readable, Subject: subject})
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
	if utf8.RuneCountInString(optional(wire.Hook)) > cfg.Render.MaxCopyRunes {
		return clip.EditPlan{}, clip.ErrCopyTooLong
	}
	result := clip.EditPlan{Ratio: *wire.Ratio, DurationMS: *wire.Duration, Hook: optional(wire.Hook)}
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
		if p.Text == nil || p.Start == nil || p.End == nil {
			return clip.EditPlan{}, outputError("plan_caption_fields")
		}
		if *p.End <= *p.Start {
			return clip.EditPlan{}, outputError("plan_caption_time")
		}
		// The 500-rune ceiling is the contract's, not a design fallback: text
		// past it is refused outright rather than shortened or dropped.
		for _, text := range []string{*p.Text, optional(p.ShortText), optional(p.Keyword)} {
			if utf8.RuneCountInString(text) > cfg.Render.MaxCopyRunes {
				return clip.EditPlan{}, clip.ErrCopyTooLong
			}
		}
		volume := 1.0
		if len(c.Volume) != 0 {
			if string(c.Volume) == "null" || json.Unmarshal(c.Volume, &volume) != nil {
				return clip.EditPlan{}, outputError("plan_volume")
			}
		}
		chips := []string{}
		if c.Chips != nil {
			for _, label := range *c.Chips {
				if !slices.Contains(design.Fact.Chips, label) {
					return clip.EditPlan{}, outputError("plan_chip_label")
				}
				chips = append(chips, label)
			}
		}
		// The caption arrives as WORDS only; the compiler places it.
		written := clip.Written{Text: *p.Text, ShortText: optional(p.ShortText), Keyword: optional(p.Keyword)}
		result.Cuts = append(result.Cuts, clip.Cut{ID: *c.ID, SourceID: source.ID, Fingerprint: source.Fingerprint, StartMS: *c.Start, EndMS: *c.End, Focal: focal, Volume: &volume, Chips: chips, Copy: clip.Caption{Text: written.Text, StartMS: *p.Start, EndMS: *p.End}})
		result.Written = append(result.Written, written)
	}
	// The timeline is compiled here; the design system's own decisions and the
	// final validation happen in Plan, which owns the measurement port.
	if err := composeTimeline(cfg, input, &result); err != nil {
		return clip.EditPlan{}, err
	}
	return result, nil
}
