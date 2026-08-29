package experiment

import "math/big"

func ResolveCost(report UsageReport, model Model) Usage {
	usage := Usage{PromptTokens: report.PromptTokens, CompletionTokens: report.CompletionTokens}
	if report.CostReported {
		usage.CostMicrousd = report.CostMicrousd
		usage.CostSource = CostReported
		return usage
	}
	if report.PromptTokens == 0 && report.CompletionTokens == 0 {
		usage.CostSource = CostUnavailable
		return usage
	}
	cost, ok := estimateMicrousd(report.PromptTokens, report.CompletionTokens, model.InputUSDPerMillion, model.OutputUSDPerMillion)
	if !ok {
		usage.CostSource = CostUnavailable
		return usage
	}
	usage.CostMicrousd = cost
	usage.CostSource = CostEstimated
	return usage
}

func estimateMicrousd(promptTokens, completionTokens int64, inputRate, outputRate string) (int64, bool) {
	if promptTokens < 0 || completionTokens < 0 || inputRate == "" || outputRate == "" {
		return 0, false
	}
	input, ok := new(big.Rat).SetString(inputRate)
	if !ok || input.Sign() < 0 {
		return 0, false
	}
	output, ok := new(big.Rat).SetString(outputRate)
	if !ok || output.Sign() < 0 {
		return 0, false
	}
	total := new(big.Rat).Mul(input, big.NewRat(promptTokens, 1))
	total.Add(total, new(big.Rat).Mul(output, big.NewRat(completionTokens, 1)))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(total.Num(), total.Denom(), remainder)
	if new(big.Int).Lsh(remainder, 1).Cmp(total.Denom()) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	if !quotient.IsInt64() {
		return 0, false
	}
	return quotient.Int64(), true
}
