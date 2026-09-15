package media

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

// Exercise a file-only preset with production resvg, fonts and measured layout.
// This deliberately different colour is a test fixture, never a shipped design.
func TestRenderSmokeFilePreset(t *testing.T) {
	if os.Getenv("CLIP_MEDIA_SMOKE") != "1" {
		t.Skip("real renderer gate runs inside Docker")
	}
	dir := t.TempDir()
	files := map[string]string{
		"bindings.json":         `{"version":1,"bindings":{"copy.clean":"proof","copy.memo":"proof","copy.bold":"proof","copy.mark":"proof","furniture":"furniture","region":"region","info":"info"}}`,
		"info/preset.json":      `{"id":"info","view":"info-v1","template":"overlay.svg"}`,
		"info/overlay.svg":      `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"proof/preset.json":     `{"id":"proof","view":"copy-v1","template":"overlay.svg"}`,
		"proof/overlay.svg":     `<svg xmlns="http://www.w3.org/2000/svg" width="{{.Width}}" height="{{.Height}}">{{with .Plate}}<rect x="{{.X}}" y="{{.Y}}" width="{{.Width}}" height="{{.Height}}" rx="{{.Radius}}" fill="#FF00FF"/>{{end}}{{range .Lines}}<text x="{{.X}}" y="{{.Y}}" font-family="{{.Family}}" font-size="{{.Size}}" font-weight="{{.Weight}}" letter-spacing="{{.Tracking}}" fill="#FF00FF">{{.Value}}</text>{{end}}</svg>`,
		"furniture/preset.json": `{"id":"furniture","view":"furniture-v1","template":"overlay.svg"}`,
		"furniture/overlay.svg": `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"region/preset.json":    `{"id":"region","view":"region-v1","template":"overlay.svg"}`,
		"region/overlay.svg":    `<svg xmlns="http://www.w3.org/2000/svg"/>`,
	}
	for name, data := range files {
		target := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a, err := New(mediaConfig(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := renderConfig(t)
	cfg.OverlayDir = dir
	r, err := NewRenderer(a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.WithWorkspace(t.Context(), "file-preset", func(ws clip.MediaWorkspace) error {
		canvas, _ := clip.ClipCanvas("vertical")
		copy := clip.Copy{Text: "한글 & 여행", Style: "clean", Anchor: "bottom", Align: "center"}
		layout, err := r.layoutCopy(t.Context(), ws, canvas, copy)
		if err != nil {
			return err
		}
		plate, err := r.copyPlate(t.Context(), ws, canvas, copy, layout, 0, Luminance{})
		if err != nil {
			return err
		}
		img, err := readPNG(plate)
		if err != nil {
			return err
		}
		p := layout.Region
		if !scan(img, p, func(red, green, blue, alpha uint32) bool {
			return alpha > 0xf000 && red > 0xf000 && green == 0 && blue > 0xf000
		}) {
			t.Fatal("file preset did not rasterize its magenta glyphs")
		}
		if _, _, _, alpha := img.At(0, 0).RGBA(); alpha != 0 {
			t.Fatal("file preset lost canvas transparency")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
