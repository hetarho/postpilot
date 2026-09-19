package app

import (
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

func TestRecoverySelectionBindsEveryAnalysisIdentity(t *testing.T) {
	s := &GenerationService{cfg: clip.GenerationConfig{Analysis: clip.AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}}}
	ref := llm.ModelRef{ProviderID: "p", ModelID: "o"}
	for _, change := range []string{"same", "source", "fingerprint", "model", "contract", "duration", "invalid-segment", "duplicate"} {
		t.Run(change, func(t *testing.T) {
			source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fingerprint", Info: clip.MediaInfo{Width: 1920, Height: 1080, DurationMS: 15000}}, Filename: "fixture.mp4"}
			chunk := clip.ChunkAnalysis{SourceID: source.ID, Fingerprint: source.Fingerprint, DurationMS: 15000, Segments: []clip.Segment{{StartMS: 0, EndMS: 15000, Event: "scene", Quality: "clear", Focal: clip.Point{X: .5, Y: .5}, Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable}}}
			r := clip.RecoveryState{Version: 1, Contract: clip.AnalysisContractVersion, Language: "ko", Observe: ref, Sources: []clip.AnalysisSource{source}, Chunks: []clip.ChunkAnalysis{chunk}, Plan: "saved", PlanDigest: "digest"}
			batch := clip.SourceBatch{Sources: []clip.SourceLease{{ID: source.ID, SourceMetadata: clip.SourceMetadata{Fingerprint: source.Fingerprint, Width: 1920, Height: 1080, DurationMS: 15000}}}}
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
	s := &GenerationService{cfg: clip.GenerationConfig{Analysis: clip.AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}}}
	source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{Width: 1080, Height: 1920, DurationMS: 15000}}, Filename: "clip.mp4"}
	batch := clip.SourceBatch{Sources: []clip.SourceLease{{ID: source.ID, SourceMetadata: clip.SourceMetadata{Fingerprint: source.Fingerprint, Width: 1080, Height: 1920, DurationMS: 15000}}}}
	for _, boxes := range [][]clip.Region{nil, {{X: .1, Y: .1, Width: .8, Height: .2}}} {
		state := clip.RecoveryState{Version: 1, Language: "ko", Contract: clip.AnalysisContractVersion, Sources: []clip.AnalysisSource{source}, Chunks: []clip.ChunkAnalysis{{SourceID: source.ID, Fingerprint: source.Fingerprint, DurationMS: 15000, Segments: []clip.Segment{{EndMS: 15000, Event: "음식", Quality: "선명함", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable, CaptionSafe: boxes}}}}}
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
