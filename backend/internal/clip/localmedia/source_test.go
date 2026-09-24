package localmedia

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"testing"
)

// An independent browser-format fixture pins byte order, short-file duplication
// and a tail that crosses streaming chunk boundaries.
func browserIdentity(m clip.SourceMetadata, data []byte) string {
	prefix := []byte{1}
	prefix = binary.BigEndian.AppendUint64(prefix, uint64(m.Bytes))
	prefix = binary.BigEndian.AppendUint32(prefix, uint32(len(m.ContentType)))
	prefix = append(prefix, m.ContentType...)
	prefix = binary.BigEndian.AppendUint64(prefix, uint64(m.DurationMS))
	prefix = append(prefix, data[:min(len(data), 64<<10)]...)
	prefix = append(prefix, data[max(0, len(data)-(64<<10)):]...)
	sum := sha256.Sum256(prefix)
	return hex.EncodeToString(sum[:])
}
func TestBoundedOriginalIdentityAndFailureCleanup(t *testing.T) {
	for _, size := range []int{15, 65536, 270001} {
		t.Run(string(rune(size)), func(t *testing.T) {
			data := bytes.Repeat([]byte("0123456789"), size/10+1)[:size]
			m := clip.SourceMetadata{Bytes: int64(size), ContentType: "video/mp4", DurationMS: 1000, Filename: "take.mp4"}
			m.Fingerprint = browserIdentity(m, data)
			for _, mode := range []string{"ok", "corrupt", "short", "long", "transport", "capacity", "cancel"} {
				t.Run(mode, func(t *testing.T) {
					ws := clip.MediaWorkspace{Path: t.TempDir(), CheckCapacity: func(int64) error { return nil }}
					if mode == "capacity" {
						ws.CheckCapacity = func(int64) error { return clip.ErrWorkspaceLimit }
					}
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					if mode == "cancel" {
						cancel()
					}
					input := bytes.Clone(data)
					switch mode {
					case "corrupt":
						input[0] ^= 1
					case "short":
						input = input[:len(input)-1]
					case "long":
						input = append(input, 0)
					}
					source, drop, err := Fetch(ctx, ws, clip.SourceLease{ID: "s", SourceMetadata: m, ActualBytes: m.Bytes}, clip.MediaInfo{}, 1<<20, true, func(_ context.Context, _ string, w io.Writer, _ int64) (int64, error) {
						if mode == "transport" {
							w.Write(input[:3])
							return 3, errors.New("private transport detail")
						}
						var n int64
						for len(input) > 0 {
							take := min(17003, len(input))
							written, err := w.Write(input[:take])
							n += int64(written)
							if err != nil {
								return n, err
							}
							input = input[take:]
						}
						return n, nil
					})
					if mode == "ok" {
						if err != nil {
							t.Fatal(err)
						}
						if source.Fingerprint != m.Fingerprint {
							t.Fatal(source)
						}
						if err = drop(); err != nil {
							t.Fatal(err)
						}
					} else if err == nil {
						t.Fatal("invalid input accepted")
					}
					entries, _ := os.ReadDir(ws.Path)
					if len(entries) != 0 {
						t.Fatal("partial original leaked")
					}
				})
			}
		})
	}
}
func TestLoaderHoldsOnlyOneOriginal(t *testing.T) {
	held, downloads := 0, 0
	loader, release := Loader(func(_ context.Context, id string) (clip.MediaSource, func() error, error) {
		held++
		downloads++
		if held != 1 {
			t.Fatal("two originals held")
		}
		return clip.MediaSource{SourceID: id}, func() error { held--; return nil }, nil
	})
	for _, id := range []string{"a", "a", "b", "a"} {
		if err := loader(t.Context(), id, func(s clip.MediaSource) error {
			if s.SourceID != id {
				t.Fatal(s)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if downloads != 3 {
		t.Fatal(downloads)
	}
	release()
	release()
	if held != 0 {
		t.Fatal(held)
	}
}
