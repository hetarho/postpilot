package rpc

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestAttemptInspectionProjectionKeepsProvenanceAndClosedDiagnostics(t *testing.T) {
	c := &clip.AttemptCheckpoint{JobID: "attempt", Stage: "analyze", CompletedChunks: 1, TotalChunks: 3, TotalSources: 2, Diagnostic: clip.AttemptDiagnostic{Check: "plan_timeline", Phase: "timeline_grow", Values: map[string]int{"after_ms": 12000, "private-field": 9}}, Observations: []clip.SourceAnalysis{{Source: clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp"}, Filename: "original.mp4"}, Segments: []clip.Segment{{Event: "recorded observation", EndMS: 1000}}}}}
	p := attemptInspectionProto("attempt", "plan", c, nil)
	if p.Status != "available" || p.Stage != "plan" || p.CompletedChunks != 1 || p.Observations.Sources[0].Segments[0].Event != "recorded observation" || p.Measurements["after_ms"] != 12000 || len(p.Measurements) != 1 {
		t.Fatal(p)
	}
	raw, err := protojson.Marshal(p)
	if err != nil || strings.Contains(string(raw), "private-field") || strings.Contains(string(raw), "editPlan") || strings.Contains(string(raw), "result") {
		t.Fatal(string(raw), err)
	}
	if attemptInspectionProto("new-attempt", "prepare", c, nil).Status != "missing" || attemptInspectionProto("attempt", "plan", nil, nil).Status != "missing" || attemptInspectionProto("attempt", "plan", nil, errors.New("private DB detail")).Status != "unavailable" {
		t.Fatal("absence or cross-attempt identity lost")
	}
}
