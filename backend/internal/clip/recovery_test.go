package clip

import (
	"github.com/postpilot/backend/internal/llm"
	"testing"
)

func TestRecoverySelectionBindsEveryAnalysisIdentity(t *testing.T) {
	s := &GenerationService{cfg: GenerationConfig{Analysis: AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}}}
	ref := llm.ModelRef{ProviderID: "p", ModelID: "o"}
	for _, change := range []string{"same", "source", "fingerprint", "model", "contract", "duration", "invalid-segment", "duplicate"} {
		t.Run(change, func(t *testing.T) {
			source := AnalysisSource{RenderSource: RenderSource{ID: "source", Fingerprint: "fingerprint", Info: MediaInfo{Width: 1920, Height: 1080, DurationMS: 15000}}, Filename: "fixture.mp4"}
			chunk := ChunkAnalysis{SourceID: source.ID, Fingerprint: source.Fingerprint, DurationMS: 15000, Segments: []Segment{{StartMS: 0, EndMS: 15000, Event: "scene", Quality: "clear", Focal: Point{X: .5, Y: .5}, Certainty: CertaintyCertain, Usability: UsabilityUsable}}}
			r := RecoveryState{Version: 1, Contract: AnalysisContractVersion, Observe: ref, Sources: []AnalysisSource{source}, Chunks: []ChunkAnalysis{chunk}, Plan: "saved", PlanDigest: "digest"}
			batch := SourceBatch{Sources: []SourceLease{{ID: source.ID, SourceMetadata: SourceMetadata{Fingerprint: source.Fingerprint, Width: 1920, Height: 1080, DurationMS: 15000}}}}
			model := ref
			switch change {
			case "source":
				batch.Sources[0].ID = "new"
			case "fingerprint":
				batch.Sources[0].Fingerprint = "new"
			case "model":
				model.ModelID = "other"
			case "contract":
				r.Contract = "older"
			case "duration":
				batch.Sources[0].DurationMS = 17000
			case "invalid-segment":
				r.Chunks[0].Segments[0].EndMS = 16000
			case "duplicate":
				r.Sources = append(r.Sources, source)
				r.Chunks = append(r.Chunks, chunk)
			}
			got := s.selectRecovery(&r, batch, model)
			want := 0
			if change == "same" || change == "duplicate" {
				want = 1
			}
			if len(got.Chunks) != want {
				t.Fatalf("unexpected reused chunks=%d", len(got.Chunks))
			}
			if change != "same" && got.Plan != "" {
				t.Fatal("incompatible/ambiguous evidence retained candidate plan")
			}
		})
	}
}
