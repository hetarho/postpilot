package usage

import (
	"context"

	"github.com/postpilot/backend/internal/llm"
)

// Work is the job a provider call is being made for. The worker stamps it on the context
// once per job so the ledger can attribute a call that happens several packages away,
// inside a loop, or inside an A/B candidate the caller never named.
//
// Attributing at the llm seam rather than at each call site is what makes "every server
// -side LLM call is on the ledger" a structural fact instead of a rule twelve call sites
// have to keep remembering.
type Work struct {
	UserID string
	Kind   string
	JobID  string
	// ObserveModel and WriteModel are the refs this job runs per stage, when it has them.
	// Photo observation is the one call inside a job that runs a different model for a
	// different purpose, so it is the one distinction the ledger's stage column records;
	// every other call is labelled with the job kind.
	ObserveModel string
	WriteModel   string
}

// StageFor labels one call within this work.
//
// A job whose two stages run the SAME model gets the job kind for both: the ref cannot tell
// them apart at this seam, and guessing "observe" would label the write call wrongly.
func (w Work) StageFor(ref llm.ModelRef) string {
	if w.ObserveModel != "" && w.ObserveModel != w.WriteModel && ref.String() == w.ObserveModel {
		return "observe"
	}
	return w.Kind
}

type workKeyType struct{}

var workKey workKeyType

// WithWork returns a context carrying the job a provider call will be attributed to.
func WithWork(ctx context.Context, work Work) context.Context {
	return context.WithValue(ctx, workKey, work)
}

// WorkFromContext returns the job in scope, if any. A call made outside a job — none
// exist today — is simply not attributable, and is dropped rather than guessed at.
func WorkFromContext(ctx context.Context) (Work, bool) {
	work, ok := ctx.Value(workKey).(Work)
	return work, ok && work.UserID != "" && work.JobID != ""
}
