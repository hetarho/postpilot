package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
	"time"
)

type speechAssetReader interface {
	GetSpeechAsset(context.Context, string, string, string) (clip.SpeechAsset, error)
}
type speechAudioReader interface {
	ReadClipSpeechAudio(context.Context, string, int64) ([]byte, error)
}
type speechPlayback struct {
	owner, project, asset, hash string
	expires                     time.Time
}
type SpeechAccess struct {
	ID, AudioHash string
	ExpiresAt     time.Time
	Bytes         int64
}

func (s *SpeechService) SpeechAccess(ctx context.Context, owner, project, asset string) (SpeechAccess, error) {
	if _, e := s.Store.GetProject(ctx, owner, project); e != nil {
		return SpeechAccess{}, e
	}
	reader, ok := s.Store.(speechAssetReader)
	if !ok {
		return SpeechAccess{}, clip.ErrCompositionUnavailable
	}
	a, e := reader.GetSpeechAsset(ctx, owner, project, asset)
	if e != nil {
		return SpeechAccess{}, e
	}
	if a.Bytes <= 0 || a.Bytes > llm.SpeechMaxAudioBytes {
		return SpeechAccess{}, clip.ErrInvalid
	}
	now := time.Now()
	id := speechID()
	expires := now.Add(time.Minute)
	s.playbackMu.Lock()
	defer s.playbackMu.Unlock()
	for id, t := range s.playback {
		if !t.expires.After(now) {
			delete(s.playback, id)
		}
	}
	if len(s.playback) >= 512 {
		return SpeechAccess{}, clip.ErrBusy
	}
	s.playback[id] = speechPlayback{owner, project, asset, a.Speech.AudioHash, expires}
	return SpeechAccess{id, a.Speech.AudioHash, expires, a.Bytes}, nil
}
func (s *SpeechService) ReadSpeechPlayback(ctx context.Context, owner, id string) ([]byte, error) {
	s.playbackMu.Lock()
	t, ok := s.playback[id]
	s.playbackMu.Unlock()
	if !ok || t.owner != owner || !t.expires.After(time.Now()) {
		return nil, clip.ErrNotFound
	}
	if _, e := s.Store.GetProject(ctx, owner, t.project); e != nil {
		return nil, e
	}
	assets, ok := s.Store.(speechAssetReader)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	objects, ok := s.Objects.(speechAudioReader)
	if !ok {
		return nil, clip.ErrCompositionUnavailable
	}
	a, e := assets.GetSpeechAsset(ctx, owner, t.project, t.asset)
	if e != nil {
		return nil, e
	}
	if a.Speech.AudioHash != t.hash || a.Bytes <= 0 || a.Bytes > llm.SpeechMaxAudioBytes {
		return nil, clip.ErrInvalid
	}
	data, e := objects.ReadClipSpeechAudio(ctx, a.ObjectKey, a.Bytes)
	if e != nil {
		return nil, e
	}
	h := sha256.Sum256(data)
	if int64(len(data)) != a.Bytes || hex.EncodeToString(h[:]) != t.hash {
		return nil, clip.ErrInvalid
	}
	// Deletion during object I/O revokes this request too.
	if _, e = assets.GetSpeechAsset(ctx, owner, t.project, t.asset); e != nil {
		return nil, e
	}
	if _, e = s.Store.GetProject(ctx, owner, t.project); e != nil {
		return nil, e
	}
	return data, nil
}
