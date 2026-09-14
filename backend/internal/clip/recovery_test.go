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
			r := RecoveryState{Version: 1, Contract: AnalysisContractVersion, Language: "ko", Observe: ref, Sources: []AnalysisSource{source}, Chunks: []ChunkAnalysis{chunk}, Plan: "saved", PlanDigest: "digest"}
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
			got := s.selectRecovery(&r, batch, model, "ko")
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

func TestRecoveryLanguageAndOptionalRegions(t *testing.T) {
	s := &GenerationService{cfg: GenerationConfig{Analysis: AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}}}
	source := AnalysisSource{RenderSource: RenderSource{ID: "source", Fingerprint: "fp", Info: MediaInfo{Width: 1080, Height: 1920, DurationMS: 15000}}, Filename: "clip.mp4"}
	batch := SourceBatch{Sources: []SourceLease{{ID: source.ID, SourceMetadata: SourceMetadata{Fingerprint: source.Fingerprint, Width: 1080, Height: 1920, DurationMS: 15000}}}}
	for _, boxes := range [][]Region{nil, {{X: .1, Y: .1, Width: .8, Height: .2}}} {
		state := RecoveryState{Version: 1, Language: "ko", Contract: AnalysisContractVersion, Sources: []AnalysisSource{source}, Chunks: []ChunkAnalysis{{SourceID: source.ID, Fingerprint: source.Fingerprint, DurationMS: 15000, Segments: []Segment{{EndMS: 15000, Event: "음식", Quality: "선명함", Certainty: CertaintyCertain, Usability: UsabilityUsable, CaptionSafe: boxes}}}}}
		for _, language := range []string{"ko", "en"} {
			got := s.selectRecovery(&state, batch, llm.ModelRef{}, language)
			if got.Language != language || (len(got.Chunks) == 1) != (language == "ko") {
				t.Fatalf("wrong reuse for %s: %+v", language, got)
			}
			if language == "ko" && len(got.Chunks[0].Segments[0].CaptionSafe) != len(boxes) {
				t.Fatal("lost optional regions")
			}
		}
		state.Language = ""
		if len(s.selectRecovery(&state, batch, llm.ModelRef{}, "ko").Chunks) != 0 {
			t.Fatal("unrecorded observation language treated as known")
		}
	}
}
