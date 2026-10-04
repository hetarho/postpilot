package media

import (
	"fmt"
	"slices"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func measurementCalls(runner *fakeRunner) int {
	n := 0
	for _, c := range runner.calls {
		if slices.Contains(c.Args, "--query-all") {
			n++
		}
	}
	return n
}

// A browser export asks for its caption frames and its assets many times over
// at one revision and draft. Those reads share one layout, so only the first
// measures the clip; another revision or another draft is laid out afresh, a
// read that names no layout is laid out every time, and only the eight most
// recently used layouts are kept.
func TestABrowserRendersReadsShareOneLayout(t *testing.T) {
	_, r, runner := previewMeasured(t, "vertical")
	plan, _ := framesPlan(t, "word-pop")
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "fp", Info: clip.MediaInfo{DurationMS: 30000, Width: 1920, Height: 1080}}}
	cfg := clip.DefaultGenerationConfig(clip.Environment{}).Preview
	read := func(key *clip.PreviewLayoutKey, frames bool) int {
		t.Helper()
		before := measurementCalls(runner)
		ctx := t.Context()
		if key != nil {
			ctx = clip.WithPreviewLayout(ctx, *key)
		}
		var err error
		if frames {
			_, err = r.PrepareGroundedCaptionFrames(ctx, plan, sources, []clip.SampledGround{}, "caption/cut", 0, cfg)
		} else {
			_, err = r.PrepareGroundedPreview(ctx, plan, sources, []clip.SampledGround{}, nil, 0, cfg)
		}
		if err != nil {
			t.Fatal(err)
		}
		return measurementCalls(runner) - before
	}
	render := &clip.PreviewLayoutKey{Render: "render", Revision: 3, Draft: "draft"}
	if read(render, true) == 0 {
		t.Fatal("the first read measured nothing")
	}
	if n := read(render, true) + read(render, false); n != 0 {
		t.Fatalf("the same render revision and draft was laid out again: %d measurements", n)
	}
	for _, other := range []*clip.PreviewLayoutKey{{Render: "render", Revision: 4, Draft: "draft"}, {Render: "render", Revision: 3, Draft: "edited"}} {
		if read(other, true) == 0 {
			t.Fatalf("%+v was answered from another layout", *other)
		}
	}
	if read(nil, true) == 0 || read(nil, true) == 0 {
		t.Fatal("a read naming no layout was answered from one")
	}
	// The render is the most recently used again: seven newer layouts keep it,
	// an eighth pushes it out.
	read(render, true)
	for i := range 7 {
		read(&clip.PreviewLayoutKey{Render: fmt.Sprintf("other-%d", i), Revision: 1, Draft: "draft"}, true)
	}
	if n := read(render, false); n != 0 {
		t.Fatalf("a layout among the eight most recent was dropped: %d measurements", n)
	}
	for i := range 8 {
		read(&clip.PreviewLayoutKey{Render: fmt.Sprintf("newer-%d", i), Revision: 1, Draft: "draft"}, true)
	}
	if read(render, true) == 0 {
		t.Fatal("more than eight layouts were kept")
	}
}
