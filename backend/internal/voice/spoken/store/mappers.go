package store

import (
	"context"
	"encoding/json"

	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/voice/spoken"
	"github.com/postpilot/backend/internal/voice/spoken/store/sqlc"
)

// Versioned persistence records deliberately do not serialize domain objects.
type profileRecord struct {
	ConnectionScope                                              string
	Version                                                      int
	ID                                                           string
	Revision                                                     int64
	Provider, Design, Synthesis, DesignLabel, SpeechLabel, Grade string
	Stability, Similarity, Style, Speed                          float64
	SpeakerBoost                                                 bool
	DescriptionMax, PreviewMax, SpeechMax                        int
	OutputFormat                                                 string
}

func encodeProfile(p spoken.Profile) (string, error) {
	b, err := json.Marshal(profileRecord{ConnectionScope: p.ConnectionScope, Version: 1, ID: p.ID, Revision: p.Revision, Provider: p.Design.ProviderID, Design: p.Design.ModelID, Synthesis: p.Synthesis.ModelID, DesignLabel: p.DesignLabel, SpeechLabel: p.SpeechLabel, Grade: p.Grade, Stability: p.Settings.Stability, Similarity: p.Settings.SimilarityBoost, Style: p.Settings.Style, Speed: p.Settings.Speed, SpeakerBoost: p.Settings.SpeakerBoost, DescriptionMax: p.DescriptionMax, PreviewMax: p.PreviewMax, SpeechMax: p.SpeechMax, OutputFormat: p.OutputFormat})
	return string(b), err
}
func decodeProfile(s string) (spoken.Profile, error) {
	var r profileRecord
	if err := json.Unmarshal([]byte(s), &r); err != nil {
		return spoken.Profile{}, err
	}
	if r.Version != 1 {
		return spoken.Profile{}, spoken.ErrInvalid
	}
	return spoken.Profile{ConnectionScope: r.ConnectionScope, ID: r.ID, Revision: r.Revision, Design: llm.ModelRef{ProviderID: r.Provider, ModelID: r.Design}, Synthesis: llm.ModelRef{ProviderID: r.Provider, ModelID: r.Synthesis}, DesignLabel: r.DesignLabel, SpeechLabel: r.SpeechLabel, Grade: r.Grade, Settings: llm.SpeechSettings{Stability: r.Stability, SimilarityBoost: r.Similarity, Style: r.Style, Speed: r.Speed, SpeakerBoost: r.SpeakerBoost}, DescriptionMax: r.DescriptionMax, PreviewMax: r.PreviewMax, SpeechMax: r.SpeechMax, OutputFormat: r.OutputFormat}, nil
}
func (s *Store) draft(ctx context.Context, r sqlc.SpokenVoiceDraft) (spoken.Draft, error) {
	p, err := decodeProfile(r.ProfileJson)
	if err != nil {
		return spoken.Draft{}, err
	}
	d := spoken.Draft{ID: r.ID, OwnerID: r.OwnerID, Revision: r.Revision, Name: r.Name, Description: r.Description, PreviewText: r.PreviewText, Profile: p, QualificationSessionID: r.QualificationSessionID, GenerationID: r.GenerationID, SelectedCandidateID: r.SelectedCandidateID, ConfirmedVoiceID: r.ConfirmedVoiceID, CreatedAt: parse(r.CreatedAt), UpdatedAt: parse(r.UpdatedAt), Candidates: []spoken.Candidate{}}
	if d.GenerationID == "" {
		return d, nil
	}
	rows, err := s.read.ListCandidates(ctx, sqlc.ListCandidatesParams{DraftID: d.ID, OwnerID: d.OwnerID, GenerationID: d.GenerationID})
	if err != nil {
		return spoken.Draft{}, err
	}
	for _, c := range rows {
		d.Candidates = append(d.Candidates, spoken.Candidate{ID: c.ID, OwnerID: c.OwnerID, DraftID: c.DraftID, GenerationID: c.GenerationID, Handle: llm.CandidateHandle(c.SupplierHandle), Ordinal: int(c.Ordinal), AssetID: c.AssetID, DurationMS: c.Samples * 1000 / c.SampleRate, AuditionedAt: optional(c.AuditionedAt)})
	}
	return d, nil
}
func voice(r sqlc.GetVoiceRow) (spoken.Voice, error) {
	p, err := decodeProfile(r.ProfileJson)
	if err != nil {
		return spoken.Voice{}, err
	}
	return spoken.Voice{ID: r.ID, OwnerID: r.OwnerID, Revision: r.Revision, Name: r.Name, Description: r.Description, PreviewText: r.PreviewText, Profile: p, Handle: llm.VoiceHandle(r.SupplierHandle), SampleAssetID: r.SampleAssetID, SampleDurationMS: r.Samples * 1000 / r.SampleRate, CreatedAt: parse(r.CreatedAt), RemovedAt: optional(r.RemovedAt)}, nil
}
func asset(r sqlc.SpokenAudioAsset) spoken.Asset {
	return spoken.Asset{ID: r.ID, OwnerID: r.OwnerID, ObjectKey: r.ObjectKey, SHA256: r.Sha256, Format: r.Format, Bytes: r.Bytes, Samples: r.Samples, SampleRate: int(r.SampleRate), Channels: int(r.Channels), ProvenanceDigest: r.ProvenanceDigest, CreatedAt: parse(r.CreatedAt), RevokedAt: optional(r.RevokedAt)}
}
