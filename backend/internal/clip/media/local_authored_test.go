package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// An opt-in harness that renders ONE hand-written plan through the product's own
// renderer: same layout, same encode profile, same bundled fonts. It exists so a
// clip can be produced and reviewed locally without a model call, a database or
// the API. It is not a gate and asserts nothing about the pixels.
//
// The binaries and fonts come from the image the service ships, so the output is
// the one production would make:
//
//	cid=$(docker create postpilot:gate-runtime)
//	for b in ffmpeg ffprobe resvg; do docker cp $cid:/usr/local/bin/$b bin/$b; done
//	docker rm $cid
//	cd backend && CLIP_LOCAL_PLAN=/path/plan.json \
//	  CLIP_LOCAL_ORIGINALS=/path/to/originals CLIP_LOCAL_OUTPUT=/path/to/output \
//	  CLIP_FFMPEG_PATH=$PWD/../bin/ffmpeg CLIP_FFPROBE_PATH=$PWD/../bin/ffprobe \
//	  CLIP_RESVG_PATH=$PWD/../bin/resvg \
//	  CLIP_FONT_PATH=$PWD/assets/fonts/wantedsans/WantedSansVariable.ttf \
//	  CLIP_FONT_PAPERLOGY_PATH=$PWD/assets/fonts/paperlogy/Paperlogy-8ExtraBold.ttf \
//	  CLIP_FONT_JUA_PATH=$PWD/assets/fonts/jua/Jua-Regular.ttf \
//	  CLIP_FONT_NANUM_MYEONGJO_PATH=$PWD/assets/fonts/nanummyeongjo/NanumMyeongjo-Regular.ttf \
//	  CLIP_FONT_NANUM_MYEONGJO_BOLD_PATH=$PWD/assets/fonts/nanummyeongjo/NanumMyeongjo-ExtraBold.ttf \
//	  go test ./internal/clip/media/ -run TestLocalAuthoredClip -timeout 3600s -v
//
// testdata/local-plan.example.json is a complete plan to copy and edit. Budget
// the wait: a 32 s clip of 11 cuts took 12 minutes with static caption styles
// and 23 with two sequence-rendered ones, because every process this package
// runs passes through one global semaphore of width 1 (workspace.go).
type localPlan struct {
	Ratio          string `json:"ratio"`
	DurationMS     int    `json:"durationMS"`
	Disclosure     string `json:"disclosure"`
	HideDisclosure bool   `json:"hideDisclosure"`
	Accent         string `json:"accent"`
	Intro          string `json:"intro"`
	Outro          string `json:"outro"`
	// The outline the clip is written against, and the values its fields take:
	// the intro, outro and badge entries it declares are what the regions draw,
	// each line in the order it is written (CDS-73). Empty is the no-template
	// document, which draws no region at all (CLIP-5).
	Body          string            `json:"body"`
	Values        map[string]string `json:"values"`
	CaptionStyles []string          `json:"captionStyles"`
	Cuts          []localCut        `json:"cuts"`
}

type localCut struct {
	File string `json:"file"`
	// Source time, not output time. TransitionMS belongs to the cut it leads
	// into: 0 hard cut, 200 fade, 300 fade through black, nothing else.
	StartMS      int         `json:"startMS"`
	EndMS        int         `json:"endMS"`
	TransitionMS int         `json:"transitionMS"`
	FocalX       float64     `json:"focalX"`
	FocalY       float64     `json:"focalY"`
	Copies       []localCopy `json:"copies"`
}

type localCopy struct {
	Text string `json:"text"`
	// One id from design.CaptionStyles, which must also appear in the plan's
	// captionStyles. A sequence-rendered style (ember, neon, glitch, pop …)
	// draws every frame and dominates the render; a static one (bold, film,
	// keynote) costs a single raster.
	Style   string `json:"style"`
	Anchor  string `json:"anchor"`
	Align   string `json:"align"`
	Keyword string `json:"keyword"`
	StartMS int    `json:"startMS"`
	EndMS   int    `json:"endMS"`
}

// localFailure names what refused the plan. renderFailure alone reports the
// substage, which for a layout refusal is always "render_layout" and says
// nothing about WHICH element the grammar or the geometry rejected; a Problem
// carries that, so it is unwrapped here.
func localFailure(err error) error {
	var p *composition.Problem
	if errors.As(err, &p) {
		return fmt.Errorf("%w (element=%q line=%d reason=%s)", renderFailure(err), p.ElementID, p.Line, p.Reason)
	}
	// renderFailure already spells out the cause chain.
	return renderFailure(err)
}

func localMediaConfig(t *testing.T) clip.MediaConfig {
	t.Helper()
	return clip.DefaultMediaConfig(clip.Environment{
		WorkRoot:       filepath.Join(t.TempDir(), "work"),
		FFmpegPath:     path("CLIP_FFMPEG_PATH", "/usr/local/bin/ffmpeg"),
		FFprobePath:    path("CLIP_FFPROBE_PATH", "/usr/local/bin/ffprobe"),
		WorkStaleAge:   time.Hour,
		MediaTimeout:   20 * time.Minute,
		SourceBatchTTL: 6 * time.Hour,
		PutTTL:         10 * time.Minute,
	})
}

func TestLocalAuthoredClip(t *testing.T) {
	spec, originals, output := os.Getenv("CLIP_LOCAL_PLAN"), os.Getenv("CLIP_LOCAL_ORIGINALS"), os.Getenv("CLIP_LOCAL_OUTPUT")
	if spec == "" || originals == "" || output == "" {
		t.Skip("requires a local plan, originals and output directory")
	}
	body, err := os.ReadFile(spec)
	if err != nil {
		t.Fatal(err)
	}
	var lp localPlan
	if err := json.Unmarshal(body, &lp); err != nil {
		t.Fatal(err)
	}
	cfg := localMediaConfig(t)
	cfg.OperationTimeout = 20 * time.Minute
	a, err := New(cfg, originalsRunner{t: t, runner: ExecRunner{StdoutLimit: cfg.StdoutLimit, StderrLimit: cfg.StderrLimit, WaitDelay: cfg.WaitDelay}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, renderConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "local-authored", func(ws clip.MediaWorkspace) error {
		plan := clip.EditPlan{
			Ratio: lp.Ratio, DurationMS: lp.DurationMS,
			Disclosure: lp.Disclosure, HideDisclosure: lp.HideDisclosure, Accent: lp.Accent,
			IntroPreset: lp.Intro, OutroPreset: lp.Outro, CaptionStyles: lp.CaptionStyles,
		}
		var sources []clip.RenderSource
		paths := map[string]string{}
		seen := map[string]string{}
		for _, c := range lp.Cuts {
			if _, ok := seen[c.File]; ok {
				continue
			}
			id := fmt.Sprintf("source-%02d", len(sources))
			local := filepath.Join(ws.Path, id+".mp4")
			if err := copyOriginal(filepath.Join(originals, c.File), local); err != nil {
				return err
			}
			info, err := a.Probe(t.Context(), ws, local)
			if err != nil {
				return err
			}
			sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: info})
			paths[id] = local
			seen[c.File] = id
		}
		// A cut reaching past its source is refused as plan_cut_range, but only
		// after every original has been copied and probed. Saying so here turns
		// a two-minute round trip into an immediate answer.
		byID := map[string]clip.RenderSource{}
		for _, source := range sources {
			byID[source.ID] = source
		}
		for i, c := range lp.Cuts {
			if source := byID[seen[c.File]]; c.EndMS > source.Info.DurationMS || c.StartMS >= c.EndMS {
				return fmt.Errorf("cut %d (%s) asks for %d-%d ms of a %d ms source", i, c.File, c.StartMS, c.EndMS, source.Info.DurationMS)
			}
		}
		for i, c := range lp.Cuts {
			id := seen[c.File]
			cut := clip.EditCut{
				ID: fmt.Sprintf("cut-%02d", i), SourceID: id, Fingerprint: id,
				StartMS: c.StartMS, EndMS: c.EndMS, TransitionMS: c.TransitionMS,
				Focal: clip.Point{X: c.FocalX, Y: c.FocalY},
			}
			for _, cp := range c.Copies {
				cut.Copies = append(cut.Copies, clip.Copy{
					Text: cp.Text, Style: cp.Style, Anchor: cp.Anchor, Align: cp.Align,
					Keyword: cp.Keyword, Accent: lp.Accent, StartMS: cp.StartMS, EndMS: cp.EndMS,
				})
			}
			plan.Cuts = append(plan.Cuts, cut)
		}
		// The plan is written into its outline here, the way a generation writes
		// it: each cut bound, the outline's fixed entries resolved over those
		// cuts, and every copy a caption on its own cut. The composition grammar
		// declares no style at all: the layout narrows the project's allowed set
		// by text.Owner alone, so every caption of a plan that leaves it empty
		// draws in the set's FIRST entry. Filling Owner.Style is not a workaround
		// — it is the one path a caption style is ever chosen on, the same one
		// step ② uses (CLIP-143, CDS-25).
		portable, err := localPortable(lp, plan, r.cfg.Composition)
		if err != nil {
			return localFailure(err)
		}
		plan.Portable = portable
		started := time.Now()
		result, err := r.Render(t.Context(), ws, plan, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
			for _, source := range sources {
				if source.ID == id {
					return consume(clip.MediaSource{SourceID: id, Fingerprint: source.Fingerprint, Info: source.Info, Path: paths[id]})
				}
			}
			return clip.ErrNotFound
		})
		if err != nil {
			return localFailure(err)
		}
		if err := os.MkdirAll(output, 0700); err != nil {
			return err
		}
		name := filepath.Join(output, "clip-result.mp4")
		_ = os.Remove(name)
		if err := copyOriginal(result.Path, name); err != nil {
			return err
		}
		// design.Verify is the LEGACY delivery check and reports a false
		// plan_layout_size here: a composition manifest leaves Cut and Copy at
		// zero, so its per-caption line count sees every caption in one slot.
		// It is logged beside the manifest rather than failing the render, so a
		// real violation can still be read off the elements.
		if verified := design.Verify(result.Manifest, plan.Ratio, plan.HideDisclosure); verified != nil {
			t.Logf("verify: %v", renderFailure(verified))
			if data, e := json.MarshalIndent(result.Manifest, "", " "); e == nil {
				_ = os.WriteFile(filepath.Join(output, "manifest.json"), data, 0600)
			}
		}
		data, err := json.MarshalIndent(result.Plan, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, "resolved-plan.json"), data, 0600); err != nil {
			return err
		}
		t.Logf("%d cuts, %d ms, %d bytes, elapsed=%s", len(plan.Cuts), result.Info.DurationMS, result.Bytes, time.Since(started))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// localPortable is the composition a generation would have written these cuts
// and copies into.
func localPortable(lp localPlan, plan clip.EditPlan, limits composition.Limits) (*clip.PortablePlan, error) {
	body := lp.Body
	if body == "" {
		body = clip.EmptyCompositionBody()
	}
	doc, problem := composition.Parse(body, limits)
	if problem != nil {
		return nil, problem
	}
	values := lp.Values
	if values == nil {
		values = map[string]string{}
	}
	portable := &clip.PortablePlan{Snapshot: clip.CompositionSnapshot{Version: clip.CompositionVersion, Body: body},
		Inputs: clip.CompositionInputs{Values: values, Items: map[string][]composition.Item{}}}
	for _, cut := range plan.Cuts {
		portable.Cuts = append(portable.Cuts, composition.Cut{ID: cut.ID, SourceID: cut.SourceID, StartMS: cut.StartMS, EndMS: cut.EndMS, TransitionMS: cut.TransitionMS, PlaybackRatePermille: cut.Rate()})
	}
	timeline, _, err := clip.ResolveSelectedComposition(doc, portable.Inputs, portable.Cuts, limits, 1<<20)
	if err != nil {
		return nil, err
	}
	for _, resolved := range timeline.Elements {
		if resolved.Element.Kind == "fixed" {
			portable.Elements = append(portable.Elements, clip.PortableText{Resolved: resolved, Scope: "context"})
		}
	}
	offset := 0
	for i, cut := range plan.Cuts {
		offset -= cut.TransitionMS
		for j, copy := range cut.Copies {
			if copy.Text == "" {
				continue
			}
			start, end := cut.CaptionWindow(j)
			id := fmt.Sprintf("copy-cut-%02d-%d", i, j)
			e := composition.Element{ID: id, Kind: "fixed", Role: "caption", Style: copy.Style, Position: copy.Anchor, Align: copy.Align, Basis: "cut", StartMS: &start, EndMS: &end, Parts: []composition.Part{{Literal: copy.Text}}}
			if copy.StartMS == 0 && copy.EndMS == 0 {
				e.StartMS, e.EndMS = nil, nil
			}
			portable.Elements = append(portable.Elements, clip.PortableText{Accent: copy.Accent, Keyword: copy.Keyword, Scope: "scene", Owner: clip.OwnerCaption{Style: copy.Style},
				Resolved: composition.ResolvedElement{InstanceID: id, CutID: cut.ID, Element: e, Text: copy.Text, StartMS: offset + start, EndMS: offset + end, AuthoredTiming: copy.StartMS != 0 || copy.EndMS != 0},
				Evidence: []clip.SourceEvidence{{SourceID: cut.SourceID, Fingerprint: cut.Fingerprint, StartMS: cut.StartMS, EndMS: cut.EndMS}}})
		}
		offset += cut.OutputDurationMS()
	}
	return portable, nil
}

type originalsRunner struct {
	t      *testing.T
	runner Runner
}

func (r originalsRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	start := time.Now()
	output := filepath.Base(command.Args[len(command.Args)-1])
	r.t.Logf("command start: %s %s", filepath.Base(command.Binary), output)
	data, err := r.runner.Run(ctx, command)
	peak, _ := os.ReadFile("/sys/fs/cgroup/memory.peak")
	r.t.Logf("command end: %s %s elapsed=%s memory_peak=%s error=%v", filepath.Base(command.Binary), output, time.Since(start).Round(time.Millisecond), peak, err)
	return data, err
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
