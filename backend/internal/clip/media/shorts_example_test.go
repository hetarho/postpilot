package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Opt-in design review with local footage and a file-only preset catalog.
func TestShortsSVGExample(t *testing.T) { renderShortsExample(t, false) }

// Opt-in comparison: existing plate, lightweight sentence, then rapid phrases.
func TestCaptionPaceExample(t *testing.T) {
	if os.Getenv("CLIP_EXAMPLE_PACE") != "1" {
		t.Skip("opt-in pace comparison")
	}
	renderShortsExample(t, true)
}
func renderShortsExample(t *testing.T, paceExample bool) {
	root, output, assets := os.Getenv("CLIP_EXAMPLE_ORIGINALS"), os.Getenv("CLIP_EXAMPLE_OUTPUT"), os.Getenv("CLIP_EXAMPLE_ASSETS")
	if root == "" || output == "" || assets == "" {
		t.Skip("requires local originals, output and example asset directories")
	}
	files, err := filepath.Glob(filepath.Join(root, "*.mp4"))
	if err != nil || len(files) != 8 {
		t.Fatal("expected eight originals", err)
	}
	cfg := mediaConfig(t)
	cfg.OperationTimeout = 15 * time.Minute
	a, err := New(cfg, originalsRunner{t: t, runner: exampleRunner{output: output, runner: ExecRunner{StdoutLimit: cfg.StdoutLimit, StderrLimit: cfg.StderrLimit, WaitDelay: cfg.WaitDelay}}})
	if err != nil {
		t.Fatal(err)
	}
	rcfg := renderConfig(t)
	rcfg.OverlayDir = assets
	r, err := NewRenderer(a, rcfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "shorts-example", func(ws clip.MediaWorkspace) error {
		plan := clip.EditPlan{Ratio: "vertical", DurationMS: 20000, Disclosure: "ad", HideDisclosure: os.Getenv("CLIP_EXAMPLE_HIDE_DISCLOSURE") == "1", Preset: "restaurant", Hook: "이 소리, 못 참지", Accent: "lime", CTA: "save", Styles: []string{"clean"},
			Facts: []clip.Answer{{Label: "상호", Text: "철판 한 끼"}, {Label: "메뉴", Text: "철판 요리 · 볶음밥"}}}
		// These are editorial sample words about visible footage. The title is
		// not a claimed merchant name, and no unknown price/location is invented.
		order := []int{6, 0, 1, 2, 3, 4, 5, 7}
		ends := []int{2800, 2800, 2600, 1400, 2800, 4000, 1400, 2600}
		captions := []string{"", "자리부터 잡고", "창밖은 초록", "시작", "양념과 함께", "노릇해질 때까지", "한 쌈", ""}
		if paceExample {
			plan.Styles = []string{"clean", "simple"}
		}
		var sources []clip.RenderSource
		paths := map[string]string{}
		for i, file := range files {
			id := fmt.Sprintf("source-%02d", i)
			local := filepath.Join(ws.Path, id+".mp4")
			if err := copyOriginal(file, local); err != nil {
				return err
			}
			info, err := a.Probe(t.Context(), ws, local)
			if err != nil {
				return err
			}
			sources = append(sources, clip.RenderSource{ID: id, Fingerprint: id, Info: info})
			paths[id] = local
		}
		for i, index := range order {
			id := sources[index].ID
			cut := clip.EditCut{ID: id, SourceID: id, Fingerprint: id, EndMS: ends[i], Focal: clip.Point{X: .5, Y: .5}}
			if index == 5 {
				cut.Focal.X = .18
			}
			if i == 1 || i == 3 {
				cut.TransitionMS = design.Transition.FadeMS
			}
			if captions[i] != "" {
				cut.Copies = []clip.Copy{{Text: captions[i], Style: "clean", Anchor: "bottom", Align: "center", Accent: "lime"}}
			}
			if paceExample && len(cut.Copies) > 0 {
				if i >= 2 {
					cut.Copies[0].Style = "simple"
				}
				if i == 4 || i == 5 {
					text := "오늘은 철판 요리를 먹어봤어요"
					if i == 5 {
						text = "지글지글 익어가면 한 입 더 먹고 싶어요"
					}
					cut.Copies[0].Text = text
					var ok bool
					cut.Copies, ok = clip.SplitRapid(cut.Copies[0], 120, ends[i]-120)
					if !ok {
						return fmt.Errorf("rapid example did not fit")
					}
				}
			}
			// The title and ending each have their own message, with no duplicate
			// caption or information strip underneath them.
			if i > 0 && i < len(order)-1 && ends[i] >= 2000 {
				cut.Chips = []string{"메뉴"}
			}
			plan.Cuts = append(plan.Cuts, cut)
		}
		laidOut, manifest, err := r.Layout(t.Context(), plan, sources)
		if err != nil {
			return err
		}
		if err := design.VerifyApproved(manifest, plan.Ratio, plan.Styles, plan.HideDisclosure); err != nil {
			return fmt.Errorf("example must be free of layout collisions: %w", err)
		}
		started := time.Now()
		result, err := r.Render(t.Context(), ws, laidOut, sources, func(_ context.Context, id string, consume func(clip.MediaSource) error) error {
			for _, source := range sources {
				if source.ID == id {
					return consume(clip.MediaSource{SourceID: id, Fingerprint: source.Fingerprint, Info: source.Info, Path: paths[id]})
				}
			}
			return clip.ErrNotFound
		})
		if err != nil {
			return err
		}
		if err := design.VerifyApproved(result.Manifest, plan.Ratio, plan.Styles, plan.HideDisclosure); err != nil {
			return err
		}
		if result.Info.DurationMS != 20000 || result.Bytes == 0 {
			return fmt.Errorf("unexpected output: %+v", result.Info)
		}
		if err := copyOriginal(result.Path, filepath.Join(output, exampleFilename(paceExample))); err != nil {
			return err
		}
		data, err := json.MarshalIndent(laidOut, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(output, "example-plan.json"), data, 0600); err != nil {
			return err
		}
		t.Logf("8 originals, %d ms, %d bytes, elapsed=%s, model cost $0", result.Info.DurationMS, result.Bytes, time.Since(started))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"memory.peak", "memory.events"} {
		data, _ := os.ReadFile("/sys/fs/cgroup/" + name)
		t.Logf("%s: %s", name, data)
	}
}

// Retain the actual filled SVGs and rasterized plates for design review, as well
// as the editable templates. Only this opt-in local harness exports them.
type exampleRunner struct {
	output string
	runner Runner
}

func (r exampleRunner) Run(ctx context.Context, command Command) ([]byte, error) {
	data, err := r.runner.Run(ctx, command)
	if err != nil || filepath.Base(command.Binary) != "resvg" || len(command.Args) < 2 || !strings.HasSuffix(command.Args[len(command.Args)-1], ".png") {
		return data, err
	}
	dir := filepath.Join(r.output, "rendered-overlays")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	for _, source := range command.Args[len(command.Args)-2:] {
		body, err := os.ReadFile(source)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(dir, filepath.Base(source)), body, 0600); err != nil {
			return nil, err
		}
	}
	return data, nil
}

func exampleFilename(rapid bool) string {
	if rapid {
		return "caption-pace-comparison.mp4"
	}
	return "shorts-example.mp4"
}
