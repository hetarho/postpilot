package openaicompat

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"sync"

	"github.com/postpilot/backend/internal/llm"
)

var errStrictBody = errors.New("bounded video reader failed its exact-size contract")

type strictBody struct {
	reader                *io.PipeReader
	source                io.ReadCloser
	done                  chan struct{}
	closeOnce, sourceOnce sync.Once
}

func (b *strictBody) Read(p []byte) (int, error) { return b.reader.Read(p) }
func (b *strictBody) closeSource()               { b.sourceOnce.Do(func() { _ = b.source.Close() }) }
func (b *strictBody) Close() error {
	b.closeOnce.Do(func() {
		_ = b.reader.Close()
		b.closeSource()
		<-b.done
	})
	return nil
}

func streamStrictBody(ctx context.Context, envelope []byte, video *llm.InlineVideo) (io.ReadCloser, int64, error) {
	if err := ctx.Err(); err != nil {
		return nil, 0, err
	}
	if video == nil {
		return io.NopCloser(bytes.NewReader(envelope)), int64(len(envelope)), nil
	}
	// Exact structural marker, not a replacement in arbitrary prompt text. A raw
	// schema containing the same structure is refused, never substituted silently.
	marker := []byte(`"url":"data:video/mp4;base64,`)
	if bytes.Count(envelope, marker) != 1 {
		return nil, 0, llm.ErrUnsupported
	}
	split := bytes.Index(envelope, marker) + len(marker)
	source, err := video.Open(ctx)
	if err != nil {
		if source != nil {
			_ = source.Close()
		}
		return nil, 0, errStrictBody
	}
	if source == nil {
		return nil, 0, errStrictBody
	}
	reader, writer := io.Pipe()
	body := &strictBody{reader: reader, source: source, done: make(chan struct{})}
	go func() {
		defer close(body.done)
		defer body.closeSource()
		writeErr := func() error {
			if _, err := writer.Write(envelope[:split]); err != nil {
				return err
			}
			encoder := base64.NewEncoder(base64.StdEncoding, writer)
			n, err := io.CopyBuffer(encoder, io.LimitReader(source, video.Size), make([]byte, 32<<10))
			if err != nil || n != video.Size {
				return errStrictBody
			}
			var extra [1]byte
			if n, err := io.ReadFull(source, extra[:]); n != 0 || err != io.EOF {
				return errStrictBody
			}
			if err := encoder.Close(); err != nil {
				return err
			}
			_, err = writer.Write(envelope[split:])
			return err
		}()
		_ = writer.CloseWithError(writeErr)
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = body.Close()
		case <-body.done:
		}
	}()
	return body, int64(len(envelope)) + (video.Size+2)/3*4, nil
}
