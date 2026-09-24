package media

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
	pb "github.com/postpilot/backend/internal/gen/postpilot/v1"
	"github.com/postpilot/backend/internal/gen/postpilot/v1/postpilotv1connect"
)

type parityArtifacts struct {
	next      int
	originals map[string]string
	dir       string
	outputs   map[string]string
}

func (p *parityArtifacts) Download(_ context.Context, _ clip.MediaLeaseCredentials, slot string, dst io.Writer, _ int64) (int64, error) {
	source, ok := strings.CutPrefix(slot, "source/")
	if !ok {
		return 0, clip.ErrInvalid
	}
	f, err := os.Open(p.originals[source])
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(dst, f)
}
func (p *parityArtifacts) Upload(ctx context.Context, _ clip.MediaLeaseCredentials, out clip.MediaOutput, path string) error {
	sum, err := worker.FileDigest(ctx, path, out.Bytes)
	if err != nil {
		return err
	}
	if sum != out.Digest {
		return clip.ErrInvalidMedia
	}
	dest := filepath.Join(p.dir, fmt.Sprintf("%d-%s.mp4", p.next, strings.ReplaceAll(out.Slot, "/", "-")))
	p.next++
	if err = copyIdentityFile(path, dest); err != nil {
		return err
	}
	p.outputs[out.Slot] = dest
	return nil
}
func parityMetadata(t *testing.T, path string, info clip.MediaInfo) clip.SourceMetadata {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	m := clip.SourceMetadata{Filename: filepath.Base(path), ContentType: "video/mp4", Bytes: int64(len(data)), DurationMS: info.DurationMS, Width: info.Width, Height: info.Height}
	prefix := []byte{1}
	prefix = binary.BigEndian.AppendUint64(prefix, uint64(m.Bytes))
	prefix = binary.BigEndian.AppendUint32(prefix, uint32(len(m.ContentType)))
	prefix = append(prefix, m.ContentType...)
	prefix = binary.BigEndian.AppendUint64(prefix, uint64(m.DurationMS))
	prefix = append(prefix, data[:min(len(data), 64<<10)]...)
	prefix = append(prefix, data[max(0, len(data)-(64<<10)):]...)
	sum := sha256.Sum256(prefix)
	m.Fingerprint = hex.EncodeToString(sum[:])
	return m
}
func parityWork(t *testing.T, op clip.MediaOperation, task clip.MediaTask) clip.MediaWork {
	t.Helper()
	raw, err := mediacodec.EncodeTask(task)
	if err != nil {
		t.Fatal(err)
	}
	return clip.MediaWork{Operation: op, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Payload: raw, InputDigest: clip.MediaPayloadDigest(raw), Credentials: clip.MediaLeaseCredentials{AttemptID: "parity-worker"}}
}
func TestWorkerExecutionParity(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real binary worker parity runs in pinned image")
	}
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := r.RuntimeProfile(t.Context(), "auto")
	if err != nil || !profile.Compatible() {
		t.Fatal("CPU readiness", err)
	}
	if _, err = r.RuntimeProfile(t.Context(), "nvenc"); err == nil {
		t.Fatal("unapproved GPU profile ready")
	}
	transfer := &parityArtifacts{originals: map[string]string{}, dir: t.TempDir(), outputs: map[string]string{}}
	var sources []clip.RenderSource
	task := clip.MediaTask{Version: clip.MediaContractVersion}
	err = a.WithWorkspace(t.Context(), "parity-fixtures", func(ws clip.MediaWorkspace) error {
		built, paths, _, err := qaFootage(t, a, ws, cfg)
		if err != nil {
			return err
		}
		// The first two originals cover all cuts; unused source three adds no case.
		for _, s := range built[:2] {
			dest := filepath.Join(transfer.dir, s.ID+".mp4")
			if err = copyIdentityFile(paths[s.ID], dest); err != nil {
				return err
			}
			transfer.originals[s.ID] = dest
			m := parityMetadata(t, dest, s.Info)
			s.Fingerprint = m.Fingerprint
			sources = append(sources, s)
			task.Sources = append(task.Sources, clip.MediaTaskSource{ID: s.ID, SourceMetadata: m, Info: s.Info})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	executor := worker.NewExecutor(a, r, transfer, cfg)
	t.Run("preparation", func(t *testing.T) {
		// Reuse the second source's single chunk; only the missing first is produced.
		prepare := task
		prepare.Sources = append([]clip.MediaTaskSource(nil), task.Sources...)
		prepare.Sources[1].ReusedChunks = []int{0}
		raw, err := executor.Execute(t.Context(), parityWork(t, clip.MediaPrepare, prepare))
		if err != nil {
			t.Fatal(err)
		}
		result, err := mediacodec.DecodeResult(raw)
		if err != nil || len(result.Outputs) != 1 {
			t.Fatal(result, err)
		}
		err = a.WithWorkspace(t.Context(), "embedded-prepare", func(ws clip.MediaWorkspace) error {
			s := sources[0]
			path := filepath.Join(ws.Path, "original.mp4")
			if err := copyIdentityFile(transfer.originals[s.ID], path); err != nil {
				return err
			}
			header, err := a.ProbeContainer(t.Context(), ws, path)
			if err != nil {
				return err
			}
			actual, err := a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{SourceID: s.ID, Fingerprint: s.Fingerprint, Info: header, Path: path}, func(chunk clip.AnalysisChunk) error {
				digest, err := worker.FileDigest(t.Context(), chunk.Path, chunk.Bytes)
				if err != nil {
					return err
				}
				want := result.Outputs[0]
				if want.Digest != digest || want.Bytes != chunk.Bytes || !reflect.DeepEqual(want.Info, chunk.Info) {
					t.Fatal("analysis bytes/measurement changed")
				}
				return nil
			})
			if err == nil && !reflect.DeepEqual(result.Sources[0].Info, actual) {
				t.Fatal("original measurement changed")
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	})
	for _, sequence := range []bool{false, true} {
		name := "footage-audio-fade"
		if sequence {
			name = "sequence-caption"
		}
		t.Run(name, func(t *testing.T) {
			plan, err := qaPlan(qaClipCases()[0], sources)
			if err != nil {
				t.Fatal(err)
			}
			for i := range plan.Cuts {
				for _, s := range sources {
					if s.ID == plan.Cuts[i].SourceID {
						plan.Cuts[i].Fingerprint = s.Fingerprint
					}
				}
			}
			if sequence {
				plan.CaptionStyles = []string{"pop"}
				for i := range plan.Portable.Elements {
					if plan.Portable.Elements[i].Resolved.Element.Role == "caption" {
						plan.Portable.Elements[i].Resolved.Element.Style = "pop"
					}
				}
			}
			plan, _, err = r.Layout(t.Context(), plan, sources)
			if err != nil {
				t.Fatal(err)
			}
			frozen := task
			frozen.Plan, err = clip.EncodeEditPlan(plan)
			if err != nil {
				t.Fatal(err)
			}
			frozen.Render = clip.FreezeMediaRenderInputs(plan)
			frozen.HideDisclosure = plan.HideDisclosure
			embedded := filepath.Join(transfer.dir, name+"-embedded.mp4")
			var expected clip.RenderedVideo
			err = a.WithWorkspace(t.Context(), "embedded-render", func(ws clip.MediaWorkspace) error {
				expected, err = r.Render(t.Context(), ws, plan, sources, func(ctx context.Context, id string, consume func(clip.MediaSource) error) error {
					path := filepath.Join(ws.Path, "original.mp4")
					if err := copyIdentityFile(transfer.originals[id], path); err != nil {
						return err
					}
					defer os.Remove(path)
					for _, s := range sources {
						if s.ID == id {
							return consume(clip.MediaSource{SourceID: id, Fingerprint: s.Fingerprint, Info: s.Info, Path: path})
						}
					}
					return clip.ErrInvalid
				})
				if err != nil {
					return err
				}
				return copyIdentityFile(expected.Path, embedded)
			})
			if err != nil {
				t.Fatal(err)
			}
			if sequence {
				found := false
				for _, element := range expected.Elements {
					if style, ok := design.LookupCaptionStyle(element.Style); element.Role == "caption" && ok && !style.Static() {
						found = true
					}
				}
				if !found {
					t.Fatal("parity fixture no longer exercises sequence frames")
				}
			}
			raw, err := executor.Execute(t.Context(), parityWork(t, clip.MediaRender, frozen))
			if err != nil {
				t.Fatal(err)
			}
			result, err := mediacodec.DecodeResult(raw)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(expected.Info, result.Outputs[0].Info) || expected.Bytes != result.Outputs[0].Bytes {
				t.Fatal("CDS output measurements changed")
			}
			err = a.WithWorkspace(t.Context(), "parity-inspect", func(ws clip.MediaWorkspace) error {
				left := fingerprintDelivered(t, a, ws, transfer.dir, embedded)
				right := fingerprintDelivered(t, a, ws, transfer.dir, transfer.outputs["result"])
				if delta := left.differences(right); len(delta) > 0 {
					t.Fatalf("embedded/worker output differs: %v", delta)
				}
				t.Logf("identical CPU bytes %s; %d sampled frames; %d bytes", left.Digest, len(left.Frames), expected.Bytes)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	entries, err := os.ReadDir(cfg.WorkRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatal("workspace leaked", entry.Name())
		}
	}
}

func TestWorkerCommandHealth(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("standalone command is in the execution image")
	}
	tokenBytes := make([]byte, 32)
	for i := range tokenBytes {
		tokenBytes[i] = byte(i)
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	mux := http.NewServeMux()
	mux.Handle(postpilotv1connect.NewClipMediaWorkerServiceHandler(&parityHealth{t: t, token: token}))
	api := httptest.NewServer(mux)
	defer api.Close()
	for _, mode := range []string{"cpu", "auto", "nvenc"} {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		base := filepath.Join(t.TempDir(), "health")
		root := worker.WorkRoot(base, "smoke-cpu")
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		release, err := worker.LockWorkRoot(root)
		if err != nil {
			t.Fatal(err)
		}
		command := exec.CommandContext(ctx, "/media-worker", "health")
		command.Env = []string{"MEDIA_API_URL=" + api.URL, "MEDIA_WORKER_ID=smoke-cpu", "MEDIA_WORKER_TOKEN=" + token, "MEDIA_ACCEL=" + mode, "CLIP_WORK_ROOT=" + base, "DATABASE_PATH=/unwritable/no-database", "PROVIDERS_CONFIG=/no-providers"}
		out, err := command.CombinedOutput()
		release()
		t.Cleanup(cancel)
		if mode == "nvenc" {
			if err == nil || !strings.Contains(string(out), "not approved") {
				t.Fatalf("nvenc readiness: %v %s", err, out)
			}
			continue
		}
		if err != nil || !strings.Contains(string(out), `"Profile":"cpu"`) || !strings.Contains(string(out), `"Ready":true`) {
			t.Fatalf("standalone %s boot: %v %s", mode, err, out)
		}
		if _, err := os.Stat(worker.WorkRoot(base, "smoke-cpu-health")); !os.IsNotExist(err) {
			t.Fatal("health workspace aliased another worker identity")
		}
		stopped := exec.CommandContext(ctx, "/media-worker", "health")
		stopped.Env = command.Env
		if out, err = stopped.CombinedOutput(); err == nil || !strings.Contains(string(out), "not running") {
			t.Fatalf("health ignored stopped worker: %v %s", err, out)
		}
	}
}

func TestWorkerImageIsolation(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("execution-only image check")
	}
	if os.Getuid() == 0 {
		t.Fatal("worker image runs as root")
	}
	for _, path := range []string{"/api", "/config/providers.yaml"} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("API artifact exists in worker: %s", path)
		}
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "/media-worker", "manifest")
	command.Env = []string{"MEDIA_ACCEL=cpu", "CLIP_WORK_ROOT=" + filepath.Join(t.TempDir(), "manifest")}
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("offline profile check: %v %s", err, raw)
	}
	var profile clip.MediaWorkerProfile
	if err = json.Unmarshal(raw, &profile); err != nil || profile.WorkerID != "" || profile.ContractVersion != clip.MediaContractVersion || profile.RendererVersion != clip.MediaRendererVersion || profile.AssetVersion != clip.MediaAssetVersion {
		t.Fatal("invalid offline contract", err)
	}
	var manifest runtimeManifest
	if err = json.Unmarshal([]byte(profile.RuntimeManifest), &manifest); err != nil || len(manifest.Fingerprint) != 64 || manifest.Tools["ffmpeg"].SHA256 == "" || len(manifest.Fonts) == 0 {
		t.Fatal("missing runtime provenance", err)
	}
	invalid := exec.CommandContext(ctx, "/media-worker", "manifest")
	invalid.Env = append(command.Env, "CLIP_ENCODE_THREADS=999")
	out, err := invalid.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "encode threads") {
		t.Fatalf("invalid worker config lost its cause: %v %s", err, out)
	}
}

type parityHealth struct {
	postpilotv1connect.UnimplementedClipMediaWorkerServiceHandler
	t     *testing.T
	token string
}

func (h *parityHealth) GetMediaRuntimeStatus(_ context.Context, r *connect.Request[pb.GetMediaRuntimeStatusRequest]) (*connect.Response[pb.GetMediaRuntimeStatusResponse], error) {
	if r.Header().Get("X-Media-Worker-ID") != "smoke-cpu" || r.Header().Get("Authorization") != "Bearer "+h.token {
		h.t.Error("worker command omitted authentication")
	}
	return connect.NewResponse(&pb.GetMediaRuntimeStatusResponse{Ready: true, ContractVersion: clip.MediaContractVersion, RendererVersion: clip.MediaRendererVersion, AssetVersion: clip.MediaAssetVersion, Waiting: 2, Active: 1}), nil
}
