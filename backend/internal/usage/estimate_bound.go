package usage

// HoldPromptTokenBound is the conservative prompt quantity ordinary admission reserves.
// Read-only estimates can share the bound without duplicating the ledger's policy.
func HoldPromptTokenBound(promptTokens int64) int64 { return max(holdInputTokens, promptTokens) }
