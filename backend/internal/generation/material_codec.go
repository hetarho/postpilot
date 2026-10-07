package generation

// Typed roles use explicit edge structs so retained plain payloads keep their
// historical field names and nil/empty identity.
type materialPartJSON struct {
	Kind  string             `json:"kind"`
	Text  string             `json:"text,omitempty"`
	Label string             `json:"label,omitempty"`
	Parts []materialPartJSON `json:"parts,omitempty"`
	Count int                `json:"count,omitempty"`
}
type stockApplicabilityJSON struct {
	Stage   string   `json:"stage"`
	Outputs []string `json:"outputs"`
}
type stockGuidelineJSON struct {
	Key           string                   `json:"key"`
	Text          string                   `json:"text"`
	SourceOrder   int                      `json:"source_order"`
	Applicability []stockApplicabilityJSON `json:"applicability"`
}

func encodeMaterialParts(parts []TemplateMaterialPart) *[]materialPartJSON {
	if parts == nil {
		return nil
	}
	result := make([]materialPartJSON, len(parts))
	for i, p := range parts {
		result[i] = materialPartJSON{Kind: p.Kind, Text: p.Text, Label: p.Label, Count: p.Count}
		if nested := encodeMaterialParts(p.Parts); nested != nil {
			result[i].Parts = *nested
		}
	}
	return &result
}
func decodeMaterialParts(wire *[]materialPartJSON) []TemplateMaterialPart {
	if wire == nil {
		return nil
	}
	result := make([]TemplateMaterialPart, len(*wire))
	for i, p := range *wire {
		result[i] = TemplateMaterialPart{Kind: p.Kind, Text: p.Text, Label: p.Label, Count: p.Count}
		if p.Parts != nil {
			result[i].Parts = decodeMaterialParts(&p.Parts)
		}
	}
	return result
}
func cloneMaterialParts(parts []TemplateMaterialPart) []TemplateMaterialPart {
	return decodeMaterialParts(encodeMaterialParts(parts))
}
func encodeStockGuidelines(rules []StockGuideline) *[]stockGuidelineJSON {
	if rules == nil {
		return nil
	}
	result := make([]stockGuidelineJSON, len(rules))
	for i, r := range rules {
		result[i] = stockGuidelineJSON{Key: r.Key, Text: r.Text, SourceOrder: r.SourceOrder, Applicability: make([]stockApplicabilityJSON, len(r.Applicability))}
		for j, a := range r.Applicability {
			result[i].Applicability[j] = stockApplicabilityJSON{Stage: a.Stage, Outputs: append([]string{}, a.Outputs...)}
		}
	}
	return &result
}
func decodeStockGuidelines(wire *[]stockGuidelineJSON) []StockGuideline {
	if wire == nil {
		return nil
	}
	result := make([]StockGuideline, len(*wire))
	for i, r := range *wire {
		result[i] = StockGuideline{Key: r.Key, Text: r.Text, SourceOrder: r.SourceOrder, Applicability: make([]StockRuleApplicability, len(r.Applicability))}
		for j, a := range r.Applicability {
			result[i].Applicability[j] = StockRuleApplicability{Stage: a.Stage, Outputs: append([]string{}, a.Outputs...)}
		}
	}
	return result
}
func cloneStockGuidelines(rules []StockGuideline) []StockGuideline {
	return decodeStockGuidelines(encodeStockGuidelines(rules))
}
