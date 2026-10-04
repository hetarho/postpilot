package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/localmedia"
)

// alternatingPlan is twelve 1.25-second hard cuts over four originals, s0 s1 s2
// s3 s0 s1 …, each from its own source offset, under two captions whose reads
// land on cuts 0, 1, 2, 3 and 4: every original, and s0 twice.
func alternatingPlan(t *testing.T) (clip.EditPlan, []clip.RenderSource) {
	t.Helper()
	body := `<clip version="1"><text id="first" kind="fixed" role="caption" basis="output-start" start="0" end="2">첫 장면</text><text id="second" kind="fixed" role="caption" basis="output-start" start="3" end="6">다음 장면</text></clip>`
	limits := clip.DefaultCompositionLimits()
	doc, problem := composition.ReadStored(body, limits)
	if problem != nil {
		t.Fatal(problem)
	}
	info := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 20000}
	var sources []clip.RenderSource
	for i := range 4 {
		id := fmt.Sprintf("s%d", i)
		sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: info})
	}
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Portable: &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: 1, Body: body}}}
	for i := range 12 {
		id, source := fmt.Sprintf("cut-%02d", i), sources[i%4].ID
		start := i * 100
		plan.Cuts = append(plan.Cuts, clip.Cut{ID: id, SourceID: source, Fingerprint: source, StartMS: start, EndMS: start + 1250, Focal: clip.Point{X: .5, Y: .5}})
		plan.Portable.Cuts = append(plan.Portable.Cuts, composition.Cut{ID: id, SourceID: source, StartMS: start, EndMS: start + 1250})
	}
	resolved, problem := composition.Resolve(doc, composition.Inputs{Cuts: plan.Portable.Cuts}, limits, 30000)
	if problem != nil {
		t.Fatal(problem)
	}
	for _, element := range resolved.Elements {
		plan.Portable.Elements = append(plan.Portable.Elements, clip.PortableText{Resolved: element})
	}
	return plan, sources
}

// countingLoader is the loader both render kinds are handed — one original at
// a time — over originals written into the workspace, noting each download.
func countingLoader(ws clip.MediaWorkspace, sources []clip.RenderSource, downloads *[]string) (clip.RenderSourceLoader, func() error) {
	return localmedia.Loader(func(_ context.Context, id string) (clip.MediaSource, func() error, error) {
		*downloads = append(*downloads, id)
		i := slices.IndexFunc(sources, func(s clip.RenderSource) bool { return s.ID == id })
		if i < 0 {
			return clip.MediaSource{}, nil, clip.ErrNotFound
		}
		path := filepath.Join(ws.Path, "source-"+id+".mp4")
		if err := os.WriteFile(path, []byte("original "+id), 0600); err != nil {
			return clip.MediaSource{}, nil, err
		}
		return clip.MediaSource{SourceID: id, Fingerprint: sources[i].Fingerprint, Info: sources[i].Info, Path: path}, func() error { return os.Remove(path) }, nil
	})
}

var errFootageDone = errors.New("the cut loop is over")

// A server render and a browser render's sampling visit a plan's cuts an
// original at a time, in the order the plan first draws them, so a plan that
// alternates four originals downloads each once rather than once per run of
// cuts; every cut is still made from its own source span into its own file
// (ARCH-48).
func TestCutLoopsLoadEachOriginalOnce(t *testing.T) {
	plan, sources := alternatingPlan(t)
	if got := cutsBySource(plan.Cuts); !slices.Equal(got, []int{0, 4, 8, 1, 5, 9, 2, 6, 10, 3, 7, 11}) {
		t.Fatalf("the cuts are visited in the order %v", got)
	}
	var ffmpeg string
	bare := map[string][]string{}
	runner := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		if c.Binary != ffmpeg {
			return fakeResvg(c)
		}
		output := c.Args[len(c.Args)-1]
		if strings.HasPrefix(filepath.Base(output), "bare-") {
			bare[filepath.Base(output)] = c.Args
			return nil, os.WriteFile(output, []byte("footage"), 0600)
		}
		if strings.HasSuffix(output, ".png") {
			for _, arg := range c.Args {
				if strings.HasSuffix(arg, ".png") {
					solidPNG(t, arg, 128)
				}
			}
			return nil, nil
		}
		// Everything after the cut loop is the merge and the overlay, which
		// this test does not follow.
		return nil, errFootageDone
	}}
	a := newAdapter(t, runner)
	ffmpeg = a.cfg.FFmpegPath
	r := testRenderer(t, a)
	want := []string{"s0", "s1", "s2", "s3"}
	t.Run("render", func(t *testing.T) {
		var downloads []string
		err := a.WithWorkspace(t.Context(), "alternating-render", func(ws clip.MediaWorkspace) error {
			load, release := countingLoader(ws, sources, &downloads)
			_, err := r.Render(t.Context(), ws, plan, sources, load)
			return errors.Join(err, release())
		})
		if !errors.Is(err, errFootageDone) {
			t.Fatalf("the render stopped at %v, not after its cut loop", err)
		}
		if !slices.Equal(downloads, want) {
			t.Fatalf("the render downloaded %v", downloads)
		}
		for i, cut := range plan.Cuts {
			args := bare[fmt.Sprintf("bare-%04d.mp4", i)]
			at, input := slices.Index(args, "-ss"), slices.Index(args, "-i")
			if at < 0 || input < 0 || args[at+1] != seconds(cut.StartMS) || filepath.Base(args[input+1]) != "source-"+cut.SourceID+".mp4" {
				t.Fatalf("cut %d was made from %v", i, args)
			}
		}
	})
	t.Run("sampling", func(t *testing.T) {
		var downloads []string
		err := a.WithWorkspace(t.Context(), "alternating-sampling", func(ws clip.MediaWorkspace) error {
			load, release := countingLoader(ws, sources, &downloads)
			grounds, err := r.SampleGrounds(t.Context(), ws, plan, sources, load)
			if err == nil && len(grounds) != 2 {
				err = fmt.Errorf("measured %d grounds, want both captions'", len(grounds))
			}
			return errors.Join(err, release())
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(downloads, want) {
			t.Fatalf("the sampling downloaded %v", downloads)
		}
	})
}
