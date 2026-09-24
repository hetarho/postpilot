package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"io"
	"sync"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/llm"
)

type analysisObjects interface {
	Download(context.Context, string, io.Writer, int64) (int64, error)
}

// checkedArtifactWriter enforces the exact immutable object contract without
// accumulating either the source originals or the complete set of copies.
type checkedArtifactWriter struct {
	dst       io.Writer
	hash      hash.Hash
	remaining int64
	err       error
}

func (w *checkedArtifactWriter) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if int64(len(b)) > w.remaining {
		w.err = clip.ErrInvalidMedia
		return 0, w.err
	}
	n, err := w.dst.Write(b)
	w.hash.Write(b[:n])
	w.remaining -= int64(n)
	w.err = err
	return n, err
}
func readPreparedArtifact(ctx context.Context, objects analysisObjects, a clip.MediaArtifact, dst io.Writer) error {
	if a.State != "accepted" || a.Bytes <= 0 || a.Bytes > 8<<20 || a.ContentType != "video/mp4" || len(a.Digest) != 64 {
		return clip.ErrInvalidMedia
	}
	w := &checkedArtifactWriter{dst: dst, hash: sha256.New(), remaining: a.Bytes}
	n, err := objects.Download(ctx, a.ObjectKey, w, a.Bytes)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil || w.err != nil || n != a.Bytes || w.remaining != 0 || hex.EncodeToString(w.hash.Sum(nil)) != a.Digest {
		return clip.ErrInvalidMedia
	}
	return nil
}

type preparedReader struct {
	*io.PipeReader
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

func (r *preparedReader) Close() error {
	r.once.Do(func() { r.cancel(); r.PipeReader.Close(); <-r.done })
	return nil
}
func artifactVideo(objects analysisObjects, a clip.MediaArtifact) llm.InlineVideo {
	return llm.InlineVideo{MIME: a.ContentType, Size: a.Bytes, DurationMS: int64(a.Info.ContainerDurationMS), Sampling: llm.VideoSamplingFixed, Open: func(ctx context.Context) (io.ReadCloser, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithCancel(ctx)
		reader, writer := io.Pipe()
		out := &preparedReader{PipeReader: reader, cancel: cancel, done: make(chan struct{})}
		go func() {
			defer close(out.done)
			defer cancel()
			err := readPreparedArtifact(ctx, objects, a, writer)
			writer.CloseWithError(err)
		}()
		return out, nil
	}}
}
