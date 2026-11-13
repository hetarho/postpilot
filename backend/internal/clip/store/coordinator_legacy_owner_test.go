package store_test

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// CLIP-235 pins a correction by source identity/fingerprint, including a retained v1
// range which crosses today's observation-chunk boundary. No new provider call or
// reshaping of the corrected range is authorized by a different observation model.
func TestCoordinatorLegacyOwnerObservationRemainsPinned(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	_, analyses := observations(t, h)
	first := h.batch.Sources[0].ID
	legacy := analyses[0].Segments[0]
	legacy.StartMS = 0
	legacy.EndMS = analyses[0].Source.Info.DurationMS
	legacy.Certainty, legacy.Usability = "", ""
	legacy.Event = "legacy source scene"
	if legacy.EndMS <= h.cfg.Analysis.ChunkMS {
		t.Fatal("fixture does not cross a current chunk boundary")
	}
	analyses[0].Segments = []clip.Segment{legacy}
	raw, err := json.Marshal(analyses)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET analysis_json=? WHERE id=?`, string(raw), h.project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{
		ObservationID: clip.ObservationID(first, 0), Event: text("owner corrected legacy scene"),
	}}); err != nil {
		t.Fatal("valid retained legacy observation could not be corrected", err)
	}
	observedUnder(t, h, llm.ModelRef{ProviderID: "p", ModelID: "older"})
	quote, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
	if err != nil {
		t.Fatal(err)
	}
	if quote.Pricing.ObservationCalls != 1 || quote.Pricing.ReusedChunks != 2 {
		t.Fatalf("legacy owner correction was priced for re-observation: got calls=%d reused=%d, want only the uncorrected source's one call and two reused chunks", quote.Pricing.ObservationCalls, quote.Pricing.ReusedChunks)
	}
	id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(quote))
	if err != nil {
		t.Fatal(err)
	}
	h.planner.id = id
	before := h.planner.observe
	h.media.durations = []int{h.batch.Sources[1].DurationMS}
	if err := h.run(t); err != nil {
		t.Fatal(err)
	}
	if h.planner.observe-before != 1 {
		t.Fatalf("observed %d chunks; only the uncorrected source was authorized", h.planner.observe-before)
	}
	if len(h.planner.storylines) != 1 {
		t.Fatal("storyline writer was not called once")
	}
	check := func(analyses []clip.SourceAnalysis) {
		t.Helper()
		at := slices.IndexFunc(analyses, func(a clip.SourceAnalysis) bool { return a.Source.ID == first })
		if at < 0 || !analyses[at].OwnerEdited || len(analyses[at].Segments) != 1 {
			t.Fatal("legacy owner range was dropped or reshaped")
		}
		segment := analyses[at].Segments[0]
		if !segment.OwnerEdited || segment.Event != "owner corrected legacy scene" || segment.StartMS != legacy.StartMS || segment.EndMS != legacy.EndMS || segment.Subject != legacy.Subject || segment.Motion != legacy.Motion || segment.Quality != legacy.Quality || segment.Certainty != legacy.Certainty || segment.Usability != legacy.Usability {
			t.Fatalf("writer did not receive the unchanged corrected legacy range: %+v", segment)
		}
	}
	check(h.planner.storylines[0].Analyses)
	_, stored := observations(t, h)
	check(stored)
}
