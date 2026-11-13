package store_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
)

const legacyCorrection = "owner corrected legacy scene"

// storeObservations replaces the project's stored observations as an earlier contract left them.
func storeObservations(t *testing.T, h *generationHarness, analyses []clip.SourceAnalysis) {
	t.Helper()
	raw, err := json.Marshal(analyses)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Writer.Exec(`UPDATE clip_projects SET analysis_json=? WHERE id=?`, string(raw), h.project.ID); err != nil {
		t.Fatal(err)
	}
}

// legacyObservation leaves the first source with one retained clip-observation-v1 range over its
// whole timeline — no status, crossing a current chunk boundary where crossing holds — corrected
// through the owner's own save when correct holds.
func legacyObservation(t *testing.T, h *generationHarness, correct, crossing bool) (string, clip.Segment) {
	t.Helper()
	_, analyses := observations(t, h)
	first := h.batch.Sources[0].ID
	legacy := analyses[0].Segments[0]
	legacy.StartMS, legacy.EndMS = 0, analyses[0].Source.Info.DurationMS
	legacy.Certainty, legacy.Usability, legacy.Event = "", "", "legacy source scene"
	if analyses[0].Source.ID != first || crossing && legacy.EndMS <= h.cfg.Analysis.ChunkMS {
		t.Fatal("fixture does not cross a current chunk boundary")
	}
	analyses[0].Segments = []clip.Segment{legacy}
	storeObservations(t, h, analyses)
	if correct {
		if _, err := h.projects.SaveObservationEdits(t.Context(), "alice", h.project.ID, []clip.ObservationEdit{{ObservationID: clip.ObservationID(first, 0), Event: text(legacyCorrection)}}); err != nil {
			t.Fatal(err)
		}
	}
	return first, legacy
}

// pinnedIntact asserts the corrected legacy range arrived whole: one scene with its original
// range, no invented status, the owner's text and flags, and its measurements as recorded.
func pinnedIntact(t *testing.T, analyses []clip.SourceAnalysis, first string, legacy clip.Segment) {
	t.Helper()
	at := slices.IndexFunc(analyses, func(a clip.SourceAnalysis) bool { return a.Source.ID == first })
	if at < 0 || !analyses[at].OwnerEdited || len(analyses[at].Segments) != 1 {
		t.Fatal("the corrected legacy range was dropped or reshaped")
	}
	s := analyses[at].Segments[0]
	if !s.OwnerEdited || s.Event != legacyCorrection || s.StartMS != legacy.StartMS || s.EndMS != legacy.EndMS || s.Observed() || s.Subject != legacy.Subject || s.Focal != legacy.Focal || s.Quality != legacy.Quality {
		t.Fatalf("the corrected legacy range changed: %+v", s)
	}
}

// chunkSlots is how many current chunk slots the first stored source and the whole batch need.
func chunkSlots(t *testing.T, h *generationHarness) (own, total int) {
	t.Helper()
	_, analyses := observations(t, h)
	for i, a := range analyses {
		n := (a.Source.Info.DurationMS + h.cfg.Analysis.ChunkMS - 1) / h.cfg.Analysis.ChunkMS
		if i == 0 {
			own = n
		}
		total += n
	}
	return own, total
}

// CLIP-235, CLIP-234: 다시 관찰하기 replaces a corrected legacy source too — every chunk of it is
// quoted and observed again under the current contract and the owner's edits are gone.
func TestReanalysisReplacesACorrectedLegacySource(t *testing.T) {
	h := generationSetup(t)
	if _, err := analyzed(t, h); err != nil {
		t.Fatal(err)
	}
	own, _ := chunkSlots(t, h)
	first, _ := legacyObservation(t, h, true, true)
	before := h.planner.observe
	q, err := analyzed(t, h, first)
	if err != nil {
		t.Fatal(err)
	}
	if q.Pricing.ObservationCalls != own || h.planner.observe-before != own {
		t.Fatalf("reanalysis observed %d chunks for a quote of %d, want the corrected source's %d", h.planner.observe-before, q.Pricing.ObservationCalls, own)
	}
	_, stored := observations(t, h)
	if stored[0].Source.ID != first || stored[0].OwnerEdited || len(stored[0].Segments) != own {
		t.Fatalf("reanalysis kept the corrected legacy source: %+v", stored[0])
	}
	for _, s := range stored[0].Segments {
		if !s.Observed() || s.OwnerEdited || s.Event == legacyCorrection {
			t.Fatalf("reanalysis kept a legacy or corrected scene: %+v", s)
		}
	}
}

// CLIP-235, CLIP-93: a retained legacy range is reused only while the owner corrected it and its
// source still has the identity and fingerprint the batch holds; otherwise the source is quoted
// and observed again, and an uncorrected legacy range is never relabelled as current evidence.
func TestALegacyObservationIsPinnedOnlyWhileCorrectedAndItsSourceMatches(t *testing.T) {
	for _, name := range []string{"uncorrected", "fingerprint", "duration"} {
		t.Run(name, func(t *testing.T) {
			h := generationSetup(t)
			if _, err := analyzed(t, h); err != nil {
				t.Fatal(err)
			}
			own, total := chunkSlots(t, h)
			first, _ := legacyObservation(t, h, name != "uncorrected", true)
			if name != "uncorrected" {
				_, analyses := observations(t, h)
				if name == "fingerprint" {
					analyses[0].Source.Fingerprint = "another-source"
				} else {
					analyses[0].Source.Info.DurationMS += 5000
				}
				storeObservations(t, h, analyses)
			}
			observedUnder(t, h, llm.ModelRef{ProviderID: "p", ModelID: "older"})
			q, err := h.service.QuoteStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w")
			if err != nil {
				t.Fatal(err)
			}
			if q.Pricing.ObservationCalls != total || q.Pricing.ReusedChunks != 0 {
				t.Fatalf("a legacy range was reused: calls=%d reused=%d, want %d and 0", q.Pricing.ObservationCalls, q.Pricing.ReusedChunks, total)
			}
			if name != "uncorrected" {
				return
			}
			id, err := h.service.StartStoryline(t.Context(), "alice", h.project.ID, h.batch.ID, "p/o", "p/w", approve(q))
			if err != nil {
				t.Fatal(err)
			}
			h.planner.id = id
			before := h.planner.observe
			if err := h.run(t); err != nil {
				t.Fatal(err)
			}
			if h.planner.observe-before != total {
				t.Fatal("the uncorrected legacy source was not observed again")
			}
			for _, analyses := range [][]clip.SourceAnalysis{h.planner.storylines[0].Analyses, func() []clip.SourceAnalysis { _, a := observations(t, h); return a }()} {
				at := slices.IndexFunc(analyses, func(a clip.SourceAnalysis) bool { return a.Source.ID == first })
				if at < 0 || analyses[at].OwnerEdited || len(analyses[at].Segments) != own {
					t.Fatal("the uncorrected legacy range reached the writer or the result")
				}
				for _, s := range analyses[at].Segments {
					if !s.Observed() || s.Event == "legacy source scene" {
						t.Fatalf("an uncorrected legacy scene was relabelled as current evidence: %+v", s)
					}
				}
			}
		})
	}
}

// CLIP-235, CLIP-203: browser preparation copies no slot of a corrected legacy source, the
// approval, the manifest and the run agree on reusing it, and the writer receives it whole.
func TestBrowserPreparationKeepsACorrectedLegacySourceUncopied(t *testing.T) {
	f := newAnalysisFixture(t)
	if _, err := analyzed(t, f.h); err != nil {
		t.Fatal(err)
	}
	own, total := chunkSlots(t, f.h)
	first, legacy := legacyObservation(t, f.h, true, true)
	f.q = quote(t, f.h)
	if f.q.Pricing.ObservationCalls != 0 || f.q.Pricing.ReusedChunks != total {
		t.Fatalf("browser quote did not reuse the corrected source: %+v", f.q.Pricing)
	}
	f.begin(t)
	pinned := 0
	for _, c := range f.p.Reused {
		if c.SourceID == first {
			pinned++
		}
	}
	if len(f.p.Copies) != 0 || len(f.p.Reused) != total || pinned != own || f.p.Sources[0].ID != first {
		t.Fatalf("browser manifest copies or omits the corrected source: copies=%d reused=%+v", len(f.p.Copies), f.p.Reused)
	}
	id := f.start(t)
	before, probes := f.h.planner.observe, f.h.media.probes
	for range 3 {
		j, err := f.h.jobs.PickNextQueued(t.Context(), f.now)
		if err != nil {
			t.Fatal(err)
		}
		f.h.planner.id = id
		err = f.h.service.Run(t.Context(), "alice", id, f.h.project.ID, j.Payload, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, job.ErrYield) {
			t.Fatal(err)
		}
		if _, err = f.a.Complete(t.Context(), "alice", f.p.ID); err != nil {
			t.Fatal(err)
		}
	}
	if f.h.planner.observe != before || f.h.media.probes != probes {
		t.Fatal("browser continuation observed or probed again")
	}
	pinnedIntact(t, f.h.planner.input.Analyses, first, legacy)
}

// CLIP-235, ARCH-45: remote preparation asks the worker for no copy of a corrected legacy source,
// observes only the uncorrected source and hands the writer the corrected range whole.
func TestRemotePreparationKeepsACorrectedLegacySourceUncopied(t *testing.T) {
	g := remoteGenerationSetup(t)
	id := g.h.start(t)
	if err := g.runJob(t, g.pick(t)); !errors.Is(err, job.ErrYield) {
		t.Fatal(err)
	}
	auth, raw := g.preparedReceipt(t)
	if err := g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal(err)
	}
	if err := g.runJob(t, g.pick(t)); err != nil {
		t.Fatal(err)
	}
	if err := g.h.sources.ReleaseAttempt(t.Context(), "alice", id, time.Now()); err != nil {
		t.Fatal(err)
	}
	own, total := chunkSlots(t, g.h)
	other := total - own
	// The remote fixture's sources are each one chunk long: the range carries no status, which
	// no current chunk would accept either.
	first, legacy := legacyObservation(t, g.h, true, false)
	observedUnder(t, g.h, llm.ModelRef{ProviderID: "p", ModelID: "older"})
	q := quote(t, g.h)
	if q.Pricing.ObservationCalls != other || q.Pricing.ReusedChunks != own {
		t.Fatalf("remote quote: calls=%d reused=%d, want %d and %d", q.Pricing.ObservationCalls, q.Pricing.ReusedChunks, other, own)
	}
	next := g.h.start(t)
	if err := g.runJob(t, g.pick(t)); !errors.Is(err, job.ErrYield) {
		t.Fatal(err)
	}
	stage, err := g.h.store.MediaStageForJob(t.Context(), next, clip.MediaPrepare)
	if err != nil {
		t.Fatal(err)
	}
	task, err := mediacodec.DecodeTask(stage.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if task.Sources[0].ID != first || len(task.Sources[0].ReusedChunks) != own || len(task.Sources[1].ReusedChunks) != 0 {
		t.Fatalf("remote task copies the corrected source: %+v", task.Sources)
	}
	auth, raw = g.preparedReceipt(t)
	result, err := mediacodec.DecodeResult(raw)
	if err != nil || len(result.Outputs) != other {
		t.Fatal("remote preparation copied a corrected slot", err)
	}
	if err = g.artifacts.Complete(t.Context(), auth, raw); err != nil {
		t.Fatal(err)
	}
	before := g.h.planner.observe
	if err = g.runJob(t, g.pick(t)); err != nil {
		t.Fatal(err)
	}
	if g.h.planner.observe-before != other || g.h.media.probes != 0 {
		t.Fatal("remote continuation observed a corrected slot or read originals on the API")
	}
	pinnedIntact(t, g.h.planner.input.Analyses, first, legacy)
	_, stored := observations(t, g.h)
	pinnedIntact(t, stored, first, legacy)
}
