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
	return analysisObservationsProto(analyses)
}

func analysisObservationsProto(analyses []clip.SourceAnalysis) *v1.ClipObservations {
	out := &v1.ClipObservations{Status: "empty"}
	for _, a := range analyses {
		source := a.Source
		item := &v1.ClipSourceObservation{Source: retainedSourceProto(source)}
		for _, s := range a.Segments {
			focal := clip.ObservedCutFocal(s)
			// Action, motion and the two status fields are carried through
			// exactly as recorded; a legacy record sends them empty rather than
			// borrowing a certainty it never had (CLIP-51).
			item.Segments = append(item.Segments, &v1.ClipObservedSegment{
				StartMs: int32(s.StartMS), EndMs: int32(s.EndMS), Event: s.Event,
				Subjects: s.Subjects, Speech: s.Speech, Quality: s.Quality,
				Action: s.Action, Motion: s.Motion, Certainty: s.Certainty, Usability: s.Usability,
				Focal: &v1.ClipFocal{X: focal.X, Y: focal.Y},
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
	// The rate set is the server's own reading of this source's verified
	// cadence; an editor offers exactly these and never invents a slow one.
	allowed := []int32{}
	for _, rate := range clip.AllowedPlaybackRates(s.Info) {
		allowed = append(allowed, int32(rate))
	}
	return &v1.ClipRetainedSource{Id: s.ID, Fingerprint: s.Fingerprint, Filename: s.Filename,
		DurationMs: int32(s.Info.DurationMS), Width: int32(s.Info.Width), Height: int32(s.Info.Height),
		AllowedRatePermille: allowed}
}
