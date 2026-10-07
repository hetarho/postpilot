package guideline

import (
	"context"
	"slices"
)

type TestRules struct {
	Stock    []StockRule
	Defaults []string
	Owner    []Guideline
}

// TestRules resolves one post's stock and owner rules in the ordinary injection
// order while preserving owned IDs/versions for one explicitly varied slot.
func (s *Service) TestRules(ctx context.Context, user, templateID, field string, target Language, withMemories bool) (TestRules, error) {
	defaults, err := s.Defaults(ctx, user, KindPost)
	if err != nil {
		return TestRules{}, err
	}
	out := TestRules{Stock: defaultPromptStock(defaults, target, withMemories)}
	out.Defaults = stockTexts(out.Stock)
	rules, err := s.store.List(ctx, user, KindPost)
	if err != nil {
		return TestRules{}, err
	}
	for _, rule := range rules {
		if rule.Scope == ScopeGlobal || rule.Scope == ScopeTemplates && templateID != "" && slices.Contains(rule.TemplateIDs, templateID) || rule.Scope == ScopeFields && field != "" && slices.Contains(rule.Fields, field) {
			out.Owner = append(out.Owner, rule)
		}
	}
	return out, nil
}
func defaultPromptTexts(states []DefaultState, target Language, withMemories bool) []string {
	return stockTexts(defaultPromptStock(states, target, withMemories))
}
