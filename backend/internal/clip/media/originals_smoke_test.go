package media

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Opt-in offline verification of owner-supplied footage. The folder is mounted
// read-only into the production runtime; no provider, credentials or DB is used.
func TestRenderOriginalsOverlap(t *testing.T) {
	root := os.Getenv("CLIP_ORIGINALS_DIR")
	if root == "" {
		t.Skip("requires a folder of eight local MP4s and production media binaries")
	}
	files, err := filepath.Glob(filepath.Join(root, "*.mp4"))
	if err != nil || len(files) != 8 {
		t.Fatalf("expected eight originals: count=%d error=%v", len(files), err)
	}
	t.Cleanup(func() {
		for _, name := range []string{"memory.peak", "memory.events"} {
			if data, err := os.ReadFile("/sys/fs/cgroup/" + name); err == nil {
				t.Logf("%s: %s", name, data)
			}
		}
	})
	cfg := mediaConfig(t)
	// Match CLIP_MEDIA_TIMEOUT's production default. The one-minute unit-test
	// helper is too short for the final 30 s encode of full-resolution footage.
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "originals-overlap", func(ws clip.MediaWorkspace) error {
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 30000, Disclosure: "ad", Preset: "restaurant", Hook: "오늘의 한 끼", Accent: "coral",
			Facts: []clip.Answer{{Label: "상호", Text: "클립 테스트"}, {Label: "위치", Text: "현장 영상"}}, Styles: []string{"clean", "bold"}}
		ends := []int{4200, 3700, 1400, 4200, 4200, 4200, 4200, 4300}
		sources := []clip.RenderSource{}
		paths := map[string]string{}
		for i, file := range files {
			path := filepath.Join(ws.Path, fmt.Sprintf("original-%02d.mp4", i))
			if err := copyOriginal(file, path); err != nil {
				return err
			}
			info, err := a.Probe(t.Context(), ws, path)
			if err != nil {
				return err
			}
			id := fmt.Sprintf("source-%02d", i)
			sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: info})
			paths[id] = path
			caption := clip.Copy{Text: "오늘의 한 끼", Style: "clean", Anchor: "lower_mid", Align: "center", Accent: "coral"}
			if i == 0 {
				caption.Style, caption.Anchor = "bold", "upper_mid"
			}
			if i == 2 {
				caption.Text = "한 끼"
			}
			transition := 0
			if i == 1 || i == 3 {
				transition = design.Transition.FadeMS
			}
			plan.Cuts = append(plan.Cuts, clip.EditCut{ID: id, SourceID: id, Fingerprint: id, EndMS: ends[i], TransitionMS: transition,
				Focal: clip.Point{X: .5, Y: .5}, Copies: []clip.Copy{caption}, Chips: []string{"위치"}})
			plan.Decisions = append(plan.Decisions, clip.Composition{Class: "DESC"})
		}
		// The same words survive initial generation and a later stored/manual plan.
		for _, mode := range []string{"generated", "manual"} {
			if mode == "manual" {
				plan.Decisions = nil
			}
			laidOut, manifest, err := r.Layout(t.Context(), plan, sources)
			if err != nil {
				return fmt.Errorf("%s layout: %w", mode, err)
			}
			if !reflect.DeepEqual(laidOut, plan) {
				return fmt.Errorf("%s changed a caption for overlap", mode)
			}
			if err := design.VerifyApproved(manifest, plan.Ratio, plan.Styles); !errors.Is(err, design.ViolationOverlap) {
				return fmt.Errorf("%s must reproduce overlap: %v", mode, err)
			}
			loads := map[string]int{}
			result, err := r.Render(t.Context(), ws, laidOut, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
				for _, source := range sources {
					if source.ID == id {
						t.Logf("%s: rendering %s", mode, id)
						loads[id]++
						return consume(clip.MediaSource{SourceID: id, Fingerprint: source.Fingerprint, Info: source.Info, Path: paths[id]})
					}
				}
				return clip.ErrNotFound
			})
			if err != nil {
				return fmt.Errorf("%s render: %w", mode, err)
			}
			if len(loads) != len(sources) || result.Info.DurationMS != plan.DurationMS || result.Bytes <= 0 {
				return fmt.Errorf("%s did not deliver all eight sources: loads=%d duration=%d bytes=%d", mode, len(loads), result.Info.DurationMS, result.Bytes)
			}
			if err := design.VerifyApproved(result.Manifest, plan.Ratio, plan.Styles); !errors.Is(err, design.ViolationOverlap) {
				return fmt.Errorf("%s final manifest lost the overlap fixture: %v", mode, err)
			}
			if output := os.Getenv("CLIP_ORIGINALS_OUTPUT"); output != "" {
				if err := copyOriginal(result.Path, filepath.Join(output, mode+".mp4")); err != nil {
					return err
				}
			}
			t.Logf("%s: %d sources, %dx%d, %d ms, %d bytes; model cost $0", mode, len(loads), result.Info.Width, result.Info.Height, result.Info.DurationMS, result.Bytes)
			if err := os.Remove(result.Path); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func copyOriginal(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(to, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(dst, src)
	return errors.Join(err, dst.Close())
}
