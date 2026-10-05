package media

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func ratedCut(id, source string, start, end, rate int) clip.Cut {
	return clip.Cut{ID: id, SourceID: source, Fingerprint: source, StartMS: start, EndMS: end,
		PlaybackRatePermille: rate, Focal: clip.Point{X: .5, Y: .5}, Volume: volume(1)}
}

// A fixed rate becomes timestamp scaling and ordinary frame-rate conversion —
// nothing else. The source trim keeps the cut's ORIGINAL range, and the output
// is the transformed length at 30 fps (CLIP-98, CDS-62, CDS-68).
func TestEveryRateScalesTimestampsAndKeepsTheSourceRange(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("vertical")
	for _, rate := range clip.PlaybackRates() {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			span := 3000 * rate / clip.RateUnitPermille
			cut := ratedCut("cut", "source", 1000, 1000+span, rate)
			frames := 3000 * 30 / 1000
			video := bareFootageGraph(r.cfg, canvas, cut, frames)
			if !strings.Contains(video, fmt.Sprintf("trim=duration=%s,", seconds(span))) {
				t.Fatalf("the source trim is not the cut's own range: %s", video)
			}
			want := "setpts=PTS-STARTPTS,fps=30"
			if rate != clip.RateUnitPermille {
				want = fmt.Sprintf("setpts=(PTS-STARTPTS)*1000/%d,fps=30", rate)
			}
			if !strings.Contains(video, want) {
				t.Fatalf("missing %q in %s", want, video)
			}
			if !strings.Contains(video, fmt.Sprintf("trim=end_frame=%d", frames)) {
				t.Fatalf("the frame budget is not the transformed length: %s", video)
			}
			audio := bareAudioGraph(r.cfg, cut, true, frames)
			if !strings.Contains(audio, fmt.Sprintf("atrim=duration=%s", seconds(span))) {
				t.Fatalf("audio was trimmed on output time: %s", audio)
			}
			tempo := ",atempo="
			if rate == clip.RateUnitPermille {
				if strings.Contains(audio, tempo) {
					t.Fatalf("1x asked for a tempo change: %s", audio)
				}
			} else if !strings.Contains(audio, fmt.Sprintf("%s%.6f", tempo, float64(rate)/float64(clip.RateUnitPermille))) {
				t.Fatalf("audio is not pitch-preserved at the cut's rate: %s", audio)
			}
			// The exact transformed sample count, whatever the rate.
			if !strings.Contains(audio, fmt.Sprintf("atrim=end_sample=%d", frames*r.cfg.AudioRate/r.cfg.FPS)) {
				t.Fatalf("audio length is not the transformed one: %s", audio)
			}
			for _, forbidden := range []string{"minterpolate", "framerate=", "tblend", "reverse", "areverse", "freezedetect", "loop=loop=-1", "setpts=N/"} {
				if strings.Contains(video+audio, forbidden) {
					t.Fatalf("the graph contains %q: %s", forbidden, video+audio)
				}
			}
		})
	}
}

// The frame budget is cumulative over the TRANSFORMED timeline, so individually
// rounded cuts cannot drift away from the captions and transitions placed on it.
func TestFrameBudgetIsCumulativeOverTheTransformedTimeline(t *testing.T) {
	plan := clip.EditPlan{Cuts: []clip.Cut{
		ratedCut("a", "s", 0, 5000, 2000),   // 2500 ms of output
		ratedCut("b", "s", 5000, 6667, 500), // 3334 ms of output
		ratedCut("c", "s", 7000, 10000, 1000),
	}}
	frames := newCutTimeline(30, plan).frames
	total, elapsed := 0, 0
	for i, c := range plan.Cuts {
		elapsed += c.OutputDurationMS()
		total += frames[i]
		if want := elapsed * 30 / 1000; total < want-1 || total > want+1 {
			t.Fatalf("cut %d ends at %d frames, want about %d", i, total, want)
		}
	}
}

// A slow rate is admitted only where the ORIGINAL's own verified cadence can
// still produce 30 fps without invented frames, and an unsuitable one is named
// before FFmpeg starts rather than replaced by 1x (CDS-68, CLIP-99).
func TestSlowPlaybackIsRecheckedAgainstVerifiedCadence(t *testing.T) {
	source := func(numerator, denominator, frames, decodedMS int) clip.RenderSource {
		return clip.RenderSource{ID: "s", Fingerprint: "s", Info: clip.MediaInfo{
			DurationMS: decodedMS, Width: 1920, Height: 1080,
			FrameRateNumerator: numerator, FrameRateDenominator: denominator,
			CadenceVerified: true, DecodedFrames: frames, DecodedDurationMS: decodedMS}}
	}
	for _, tc := range []struct {
		name    string
		source  clip.RenderSource
		rate    int
		allowed bool
	}{
		{"60 fps at 0.5x", source(60, 1, 600, 10000), 500, true},
		{"40 fps at 0.75x", source(40, 1, 400, 10000), 750, true},
		{"40 fps at 0.5x", source(40, 1, 400, 10000), 500, false},
		{"30 fps at 0.75x", source(30, 1, 300, 10000), 750, false},
		{"30 fps at 1x", source(30, 1, 300, 10000), 1000, true},
		{"30 fps sped up", source(30, 1, 300, 10000), 2000, true},
		{"unmeasured source", source(60, 1, 0, 0), 500, false},
		{"undeclared cadence", source(0, 0, 600, 10000), 500, false},
		// Variable-rate footage: the container averages 60 fps but the decode
		// counted far fewer frames, so nothing is proved and slow is refused.
		{"variable rate", source(60, 1, 200, 10000), 500, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			plan := clip.EditPlan{Cuts: []clip.Cut{ratedCut("cut", "s", 0, 4000, tc.rate)}}
			err := clip.RefuseUnrenderableRates(plan, []clip.RenderSource{tc.source})
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v, err=%v", tc.allowed, err)
			}
			if tc.allowed {
				return
			}
			var code interface{ OutputValidationCode() string }
			if !errors.As(err, &code) || code.OutputValidationCode() != "plan_cut_rate" {
				t.Fatalf("untyped refusal: %v", err)
			}
			// The cut keeps the rate it asked for; nothing substitutes 1x.
			if plan.Cuts[0].Rate() != tc.rate {
				t.Fatalf("the rate was changed to %d", plan.Cuts[0].Rate())
			}
		})
	}
	// A cut whose source is not in the manifest proves no cadence at all.
	plan := clip.EditPlan{Cuts: []clip.Cut{ratedCut("cut", "missing", 0, 4000, 1000)}}
	if clip.RefuseUnrenderableRates(plan, nil) == nil {
		t.Fatal("an unproved source was rendered")
	}
}

// One golden graph per rate, so a future edit to the rate chain shows up as a
// diff rather than as a silently different clip.
func TestRateFilterGoldens(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("vertical")
	for _, rate := range clip.PlaybackRates() {
		span := 3000 * rate / clip.RateUnitPermille
		cut := ratedCut("cut", "source", 1000, 1000+span, rate)
		name := fmt.Sprintf("rate-%d", rate)
		golden(t, name+".filter", bareFootageGraph(r.cfg, canvas, cut, 90)+"\n")
		golden(t, name+".audio.filter", bareAudioGraph(r.cfg, cut, true, 90)+"\n")
	}
	// A disabled or silent cut is the same length of explicit silence.
	golden(t, "rate-silent.audio.filter", bareAudioGraph(r.cfg, ratedCut("cut", "source", 0, 6000, 2000), false, 90)+"\n")
}

// An unrenderable rate stops the whole render before a single FFmpeg process
// starts, so an unsuitable transform costs no media work and leaves the
// validated assembly exactly as it was (CLIP-97, CLIP-99).
func TestAnUnrenderableRateFailsBeforeFFmpegStarts(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, Command) ([]byte, error) {
		t.Fatal("a refused rate reached FFmpeg")
		return nil, nil
	}}
	a := newAdapter(t, runner)
	r := testRenderer(t, a)
	// 30 fps footage cannot be slowed without inventing frames.
	source := clip.RenderSource{ID: "s", Fingerprint: "s", Info: clip.MediaInfo{DurationMS: 20000, Width: 1920, Height: 1080,
		FrameRateNumerator: 30, FrameRateDenominator: 1, CadenceVerified: true, DecodedFrames: 600, DecodedDurationMS: 20000}}
	plan := clip.EditPlan{Ratio: "vertical", DurationMS: 16000, Cuts: []clip.Cut{ratedCut("cut", "s", 0, 8000, 500)}}
	before := plan
	err := a.WithWorkspace(t.Context(), "refused-rate", func(ws clip.MediaWorkspace) error {
		_, err := r.Render(t.Context(), ws, plan, []clip.RenderSource{source}, func(context.Context, string, func(clip.MediaSource) error) error {
			t.Fatal("a refused rate loaded original media")
			return nil
		})
		return err
	})
	var code interface{ OutputValidationCode() string }
	if err == nil || !errors.As(err, &code) || code.OutputValidationCode() != "plan_cut_rate" {
		t.Fatalf("untyped or missing refusal: %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("%d media commands ran for a refused plan", len(runner.calls))
	}
	if !reflect.DeepEqual(plan, before) {
		t.Fatal("the refused plan was rewritten")
	}
}

// The whole delivered graph, at every rate, contains no filter that could
// invent a frame or play footage backwards (CLIP-28).
func TestDeliveredGraphsContainNoProhibitedFilter(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("vertical")
	prohibited := regexp.MustCompile(`minterpolate|tblend|framerate=|[^a]reverse|areverse|freezedetect|setpts=N/|mci|blend=all_mode`)
	for _, rate := range clip.PlaybackRates() {
		cut := ratedCut("cut", "s", 0, 3000*rate/clip.RateUnitPermille, rate)
		graphs := []string{
			bareFootageGraph(r.cfg, canvas, cut, 90),
			bareAudioGraph(r.cfg, cut, true, 90),
		}
		for _, graph := range graphs {
			if prohibited.MatchString(graph) {
				t.Fatalf("rate %d graph contains a prohibited filter: %s", rate, graph)
			}
		}
	}
}
