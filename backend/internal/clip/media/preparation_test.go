package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func TestOversizeRetriesExactlySameIntervalOnce(t *testing.T) {
	for _, twice := range []bool{false, true} {
		t.Run(map[bool]string{false: "second fits", true: "second fails"}[twice], func(t *testing.T) {
			encodes, consumed := 0, 0
			base := chunkRunner()
			r := &fakeRunner{run: func(ctx context.Context, c Command) ([]byte, error) {
				if slices.Contains(c.Args, "-c:v") {
					encodes++
					if encodes == 1 || twice {
						f, err := os.Create(c.Args[len(c.Args)-1])
						if err != nil {
							return nil, err
						}
						err = f.Truncate(8<<20 + 1)
						return nil, errors.Join(err, f.Close())
					}
				}
				return base(ctx, c)
			}}
			a := newAdapter(t, r)
			err := a.WithWorkspace(t.Context(), "oversize", func(ws clip.MediaWorkspace) error {
				return a.PrepareAnalysisChunks(t.Context(), ws, clip.MediaSource{Path: sourceFile(t, ws), SourceID: "one", Fingerprint: "hash", Info: clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 60000}}, func(clip.AnalysisChunk) error { consumed++; return nil })
			})
			if twice && !errors.Is(err, clip.ErrAnalysisTooLarge) || !twice && err != nil {
				t.Fatal(err)
			}
			if encodes != 2 || consumed != map[bool]int{true: 0, false: 1}[twice] {
				t.Fatal(encodes, consumed)
			}
			first, second := r.calls[0].Args, r.calls[1].Args
			for _, flag := range []string{"-i", "-ss", "-t", "-vf"} {
				if first[slices.Index(first, flag)+1] != second[slices.Index(second, flag)+1] {
					t.Fatal("retry changed coverage", flag)
				}
			}
			if second[slices.Index(second, "-maxrate")+1] != "650000" || second[slices.Index(second, "-bufsize")+1] != "1300000" {
				t.Fatal(second)
			}
		})
	}
}

func TestWorkspaceCapacityCountsPreparedAndOriginalFiles(t *testing.T) {
	a := newAdapter(t, &fakeRunner{})
	a.cfg.WorkspaceMaxBytes, a.cfg.PreparedMaxBytes = 128, 64
	if err := a.WithWorkspace(t.Context(), "limits", func(ws clip.MediaWorkspace) error {
		if err := ws.CheckCapacity(129); !errors.Is(err, clip.ErrWorkspaceLimit) {
			t.Fatal(err)
		}
		original := filepath.Join(ws.Path, "source.mp4")
		if err := os.WriteFile(original, []byte(strings.Repeat("x", 60)), 0600); err != nil {
			return err
		}
		if err := ws.CheckCapacity(68); err != nil {
			t.Fatal(err)
		}
		if err := ws.CheckCapacity(69); !errors.Is(err, clip.ErrWorkspaceLimit) {
			t.Fatal(err)
		}
		proxy := filepath.Join(ws.Path, "proxy-one.mp4")
		if err := os.WriteFile(proxy, []byte(strings.Repeat("p", 65)), 0600); err != nil {
			return err
		}
		if err := ws.CheckCapacity(0); !errors.Is(err, clip.ErrWorkspaceLimit) {
			t.Fatal("prepared cap ignored", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestFreeSpaceRefusalPrecedesAnySubprocess(t *testing.T) {
	r := &fakeRunner{run: func(context.Context, Command) ([]byte, error) { t.Fatal("low disk invoked binary"); return nil, nil }}
	a := newAdapter(t, r)
	a.diskCheck = func(string, int64) error { return clip.ErrWorkspaceLimit }
	err := a.WithWorkspace(t.Context(), "low disk", func(ws clip.MediaWorkspace) error { _, err := a.run(t.Context(), ws, "fake"); return err })
	if !errors.Is(err, clip.ErrWorkspaceLimit) || len(r.calls) != 0 {
		t.Fatal(err)
	}
}

func TestDiskMonitorInterruptsWritingSubprocess(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		t.Run(map[bool]string{false: "workspace", true: "proxy"}[proxy], func(t *testing.T) {
			r := &fakeRunner{run: func(ctx context.Context, c Command) ([]byte, error) {
				if err := os.WriteFile(c.Args[0], []byte(strings.Repeat("x", 129)), 0600); err != nil {
					return nil, err
				}
				<-ctx.Done()
				return nil, ctx.Err()
			}}
			a := newAdapter(t, r)
			a.cfg.DiskCheckInterval = time.Millisecond
			if !proxy {
				a.cfg.WorkspaceMaxBytes = 128
			}
			err := a.WithWorkspace(t.Context(), "monitor", func(ws clip.MediaWorkspace) error {
				path := filepath.Join(ws.Path, "output.mp4")
				if proxy {
					_, err := a.runBounded(t.Context(), ws, "encoder", path, 128, clip.ErrAnalysisTooLarge, path)
					return err
				}
				_, err := a.run(t.Context(), ws, "encoder", path)
				return err
			})
			want := clip.ErrWorkspaceLimit
			if proxy {
				want = clip.ErrAnalysisTooLarge
			}
			if !errors.Is(err, want) {
				t.Fatal(err)
			}
		})
	}
}

func TestSubprocessesSerializeAcrossNestedAndConcurrentWorkspaces(t *testing.T) {
	var active, peak atomic.Int32
	r := &fakeRunner{run: func(ctx context.Context, _ Command) ([]byte, error) {
		n := active.Add(1)
		if n > peak.Load() {
			peak.Store(n)
		}
		defer active.Add(-1)
		select {
		case <-time.After(5 * time.Millisecond):
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}}
	a := newAdapter(t, r)
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for range 3 {
		wg.Go(func() {
			errs <- a.WithWorkspace(t.Context(), "outer", func(clip.MediaWorkspace) error {
				return a.WithWorkspace(t.Context(), "nested caption", func(ws clip.MediaWorkspace) error { _, err := a.run(t.Context(), ws, "fake"); return err })
			})
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if peak.Load() != 1 {
		t.Fatal("parallel media processes", peak.Load())
	}
}

func TestChunkValidationRejectsIncompleteOrAlteredProxy(t *testing.T) {
	a := newAdapter(t, &fakeRunner{})
	s := clip.MediaSource{Info: clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 60000, HasAudio: true}}
	c := clip.AnalysisChunk{DurationMS: 60000, Info: clip.MediaInfo{Width: 720, Height: 404, DurationMS: 60011, ContainerDurationMS: 60000, VideoDurationMS: 60000, AudioDurationMS: 60000, AudioChannels: 1, AudioRate: 48000, HasAudio: true, PixelFormat: "yuv420p", SampleAspectRatio: "1:1", FrameRateNumerator: 15, FrameRateDenominator: 1, Streams: []clip.MediaStream{{Kind: "video", Codec: "h264"}, {Kind: "audio", Codec: "aac"}}}}
	if !a.validChunk(s, c) {
		t.Fatal("valid proxy refused")
	}
	for _, mutate := range []func(*clip.MediaInfo){
		func(p *clip.MediaInfo) { p.VideoDurationMS = 58000 },
		func(p *clip.MediaInfo) { p.ContainerDurationMS = 60001 },
		func(p *clip.MediaInfo) { p.AudioDurationMS = 59000 },
		func(p *clip.MediaInfo) { p.AudioChannels = 2 },
		func(p *clip.MediaInfo) { p.Width = 721 },
		func(p *clip.MediaInfo) { p.Rotation = 90 },
		func(p *clip.MediaInfo) { p.FrameRateNumerator = 30 },
	} {
		copy := c
		mutate(&copy.Info)
		if a.validChunk(s, copy) {
			t.Fatal("incomplete/altered proxy accepted", copy.Info)
		}
	}
}
