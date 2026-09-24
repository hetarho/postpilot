// Package localmedia shares bounded original downloads and render source ownership
// between embedded execution and the standalone worker. It has no storage client.
package localmedia

import (
	"context"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"path/filepath"
)

type Download func(context.Context, string, io.Writer, int64) (int64, error)

func Fetch(ctx context.Context, ws clip.MediaWorkspace, v clip.SourceLease, info clip.MediaInfo, maxBytes int64, verify bool, download Download) (source clip.MediaSource, release func() error, err error) {
	if v.Bytes <= 0 || v.Bytes > maxBytes || v.ActualBytes != v.Bytes {
		return clip.MediaSource{}, nil, clip.ErrInvalidMedia
	}
	if ws.CheckCapacity == nil {
		return clip.MediaSource{}, nil, clip.ErrWorkspaceLimit
	}
	if err := ws.CheckCapacity(v.Bytes); err != nil {
		return clip.MediaSource{}, nil, err
	}
	f, err := os.CreateTemp(ws.Path, "source-*"+filepath.Ext(v.Filename))
	if err != nil {
		return clip.MediaSource{}, nil, err
	}
	name := f.Name()
	remove := func() error {
		if remove := os.Remove(name); remove != nil && !os.IsNotExist(remove) {
			return remove
		}
		return nil
	}
	// A failed download leaves nothing behind; the caller gets no release to call.
	defer func() {
		if err != nil {
			_ = f.Close()
			err = errors.Join(err, remove())
		}
	}()
	w := &sourceWriter{ctx: ctx, file: f, remaining: v.Bytes, capacity: ws.CheckCapacity, fingerprint: newFingerprint(v.SourceMetadata)}
	n, err := download(ctx, v.Key, w, v.Bytes)
	if err != nil {
		if ctx.Err() != nil {
			return clip.MediaSource{}, nil, ctx.Err()
		}
		if w.err != nil {
			return clip.MediaSource{}, nil, w.err
		}
		for _, typed := range []error{clip.ErrInvalidMedia, clip.ErrMediaUnavailable, clip.ErrMediaLeaseLost, clip.ErrMediaCancelled, clip.ErrMediaUnauthenticated} {
			if errors.Is(err, typed) {
				return clip.MediaSource{}, nil, typed
			}
		}
		return clip.MediaSource{}, nil, errors.New("clip source download failed")
	}
	if w.err != nil {
		return clip.MediaSource{}, nil, w.err
	}
	if n != v.Bytes || w.remaining != 0 {
		return clip.MediaSource{}, nil, clip.ErrInvalidMedia
	}
	if verify && w.fingerprint.Sum() != v.Fingerprint {
		return clip.MediaSource{}, nil, clip.ErrInvalidMedia
	}
	if err = f.Close(); err != nil {
		return clip.MediaSource{}, nil, err
	}
	return clip.MediaSource{Path: name, SourceID: v.ID, Fingerprint: v.Fingerprint, Info: info}, remove, nil
}

type sourceWriter struct {
	ctx         context.Context
	file        *os.File
	remaining   int64
	capacity    func(int64) error
	err         error
	fingerprint *Fingerprint
}

func (w *sourceWriter) Write(p []byte) (int, error) {
	if w.err == nil {
		w.err = w.ctx.Err()
	}
	if w.err == nil && int64(len(p)) > w.remaining {
		w.err = clip.ErrInvalidMedia
	}
	if w.err == nil {
		w.err = w.capacity(int64(len(p)))
	}
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.file.Write(p)
	w.fingerprint.Write(p[:n])
	w.remaining -= int64(n)
	w.err = err
	return n, err
}

// Loader holds at most one original, reusing consecutive requests for that source.
func Loader(fetch func(context.Context, string) (clip.MediaSource, func() error, error)) (clip.RenderSourceLoader, func() error) {
	var id string
	var held clip.MediaSource
	var drop func() error
	release := func() error {
		if drop == nil {
			return nil
		}
		d := drop
		drop, id = nil, ""
		return d()
	}
	return func(ctx context.Context, next string, consume func(clip.MediaSource) error) error {
		if drop != nil && id == next {
			return consume(held)
		}
		if err := release(); err != nil {
			return err
		}
		source, remove, err := fetch(ctx, next)
		if err != nil {
			return err
		}
		held, id, drop = source, next, remove
		return consume(source)
	}, release
}
