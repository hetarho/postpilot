package worker_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
)

type taskMedia struct {
	dir    string
	info   clip.MediaInfo
	copies []int
	calls  int
}

func (m *taskMedia) WithWorkspace(ctx context.Context, _ string, fn func(clip.MediaWorkspace) error) error {
	dir, err := os.MkdirTemp(m.dir, "attempt-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	return fn(clip.MediaWorkspace{Path: dir, CheckCapacity: func(int64) error { return nil }})
}
func (m *taskMedia) Probe(context.Context, clip.MediaWorkspace, string) (clip.MediaInfo, error) {
	m.calls++
	return m.info, nil
}
func (m *taskMedia) ProbeContainer(context.Context, clip.MediaWorkspace, string) (clip.MediaInfo, error) {
	m.calls++
	return m.info, nil
}
func (m *taskMedia) PrepareAnalysisChunksExcept(ctx context.Context, ws clip.MediaWorkspace, s clip.MediaSource, skip func(int) bool, consume func(clip.AnalysisChunk) error) (clip.MediaInfo, error) {
	for i := 0; i < 2; i++ {
		c := clip.AnalysisChunk{SourceID: s.SourceID, Fingerprint: s.Fingerprint, Index: i, OffsetMS: i * 60000, DurationMS: 60000}
		if !skip(i) {
			m.copies = append(m.copies, i)
			c.Path = filepath.Join(ws.Path, "chunk.mp4")
			c.Bytes = 4
			c.Info = clip.MediaInfo{Width: 720, Height: 404, DurationMS: 60000}
			if err := os.WriteFile(c.Path, []byte("copy"), 0600); err != nil {
				return clip.MediaInfo{}, err
			}
		}
		if err := consume(c); err != nil {
			return clip.MediaInfo{}, err
		}
	}
	return m.info, nil
}

type taskTransfer struct {
	data     []byte
	uploaded []clip.MediaOutput
	fail     bool
}

func (a *taskTransfer) Download(_ context.Context, _ clip.MediaLeaseCredentials, _ string, w io.Writer, _ int64) (int64, error) {
	return io.Copy(w, bytes.NewReader(a.data))
}
func (a *taskTransfer) Upload(_ context.Context, _ clip.MediaLeaseCredentials, out clip.MediaOutput, _ string) error {
	if a.fail {
		return clip.ErrMediaUnavailable
	}
	a.uploaded = append(a.uploaded, out)
	return nil
}
func executorWork(t *testing.T, metadata clip.SourceMetadata, info clip.MediaInfo) clip.MediaWork {
	t.Helper()
	task := clip.MediaTask{Version: clip.MediaContractVersion, Sources: []clip.MediaTaskSource{{ID: "source", SourceMetadata: metadata, Info: info, ReusedChunks: []int{0}}}}
	payload, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	return clip.MediaWork{Operation: clip.MediaPrepare, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: payload, InputDigest: clip.MediaPayloadDigest(payload), Credentials: clip.MediaLeaseCredentials{AttemptID: "attempt"}}
}
func TestPrepareMissingCopiesIdentityAndCleanupOnEveryExit(t *testing.T) {
	data := []byte("original")
	info := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 120000}
	m := clip.SourceMetadata{Filename: "take.mp4", ContentType: "video/mp4", Bytes: int64(len(data)), DurationMS: info.DurationMS, Width: info.Width, Height: info.Height}
	header := []byte{1}
	header = binary.BigEndian.AppendUint64(header, uint64(m.Bytes))
	header = binary.BigEndian.AppendUint32(header, uint32(len(m.ContentType)))
	header = append(header, m.ContentType...)
	header = binary.BigEndian.AppendUint64(header, uint64(m.DurationMS))
	header = append(header, data...)
	header = append(header, data...)
	sum := sha256.Sum256(header)
	m.Fingerprint = hex.EncodeToString(sum[:])
	for _, mode := range []string{"ok", "corrupt", "upload", "cancel", "contract"} {
		t.Run(mode, func(t *testing.T) {
			media := &taskMedia{dir: t.TempDir(), info: info}
			transfer := &taskTransfer{data: bytes.Clone(data), fail: mode == "upload"}
			if mode == "corrupt" {
				transfer.data[0] ^= 1
			}
			e := worker.NewExecutor(media, nil, transfer, clip.DefaultMediaConfig(clip.Environment{}))
			w := executorWork(t, m, info)
			if mode == "contract" {
				w.ContractVersion++
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "cancel" {
				cancel()
			}
			raw, err := e.Execute(ctx, w)
			if mode == "ok" {
				if err != nil {
					t.Fatal(err)
				}
				result, err := mediacodec.DecodeResult(raw)
				if err != nil || len(result.Outputs) != 1 || result.Outputs[0].Index != 1 || len(media.copies) != 1 || media.copies[0] != 1 {
					t.Fatal("reused chunk regenerated", result, err)
				}
			} else {
				if err == nil || raw != "" {
					t.Fatal("failed execution produced receipt")
				}
				if mode == "corrupt" && (!errors.Is(err, clip.ErrInvalidMedia) || media.calls != 0) {
					t.Fatal("corrupt original decoded", err)
				}
			}
			dirs, _ := os.ReadDir(media.dir)
			if len(dirs) > 0 {
				t.Fatal("workspace leaked on", mode)
			}
		})
	}
}

func TestFullyObservedSourceNeedsNoDownloadOrDecode(t *testing.T) {
	info := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 120000}
	source := clip.MediaTaskSource{ID: "source", SourceMetadata: clip.SourceMetadata{Filename: "take.mp4", ContentType: "video/mp4", Bytes: 100, DurationMS: 120000, Width: 1280, Height: 720, Fingerprint: "already-verified"}, Info: info, ReusedChunks: []int{0, 1}}
	task := clip.MediaTask{Version: clip.MediaContractVersion, Sources: []clip.MediaTaskSource{source}}
	payload, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	media := &taskMedia{dir: t.TempDir(), info: info}
	// Nil artifact/render ports prove the all-observed request performs no I/O.
	executor := worker.NewExecutor(media, nil, nil, clip.DefaultMediaConfig(clip.Environment{}))
	raw, err := executor.Execute(t.Context(), clip.MediaWork{Operation: clip.MediaPrepare, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: payload, InputDigest: clip.MediaPayloadDigest(payload)})
	if err != nil {
		t.Fatal(err)
	}
	result, err := mediacodec.DecodeResult(raw)
	if err != nil || len(result.Sources) != 1 || len(result.Outputs) != 0 || media.calls != 0 || len(media.copies) != 0 {
		t.Fatal("completed media work repeated", result, err)
	}
}
