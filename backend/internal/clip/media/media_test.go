package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/platform/config"
)

type fakeRunner struct {
	calls []Command
	run   func(context.Context, Command) ([]byte, error)
}

func (r *fakeRunner) Run(ctx context.Context, c Command) ([]byte, error) {
	r.calls = append(r.calls, c)
	return r.run(ctx, c)
}
func mediaConfig(t *testing.T) clip.MediaConfig {
	t.Helper()
	return config.ClipMedia(&config.Config{ClipWorkRoot: filepath.Join(t.TempDir(), "work"), ClipFFmpegPath: "/usr/local/bin/ffmpeg", ClipFFprobePath: "/usr/local/bin/ffprobe", ClipWorkStaleAge: time.Hour, ClipMediaTimeout: time.Minute, ClipSourceBatchTTL: 6 * time.Hour, PresignPutTTL: 10 * time.Minute})
}
func newAdapter(t *testing.T, r Runner) *Adapter {
	t.Helper()
	a, err := New(mediaConfig(t), r)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
func sourceFile(t *testing.T, ws clip.MediaWorkspace) string {
	t.Helper()
	p := filepath.Join(ws.Path, "source.mp4")
	if err := os.WriteFile(p, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

const probeJSON = `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":1080,"avg_frame_rate":"30000/1001","sample_aspect_ratio":"1:1","side_data_list":[{"rotation":-90},{"side_data_type":"Mastering display metadata"}]},{"index":1,"codec_type":"audio","codec_name":"aac"}],"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"99999"}}`

func TestProbeUsesDecodedClockAndRotation(t *testing.T) {
	r := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if strings.HasSuffix(c.Binary, "ffprobe") {
			return []byte(probeJSON), nil
		}
		return []byte("frame=1830\nout_time_us=61000000\nprogress=end\n"), nil
	}}
	a := newAdapter(t, r)
	err := a.WithWorkspace(t.Context(), "job", func(ws clip.MediaWorkspace) error {
		p := sourceFile(t, ws)
		info, err := a.Probe(t.Context(), ws, p)
		if err != nil {
			return err
		}
		if info.DurationMS != 61000 || info.Width != 1080 || info.Height != 1920 || info.Rotation != 270 || !info.HasAudio || len(info.Streams) != 2 || info.FrameRateNumerator != 30000 || info.FrameRateDenominator != 1001 {
			t.Fatalf("probe=%+v", info)
		}
		if !reflect.DeepEqual(r.calls[0].Args, []string{"-v", "error", "-protocol_whitelist", "file,pipe", "-show_streams", "-show_format", "-of", "json", p}) {
			t.Fatal(r.calls[0])
		}
		if !strings.Contains(strings.Join(r.calls[1].Args, " "), "-xerror") || !strings.Contains(strings.Join(r.calls[1].Args, " "), "-progress pipe:1") {
			t.Fatal(r.calls[1])
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestProbeRefusesInvalidInputs(t *testing.T) {
	for name, doc := range map[string]string{"corrupt": "not json", "empty": "{}", "audio only": `{"streams":[{"codec_type":"audio"}],"format":{"format_name":"mov"}}`, "container": strings.Replace(probeJSON, "mov,mp4,m4a,3gp,3g2,mj2", "avi", 1), "dimensions": strings.Replace(probeJSON, `"width":1920`, `"width":0`, 1), "rotation": strings.Replace(probeJSON, `-90`, `45`, 1)} {
		t.Run(name, func(t *testing.T) {
			r := &fakeRunner{run: func(context.Context, Command) ([]byte, error) { return []byte(doc), nil }}
			a := newAdapter(t, r)
			if err := a.WithWorkspace(t.Context(), "job", func(ws clip.MediaWorkspace) error { _, err := a.Probe(t.Context(), ws, sourceFile(t, ws)); return err }); !errors.Is(err, clip.ErrInvalidMedia) {
				t.Fatal(err)
			}
			if len(r.calls) != 1 {
				t.Fatal("decoded an invalid declaration")
			}
		})
	}
	for _, progress := range []string{"frame=0\nout_time_us=0\nprogress=end", "frame=1\nout_time_us=1801000000\nprogress=end", "frame=1\nout_time_us=1000000\nprogress=continue"} {
		r := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
			if strings.HasSuffix(c.Binary, "ffprobe") {
				return []byte(probeJSON), nil
			}
			return []byte(progress), nil
		}}
		a := newAdapter(t, r)
		if err := a.WithWorkspace(t.Context(), "job", func(ws clip.MediaWorkspace) error { _, err := a.Probe(t.Context(), ws, sourceFile(t, ws)); return err }); !errors.Is(err, clip.ErrInvalidMedia) {
			t.Fatal(err)
		}
	}
}
func TestChunksAreSequentialBoundedAndRemoved(t *testing.T) {
	for _, audio := range []bool{false, true} {
		t.Run(map[bool]string{true: "audio", false: "silent"}[audio], func(t *testing.T) {
			r := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
				return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("proxy"), 0600)
			}}
			a := newAdapter(t, r)
			var chunks []clip.AnalysisChunk
			err := a.WithWorkspace(t.Context(), "job", func(ws clip.MediaWorkspace) error {
				return a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{Path: sourceFile(t, ws), SourceID: "one", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 61000, HasAudio: audio}}, func(c clip.AnalysisChunk) error {
					files, err := os.ReadDir(ws.Path)
					if err != nil {
						return err
					}
					if len(files) != 2 {
						t.Fatalf("source+current proxy expected: %v", files)
					}
					if len(chunks) > 0 {
						if _, err := os.Stat(chunks[0].Path); !os.IsNotExist(err) {
							t.Fatal("previous chunk remains")
						}
					}
					chunks = append(chunks, c)
					return nil
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(chunks) != 2 || chunks[0].OffsetMS != 0 || chunks[0].DurationMS != 60000 || chunks[1].OffsetMS != 60000 || chunks[1].DurationMS != 1000 {
				t.Fatal(chunks)
			}
			for _, c := range r.calls {
				args := strings.Join(c.Args, " ")
				for _, want := range []string{"-protocol_whitelist file,pipe", "-c:v libx264", "fps=30", "scale=720:720", "format=yuv420p", "-movflags +faststart", "-map_metadata -1"} {
					if !strings.Contains(args, want) {
						t.Fatalf("missing %s: %s", want, args)
					}
				}
				if strings.Contains(args, "-c:a aac") != audio {
					t.Fatal(args)
				}
			}
		})
	}
}
func TestWorkspaceCleansSuccessFailureAndPanic(t *testing.T) {
	for _, mode := range []string{"success", "failure", "panic"} {
		t.Run(mode, func(t *testing.T) {
			a := newAdapter(t, &fakeRunner{})
			var path string
			sentinel := errors.New("callback failed")
			func() {
				defer func() {
					if v := recover(); v != nil && mode != "panic" {
						t.Fatal(v)
					}
				}()
				err := a.WithWorkspace(t.Context(), "job; never a path", func(ws clip.MediaWorkspace) error {
					path = ws.Path
					sourceFile(t, ws)
					if mode == "panic" {
						panic("worker panic")
					}
					if mode == "failure" {
						return sentinel
					}
					return nil
				})
				if mode == "failure" && !errors.Is(err, sentinel) {
					t.Fatal(err)
				}
			}()
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("workspace remains: %v", err)
			}
			if _, err := os.Stat(a.cfg.WorkRoot); err != nil {
				t.Fatal("work root was removed")
			}
		})
	}
}
func TestCancellationAndChunkFailureCleanup(t *testing.T) {
	for _, failure := range []string{"runner", "consumer", "panic", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			r := &fakeRunner{run: func(ctx context.Context, c Command) ([]byte, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("no operation deadline")
				}
				_ = os.WriteFile(c.Args[len(c.Args)-1], []byte("partial"), 0600)
				if failure == "runner" {
					return nil, errors.New("binary failed")
				}
				if failure == "cancel" {
					cancel()
					return nil, ctx.Err()
				}
				return nil, nil
			}}
			a := newAdapter(t, r)
			var path string
			func() {
				defer func() { _ = recover() }()
				err := a.WithWorkspace(ctx, "job", func(ws clip.MediaWorkspace) error {
					path = ws.Path
					return a.PrepareAnalysisChunks(ctx, ws, clip.MediaSource{Path: sourceFile(t, ws), SourceID: "one", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 61000}}, func(clip.AnalysisChunk) error {
						if failure == "panic" {
							panic("consumer")
						}
						return errors.New("observation failed")
					})
				})
				if err == nil {
					t.Fatal("accepted failure")
				}
			}()
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatal("failed workspace remains")
			}
			if len(r.calls) != 1 {
				t.Fatal("prepared the next proxy after failure")
			}
		})
	}
}
func TestStaleCleanupNeverTargetsForeignDirectoriesOrSymlinks(t *testing.T) {
	a := newAdapter(t, &fakeRunner{})
	now := time.Now()
	old := now.Add(-2 * time.Hour)
	stale := filepath.Join(a.cfg.WorkRoot, workspacePrefix+strings.Repeat("a", 32))
	fresh := filepath.Join(a.cfg.WorkRoot, workspacePrefix+strings.Repeat("b", 32))
	foreign := filepath.Join(a.cfg.WorkRoot, "foreign")
	bad := filepath.Join(a.cfg.WorkRoot, "postpilot-clip-not-a-validated-id")
	for _, p := range []string{stale, fresh, foreign, bad} {
		if err := os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{stale, foreign, bad} {
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	link := filepath.Join(a.cfg.WorkRoot, workspacePrefix+strings.Repeat("c", 32))
	if err := os.Symlink(out, link); err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "active", func(ws clip.MediaWorkspace) error {
		_ = os.Chtimes(ws.Path, old, old)
		if err := a.CleanupStale(t.Context(), now); err != nil {
			return err
		}
		if _, err := os.Stat(ws.Path); err != nil {
			t.Fatal("active workspace removed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatal("stale workspace remains")
	}
	for _, p := range []string{fresh, foreign, bad, out, link} {
		if _, err := os.Lstat(p); err != nil {
			t.Fatalf("foreign/fresh path removed: %s", p)
		}
	}
}
func TestRejectsUnsafeRootsAndSourcePaths(t *testing.T) {
	home, _ := os.UserHomeDir()
	for _, root := range []string{"", "/", "/tmp", home, "relative", "/tmp/../etc", "/tmp/$UNRESOLVED", "/var/tmp"} {
		cfg := mediaConfig(t)
		cfg.WorkRoot = root
		if _, err := New(cfg, &fakeRunner{}); err == nil {
			t.Fatalf("accepted root %s", root)
		}
	}
	cfg := mediaConfig(t)
	if err := os.Mkdir(cfg.WorkRoot, 0700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(cfg.WorkRoot, "belongs-to-user")
	if err := os.WriteFile(keep, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(cfg, &fakeRunner{}); err == nil {
		t.Fatal("claimed a nonempty foreign directory")
	}
	if value, err := os.ReadFile(keep); err != nil || string(value) != "keep" {
		t.Fatal("foreign data changed")
	}
	r := &fakeRunner{}
	a := newAdapter(t, r)
	if err := a.WithWorkspace(t.Context(), "job", func(ws clip.MediaWorkspace) error {
		outside := filepath.Join(t.TempDir(), "outside.mp4")
		_ = os.WriteFile(outside, []byte("outside"), 0600)
		link := filepath.Join(ws.Path, "link.mp4")
		_ = os.Symlink(outside, link)
		for _, p := range []string{outside, link, filepath.Join(ws.Path, "missing.mp4")} {
			if _, err := a.Probe(t.Context(), ws, p); err == nil {
				t.Fatal("accepted unsafe path")
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 0 {
		t.Fatal("ran binary on unsafe path")
	}
}
