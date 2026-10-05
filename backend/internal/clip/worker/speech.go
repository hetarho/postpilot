package worker

import (
	"context"
	"errors"
	"io"
	"slices"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/localmedia"
)

func (e *Executor) speechLoader(ws clip.MediaWorkspace, w clip.MediaWork, task clip.MediaTask) (clip.RenderSpeechLoader, func() error) {
	drops := []func() error{}
	held := map[string]string{}
	release := func() error {
		var err error
		for _, drop := range drops {
			err = errors.Join(err, drop())
		}
		drops = nil
		clear(held)
		return err
	}
	return func(ctx context.Context, id string, consume func(string) error) error {
		if path, exists := held[id]; exists {
			return consume(path)
		}
		index := slices.IndexFunc(task.Speech, func(s clip.MediaTaskSpeech) bool { return s.AssetID == id })
		if index < 0 {
			return clip.ErrInvalidMedia
		}
		asset := task.Speech[index]
		lease := clip.SourceLease{ID: id, ActualBytes: asset.Bytes, Key: "speech/" + id, SourceMetadata: clip.SourceMetadata{Bytes: asset.Bytes, Filename: "speech.mp3", ContentType: "audio/mpeg"}}
		source, drop, err := localmedia.Fetch(ctx, ws, lease, clip.MediaInfo{}, clip.SpeechMaxAssetBytes, false, func(ctx context.Context, slot string, dst io.Writer, n int64) (int64, error) {
			return e.artifacts.Download(ctx, w.Credentials, slot, dst, n)
		})
		if err != nil {
			return err
		}
		hash, err := FileDigest(ctx, source.Path, asset.Bytes)
		if err != nil || hash != asset.AudioHash {
			return errors.Join(clip.ErrInvalidMedia, err, drop())
		}
		drops = append(drops, drop)
		held[id] = source.Path
		return consume(source.Path)
	}, release
}
