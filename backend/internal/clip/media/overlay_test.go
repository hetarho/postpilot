package media

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
	"github.com/postpilot/backend/internal/clip/overlay"
)

// Existing goldens continue to describe the delivered drawings, byte for byte.
// Their test helper follows the same catalog/view path as production plates.
func builtinSVG(binding string, view any) string {
	catalog, err := overlay.Builtin()
	if err != nil {
		panic(err)
	}
	svg, err := catalog.Render(binding, view)
	if err != nil {
		panic(err)
	}
	return svg
}
func copySVG(canvas clip.Canvas, c clip.Copy, l copyLayout, ground Luminance) string {
	return builtinSVG("copy."+c.Style, copyView(canvas, c, l, ground))
}
func furnitureSVG(canvas clip.Canvas, f furniture) string {
	return builtinSVG("furniture", furnitureView(canvas, f))
}
func cardSVG(canvas clip.Canvas, c cardLayout) string {
	return builtinSVG("card."+c.Kind, cardView(canvas, c))
}

func overlayDirectory(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	err := fs.WalkDir(os.DirFS("../overlay/presets"), ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(target, 0700)
		}
		data, err := os.ReadFile(filepath.Join("../overlay/presets", filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0600)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRendererUsesNewPresetFromFilesWithoutAnotherStyleSwitch(t *testing.T) {
	directory := overlayDirectory(t)
	if err := os.Mkdir(filepath.Join(directory, "editorial"), 0700); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(directory, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("editorial/preset.json", `{"id":"editorial","view":"copy-v1","template":"overlay.svg"}`)
	write("editorial/overlay.svg", `<svg xmlns="http://www.w3.org/2000/svg" width="{{.Width}}" height="{{.Height}}" data-preset="editorial">{{range .Lines}}<text x="{{.X}}" y="{{.Y}}" font-family="{{.Family}}" font-size="{{.Size}}">{{.Value}}</text>{{end}}</svg>`)
	data, err := os.ReadFile(filepath.Join(directory, "bindings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var bindings struct {
		Version  int               `json:"version"`
		Bindings map[string]string `json:"bindings"`
	}
	if err := json.Unmarshal(data, &bindings); err != nil {
		t.Fatal(err)
	}
	bindings.Bindings["copy.clean"] = "editorial"
	data, err = json.Marshal(bindings)
	if err != nil {
		t.Fatal(err)
	}
	write("bindings.json", string(data))
	var drawn string
	fake := &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) {
		data, err := os.ReadFile(c.Args[len(c.Args)-2])
		if err != nil {
			return nil, err
		}
		drawn = string(data)
		return nil, os.WriteFile(c.Args[len(c.Args)-1], []byte("png"), 0600)
	}}
	a := newAdapter(t, fake)
	cfg := testRenderer(t, a).cfg
	cfg.OverlayDir = directory
	r, err := NewRenderer(a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	// A bad edit after boot must not alter an already running catalog.
	write("editorial/overlay.svg", "invalid")
	canvas, _ := clip.ClipCanvas("vertical")
	copy := clip.Copy{Text: `한글 & <여행>`, Style: "clean", Anchor: "bottom", Align: "center"}
	l, err := fitCopy(canvas, copy, [][]string{{copy.Text}}, map[string]clip.Region{copy.Text: {X: 1, Y: -80, Width: 500, Height: 100}})
	if err != nil {
		t.Fatal(err)
	}
	err = a.WithWorkspace(t.Context(), "asset-preset", func(ws clip.MediaWorkspace) error {
		_, err := r.copyPlate(t.Context(), ws, canvas, copy, l, 0, Luminance{})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(drawn, `data-preset="editorial"`) || !strings.Contains(drawn, `한글 &amp; &lt;여행&gt;`) {
		t.Fatal("preset or plain text was not used", drawn)
	}
	if l.Style != design.Styles["clean"] {
		t.Fatal("asset changed layout policy")
	}
	if _, err := NewRenderer(a, cfg); err == nil {
		t.Fatal("new startup accepted malformed SVG")
	}
}

func TestRendererChecksAllAssetContractsAtStartup(t *testing.T) {
	dir := overlayDirectory(t)
	if err := os.WriteFile(filepath.Join(dir, "caption", "overlay.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg">{{.Nonexistent}}</svg>`), 0600); err != nil {
		t.Fatal(err)
	}
	a := newAdapter(t, &fakeRunner{})
	cfg := testRenderer(t, a).cfg
	cfg.OverlayDir = dir
	if _, err := NewRenderer(a, cfg); err == nil {
		t.Fatal("missing view field accepted")
	}
}
