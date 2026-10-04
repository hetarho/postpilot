package worker_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/mediacodec"
	"github.com/postpilot/backend/internal/clip/worker"
)

// sourceRenderer stands in for the renderer's cut loop: it loads the plan's
// originals a source at a time, as the composition visits its cuts, and writes
// a result the render stage accepts.
type sourceRenderer struct {
	loaded []string
	infos  []clip.MediaInfo
}

func (r *sourceRenderer) Render(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, _ []clip.RenderSource, load clip.RenderSourceLoader) (clip.RenderedVideo, error) {
	groups, order := map[string]int{}, []string{}
	for _, cut := range plan.Cuts {
		if groups[cut.SourceID] == 0 {
			order = append(order, cut.SourceID)
		}
		groups[cut.SourceID]++
	}
	for _, id := range order {
		for range groups[id] {
			err := load(ctx, id, func(m clip.MediaSource) error {
				data, err := os.ReadFile(m.Path)
				if err != nil {
					return err
				}
				r.loaded, r.infos = append(r.loaded, string(data)), append(r.infos, m.Info)
				return nil
			})
			if err != nil {
				return clip.RenderedVideo{}, err
			}
		}
	}
	path := filepath.Join(ws.Path, "clip-result.mp4")
	if err := os.WriteFile(path, []byte("rendered"), 0600); err != nil {
		return clip.RenderedVideo{}, err
	}
	return clip.RenderedVideo{Path: path, Bytes: 8, Info: clip.MediaInfo{Width: 1080, Height: 1920, DurationMS: plan.DurationMS, FrameRateNumerator: 30, FrameRateDenominator: 1,
		PixelFormat: "yuv420p", SampleAspectRatio: "1:1", DecodedFrames: plan.DurationMS * 30 / 1000, Streams: []clip.MediaStream{{Kind: "video", Codec: "h264", Profile: "High"}}}}, nil
}

// alternating is twelve cuts over four originals, s0 s1 s2 s3 s0 s1 …, and a
// fifth original the task names but no cut draws.
func alternating() ([]clip.MediaTaskSource, map[string][]byte, []int) {
	info := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 15000}
	var sources []clip.MediaTaskSource
	originals := map[string][]byte{}
	for i := range 5 {
		id := fmt.Sprintf("s%d", i)
		data := []byte("original " + id)
		sources = append(sources, taskOriginal(id, data, info))
		originals["source/"+id] = data
	}
	draws := make([]int, 12)
	for i := range draws {
		draws[i] = i % 4
	}
	return sources, originals, draws
}

// A server render downloads each original once and verifies it inside that one
// download, the first time a cut loads it: the probe the delivered file is
// judged on, then the cuts. An original no cut draws is still downloaded,
// verified and reported, once, after the render (ARCH-48).
func TestARenderDownloadsAndVerifiesEachOriginalOnce(t *testing.T) {
	sources, originals, draws := alternating()
	probed := clip.MediaInfo{Width: 1280, Height: 720, DurationMS: 15000, DecodedFrames: 450, DecodedDurationMS: 15000}
	media := &taskMedia{dir: t.TempDir(), info: probed}
	transfer := &taskTransfer{originals: originals}
	renderer := &sourceRenderer{}
	raw, err := worker.NewExecutor(media, renderer, transfer, clip.DefaultMediaConfig(clip.Environment{})).Execute(t.Context(), planWork(t, clip.MediaRender, sources, draws))
	if err != nil {
		t.Fatal(err)
	}
	result, err := mediacodec.DecodeResult(raw)
	if err != nil || len(result.Outputs) != 1 || len(transfer.uploaded) != 1 {
		t.Fatalf("the render reported %+v %v", result, err)
	}
	for _, s := range sources {
		if n := transfer.downloads["source/"+s.ID]; n != 1 {
			t.Errorf("%s was downloaded %d times, want once", s.ID, n)
		}
	}
	if media.calls != len(sources) {
		t.Errorf("%d probes for %d originals", media.calls, len(sources))
	}
	if len(result.Sources) != len(sources) {
		t.Fatalf("%d verified sources reported for %d", len(result.Sources), len(sources))
	}
	for i, s := range sources {
		if v := result.Sources[i]; v.ID != s.ID || v.Fingerprint != s.Fingerprint || !reflect.DeepEqual(v.Info, probed) {
			t.Errorf("source %d reported as %+v", i, v)
		}
	}
	want := []string{}
	for _, id := range []string{"s0", "s1", "s2", "s3"} {
		want = append(want, "original "+id, "original "+id, "original "+id)
	}
	if !reflect.DeepEqual(renderer.loaded, want) {
		t.Fatalf("the cuts read %v", renderer.loaded)
	}
	for _, info := range renderer.infos {
		if !reflect.DeepEqual(info, probed) {
			t.Fatalf("a cut was handed %+v, not the probed original", info)
		}
	}
	if dirs, _ := os.ReadDir(media.dir); len(dirs) > 0 {
		t.Fatal("workspace leaked")
	}
}

// An original whose decode disagrees with what the task froze still fails the
// render as invalid media, and nothing is uploaded.
func TestARenderRefusesAnOriginalItsProbeDisagreesWith(t *testing.T) {
	sources, originals, draws := alternating()
	media := &taskMedia{dir: t.TempDir(), info: clip.MediaInfo{Width: 1280, Height: 700, DurationMS: 15000}}
	transfer := &taskTransfer{originals: originals}
	raw, err := worker.NewExecutor(media, &sourceRenderer{}, transfer, clip.DefaultMediaConfig(clip.Environment{})).Execute(t.Context(), planWork(t, clip.MediaRender, sources, draws))
	if !errors.Is(err, clip.ErrInvalidMedia) || raw != "" {
		t.Fatalf("a mismatched original rendered: %q %v", raw, err)
	}
	if len(transfer.uploaded) != 0 {
		t.Fatal("a refused render uploaded a result")
	}
	if dirs, _ := os.ReadDir(media.dir); len(dirs) > 0 {
		t.Fatal("workspace leaked")
	}
}

// A sampling job downloads only the originals its reads need, once each, and
// decodes none of them in full: the task's frozen facts stand for every
// original, the ones it never reads included (CLIP-192).
func TestASampleStageDownloadsOnlyWhatItReadsAndProbesNothing(t *testing.T) {
	sources, originals, draws := alternating()
	sources = sources[:4]
	media := &taskMedia{dir: t.TempDir(), info: clip.MediaInfo{Width: 1, Height: 1, DurationMS: 1}}
	transfer := &taskTransfer{originals: originals}
	sampler := &groundSampler{reads: []string{"s1", "s3"}, grounds: []clip.SampledGround{}}
	raw, err := worker.NewExecutor(media, sampler, transfer, clip.DefaultMediaConfig(clip.Environment{})).Execute(t.Context(), planWork(t, clip.MediaSample, sources, draws))
	if err != nil {
		t.Fatal(err)
	}
	result, err := mediacodec.DecodeResult(raw)
	if err != nil || len(result.Sources) != len(sources) {
		t.Fatalf("the sample stage reported %+v %v", result, err)
	}
	for i, s := range sources {
		want := 0
		if s.ID == "s1" || s.ID == "s3" {
			want = 1
		}
		if n := transfer.downloads["source/"+s.ID]; n != want {
			t.Errorf("%s was downloaded %d times, want %d", s.ID, n, want)
		}
		if v := result.Sources[i]; v.ID != s.ID || v.Fingerprint != s.Fingerprint || !reflect.DeepEqual(v.Info, s.Info) {
			t.Errorf("source %d reported as %+v", i, v)
		}
	}
	if media.calls != 0 {
		t.Fatalf("a sample stage probed %d originals", media.calls)
	}
	if !reflect.DeepEqual(sampler.loaded, []string{"original s1", "original s3"}) {
		t.Fatalf("the sampler read %v", sampler.loaded)
	}
	if dirs, _ := os.ReadDir(media.dir); len(dirs) > 0 {
		t.Fatal("workspace leaked")
	}
}
