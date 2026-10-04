package rpc

import (
	v1 "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/voice/spoken"
	"time"
)

func timestamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func profile(p spoken.Profile) *v1.SpokenProfileSnapshot {
	return &v1.SpokenProfileSnapshot{Id: p.ID, Revision: p.Revision, ProviderId: p.Design.ProviderID, DesignModelId: p.Design.ModelID, SpeechModelId: p.Synthesis.ModelID, DesignLabel: p.DesignLabel, SpeechLabel: p.SpeechLabel, Grade: p.Grade, DescriptionMax: int32(p.DescriptionMax), PreviewMax: int32(p.PreviewMax), SpeechMax: int32(p.SpeechMax), OutputFormat: p.OutputFormat}
}
func draft(d spoken.Draft) *v1.SpokenDraft {
	out := &v1.SpokenDraft{Id: d.ID, Revision: d.Revision, Name: d.Name, Description: d.Description, PreviewText: d.PreviewText, Profile: profile(d.Profile), Phase: d.Phase(), SelectedCandidateId: d.SelectedCandidateID, ConfirmedVoiceId: d.ConfirmedVoiceID, CreatedAt: timestamp(&d.CreatedAt), UpdatedAt: timestamp(&d.UpdatedAt), QualificationSessionId: d.QualificationSessionID}
	for _, c := range d.Candidates {
		out.Candidates = append(out.Candidates, &v1.SpokenCandidate{Id: c.ID, AssetId: c.AssetID, DurationMs: c.DurationMS, AuditionedAt: timestamp(c.AuditionedAt)})
	}
	return out
}
func voice(v spoken.Voice) *v1.SpokenVoice {
	return &v1.SpokenVoice{Id: v.ID, Revision: v.Revision, Name: v.Name, Description: v.Description, PreviewText: v.PreviewText, Profile: profile(v.Profile), SampleAssetId: v.SampleAssetID, SampleDurationMs: v.SampleDurationMS, CreatedAt: timestamp(&v.CreatedAt), RemovedAt: timestamp(v.RemovedAt)}
}
