package voice

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

func EncodeAcceptedAnalysisSnapshot(sources []AcceptedSource, materials []AcceptedMaterial) ([]byte, error) {
	job := AnalysisJob{AcceptedSources: sources, AcceptedMaterials: materials}
	if _, err := analysisJobSamples(job); err != nil {
		return nil, err
	}
	ids := make([]string, len(sources))
	for i, source := range sources {
		ids[i] = source.SampleID
	}
	return json.Marshal(snapshotJSON{MaterialIDs: ids, AcceptedSources: sources, AcceptedMaterials: materials})
}

// A legacy payload remains decodable for reconciliation. Without captured prose
// it cannot silently reconstruct a historical request from today's edited row.
func DecodeAcceptedAnalysisSnapshot(raw []byte) (AnalysisJob, error) {
	var snapshot snapshotJSON
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return AnalysisJob{}, fmt.Errorf("decode accepted analysis snapshot: %w", err)
	}
	if len(snapshot.MaterialIDs) == 0 {
		return AnalysisJob{}, ErrAcceptedSourceWithdrawn
	}
	job := AnalysisJob{MaterialIDs: snapshot.MaterialIDs, AcceptedSources: snapshot.AcceptedSources, AcceptedMaterials: snapshot.AcceptedMaterials}
	if len(job.AcceptedSources) != 0 || len(job.AcceptedMaterials) != 0 {
		if _, err := analysisJobSamples(job); err != nil {
			return AnalysisJob{}, err
		}
	}
	return job, nil
}

func analysisJobSamples(job AnalysisJob) ([]Sample, error) {
	if len(job.AcceptedSources) == 0 || len(job.AcceptedSources) != len(job.AcceptedMaterials) || (len(job.MaterialIDs) != 0 && len(job.MaterialIDs) != len(job.AcceptedSources)) {
		return nil, ErrAcceptedSourceWithdrawn
	}
	seen := make(map[string]bool, len(job.AcceptedSources))
	samples := make([]Sample, 0, len(job.AcceptedSources))
	for i, source := range job.AcceptedSources {
		material := job.AcceptedMaterials[i]
		if source.SampleID == "" || source.ContentRevision <= 0 || seen[source.SampleID] || source != material.Source || (len(job.MaterialIDs) != 0 && job.MaterialIDs[i] != source.SampleID) || !utf8.ValidString(material.Body) || strings.TrimSpace(material.Body) == "" || (material.Kind != SampleKindPost && material.Kind != SampleKindAnswer) {
			return nil, ErrAcceptedSourceWithdrawn
		}
		seen[source.SampleID] = true
		samples = append(samples, Sample{ID: source.SampleID, ContentRevision: source.ContentRevision,
			Kind: material.Kind, PromptKey: material.PromptKey, Label: material.Label, Body: material.Body,
			Chars: utf8.RuneCountInString(material.Body), PhotoKey: material.PhotoKey,
			PhotoWidth: material.PhotoWidth, PhotoHeight: material.PhotoHeight, CreatedAt: material.CreatedAt,
		})
	}
	return samples, nil
}
