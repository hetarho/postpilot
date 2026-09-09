package main

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/job"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
	"testing"
)

func TestClipMeteringFailsClosedBeforeProviderOrLedger(t *testing.T) {
	ctx := usage.WithWork(context.Background(), usage.Work{UserID: "alice", JobID: "clip", Kind: job.KindGenerateClip})
	// Nil dependencies intentionally panic if the fail-closed guard ever runs too late.
	_, err := (meteredRegistry{}).Complete(ctx, llm.ModelRef{ProviderID: "p", ModelID: "o"}, llm.Request{MaxTokens: 8192})
	if !errors.Is(err, job.ErrCreditAllowance) {
		t.Fatal(err)
	}
}
