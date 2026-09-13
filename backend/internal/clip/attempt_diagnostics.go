package clip

import (
	"context"
	"errors"
	"log/slog"
)

// Checkpoints contain validated domain evidence, never a raw provider response.
// They are separate from canonical project content and cannot be rendered/exported.
var ErrInputTooLarge = errors.New("clip request input exceeds its approved allowance")

const AttemptCheckpointMaxBytes = 2 * 1024 * 1024

type AttemptRange struct {
	Cut, Source    int
	StartMS, EndMS int
	Valid          bool
}

type AttemptDiagnostic struct {
	Check, Phase string
	Values       map[string]int
	Ranges       []AttemptRange
}

type AttemptCheckpoint struct {
	EvidenceLimited                                              bool
	Version                                                      int
	JobID, Stage                                                 string
	CompletedChunks, TotalChunks, CompletedSources, TotalSources int
	Observations                                                 []SourceAnalysis
	Diagnostic                                                   AttemptDiagnostic
}

type AttemptCheckpointStore interface {
	SaveAttemptCheckpoint(context.Context, string, string, AttemptCheckpoint) error
	GetAttemptCheckpoint(context.Context, string, string, string) (*AttemptCheckpoint, error)
}

type attemptDiagnosticError struct {
	cause      error
	diagnostic AttemptDiagnostic
}

func (e *attemptDiagnosticError) Error() string                        { return "clip attempt validation failed" }
func (e *attemptDiagnosticError) Unwrap() error                        { return e.cause }
func (e *attemptDiagnosticError) AttemptDiagnostic() AttemptDiagnostic { return e.diagnostic }

func WithAttemptDiagnostic(err error, d AttemptDiagnostic) error {
	if err == nil {
		return nil
	}
	return &attemptDiagnosticError{err, d}
}

func DiagnosticFromError(err error) (AttemptDiagnostic, bool) {
	var e interface{ AttemptDiagnostic() AttemptDiagnostic }
	if errors.As(err, &e) {
		return e.AttemptDiagnostic(), true
	}
	return AttemptDiagnostic{}, false
}

func AttemptRangeDiagnostics(plan EditPlan, analyses []SourceAnalysis) []AttemptRange {
	out := make([]AttemptRange, 0, min(len(plan.Cuts), 100))
	for i, c := range plan.Cuts {
		if i >= 100 {
			break
		}
		r := AttemptRange{Cut: i + 1}
		for j, a := range analyses {
			if c.SourceID != a.Source.ID {
				continue
			}
			r.Source = j + 1
			if c.StartMS >= 0 && c.EndMS > c.StartMS && c.EndMS <= a.Source.Info.DurationMS {
				r.StartMS, r.EndMS = c.StartMS, c.EndMS
				c.Fingerprint = a.Source.Fingerprint
				_, r.Valid = CutEvidence(analyses, c)
			}
			break
		}
		out = append(out, r)
	}
	return out
}

// This is the single numeric allowlist shared by persistence and logging.
func SafeAttemptValues(values map[string]int) map[string]int {
	out := map[string]int{}
	for _, key := range []string{"input_bytes", "input_limit_bytes", "system_bytes", "content_bytes", "schema_bytes", "source", "chunk", "cut", "cut_count", "target_ms", "before_ms", "after_ms", "remaining_ms", "min_ms", "max_ms", "transition_ms", "backward_ms", "segment", "segment_count", "duration_ms", "expected_index", "event_runes", "speech_runes", "quality_runes", "subject_count", "subject_runes"} {
		if n, ok := values[key]; ok && n >= 0 && n <= 180000000 {
			out[key] = n
		}
	}
	for _, key := range []string{"actual_index", "start_ms", "end_ms", "previous_end_ms", "raw_start_ms", "raw_end_ms", "focal_x_ppm", "focal_y_ppm", "subject_x_ppm", "subject_y_ppm", "subject_width_ppm", "subject_height_ppm"} {
		if n, ok := values[key]; ok && n >= -180000000 && n <= 180000000 {
			out[key] = n
		}
	}
	return out
}

func SafeAttemptPhase(phase string) string {
	switch phase {
	case "input", "decode", "selection", "timeline_grow", "timeline_shrink", "timeline_total", "composition", "validation", "observation":
		return phase
	}
	return ""
}

var ErrAttemptCheckpointUnavailable = errors.New("clip checkpoint unavailable")

func (s *GenerationService) checkpoint(ctx context.Context, user, project string, c AttemptCheckpoint) error {
	store, ok := s.store.(AttemptCheckpointStore)
	if !ok {
		return nil
	}
	c.Version = 1
	c.Diagnostic.Check = SafeAttemptCheck(c.Diagnostic.Check)
	c.Diagnostic.Values = SafeAttemptValues(c.Diagnostic.Values)
	c.Diagnostic.Phase = SafeAttemptPhase(c.Diagnostic.Phase)
	if err := store.SaveAttemptCheckpoint(ctx, user, project, c); err != nil {
		slog.Warn("clip checkpoint unavailable", "job", c.JobID, "stage", c.Stage)
		return ErrAttemptCheckpointUnavailable
	}
	return nil
}

func (s *GenerationService) AttemptCheckpoint(ctx context.Context, user, project, job string) (*AttemptCheckpoint, error) {
	if _, err := s.projects.store.GetProject(ctx, user, project); err != nil {
		return nil, err
	}
	store, ok := s.store.(AttemptCheckpointStore)
	if !ok {
		return nil, nil
	}
	return store.GetAttemptCheckpoint(ctx, user, project, job)
}

func logAttemptDiagnostic(job, stage string, d AttemptDiagnostic) {
	attrs := []any{"job", job, "stage", stage, "validation_phase", SafeAttemptPhase(d.Phase), "validation_check", SafeAttemptCheck(d.Check)}
	// Check is supplied only by the owned parser; the public diagnostic projection
	// still normalizes it against its vocabulary before display.
	for key, value := range SafeAttemptValues(d.Values) {
		attrs = append(attrs, key, value)
	}
	slog.Info("clip attempt diagnostic", attrs...)
	for _, r := range d.Ranges {
		slog.Info("clip candidate range", "job", job, "cut", r.Cut, "source", r.Source, "start_ms", r.StartMS, "end_ms", r.EndMS, "range_valid", r.Valid)
	}
}

func SafeAttemptCheck(check string) string {
	switch check {
	case "input_prompt_limit", "input_settings", "input_sources":
		return check
	case "observe_source_identity", "observe_chunk_identity", "observe_segment_fields", "observe_segment_count", "observe_segment_time", "observe_segment_overlap", "observe_focal", "observe_subject_bounds", "observe_text_length", "observe_quality", "observe_subject_count", "observe_subject_text", "observe_description", "observe_scene", "observe_silent_speech":
		return check
	case "caption_measurement", "composition_cut_evidence", "composition_cut_identity", "composition_generated_bounds", "composition_generated_identity", "composition_generated_rows", "composition_item_order", "composition_observation_gap", "composition_plan_bounds", "composition_section_order", "output_encoding_or_size", "output_field_type", "output_json", "output_shape", "plan_accent", "plan_caption_fields", "plan_caption_time", "plan_chip_count", "plan_chip_label", "plan_copy_chars", "plan_copy_classes", "plan_copy_count", "plan_copy_exposure", "plan_copy_format", "plan_copy_keyword", "plan_copy_lines", "plan_copy_second_cut", "plan_copy_sequence", "plan_cut_count", "plan_cut_fade", "plan_cut_fields", "plan_cut_identity", "plan_cut_range", "plan_cut_transition", "plan_duration_limit", "plan_duration_range", "plan_focal", "plan_hook", "plan_ratio", "plan_required", "plan_source", "plan_source_metadata", "plan_style", "plan_target_duration", "plan_timeline", "plan_volume":
		return check
	}
	return "unknown"
}
