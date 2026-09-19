package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

// stageStore answers the two reads accept makes; every other port method is unreachable
// from the stages under test.
type stageStore struct {
	clip.GenerationStore
	batch clip.SourceBatch
	quote clip.GenerationQuote
}

func (s stageStore) BatchForJob(context.Context, string, string) (clip.SourceBatch, error) {
	return s.batch, nil
}
func (s stageStore) SaveQuote(context.Context, clip.GenerationQuote, time.Time) error { return nil }
func (s stageStore) GetQuote(context.Context, string, string) (clip.GenerationQuote, error) {
	return s.quote, nil
}
func (s stageStore) LinkApprovedSourceJob(context.Context, clip.GenerationQuote, string, time.Time) error {
	return nil
}

// stagePlanner accepts every brief and answers observations from a script.
type stagePlanner struct {
	budgets      clip.CompletionBudgets
	observations []clip.ChunkAnalysis
	observed     int
}

func (p *stagePlanner) ValidateModels(llm.ModelRef, llm.ModelRef) error { return nil }
func (p *stagePlanner) ValidatePreparation(llm.ModelRef, clip.PlanningInput, []clip.AnalysisSource) error {
	return nil
}
func (p *stagePlanner) Budgets() clip.CompletionBudgets { return p.budgets }
func (p *stagePlanner) ObserveChunk(context.Context, llm.ModelRef, clip.ChunkInput) (clip.ChunkAnalysis, llm.Usage, error) {
	out := p.observations[p.observed]
	p.observed++
	return out, llm.Usage{}, nil
}
func (p *stagePlanner) Flow(context.Context, llm.ModelRef, clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	return clip.EditPlan{}, llm.Usage{}, errors.New("not in this test")
}
func (p *stagePlanner) Narrate(context.Context, llm.ModelRef, clip.NarrationInput) (clip.EditPlan, llm.Usage, error) {
	return clip.EditPlan{}, llm.Usage{}, errors.New("not in this test")
}
func (p *stagePlanner) Revise(context.Context, llm.ModelRef, clip.RevisionInput) (clip.EditPlan, llm.Usage, error) {
	return clip.EditPlan{}, llm.Usage{}, errors.New("not in this test")
}
func (p *stagePlanner) Plan(context.Context, llm.ModelRef, clip.PlanningInput) (clip.EditPlan, llm.Usage, error) {
	return clip.EditPlan{}, llm.Usage{}, errors.New("not in this test")
}

type stageFinisher struct {
	completed []clip.AttemptResult
	err       error
}

func (f *stageFinisher) Complete(_ context.Context, c clip.AttemptResult) error {
	f.completed = append(f.completed, c)
	return f.err
}
func (f *stageFinisher) Recover(context.Context) error { return nil }

func approvedWork(t *testing.T) (clip.GenerationPayload, clip.SourceBatch, clip.GenerationQuote) {
	t.Helper()
	pricing := approvedPricing(1)
	lease := clip.SourceLease{ID: "src", SourceMetadata: clip.SourceMetadata{Filename: "a.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 16000, Width: 640, Height: 640, Fingerprint: "f"}}
	batch := clip.SourceBatch{ID: "batch", UserID: "alice", ProjectID: "clip", State: "consuming", Sources: []clip.SourceLease{lease}}
	payload := clip.GenerationPayload{Version: 3, ProjectID: "clip", Batch: batch, Observe: "p/o", Write: "p/w", Language: "ko", Ratio: "vertical", TargetDurationMS: 15000,
		Approval: &clip.GenerationApproval{QuoteID: "quote", MaxCredits: pricing.MaxCredits, Pricing: pricing}}
	quote := clip.GenerationQuote{ID: "quote", UserID: "alice", ProjectID: "clip", BatchID: "batch", ConsumedJobID: "job", Pricing: pricing}
	return payload, batch, quote
}

func newRun(store clip.GenerationStore, planner clip.Planner, finisher clip.ClipFinisher) *generationRun {
	s := &GenerationService{store: store, planner: planner, finisher: finisher, jobs: neutralRunJobs{}, cfg: clip.GenerationConfig{Analysis: clip.AnalysisLimits{ChunkMS: 60000, MaxSources: 20, MaxSourceDurationMS: 1800000, MaxSegments: 60, MaxTextRunes: 2000, MaxSubjects: 20}, ReadTTL: time.Minute, CleanupTimeout: time.Second, OrphanMinAge: time.Hour}}
	return &generationRun{s: s, ctx: context.Background(), user: "alice", job: "job", project: "clip", stage: "prepare", checkpoint: clip.AttemptCheckpoint{Version: 1, JobID: "job", Stage: "prepare"}}
}

type neutralRunJobs struct{}

func (neutralRunJobs) Enqueue(context.Context, clip.GenerationStart) (string, error) {
	return "", clip.ErrBusy
}
func (neutralRunJobs) Activate(context.Context, string, string) error           { return nil }
func (neutralRunJobs) FailQueued(context.Context, string, string) (bool, error) { return false, nil }
func (neutralRunJobs) ReserveApproved(ctx context.Context, _, _ string, _ clip.GenerationApproval, _ int) (context.Context, error) {
	return ctx, nil
}
func (neutralRunJobs) Active(context.Context, string, string) (*clip.ClipJob, error) { return nil, nil }
func (neutralRunJobs) Get(context.Context, string, string) (*clip.ClipJob, error)    { return nil, nil }

func TestAcceptRefusesWhatIsNotTheApprovedWork(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(p *clip.GenerationPayload, b *clip.SourceBatch, q *clip.GenerationQuote) []byte
		want   error
	}{
		{"malformed payload", func(*clip.GenerationPayload, *clip.SourceBatch, *clip.GenerationQuote) []byte {
			return []byte(`{"version":`)
		}, clip.ErrInvalid},
		{"unsupported version", func(p *clip.GenerationPayload, _ *clip.SourceBatch, _ *clip.GenerationQuote) []byte {
			p.Version = 2
			return nil
		}, clip.ErrQuoteRequired},
		{"no approval", func(p *clip.GenerationPayload, _ *clip.SourceBatch, _ *clip.GenerationQuote) []byte {
			p.Approval = nil
			return nil
		}, clip.ErrQuoteRequired},
		{"another batch", func(p *clip.GenerationPayload, _ *clip.SourceBatch, _ *clip.GenerationQuote) []byte {
			p.Batch.ID = "other"
			return nil
		}, clip.ErrInvalid},
		{"batch no longer consuming", func(_ *clip.GenerationPayload, b *clip.SourceBatch, _ *clip.GenerationQuote) []byte {
			b.State = "ready"
			return nil
		}, clip.ErrSourceState},
		{"writer differs from the priced one", func(p *clip.GenerationPayload, _ *clip.SourceBatch, _ *clip.GenerationQuote) []byte {
			p.Write = "p/other"
			return nil
		}, clip.ErrPricingUnavailable},
		{"quote consumed by another job", func(_ *clip.GenerationPayload, _ *clip.SourceBatch, q *clip.GenerationQuote) []byte {
			q.ConsumedJobID = "job-2"
			return nil
		}, clip.ErrQuoteChanged},
		{"quote priced differently", func(_ *clip.GenerationPayload, _ *clip.SourceBatch, q *clip.GenerationQuote) []byte {
			q.Pricing.MaxCredits++
			return nil
		}, clip.ErrQuoteChanged},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, b, q := approvedWork(t)
			raw := c.mutate(&p, &b, &q)
			if raw == nil {
				var err error
				if raw, err = json.Marshal(p); err != nil {
					t.Fatal(err)
				}
			}
			r := newRun(stageStore{batch: b, quote: q}, &stagePlanner{}, &stageFinisher{})
			if err := r.accept(raw); !errors.Is(err, c.want) {
				t.Fatalf("accept = %v, want %v", err, c.want)
			}
		})
	}
	t.Run("the approved work is accepted", func(t *testing.T) {
		p, b, q := approvedWork(t)
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		planner := &stagePlanner{budgets: clip.CompletionBudgets{Observe: 8192, Flow: 32768, Narration: 32768}}
		r := newRun(stageStore{batch: b, quote: q}, planner, &stageFinisher{})
		if err := r.accept(raw); err != nil {
			t.Fatal(err)
		}
		if r.b.ID != "batch" || r.pricing != p.Approval.Pricing || r.recovery.JobID != "job" || r.recovery.Plan != "" {
			t.Fatalf("accepted run = %+v", r)
		}
	})
	t.Run("a budget the planner no longer runs changes the quote", func(t *testing.T) {
		p, b, q := approvedWork(t)
		raw, _ := json.Marshal(p)
		r := newRun(stageStore{batch: b, quote: q}, &stagePlanner{budgets: clip.CompletionBudgets{Observe: 4096, Flow: 32768, Narration: 32768}}, &stageFinisher{})
		if err := r.accept(raw); !errors.Is(err, clip.ErrQuoteChanged) {
			t.Fatalf("accept = %v, want %v", err, clip.ErrQuoteChanged)
		}
	})
}

func TestAnalyzeChecksEachObservationAgainstItsChunk(t *testing.T) {
	source := clip.AnalysisSource{RenderSource: clip.RenderSource{ID: "src", Fingerprint: "f", Info: clip.MediaInfo{DurationMS: 16000, Width: 640, Height: 640}}, Filename: "a.mp4"}
	chunk := func(t *testing.T) clip.AnalysisChunk {
		path := filepath.Join(t.TempDir(), "chunk.mp4")
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return clip.AnalysisChunk{Index: 0, OffsetMS: 0, DurationMS: 16000, Path: path, Bytes: 1}
	}
	good := clip.ChunkAnalysis{SourceID: "src", Fingerprint: "f", Index: 0, OffsetMS: 0, DurationMS: 16000, Segments: []clip.Segment{{
		StartMS: 0, EndMS: 16000, Event: "a bowl is set down", Quality: "steady", Certainty: clip.CertaintyCertain, Usability: clip.UsabilityUsable, Focal: clip.Point{X: .5, Y: .5},
	}}}
	t.Run("an observation about another source is refused before it is kept", func(t *testing.T) {
		wrong := good
		wrong.SourceID = "other"
		r := newRun(stageStore{}, &stagePlanner{observations: []clip.ChunkAnalysis{wrong}}, &stageFinisher{})
		r.pricing, r.sources = approvedPricing(1), []clip.AnalysisSource{source}
		r.prepared = []preparedChunk{{source: 0, chunk: chunk(t)}}
		r.checkpoint.Observations = make([]clip.SourceAnalysis, 1)
		if err := r.analyze(); !errors.Is(err, clip.ErrInvalid) || len(r.recovery.Chunks) != 0 {
			t.Fatalf("analyze = %v, recovery chunks = %d", err, len(r.recovery.Chunks))
		}
	})
	t.Run("a reused chunk costs no call and a fresh one is kept for recovery", func(t *testing.T) {
		planner := &stagePlanner{observations: []clip.ChunkAnalysis{good}}
		r := newRun(stageStore{}, planner, &stageFinisher{})
		r.pricing, r.sources = approvedPricing(1), []clip.AnalysisSource{source}
		reused := good
		r.prepared = []preparedChunk{{source: 0, chunk: chunk(t), reused: &reused}}
		r.checkpoint.Observations = make([]clip.SourceAnalysis, 1)
		if err := r.analyze(); err != nil {
			t.Fatal(err)
		}
		if planner.observed != 0 || r.checkpoint.CompletedChunks != 1 || r.checkpoint.CompletedSources != 1 || len(r.analyses) != 1 || r.stage != "analyze" {
			t.Fatalf("reused chunk: observed=%d checkpoint=%+v analyses=%d stage=%s", planner.observed, r.checkpoint, len(r.analyses), r.stage)
		}
		fresh := newRun(stageStore{}, planner, &stageFinisher{})
		fresh.pricing, fresh.sources = approvedPricing(1), []clip.AnalysisSource{source}
		fresh.prepared = []preparedChunk{{source: 0, chunk: chunk(t)}}
		fresh.checkpoint.Observations = make([]clip.SourceAnalysis, 1)
		if err := fresh.analyze(); err != nil || planner.observed != 1 || len(fresh.recovery.Chunks) != 1 {
			t.Fatalf("fresh chunk: err=%v observed=%d recovery=%d", err, planner.observed, len(fresh.recovery.Chunks))
		}
	})
}

func TestFinishCommitsThePlanWithTheJobThenReportsCleanup(t *testing.T) {
	finisher := &stageFinisher{}
	var stages []string
	r := newRun(stageStore{}, &stagePlanner{}, finisher)
	r.progress = func(stage string, _, _ int) { stages = append(stages, stage) }
	r.analysisJSON, r.planJSON = []byte("[]"), "plan"
	if err := r.finish(clip.Project{EditPlanRevision: 4}); err != nil {
		t.Fatal(err)
	}
	if len(finisher.completed) != 1 || finisher.completed[0] != (clip.AttemptResult{JobID: "job", UserID: "alice", ProjectID: "clip", ExpectedRevision: 4, Analysis: "[]", EditPlan: "plan"}) {
		t.Fatalf("commit = %+v", finisher.completed)
	}
	if len(stages) != 1 || stages[0] != "cleanup" || r.stage != "cleanup" {
		t.Fatalf("stages = %v", stages)
	}
	finisher.err = clip.ErrBusy
	if err := r.finish(clip.Project{}); !errors.Is(err, clip.ErrBusy) {
		t.Fatalf("a refused commit must surface: %v", err)
	}
}

type diagnosedError struct{ d clip.AttemptDiagnostic }

func (e diagnosedError) Error() string                             { return "diagnosed" }
func (e diagnosedError) AttemptDiagnostic() clip.AttemptDiagnostic { return e.d }

func TestRecordFailureKeepsTheWorkerPositionOverTheParserIndexes(t *testing.T) {
	r := newRun(stageStore{}, &stagePlanner{}, &stageFinisher{})
	r.stage = "analyze"
	r.checkpoint.Diagnostic.Values = map[string]int{"source": 2, "chunk": 3, "target_ms": 15000}
	r.recordFailure(context.Background(), &clip.StageFailure{Stage: "analyze", Cause: diagnosedError{clip.AttemptDiagnostic{Check: "segments", Values: map[string]int{"source": 9, "chunk": 1, "retry": 1}}}})
	got := r.checkpoint.Diagnostic
	if got.Check != "segments" || got.Values["source"] != 2 || got.Values["chunk"] != 3 || got.Values["target_ms"] != 15000 || got.Values["retry"] != 1 || r.checkpoint.Stage != "analyze" {
		t.Fatalf("diagnostic = %+v", got)
	}
}
