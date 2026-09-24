package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

type artifactDownload func(context.Context, string, io.Writer, int64) (int64, error)

func (f artifactDownload) Download(ctx context.Context, key string, w io.Writer, n int64) (int64, error) {
	return f(ctx, key, w, n)
}
func artifactRecord(data []byte) clip.MediaArtifact {
	sum := sha256.Sum256(data)
	return clip.MediaArtifact{State: "accepted", ObjectKey: "clip-media/u/p/a/key.mp4", MediaOutput: clip.MediaOutput{Bytes: int64(len(data)), ContentType: "video/mp4", Digest: hex.EncodeToString(sum[:]), Info: clip.MediaInfo{ContainerDurationMS: 1000}}}
}
func TestPreparedArtifactsValidateBeforeReadingAndReopenForCorrection(t *testing.T) {
	data := []byte("bounded proxy")
	a := artifactRecord(data)
	calls := 0
	objects := artifactDownload(func(_ context.Context, key string, w io.Writer, limit int64) (int64, error) {
		calls++
		if key != a.ObjectKey || limit != a.Bytes {
			t.Fatal("wrong artifact grant")
		}
		return io.Copy(w, bytes.NewReader(data))
	})
	if err := readPreparedArtifact(t.Context(), objects, a, io.Discard); err != nil {
		t.Fatal(err)
	}
	v := artifactVideo(objects, a)
	for range 4 {
		reader, err := v.Open(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(reader)
		reader.Close()
		if err != nil || !bytes.Equal(got, data) {
			t.Fatal("retry read changed", err)
		}
	}
	if calls != 5 {
		t.Fatal("correction reused an exhausted reader", calls)
	}
	for _, bad := range [][]byte{data[:len(data)-1], append(bytes.Clone(data), 'x'), bytes.Repeat([]byte("x"), len(data))} {
		broken := artifactDownload(func(_ context.Context, _ string, w io.Writer, _ int64) (int64, error) {
			return io.Copy(w, bytes.NewReader(bad))
		})
		if err := readPreparedArtifact(t.Context(), broken, a, io.Discard); !errors.Is(err, clip.ErrInvalidMedia) {
			t.Fatal("invalid copy accepted", err)
		}
	}
}
func TestPreparedReaderCloseInterruptsTransfer(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 8<<20)
	a := artifactRecord(data)
	var stopped atomic.Bool
	objects := artifactDownload(func(ctx context.Context, _ string, w io.Writer, _ int64) (int64, error) {
		defer stopped.Store(true)
		var n int64
		for len(data) > 0 {
			if err := ctx.Err(); err != nil {
				return n, err
			}
			size, err := w.Write(data[:min(32768, len(data))])
			n += int64(size)
			if err != nil {
				return n, err
			}
			data = data[size:]
		}
		return n, nil
	})
	reader, err := artifactVideo(objects, a).Open(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	one := make([]byte, 1)
	if _, err = reader.Read(one); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { reader.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close left a blocked storage stream")
	}
	if !stopped.Load() {
		t.Fatal("transfer outlived reader")
	}
}
