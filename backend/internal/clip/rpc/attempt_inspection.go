package rpc

import (
	"github.com/postpilot/backend/internal/clip"
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
)

func attemptInspectionProto(job, stage string, c *clip.AttemptCheckpoint, err error) *v1.ClipAttemptInspection {
	out := &v1.ClipAttemptInspection{JobId: job, Stage: stage, Status: "missing"}
	if err != nil {
		out.Status = "unavailable"
		return out
	}
	if c == nil || c.JobID != job {
		return out
	}
	out.Status = "available"
	out.EvidenceLimited = c.EvidenceLimited
	out.CompletedChunks, out.TotalChunks = int32(c.CompletedChunks), int32(c.TotalChunks)
	out.CompletedSources, out.TotalSources = int32(c.CompletedSources), int32(c.TotalSources)
	out.Observations = analysisObservationsProto(c.Observations)
	out.ValidationCheck, out.ValidationPhase = clip.SafeAttemptCheck(c.Diagnostic.Check), clip.SafeAttemptPhase(c.Diagnostic.Phase)
	out.Measurements = map[string]int32{}
	for key, value := range clip.SafeAttemptValues(c.Diagnostic.Values) {
		out.Measurements[key] = int32(value)
	}
	for _, r := range c.Diagnostic.Ranges {
		out.Ranges = append(out.Ranges, &v1.ClipAttemptRange{Cut: int32(r.Cut), Source: int32(r.Source), StartMs: int32(r.StartMS), EndMs: int32(r.EndMS), Valid: r.Valid})
	}
	return out
}
