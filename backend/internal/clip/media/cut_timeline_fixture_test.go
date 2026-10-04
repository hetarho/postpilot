package media

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// A browser export composes its frames from the cuts the server's timeline
// names, so a browser and a server export of one plan show the same frames
// (CLIP-192). This fixture is that timeline for a hard cut, a 200 ms dissolve, a
// 300 ms fade through black and rate-changed cuts: for every output frame, the
// cut or cuts it is made of, the source instant each shows and the weight xfade
// gives it. The frontend copy stays byte-identical and the browser's drawing
// reproduces it. Re-record with UPDATE_CUT_TIMELINE=1.
const (
	cutTimelineFixture       = "testdata/cut-timeline.json"
	cutTimelineFixtureMirror = "../../../../frontend/src/entities/clip-preview/model/cut-timeline.fixture.json"
)

type timelineFixtureCut struct {
	ID                   string `json:"id"`
	StartMS              int    `json:"startMs"`
	EndMS                int    `json:"endMs"`
	TransitionMS         int    `json:"transitionMs"`
	PlaybackRatePermille int    `json:"playbackRatePermille"`
}

// timelineFixtureLayer is one cut's frame in an output frame, in drawing order:
// the outgoing cut before the incoming one inside a transition.
type timelineFixtureLayer struct {
	Cut      int     `json:"cut"`
	SourceMS int     `json:"sourceMs"`
	Weight   float64 `json:"weight"`
}

type timelineFixtureCase struct {
	Name   string                   `json:"name"`
	FPS    int                      `json:"fps"`
	Cuts   []timelineFixtureCut     `json:"cuts"`
	Frames [][]timelineFixtureLayer `json:"frames"`
}

// takeSourceMS is the instant of its original a cut's own frame shows: the
// bare footage chain scales the decoded timestamps by the cut's rate and keeps
// the frame nearest each output frame (rateChain), so local frame k is k output
// frames of source time at that rate after the cut's start.
func takeSourceMS(cut clip.Cut, local, fps int) int {
	return cut.StartMS + int(math.Round(float64(local*cut.Rate())/float64(fps)))
}

func r6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

func cutTimelineFixtureCases(t *testing.T) []byte {
	t.Helper()
	fps := clip.DefaultRenderConfig(clip.Environment{}).FPS
	plans := []struct {
		name string
		cuts []clip.Cut
	}{
		{"hard-cut", []clip.Cut{{ID: "a", StartMS: 0, EndMS: 1000}, {ID: "b", StartMS: 500, EndMS: 1500}}},
		{"fade-200", []clip.Cut{{ID: "a", StartMS: 0, EndMS: 1000}, {ID: "b", StartMS: 0, EndMS: 1000, TransitionMS: design.Transition.FadeMS}}},
		{"fade-through-black-300", []clip.Cut{{ID: "a", StartMS: 0, EndMS: 1000}, {ID: "b", StartMS: 0, EndMS: 1000, TransitionMS: design.Transition.BlackMS}}},
		{"rate-changed", []clip.Cut{
			{ID: "a", StartMS: 1000, EndMS: 5000, PlaybackRatePermille: 2000},
			{ID: "b", StartMS: 0, EndMS: 1000, PlaybackRatePermille: 750},
			{ID: "c", StartMS: 0, EndMS: 1500, PlaybackRatePermille: 1500, TransitionMS: design.Transition.FadeMS},
		}},
	}
	var cases []timelineFixtureCase
	for _, p := range plans {
		plan := clip.EditPlan{Cuts: p.cuts}
		c := timelineFixtureCase{Name: p.name, FPS: fps}
		for _, cut := range plan.Cuts {
			c.Cuts = append(c.Cuts, timelineFixtureCut{ID: cut.ID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
		}
		timeline := newCutTimeline(fps, plan)
		for frame := range timeline.total {
			footage, ok := timeline.at(frame)
			if !ok {
				t.Fatalf("%s: frame %d of %d has no footage", p.name, frame, timeline.total)
			}
			outgoing, incoming := footage.weights()
			layers := []timelineFixtureLayer{}
			if footage.blended() {
				take := footage.outgoing
				layers = append(layers, timelineFixtureLayer{Cut: take.cut, SourceMS: takeSourceMS(plan.Cuts[take.cut], take.local, fps), Weight: r6(outgoing)})
			}
			take := footage.incoming
			layers = append(layers, timelineFixtureLayer{Cut: take.cut, SourceMS: takeSourceMS(plan.Cuts[take.cut], take.local, fps), Weight: r6(incoming)})
			c.Frames = append(c.Frames, layers)
		}
		cases = append(cases, c)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(cases); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// The two copies stay byte-identical; the weights are compared within a
// millionth because arm64 fuses the smoothstep's multiply-adds and amd64 does
// not, and the frontend matches them at 1/255.
func TestCutTimelineFixtureIsCurrent(t *testing.T) {
	want := cutTimelineFixtureCases(t)
	paths := []string{cutTimelineFixture, cutTimelineFixtureMirror}
	if os.Getenv("UPDATE_CUT_TIMELINE") == "1" {
		for _, path := range paths {
			if err := os.WriteFile(path, want, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	var recorded []byte
	for _, path := range paths {
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if recorded != nil && !bytes.Equal(got, recorded) {
			t.Fatalf("%s differs from %s; re-record with UPDATE_CUT_TIMELINE=1", path, paths[0])
		}
		recorded = got
	}
	var got, laid []timelineFixtureCase
	if err := json.Unmarshal(recorded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(want, &laid); err != nil {
		t.Fatal(err)
	}
	stale := len(got) != len(laid)
	for i := 0; !stale && i < len(laid); i++ {
		g, w := got[i], laid[i]
		stale = g.Name != w.Name || g.FPS != w.FPS || len(g.Cuts) != len(w.Cuts) || len(g.Frames) != len(w.Frames)
		for j := 0; !stale && j < len(w.Cuts); j++ {
			stale = g.Cuts[j] != w.Cuts[j]
		}
		for f := 0; !stale && f < len(w.Frames); f++ {
			stale = len(g.Frames[f]) != len(w.Frames[f])
			for l := 0; !stale && l < len(w.Frames[f]); l++ {
				a, b := g.Frames[f][l], w.Frames[f][l]
				stale = a.Cut != b.Cut || a.SourceMS != b.SourceMS || math.Abs(a.Weight-b.Weight) > 2e-6
			}
		}
	}
	if stale {
		t.Fatalf("%s is stale; re-record with UPDATE_CUT_TIMELINE=1", paths[0])
	}
}

// The fixture covers what it claims: every kind of join, both sides of each
// transition at weights that move, and cuts whose source runs at another rate.
func TestCutTimelineFixtureCoversEveryJoin(t *testing.T) {
	var cases []timelineFixtureCase
	if err := json.Unmarshal(cutTimelineFixtureCases(t), &cases); err != nil {
		t.Fatal(err)
	}
	blended := map[string]int{}
	for _, c := range cases {
		for _, layers := range c.Frames {
			if len(layers) == 2 {
				blended[c.Name]++
			}
			sum := 0.0
			for _, layer := range layers {
				sum += layer.Weight
			}
			if sum > 1+1e-6 || len(layers) == 1 && layers[0].Weight != 1 {
				t.Fatalf("%s mixes %+v", c.Name, layers)
			}
		}
	}
	if blended["hard-cut"] != 0 || blended["fade-200"] != 6 || blended["fade-through-black-300"] != 9 || blended["rate-changed"] != 6 {
		t.Fatalf("blended frames %v", blended)
	}
	// Part-way through the fade through black the outgoing cut has gone and
	// the incoming one has barely arrived: the frame is mostly the black xfade
	// passes through, which no dissolve shows.
	black := false
	for _, layers := range cases[2].Frames {
		black = black || len(layers) == 2 && layers[0].Weight == 0 && layers[1].Weight < .1
	}
	if !black {
		t.Fatal("the fade through black never shows its black")
	}
}
