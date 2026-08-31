package experiment

import "github.com/postpilot/backend/internal/llm"

// ResolveCost maps this context's report/model pair onto the shared resolver. The
// precedence itself lives in llm.ResolveCost so the leaderboard and the account usage
// ledger can never price the same call differently.
func ResolveCost(report UsageReport, model Model) Usage {
	resolved := llm.ResolveCost(llm.CostInput{
		PromptTokens:        report.PromptTokens,
		CompletionTokens:    report.CompletionTokens,
		ReportedMicrousd:    report.CostMicrousd,
		Reported:            report.CostReported,
		InputUSDPerMillion:  model.InputUSDPerMillion,
		OutputUSDPerMillion: model.OutputUSDPerMillion,
	})
	return Usage{
		PromptTokens:     report.PromptTokens,
		CompletionTokens: report.CompletionTokens,
		CostMicrousd:     resolved.Microusd,
		CostSource:       CostSource(resolved.Source),
	}
}
