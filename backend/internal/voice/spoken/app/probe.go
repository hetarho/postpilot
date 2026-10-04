package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/postpilot/backend/internal/llm"
	"github.com/postpilot/backend/internal/usage"
	"github.com/postpilot/backend/internal/voice/spoken"
	"time"
)

const ProbeUploadTimeout = 30 * time.Second

func (s *GenerationService) recordEvidence(ctx context.Context, o spoken.Operation, index int, e llm.SpeechEvidence) {
	audit, cancel := context.WithTimeout(context.WithoutCancel(ctx), JournalTimeout)
	defer cancel()
	_ = s.Operations.RecordOperationEvidence(audit, o.OwnerID, o.ID, index, e)
}
func (s *GenerationService) runProbe(ctx context.Context, o spoken.Operation) error {
	for i := range o.Texts {
		if o.AssetIDs[i] != "" {
			a, err := s.Operations.GetAsset(ctx, o.OwnerID, o.AssetIDs[i])
			if err != nil {
				return err
			}
			if a.RevokedAt != nil {
				return spoken.ErrNotFound
			}
			continue
		}
		if err := s.coordinator.ClaimProbe(ctx, o.OwnerID, o.ID, i); err != nil {
			return err
		}
		response, err := s.Models.SynthesizeSpeech(ctx, speechRequest(o, i))
		s.recordEvidence(ctx, o, i, response.Evidence)
		if err != nil {
			return err
		}
		if err := s.saveProbeAudio(ctx, o, i, response); err != nil {
			return err
		}
	}
	return s.coordinator.PublishProbe(ctx, o.OwnerID, o.ID)
}
func (s *GenerationService) saveProbeAudio(ctx context.Context, o spoken.Operation, index int, r llm.SpeechResponse) error {
	ctx, publicationCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer publicationCancel()
	audio := r.Audio
	sum := sha256.Sum256(audio.Bytes)
	if audio.Format != llm.SpeechOutputFormat || len(audio.Bytes) == 0 || len(audio.Bytes) > llm.SpeechMaxAudioBytes || audio.Samples <= 0 || audio.SampleRate != 44100 || audio.Channels != 2 || audio.Duration() > llm.SpeechMaxDuration || hex.EncodeToString(sum[:]) != audio.SHA256 {
		return spoken.ErrInvalid
	}
	id := s.newID()
	now := time.Now().UTC()
	a := spoken.Asset{ID: id, OwnerID: o.OwnerID, ObjectKey: spoken.AudioPrefix + id + ".mp3", SHA256: audio.SHA256, Format: audio.Format, Bytes: int64(len(audio.Bytes)), Samples: audio.Samples, SampleRate: audio.SampleRate, Channels: audio.Channels, ProvenanceDigest: usage.UnitDigest("spoken-probe-audio-v1", o.VoiceID, o.SpeechInputs[index], o.Profile.ConnectionScope), CreatedAt: now}
	if err := s.Operations.PrepareCleanup(ctx, spoken.Cleanup{ID: id, ObjectKey: a.ObjectKey, CreatedAt: now}); err != nil {
		return err
	}
	upload, cancel := context.WithTimeout(ctx, ProbeUploadTimeout)
	err := s.Objects.PutSpokenAudio(upload, a.ObjectKey, audio.Bytes)
	cancel()
	if err != nil {
		return spoken.ErrMediaUnavailable
	}
	// Raw-text timing alone may label exact input characters; normalized timing
	// is kept by the llm boundary and is never relabeled as exact word alignment.
	p := spoken.ProbeAudio{OperationID: o.ID, JobID: o.JobID, OwnerID: o.OwnerID, VoiceID: o.VoiceID, InputDigest: o.SpeechInputs[index], AssetID: id, Evidence: r.Evidence, Timing: r.Alignment}
	return s.Operations.SaveProbeAudio(ctx, o, index, a, p)
}
