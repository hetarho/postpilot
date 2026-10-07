package voice

import (
	"context"
	"fmt"
)

func samplePresence(samples []Sample) map[string]bool {
	present := make(map[string]bool, len(samples))
	for _, sample := range samples {
		present[sample.ID] = true
	}
	return present
}

func sourceNotice(analysis Analysis, samples []Sample) Notice {
	if !analysis.SourceVersionsKnown {
		// Old ids can still prove an addition or withdrawal, but never prove that
		// current edited prose matches the accepted historical version.
		return noticeOf(analysis.MaterialIDs, samplePresence(samples))
	}
	present := make(map[string]int64, len(samples))
	for _, sample := range samples {
		present[sample.ID] = sample.ContentRevision
	}
	accepted := make(map[string]bool, len(analysis.AcceptedSources))
	for _, source := range analysis.AcceptedSources {
		if present[source.SampleID] != source.ContentRevision {
			return Notice{Kind: NoticeChanged}
		}
		accepted[source.SampleID] = true
	}
	added := 0
	for id := range present {
		if !accepted[id] {
			added++
		}
	}
	if added > 0 {
		return Notice{Kind: NoticeAdded, Count: added}
	}
	return Notice{}
}

func acceptMaterials(samples []Sample) ([]AcceptedSource, []AcceptedMaterial) {
	sources := make([]AcceptedSource, 0, len(samples))
	materials := make([]AcceptedMaterial, 0, len(samples))
	for _, sample := range samples {
		source := AcceptedSource{SampleID: sample.ID, ContentRevision: sample.ContentRevision}
		sources = append(sources, source)
		materials = append(materials, AcceptedMaterial{
			Source: source, Body: sample.Body, PhotoKey: sample.PhotoKey,
			PhotoWidth: sample.PhotoWidth, PhotoHeight: sample.PhotoHeight,
			Kind: sample.Kind, PromptKey: sample.PromptKey, Label: sample.Label, CreatedAt: sample.CreatedAt,
		})
	}
	return sources, materials
}

// acceptedSamples uses only stored accepted prose. Current rows authorize
// presence/ownership; their edited body is never substituted for the snapshot.
func acceptedSamples(analysis Analysis, current []Sample) []Sample {
	if !analysis.SourceVersionsKnown {
		return nil
	}
	present := samplePresence(current)
	result := make([]Sample, 0, len(analysis.AcceptedMaterials))
	for _, material := range analysis.AcceptedMaterials {
		if present[material.Source.SampleID] {
			result = append(result, Sample{
				ID: material.Source.SampleID, ContentRevision: material.Source.ContentRevision,
				Kind: material.Kind, PromptKey: material.PromptKey, Label: material.Label,
				Body: material.Body, Chars: len([]rune(material.Body)), PhotoKey: material.PhotoKey,
				PhotoWidth: material.PhotoWidth, PhotoHeight: material.PhotoHeight, CreatedAt: material.CreatedAt,
			})
		}
	}
	return result
}

// ValidateAcceptedSources is the last presence fence for a previously frozen
// projection. Later editing/reanalysis cannot replace its accepted input.
func (s *Service) ValidateAcceptedSources(ctx context.Context, userID, voiceID string, sources []AcceptedSource) error {
	voice, err := s.activeVoice(ctx, userID, voiceID)
	if err != nil {
		return err
	}
	if !voice.Made {
		return ErrVoiceNotMade
	}
	return s.validateMaterialPresence(ctx, userID, voiceID, sources)
}

func (s *Service) validateMaterialPresence(ctx context.Context, userID, voiceID string, sources []AcceptedSource) error {
	if len(sources) == 0 {
		return nil
	}
	samples, err := s.samples.ListSampleBodies(ctx, userID, voiceID)
	if err != nil {
		return fmt.Errorf("validate accepted materials: %w", err)
	}
	present := samplePresence(samples)
	seen := make(map[string]bool, len(sources))
	for _, source := range sources {
		if source.SampleID == "" || source.ContentRevision <= 0 || seen[source.SampleID] || !present[source.SampleID] {
			return ErrAcceptedSourceWithdrawn
		}
		seen[source.SampleID] = true
	}
	return nil
}
