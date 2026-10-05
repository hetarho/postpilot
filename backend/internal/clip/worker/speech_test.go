package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/postpilot/backend/internal/clip"
	"io"
	"os"
	"testing"
)

type speechTransfer struct {
	Artifacts
	data  []byte
	calls int
	err   error
}

func (s *speechTransfer) Download(ctx context.Context, _ clip.MediaLeaseCredentials, slot string, dst io.Writer, _ int64) (int64, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if s.err != nil {
		return 0, s.err
	}
	if slot != "speech/asset" {
		return 0, clip.ErrInvalid
	}
	return io.Copy(dst, bytes.NewReader(s.data))
}
func TestSpeechLoaderBoundsHashLeaseCacheAndCleanup(t *testing.T) {
	data := []byte("immutable fixture")
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	for _, name := range []string{"valid", "corrupt", "lease-revoked", "cancelled", "unknown"} {
		t.Run(name, func(t *testing.T) {
			transfer := &speechTransfer{data: data}
			if name == "corrupt" {
				transfer.data = []byte("immutable Fixture")
			}
			if name == "lease-revoked" {
				transfer.err = clip.ErrMediaLeaseLost
			}
			executor := &Executor{artifacts: transfer}
			ws := clip.MediaWorkspace{Path: t.TempDir(), CheckCapacity: func(int64) error { return nil }}
			loader, release := executor.speechLoader(ws, clip.MediaWork{}, clip.MediaTask{Speech: []clip.MediaTaskSpeech{{AssetID: "asset", AudioHash: hash, Bytes: int64(len(data))}}})
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if name == "cancelled" {
				cancel()
			}
			id := "asset"
			if name == "unknown" {
				id = "other"
			}
			var path string
			load := func() error {
				return loader(ctx, id, func(p string) error {
					path = p
					got, e := os.ReadFile(p)
					if e != nil {
						return e
					}
					if !bytes.Equal(data, got) {
						t.Fatal("changed bytes")
					}
					return nil
				})
			}
			err := load()
			if name == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				if err = load(); err != nil || transfer.calls != 1 {
					t.Fatal("duplicate immutable download", err, transfer.calls)
				}
			} else if err == nil {
				t.Fatal("unsafe input admitted")
			}
			if name == "lease-revoked" && !errors.Is(err, clip.ErrMediaLeaseLost) {
				t.Fatal(err)
			}
			if err = release(); err != nil {
				t.Fatal(err)
			}
			if path != "" {
				if _, err = os.Stat(path); !os.IsNotExist(err) {
					t.Fatal("private input remains", err)
				}
			}
			entries, err := os.ReadDir(ws.Path)
			if err != nil || len(entries) != 0 {
				t.Fatal("leaked partial speech", entries, err)
			}
		})
	}
}
