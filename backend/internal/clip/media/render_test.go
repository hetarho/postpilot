package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func testRenderer(t *testing.T, a *Adapter) *Rendering {
	t.Helper()
	cfg := renderConfig(t)
	var err error
	cfg.FontPath, err = filepath.Abs("../../../assets/fonts/pretendard/PretendardVariable.ttf")
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRenderer(a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestBundledFontAndGraphemeBoundaries(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	for _, text := range []string{"정확한 한글 & 여행", `<hello> "world"`} {
		if err := r.checkCopy(text); err != nil {
			t.Fatalf("%q: %v", text, err)
		}
	}
	for _, text := range []string{"\x00", "\r", "🙂"} {
		if err := r.checkCopy(text); err == nil {
			t.Fatalf("unsupported text %q", text)
		}
	}
	candidates, _, err := copyCandidates("a\u0308한글")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range candidates {
		if strings.Join(c, "") != "a\u0308한글" {
			t.Fatal("text was changed")
		}
		if len(c) == 2 && c[0] == "a" {
			t.Fatal("split a grapheme")
		}
	}
	if _, _, err := copyCandidates("one\ntwo\nthree"); !errors.Is(err, clip.ErrCopyTooLong) {
		t.Fatal(err)
	}
	if _, values, err := copyCandidates("same\nsame"); err != nil || len(values) != 1 {
		t.Fatalf("duplicate line measurement: %v %v", values, err)
	}
	cfg := r.cfg
	cfg.FontPath = filepath.Join(t.TempDir(), "font.ttf")
	_ = os.WriteFile(cfg.FontPath, []byte("fake"), 0600)
	if _, err := NewRenderer(r.media, cfg); err == nil {
		t.Fatal("wrong font accepted")
	}
}
func TestCopyLayoutAndSVGGolden(t *testing.T) {
	canvas, _ := clip.ClipCanvas("vertical")
	text := `한글 & <여행>`
	bounds := map[string]clip.Region{text: {X: 1, Y: -80, Width: 500, Height: 100}}
	for _, style := range []string{"clean", "diary", "emphasis"} {
		c := clip.Copy{Text: text, Position: "bottom", Style: style, Accent: "coral"}
		l, err := fitCopy(canvas, c, [][]string{{text}}, bounds)
		if err != nil {
			t.Fatal(err)
		}
		svg := copySVG(canvas, c, l)
		golden(t, "copy-"+style+".svg", svg+"\n")
		if strings.Contains(svg, "<여행>") || !strings.Contains(svg, "&amp; &lt;여행&gt;") {
			t.Fatal("text not escaped")
		}
		b := map[string]clip.Region{text: {Width: 100000, Height: 100}}
		if _, err := fitCopy(canvas, c, [][]string{{text}}, b); !errors.Is(err, clip.ErrCopyTooLong) {
			t.Fatal(err)
		}
	}
	c := clip.Copy{Text: "one two", Style: "clean", Position: "top"}
	b := map[string]clip.Region{"one two": {Width: 2500, Height: 100}, "one ": {Width: 800, Height: 100}, "two": {Width: 800, Height: 100}}
	l, err := fitCopy(canvas, c, [][]string{{"one two"}, {"one ", "two"}}, b)
	if err != nil || len(l.Lines) != 2 || l.FontSize != 54 {
		t.Fatalf("%+v %v", l, err)
	}
}
func golden(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_CLIP_GOLDENS") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(actual), 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if actual != string(want) {
		t.Fatalf("golden %s mismatch\n%s", name, actual)
	}
}
func TestRenderFilterGoldens(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	canvas, _ := clip.ClipCanvas("horizontal")
	c := clip.EditCut{StartMS: 100, EndMS: 7700, Focal: clip.Point{X: .25, Y: .75}, Volume: volume(.5)}
	golden(t, "cut.filter", cutGraph(r.cfg, canvas, c, clip.MediaInfo{HasAudio: true}, 228, true, true)+"\n")
	golden(t, "composition.filter", compositionGraph(r.cfg, []int{156, 150, 156}, true)+"\n")
	if strings.Contains(cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 228, false, false), "[a]") {
		t.Fatal("invented audio")
	}
	if !strings.Contains(cutGraph(r.cfg, canvas, c, clip.MediaInfo{}, 228, false, true), "anullsrc=r=48000:cl=stereo") {
		t.Fatal("missing synthesized silence")
	}
	frames := cutFrames(clip.EditPlan{Cuts: []clip.EditCut{{EndMS: 5011}, {EndMS: 5022}, {EndMS: 5367}}}, 30)
	if !reflect.DeepEqual(frames, []int{150, 151, 161}) {
		t.Fatal(frames)
	}
}
func TestCopyMeasurementAndExplicitFontArguments(t *testing.T) {
	fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) { return []byte("m0,1,120,500,100\n"), nil }}
	a := newAdapter(t, fake)
	r := testRenderer(t, a)
	if err := a.WithWorkspace(t.Context(), "copy", func(ws clip.MediaWorkspace) error {
		bounds, err := r.measure(t.Context(), ws, []string{"한글"}, 600)
		if err != nil {
			return err
		}
		if bounds["한글"].Y != -80 {
			t.Fatal(bounds)
		}
		args := fake.calls[0].Args
		if !slices.Contains(args, "--skip-system-fonts") || !slices.Contains(args, r.cfg.FontPath) || !slices.Contains(args, "--query-all") {
			t.Fatal(args)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCopyRecipesMatchFrontendPreview(t *testing.T) {
	data, err := os.ReadFile("../../../../frontend/src/entities/clip-template/model/types.ts")
	if err != nil {
		t.Fatal(err)
	}
	for style, r := range copyRecipes {
		want := fmt.Sprintf("%s: { fontSize: %d, minFontSize: %d, weight: %d, padding: %d, radius: %d }", style, r.FontSize, r.MinFontSize, r.Weight, r.Padding, r.Radius)
		if !strings.Contains(string(data), want) {
			t.Fatalf("preview and render recipe differ: %s", style)
		}
	}
}
func TestRenderDoesNotLoadInvalidPlansOrLeakOnFailure(t *testing.T) {
	for _, mode := range []string{"invalid", "loader", "runner", "panic", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			fake := &fakeRunner{run: func(context.Context, Command) ([]byte, error) { return nil, fmt.Errorf("ffmpeg failed") }}
			a := newAdapter(t, fake)
			r := testRenderer(t, a)
			s := clip.RenderSource{ID: "source", Fingerprint: "hash", Info: clip.MediaInfo{DurationMS: 20000, Width: 1920, Height: 1080}}
			plan := clip.EditPlan{Ratio: "vertical", DurationMS: 15000, Cuts: []clip.EditCut{{ID: "one", SourceID: s.ID, Fingerprint: s.Fingerprint, EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, Copy: clip.Copy{Style: "clean", Position: "bottom"}}}}
			if mode == "invalid" {
				plan.DurationMS = 14000
			}
			var workspace string
			loaded := false
			func() {
				defer func() {
					if recovered := recover(); recovered != nil && mode != "panic" {
						t.Fatal(recovered)
					}
				}()
				err := a.WithWorkspace(t.Context(), "render", func(ws clip.MediaWorkspace) error {
					workspace = ws.Path
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					if mode == "cancel" {
						cancel()
					}
					_, err := r.Render(ctx, ws, plan, []clip.RenderSource{s}, func(_ context.Context, _ string, consume func(clip.MediaSource) error) error {
						loaded = true
						if mode == "loader" {
							return errors.New("download failed")
						}
						if mode == "panic" {
							panic("loader panicked")
						}
						return consume(clip.MediaSource{SourceID: s.ID, Fingerprint: s.Fingerprint, Info: s.Info, Path: sourceFile(t, ws)})
					})
					return err
				})
				if err == nil {
					t.Fatal("failure accepted")
				}
			}()
			if (mode == "invalid" || mode == "cancel") && loaded {
				t.Fatal("loaded invalid plan")
			}
			if _, err := os.Stat(workspace); !os.IsNotExist(err) {
				t.Fatal("workspace leaked")
			}
		})
	}
}
