package spoken

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/postpilot/backend/internal/llm"
)

func (s *Service) SampleAccess(ctx context.Context, owner, assetID string) (Playback, error) {
	a, err := s.store.GetAsset(ctx, owner, assetID)
	if err != nil {
		return Playback{}, err
	}
	if a.RevokedAt != nil {
		return Playback{}, ErrNotFound
	}
	p := Playback{ID: s.newID(), OwnerID: owner, AssetID: a.ID, ExpiresAt: s.now().Add(PlaybackTTL)}
	if err := s.store.SavePlayback(ctx, p); err != nil {
		return Playback{}, err
	}
	return p, nil
}

// ReadPlayback rechecks ownership and revocation on every request. A copied
// ticket is insufficient without the same authenticated account.
func (s *Service) ReadPlayback(ctx context.Context, owner, ticketID string) ([]byte, error) {
	p, a, err := s.store.GetPlayback(ctx, owner, ticketID, s.now())
	if err != nil {
		return nil, err
	}
	data, err := s.objects.ReadSpokenAudio(ctx, a.ObjectKey, a.Bytes)
	if err != nil {
		return nil, ErrMediaUnavailable
	}
	h := sha256.Sum256(data)
	if int64(len(data)) != a.Bytes || hex.EncodeToString(h[:]) != a.SHA256 {
		return nil, ErrMediaUnavailable
	}
	if err := s.store.MarkPlaybackServed(ctx, owner, p.ID, s.now()); err != nil {
		return nil, err
	}
	return data, nil
}

// SaveCandidates is a worker-owned capability, absent from customer RPCs.
// Upload intents precede I/O so a crash, upload failure or stale result can be
// collected later. No writer transaction spans object-storage I/O.
func (s *Service) SaveCandidates(ctx context.Context, owner, draftID string, revision int64, generationID string, candidates []llm.VoiceCandidate) (Draft, error) {
	ctx, cancelPublication := context.WithTimeout(ctx, AudioPublicationTimeout)
	defer cancelPublication()
	d, err := s.store.GetDraft(ctx, owner, draftID)
	if err != nil {
		return Draft{}, err
	}
	if d.GenerationID == generationID && len(d.Candidates) == len(candidates) && len(candidates) == llm.SpeechMaxCandidates {
		for i, c := range candidates {
			a, err := s.store.GetAsset(ctx, owner, d.Candidates[i].AssetID)
			if err != nil {
				return Draft{}, err
			}
			if d.Candidates[i].Handle != c.Handle || a.SHA256 != c.Audio.SHA256 {
				return Draft{}, ErrConflict
			}
		}
		return d, nil
	}
	if d.Revision != revision {
		return Draft{}, ErrConflict
	}
	if d.ConfirmedVoiceID != "" {
		return Draft{}, ErrImmutable
	}
	if generationID == "" || len(candidates) != llm.SpeechMaxCandidates {
		return Draft{}, ErrInvalid
	}
	assets := make([]Asset, 0, len(candidates))
	saved := make([]Candidate, 0, len(candidates))
	seen := map[llm.CandidateHandle]bool{}
	for _, candidate := range candidates {
		audio := candidate.Audio
		if candidate.Handle == "" || seen[candidate.Handle] || len(audio.Bytes) == 0 || len(audio.Bytes) > llm.SpeechMaxAudioBytes || audio.Format != llm.SpeechOutputFormat || audio.SampleRate != 44100 || audio.Channels != 2 || audio.Samples <= 0 || audio.Duration() > llm.SpeechMaxDuration {
			return Draft{}, ErrInvalid
		}
		seen[candidate.Handle] = true
		h := sha256.Sum256(audio.Bytes)
		if hex.EncodeToString(h[:]) != audio.SHA256 {
			return Draft{}, ErrInvalid
		}
		id := s.newID()
		now := s.now()
		a := Asset{ID: id, OwnerID: owner, ObjectKey: AudioPrefix + id + ".mp3", SHA256: audio.SHA256, Format: audio.Format, Bytes: int64(len(audio.Bytes)), Samples: audio.Samples, SampleRate: audio.SampleRate, Channels: audio.Channels,
			ProvenanceDigest: digest("spoken-audition-v1", d.ID, generationID, d.Description, d.PreviewText, d.Profile.Design.String(), d.Profile.Synthesis.String(), d.Profile.Settings.Digest(), string(candidate.Handle)), CreatedAt: now}
		if err := s.store.PrepareCleanup(ctx, Cleanup{ID: id, ObjectKey: a.ObjectKey, CreatedAt: now}); err != nil {
			return Draft{}, err
		}
		uploadCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := s.objects.PutSpokenAudio(uploadCtx, a.ObjectKey, audio.Bytes)
		cancel()
		if err != nil {
			return Draft{}, ErrMediaUnavailable
		}
		assets = append(assets, a)
		saved = append(saved, Candidate{ID: s.newID(), OwnerID: owner, DraftID: draftID, GenerationID: generationID, Handle: candidate.Handle, AssetID: a.ID, DurationMS: a.DurationMS(), Ordinal: len(saved)})
	}
	identity, err := request(owner, "save_candidates", generationID, draftID)
	if err != nil {
		return Draft{}, err
	}
	_, err = s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{draftID, revision + 1}, tx.AttachCandidates(ctx, owner, draftID, revision, generationID, saved, assets)
	})
	if err != nil {
		return Draft{}, err
	}
	return s.store.GetDraft(ctx, owner, draftID)
}

// SaveConfirmation preserves the exact auditioned candidate and its stored
// sample. Only the generation worker may pass a confirmed supplier handle.
func (s *Service) SaveConfirmation(ctx context.Context, owner, draftID string, revision int64, operationID, candidateID string, handle llm.VoiceHandle) (Voice, error) {
	d, err := s.store.GetDraft(ctx, owner, draftID)
	if err != nil {
		return Voice{}, err
	}
	if !hasCandidate(d, candidateID) {
		return Voice{}, ErrNotFound
	}
	if d.ConfirmedVoiceID != "" {
		v, err := s.store.GetVoice(ctx, owner, d.ConfirmedVoiceID)
		if err != nil {
			return Voice{}, err
		}
		if v.Handle != handle {
			return Voice{}, ErrImmutable
		}
		return v, nil
	}
	if d.Revision != revision {
		return Voice{}, ErrConflict
	}
	if handle == "" || d.SelectedCandidateID != candidateID {
		return Voice{}, ErrAuditionRequired
	}
	var chosen *Candidate
	for i := range d.Candidates {
		c := &d.Candidates[i]
		if c.ID == candidateID && c.GenerationID == d.GenerationID {
			chosen = c
		}
	}
	if chosen == nil {
		return Voice{}, ErrNotFound
	}
	if chosen.AuditionedAt == nil {
		return Voice{}, ErrAuditionRequired
	}
	v := Voice{ID: s.newID(), OwnerID: owner, Revision: 1, Name: d.Name, Description: d.Description, PreviewText: d.PreviewText, Profile: d.Profile, Handle: handle, SampleAssetID: chosen.AssetID, SampleDurationMS: chosen.DurationMS, CreatedAt: s.now()}
	identity, err := request(owner, "confirm_voice", operationID, draftID, candidateID, string(handle))
	if err != nil {
		return Voice{}, err
	}
	result, err := s.store.Mutate(ctx, identity, func(tx Storage) (MutationResult, error) {
		return MutationResult{v.ID, 1}, tx.ConfirmVoice(ctx, owner, draftID, revision, candidateID, v)
	})
	if err != nil {
		return Voice{}, err
	}
	return s.store.GetVoice(ctx, owner, result.ID)
}

func (s *Service) AcquireVoice(ctx context.Context, owner, id string, revision int64, referenceID string) (Voice, error) {
	if referenceID == "" {
		return Voice{}, ErrInvalid
	}
	return s.store.AcquireVoice(ctx, owner, id, revision, referenceID)
}

func (s *Service) Cleanup(ctx context.Context, before time.Time) error {
	intents, err := s.store.PendingCleanup(ctx, before)
	if err != nil {
		return err
	}
	for _, intent := range intents {
		retained, err := s.store.AssetRetained(ctx, intent.ID)
		if err != nil {
			return err
		}
		if retained {
			if err := s.store.DiscardRetainedCleanup(ctx, intent.ID); err != nil {
				return err
			}
			continue
		}
		if err := s.objects.DeleteSpokenAudio(ctx, intent.ObjectKey); err != nil {
			return err
		}
		if err := s.store.CompleteCleanup(ctx, intent.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) DeleteOwner(ctx context.Context, owner string) error {
	return s.store.DeleteOwner(ctx, owner)
}
