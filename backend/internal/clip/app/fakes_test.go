package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	_ "modernc.org/sqlite"
)

// memoryWriter is a schema-less SQLite handle: WriteTx needs a real
// transaction to open and commit, the fakes below need no tables.
func memoryWriter(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	return d
}

type fakeJobs struct {
	jobs     map[string]job.Job
	active   *job.Job
	finished []string
	getErr   error
}

func (f *fakeJobs) GetByID(_ context.Context, id string) (job.Job, error) {
	if f.getErr != nil {
		return job.Job{}, f.getErr
	}
	j, ok := f.jobs[id]
	if !ok {
		return job.Job{}, job.ErrNotFound
	}
	return j, nil
}
func (f *fakeJobs) Finish(_ context.Context, id, status string, _ *job.Failure, _ time.Time) error {
	j := f.jobs[id]
	j.Status = status
	f.jobs[id] = j
	f.finished = append(f.finished, id)
	return nil
}
func (f *fakeJobs) ActiveFor(context.Context, job.Subject, job.Filter) (*job.Job, error) {
	return f.active, nil
}

type fakeClips struct {
	project    clip.Project
	projectErr error
	staged     map[string]clip.AttemptResult
	applied    []clip.AttemptResult
	discarded  []string
	deleted    []string
	finalized  []clip.FinalizationRequest
	recordErr  error
	stageErr   error
	txReadErr  error
}

func newFakeClips(p clip.Project) *fakeClips {
	return &fakeClips{project: p, staged: map[string]clip.AttemptResult{}}
}
func (f *fakeClips) GetProject(context.Context, string, string) (clip.Project, error) {
	if f.projectErr != nil {
		return clip.Project{}, f.projectErr
	}
	return f.project, nil
}
func (f *fakeClips) StageAttemptResult(_ context.Context, c clip.AttemptResult) error {
	if f.stageErr != nil {
		return f.stageErr
	}
	f.staged[c.JobID] = c
	return nil
}
func (f *fakeClips) GetAttemptResult(_ context.Context, id string) (clip.AttemptResult, error) {
	if f.txReadErr != nil {
		return clip.AttemptResult{}, f.txReadErr
	}
	c, ok := f.staged[id]
	if !ok {
		return clip.AttemptResult{}, errors.New("no staged result")
	}
	return c, nil
}
func (f *fakeClips) ApplyAttemptResult(_ context.Context, c clip.AttemptResult) error {
	f.applied = append(f.applied, c)
	if c.Result.Key != "" {
		r := c.Result
		f.project.Result = &r
	}
	if c.EditPlan != "" {
		f.project.EditPlan = c.EditPlan
		f.project.EditPlanRevision = c.ExpectedRevision + 1
	}
	return nil
}
func (f *fakeClips) DeleteAttemptResult(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	delete(f.staged, id)
	return nil
}
func (f *fakeClips) DiscardAttemptResult(_ context.Context, id string) error {
	f.discarded = append(f.discarded, id)
	delete(f.staged, id)
	return nil
}
func (f *fakeClips) PendingAttemptResults(context.Context) ([]clip.AttemptResult, error) {
	out := []clip.AttemptResult{}
	for _, c := range f.staged {
		out = append(out, c)
	}
	return out, nil
}
func (f *fakeClips) RecordFinalization(_ context.Context, req clip.FinalizationRequest, _ time.Time) (clip.Project, error) {
	if f.recordErr != nil {
		return clip.Project{}, f.recordErr
	}
	f.finalized = append(f.finalized, req)
	p := f.project
	p.Finalized = &clip.Finalization{PlanRevision: req.ExpectedRevision, ResultID: req.ExpectedResultID}
	return p, nil
}

type fakeAdmission struct {
	held []Hold
	err  error
}

func (a *fakeAdmission) Hold(_ context.Context, h Hold) error {
	a.held = append(a.held, h)
	return a.err
}

// bindFakes ignores the transaction: the fakes are the tx-scoped ports.
func bindFakes(jobs *fakeJobs, clips *fakeClips, admission Admission) Binder {
	return func(*sql.Tx) Ports { return Ports{Jobs: jobs, Clips: clips, Admission: admission} }
}

func observePolicy() llm.CallPolicy {
	return llm.CallPolicy{Ref: llm.ModelRef{ProviderID: "p", ModelID: "o"}, Stage: "observe", CompletionTokens: 8192, InputUSDPerMillion: "0.15", OutputUSDPerMillion: "0", Pricing: llm.CallPricing{Version: llm.CallPricingVersion, Fingerprint: strings.Repeat("a", 64), Delivery: llm.ExecutionInlineStatic, Endpoint: "leaf", RequiredParameters: "max_tokens", PromptUSDPerMillion: "0.15", CompletionUSDPerMillion: "0", RequestUSD: "0", ImageUSD: "0", AudioUSDPerToken: "0"}}
}
func writePolicy() llm.CallPolicy {
	w := observePolicy()
	w.Ref.ModelID = "w"
	w.Stage = "write"
	w.CompletionTokens = 32768
	w.Pricing.Delivery = llm.ExecutionTextOnly
	return w
}
func approvedPricing(chunks int) clip.GenerationPricing {
	p := clip.GenerationPricing{Version: clip.PricingPolicyVersion, Observe: observePolicy(), Plan: writePolicy(), Narration: writePolicy(), ObservationCalls: chunks}
	credits, err := QuoteCredits(p, chunks, DefaultQuoteRetries)
	if err != nil {
		panic(err)
	}
	p.MaxCredits = credits
	return p
}
