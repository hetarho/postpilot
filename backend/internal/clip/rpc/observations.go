package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

// This allowlisted projection never exposes the retained renderer's metadata,
// storage keys or raw analysis JSON. Reading it does not invoke a model.
func observationsProto(p clip.Project) *v1.ClipObservations {
	analyses, err := clip.RetainedObservations(p)
	if err != nil {
		return &v1.ClipObservations{Status: "unavailable"}
	}
	out := &v1.ClipObservations{Status: "empty"}
	for _, a := range analyses {
		source := a.Source
		item := &v1.ClipSourceObservation{Source: retainedSourceProto(source)}
		for _, s := range a.Segments {
			item.Segments = append(item.Segments, &v1.ClipObservedSegment{
				StartMs: int32(s.StartMS), EndMs: int32(s.EndMS), Event: s.Event,
				Subjects: s.Subjects, Speech: s.Speech, Quality: s.Quality,
			})
		}
		out.Sources = append(out.Sources, item)
	}
	if len(out.Sources) > 0 {
		out.Status = "available"
	}
	return out
}

func retainedSourceProto(s clip.AnalysisSource) *v1.ClipRetainedSource {
	return &v1.ClipRetainedSource{Id: s.ID, Fingerprint: s.Fingerprint, Filename: s.Filename,
		DurationMs: int32(s.Info.DurationMS), Width: int32(s.Info.Width), Height: int32(s.Info.Height)}
}
