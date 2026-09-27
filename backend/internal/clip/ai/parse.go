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
	Start       *int         `json:"start_ms"`
	End         *int         `json:"end_ms"`
	Event       *string      `json:"event"`
	Action      *string      `json:"action"`
	Motion      *string      `json:"motion"`
	Subjects    *[]string    `json:"subjects"`
	Speech      *string      `json:"speech"`
	Quality     *string      `json:"quality"`
	Focal       *pointJSON   `json:"focal"`
	Scene       *string      `json:"scene"`
	Readable    *bool        `json:"readable_text"`
	Subject     *regionJSON  `json:"subject"`
	CaptionSafe []regionJSON `json:"caption_safe"`
	Certainty   *string      `json:"certainty"`
	Usability   *string      `json:"usability"`
}
type chunkJSON struct {
	SourceID *string        `json:"source_id"`
	Index    *int           `json:"chunk_index"`
	Segments *[]segmentJSON `json:"segments"`
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
func parseChunk(cfg Config, input clip.ChunkInput, raw string) (out clip.ChunkAnalysis, err error) {
	var wire chunkJSON
	phase := "decode"
	segment := 0
	defer func() {
		if err == nil {
			return
		}
		d, exists := clip.DiagnosticFromError(err)
		if !exists {
			check := "unknown"
			if code, ok := err.(interface{ OutputValidationCode() string }); ok {
				check = code.OutputValidationCode()
			}
			d = clip.AttemptDiagnostic{Check: check, Phase: phase}
		}
		d.Values = clip.SafeAttemptValues(d.Values)
		d.Values["duration_ms"] = input.DurationMS
		if wire.Segments != nil {
			d.Values["segment_count"] = len(*wire.Segments)
		}
		if n := d.Values["segment"]; n > 0 {
			segment = n
		}
		if segment > 0 && wire.Segments != nil && segment <= len(*wire.Segments) {
			d.Values["segment"] = segment
			s := (*wire.Segments)[segment-1]
			if s.Start != nil {
				d.Values["raw_start_ms"] = *s.Start
			}
			if s.End != nil {
				d.Values["raw_end_ms"] = *s.End
			}
		}
		d.Values = clip.SafeAttemptValues(d.Values)
		err = clip.WithAttemptDiagnostic(outputError(clip.SafeAttemptCheck(d.Check)), d)
	}()
	if err := decode(raw, cfg.MaxResponseBytes, chunkShape, &wire); err != nil {
		return clip.ChunkAnalysis{}, err
	}
	phase = "observation"
	if wire.SourceID == nil || *wire.SourceID != input.Source.ID {
		return clip.ChunkAnalysis{}, outputError("observe_source_identity")
	}
	if wire.Index == nil || *wire.Index != input.Index {
		values := map[string]int{"expected_index": input.Index}
		if wire.Index != nil {
			values["actual_index"] = *wire.Index
		}
		return clip.ChunkAnalysis{}, clip.WithAttemptDiagnostic(outputError("observe_chunk_identity"), clip.AttemptDiagnostic{Check: "observe_chunk_identity", Phase: phase, Values: values})
	}
	if wire.Segments == nil {
		return clip.ChunkAnalysis{}, outputError("observe_segment_count")
	}
	result := clip.ChunkAnalysis{SourceID: input.Source.ID, Fingerprint: input.Source.Fingerprint, Index: input.Index, OffsetMS: input.OffsetMS, DurationMS: input.DurationMS}
	for i, s := range *wire.Segments {
		segment = i + 1
		focal, focalOK := s.Focal.domain()
		subject, subjectOK := s.Subject.domain()
		var captionSafe []clip.Region
		for _, box := range s.CaptionSafe {
			region, ok := box.domain()
			if !ok {
				return clip.ChunkAnalysis{}, outputError("observe_segment_fields")
			}
			captionSafe = append(captionSafe, region)
		}
		if s.Start == nil || s.End == nil || s.Event == nil || s.Subjects == nil || s.Speech == nil || s.Quality == nil || s.Action == nil || s.Motion == nil || s.Certainty == nil || s.Usability == nil || !focalOK || !subjectOK {
			return clip.ChunkAnalysis{}, outputError("observe_segment_fields")
		}
		// The model is told the seven scene ids, so an eighth is bad output, not
		// something to map away. The tolerance for a scene-less segment belongs
		// to STORED analyses written before scenes existed, and lives in
		// design.Scene where the domain reads them.
		scene, readable := design.DefaultScene, false
		if s.Scene != nil {
			if !slices.Contains(design.Scenes, *s.Scene) {
				return clip.ChunkAnalysis{}, outputError("observe_scene")
			}
			scene = *s.Scene
		}
		if s.Readable != nil {
			readable = *s.Readable
		}
		if !input.Source.Info.HasAudio && strings.TrimSpace(*s.Speech) != "" {
			return clip.ChunkAnalysis{}, outputError("observe_silent_speech")
		}
		// Times stay CHUNK-LOCAL here, exactly as the model returned them: a
		// negative, overflowing or out-of-order value is refused below rather
		// than clamped into something the model never said, so the correction
		// feedback describes the response that actually failed.
		result.Segments = append(result.Segments, clip.Segment{StartMS: *s.Start, EndMS: *s.End, Event: *s.Event, Action: *s.Action, Motion: *s.Motion, Subjects: *s.Subjects, Speech: *s.Speech, Quality: *s.Quality, Focal: focal, Scene: scene, ReadableText: readable, Subject: subject, CaptionSafe: captionSafe, Certainty: *s.Certainty, Usability: *s.Usability})
	}
	segment = 0
	if err := clip.ValidateSegments(cfg.Analysis, result.Segments, 0, input.DurationMS); err != nil {
		return clip.ChunkAnalysis{}, err
	}
	// The frozen source offset is added ONCE, after the chunk-local record has
	// been proved complete and in order.
	for i := range result.Segments {
		result.Segments[i].StartMS += input.OffsetMS
		result.Segments[i].EndMS += input.OffsetMS
	}
	return result, nil
}
