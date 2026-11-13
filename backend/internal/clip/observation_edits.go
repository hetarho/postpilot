package clip

import (
	"slices"
	"strconv"
	"strings"
)

// ObservationEdit is the owner's correction of one observed scene's descriptive text
// (CLIP-235), keyed by its observation id `<sourceId>/<index>`. A nil field is left as
// observed; Subjects replaces the list only when SubjectsSet holds, so an emptied list is
// told apart from an untouched one. Time range, scene, motion, quality, the status pair,
// the principal-subject box, readable text and caption-safe regions are measurements
// composition reads, so no edit carries them.
type ObservationEdit struct {
	ObservationID         string
	Event, Action, Speech *string
	Subjects              []string
	SubjectsSet           bool
}

// The descriptive fields an owner correction changes, as a segment's OwnerFields name them.
const (
	OwnerFieldEvent    = "event"
	OwnerFieldAction   = "action"
	OwnerFieldSubjects = "subjects"
	OwnerFieldSpeech   = "speech"
)

// ObservationEditError names the observation and the field a correction was refused over.
type ObservationEditError struct{ ObservationID, Field, Check string }

func (e *ObservationEditError) Error() string {
	return "clip observation edit refused: " + e.Check
}

// ObservationIndex splits an observation id into its source and its scene index.
func ObservationIndex(id string) (string, int, bool) {
	at := strings.LastIndex(id, "/")
	if at <= 0 {
		return "", 0, false
	}
	index, err := strconv.Atoi(id[at+1:])
	if err != nil || index < 0 || strconv.Itoa(index) != id[at+1:] {
		return "", 0, false
	}
	return id[:at], index, true
}

// audible is whether a scene records speech: the fact CLIP-127 and CLIP-129 read.
func audible(s Segment) bool { return strings.TrimSpace(s.Speech) != "" }

// ApplyObservationEdits returns the analyses with the owner's corrections applied (CLIP-235)
// and leaves its input untouched. Each changed scene records the fields the owner changed
// and its source is marked edited by hand; a value equal to the recorded one changes
// nothing. An unknown or repeated id, a text past the observation's own bounds and a
// correction that would make a scene with speech silent, or a silent one speak, are
// refused, since audibility binds the rate rules as observed (CLIP-129).
func ApplyObservationEdits(l AnalysisLimits, analyses []SourceAnalysis, edits []ObservationEdit) ([]SourceAnalysis, error) {
	out := make([]SourceAnalysis, len(analyses))
	for i, a := range analyses {
		out[i] = a
		out[i].Segments = slices.Clone(a.Segments)
	}
	seen := map[string]bool{}
	for _, e := range edits {
		source, index, ok := ObservationIndex(e.ObservationID)
		at := slices.IndexFunc(out, func(a SourceAnalysis) bool { return a.Source.ID == source })
		if !ok || at < 0 || index >= len(out[at].Segments) {
			return nil, &ObservationEditError{ObservationID: e.ObservationID, Field: "observation_id", Check: "observation_unknown"}
		}
		if seen[e.ObservationID] {
			return nil, &ObservationEditError{ObservationID: e.ObservationID, Field: "observation_id", Check: "observation_repeated"}
		}
		seen[e.ObservationID] = true
		segment := out[at].Segments[index]
		changed := slices.Clone(segment.OwnerFields)
		dirty := false
		set := func(field string, current *string, value *string) {
			if value == nil || strings.TrimSpace(*value) == *current {
				return
			}
			*current = strings.TrimSpace(*value)
			changed, dirty = append(changed, field), true
		}
		before := audible(segment)
		set(OwnerFieldEvent, &segment.Event, e.Event)
		set(OwnerFieldAction, &segment.Action, e.Action)
		set(OwnerFieldSpeech, &segment.Speech, e.Speech)
		if audible(segment) != before {
			return nil, &ObservationEditError{ObservationID: e.ObservationID, Field: OwnerFieldSpeech, Check: "speech_audibility"}
		}
		if e.SubjectsSet {
			subjects := make([]string, 0, len(e.Subjects))
			for _, subject := range e.Subjects {
				subjects = append(subjects, strings.TrimSpace(subject))
			}
			if !slices.Equal(subjects, segment.Subjects) {
				segment.Subjects = subjects
				changed, dirty = append(changed, OwnerFieldSubjects), true
			}
		}
		if !dirty {
			continue
		}
		slices.Sort(changed)
		changed = slices.Compact(changed)
		// The corrected scene still owes what any observed scene owes: bounded text and
		// a description unless its footage is unknowable (CLIP-10).
		if err := validateSegments(l, []Segment{segment}, segment.StartMS, segment.EndMS, segment.Observed()); err != nil {
			check := "observation_invalid"
			if d, ok := DiagnosticFromError(err); ok && d.Check != "" {
				check = d.Check
			}
			return nil, &ObservationEditError{ObservationID: e.ObservationID, Check: check}
		}
		segment.OwnerEdited, segment.OwnerFields = true, changed
		out[at].Segments[index] = segment
		out[at].OwnerEdited = true
	}
	return out, nil
}
