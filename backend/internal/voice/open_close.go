package voice

import "context"

// withRecurringOpenClose recounts ⑤ of a stored analysis from the accepted material bodies it
// froze, so an analysis stored before ⑤ kept only recurring lines reads the same as a new one
// (VOICE-24). A synthetic analysis recounts over its one sample and so holds no recurring line.
// An analysis with no provable accepted bodies is returned as stored; current edited material is
// never used to reconstruct it (VOICE-74). Nothing else in Counted changes and nothing is saved.
func withRecurringOpenClose(analysis Analysis) Analysis {
	switch {
	case analysis.Origin == OriginSynthetic:
		if analysis.SyntheticSample == "" {
			return analysis
		}
		analysis.Counted.OpenClose = FingerprintOf([]Material{{Text: analysis.SyntheticSample, Kind: SampleKindPost}}).OpenClose
	case len(analysis.AcceptedMaterials) > 0:
		materials := make([]Material, 0, len(analysis.AcceptedMaterials))
		for _, accepted := range analysis.AcceptedMaterials {
			material := Material{ID: accepted.Source.SampleID, Kind: accepted.Kind, CreatedAt: accepted.CreatedAt, Text: accepted.Body}
			if prompt, ok := PromptByKey(accepted.PromptKey); ok {
				material.Part = prompt.Part
			}
			materials = append(materials, material)
		}
		analysis.Counted.OpenClose = FingerprintOf(materials).OpenClose
	}
	return analysis
}

// recurringAnalyses applies withRecurringOpenClose at the store seam, so every reader of a
// voice's current analysis sees the same ⑤; publishing and restoring pass through unchanged.
type recurringAnalyses struct{ AnalysisStore }

func (r recurringAnalyses) CurrentAnalysis(ctx context.Context, userID, voiceID string) (*Analysis, error) {
	found, err := r.AnalysisStore.CurrentAnalysis(ctx, userID, voiceID)
	if err != nil || found == nil {
		return found, err
	}
	recounted := withRecurringOpenClose(*found)
	return &recounted, nil
}
