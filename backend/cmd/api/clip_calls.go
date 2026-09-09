package main

import (
	"github.com/postpilot/backend/internal/clip/ai"
	"github.com/postpilot/backend/internal/job"
)

// T076 uses the verified probe count, never the browser's declared duration. The
// completion caps are precisely the budgets used by the clip AI service itself.
func clipPricingCalls(observe, write string, chunks int, budget ai.Budgets) []job.PlannedCall {
	return []job.PlannedCall{
		{Ref: observe, Count: chunks, CompletionTokens: budget.Observe},
		{Ref: write, Count: 1, CompletionTokens: budget.Plan},
	}
}
