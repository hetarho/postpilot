package overlay

import (
	"encoding/json"
	"encoding/xml"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func fixture(body string) fstest.MapFS {
	return fstest.MapFS{
		"bindings.json":      {Data: []byte(`{"version":1,"bindings":{"copy.clean":"custom"}}`)},
		"custom/preset.json": {Data: []byte(`{"id":"custom","view":"copy-v1","template":"overlay.svg"}`)},
		"custom/overlay.svg": {Data: []byte(body)},
	}
}

func TestCatalogDiscoversAssetsEscapesTextAndFreezesFiles(t *testing.T) {
	fsys := fixture(`<svg xmlns="http://www.w3.org/2000/svg"><text data-label="{{.Text}}">{{.Text}}</text></svg>`)
	catalog, err := Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	input := `한글 & <script>alert("private")</script> " '`
	view := map[string]string{"Text": input}
	out, err := catalog.Render("copy.clean", view)
	if err != nil {
		t.Fatal(err)
	}
	d := xml.NewDecoder(strings.NewReader(out))
	texts := 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			if v.Name.Local == "script" {
				t.Fatal("text became markup")
			}
			if v.Name.Local == "text" && (len(v.Attr) != 1 || v.Attr[0].Value != input) {
				t.Fatal("attribute changed", v)
			}
		case xml.CharData:
			if string(v) != input {
				t.Fatal("text changed", string(v))
			}
			texts++
		}
	}
	if texts != 1 {
		t.Fatal("lost text")
	}
	fsys["custom/overlay.svg"].Data = []byte("broken")
	fsys["bindings.json"].Data = []byte("broken")
	presets := catalog.Presets()
	presets[0].ID = "changed"
	again, err := catalog.Render("copy.clean", view)
	if err != nil || again != out || catalog.Presets()[0].ID != "custom" {
		t.Fatal("catalog changed after startup", err)
	}
}

func TestCatalogRejectsInvalidRegistration(t *testing.T) {
	for _, tc := range []struct{ name, file, content string }{
		{"version", "bindings.json", `{"version":2,"bindings":{"copy.clean":"custom"}}`},
		{"unknown-field", "bindings.json", `{"version":1,"bindings":{"copy.clean":"custom"},"secret":true}`},
		{"missing-preset", "bindings.json", `{"version":1,"bindings":{"copy.clean":"absent"}}`},
		{"wrong-view", "bindings.json", `{"version":1,"bindings":{"region":"custom"}}`},
		{"wrong-id", "custom/preset.json", `{"id":"other","view":"copy-v1","template":"overlay.svg"}`},
		{"path", "custom/preset.json", `{"id":"custom","view":"copy-v1","template":"../private.svg"}`},
		{"bad-template", "custom/overlay.svg", `{{if .Missing}}`},
		{"trailing-json", "custom/preset.json", `{"id":"custom","view":"copy-v1","template":"overlay.svg"}{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixture(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
			f[tc.file].Data = []byte(tc.content)
			if _, err := Load(f); err == nil {
				t.Fatal("accepted invalid registration")
			}
		})
	}
}

func TestCatalogRejectsMissingFieldsAndInvalidSVG(t *testing.T) {
	for name, body := range map[string]string{
		"missing-field":  `<svg xmlns="http://www.w3.org/2000/svg">{{.Missing}}</svg>`,
		"bad-xml":        `<svg xmlns="http://www.w3.org/2000/svg"><text></svg>`,
		"multiple-roots": `<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`,
		"html":           `<div/>`,
		"script":         `<svg xmlns="http://www.w3.org/2000/svg"><script>oops</script></svg>`,
		"external":       `<svg xmlns="http://www.w3.org/2000/svg"><image href="file:///private"/></svg>`,
		"event":          `<svg xmlns="http://www.w3.org/2000/svg" onload="oops"/>`,
		"animation":      `<svg xmlns="http://www.w3.org/2000/svg"><animate/></svg>`,
		"doctype":        `<!DOCTYPE svg SYSTEM "private"><svg xmlns="http://www.w3.org/2000/svg"/>`,
	} {
		t.Run(name, func(t *testing.T) {
			c, err := Load(fixture(body))
			if err != nil {
				return
			}
			if _, err := c.Render("copy.clean", map[string]string{}); err == nil {
				t.Fatal("accepted invalid SVG")
			}
		})
	}
}

func TestCatalogBoundsAssetAndExpandedOutput(t *testing.T) {
	f := fixture(strings.Repeat("x", maxAssetBytes+1))
	if _, err := Load(f); err == nil {
		t.Fatal("unbounded asset")
	}
	f = fixture(`<svg xmlns="http://www.w3.org/2000/svg"><text>{{.Text}}</text></svg>`)
	c, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Render("copy.clean", map[string]string{"Text": strings.Repeat("&", maxOutputBytes/4)}); err == nil {
		t.Fatal("escaped output exceeded its bound")
	}
	// Catalog bytes have their own bound even when every file fits.
	f = fstest.MapFS{"bindings.json": {Data: []byte(`{"version":1,"bindings":{"copy.clean":"asset-a"}}`)}}
	for i := 0; i < 20; i++ {
		id := "asset-" + string(rune('a'+i))
		manifest, _ := json.Marshal(Preset{ID: id, View: "copy-v1", Template: "overlay.svg"})
		f[id+"/preset.json"] = &fstest.MapFile{Data: manifest}
		f[id+"/overlay.svg"] = &fstest.MapFile{Data: []byte(strings.Repeat(" ", maxAssetBytes-1))}
	}
	if _, err := Load(f); err == nil {
		t.Fatal("unbounded catalog")
	}
}

func TestCatalogRejectsSymlinkAssets(t *testing.T) {
	root := t.TempDir()
	for name, file := range fixture(`<svg xmlns="http://www.w3.org/2000/svg"/>`) {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, file.Data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	p := filepath.Join(root, "custom", "overlay.svg")
	if err := os.Rename(p, p+".original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(p+".original", p); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(os.DirFS(root)); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestBuiltinCatalogUsesDiscovery(t *testing.T) {
	root, err := fs.Sub(embedded, "presets")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Presets(); len(got) != 4 || got[0].ID != "caption" || got[1].ID != "furniture" || got[2].ID != "info" || got[3].ID != "region" {
		t.Fatal(got)
	}
}

// A region-v1 asset still loads and binds, drawing the lines and rules it knows
// from a region-v2 view; a region binding to any other view is refused.
func TestRegionBindingAcceptsBothRegionViews(t *testing.T) {
	for view, ok := range map[string]bool{"region-v1": true, "region-v2": true, "region-v3": false, "copy-v1": false} {
		fsys := fstest.MapFS{
			"bindings.json":   {Data: []byte(`{"version":1,"bindings":{"region":"old"}}`)},
			"old/preset.json": {Data: []byte(`{"id":"old","view":"` + view + `","template":"overlay.svg"}`)},
			"old/overlay.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg">{{range .Rules}}<rect width="{{.Width}}"/>{{end}}{{range .Lines}}<text>{{.Value}}</text>{{end}}</svg>`)},
		}
		catalog, err := Load(fsys)
		if (err == nil) != ok {
			t.Fatal(view, err)
		}
		if !ok {
			continue
		}
		out, err := catalog.Render("region", RegionView{CopyView: CopyView{Lines: []Text{{Value: "한우"}}}, Rules: []Box{{Width: 2}}, Turn: &Turn{Deg: -4}, Shapes: []Shape{{Circle: true}}})
		if err != nil || !strings.Contains(out, "<text>한우</text>") || !strings.Contains(out, `<rect width="2"/>`) {
			t.Fatal(view, out, err)
		}
	}
}

// region-v2 draws the rotation group, circles, arcs and the radial scrim, and
// draws a view without them exactly as region-v1 did.
func TestRegionV2DrawsItsDecoration(t *testing.T) {
	root, err := fs.Sub(embedded, "presets")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	text := Text{Family: "Jua", Size: 40, Weight: 400, Fill: "#FFFFFF", Opacity: "1", Stroke: "none", StrokeOpacity: "1", Value: "한우"}
	out, err := c.Render("region", RegionView{
		CopyView: CopyView{Canvas: Canvas{Width: 1080, Height: 1920}},
		Radial:   &Radial{CX: 540, CY: 960, RX: 680, RY: 400, Fill: "#000000", From: "0.5", Mid: "0.3", MidAt: "0.55", To: "0"},
		Turn:     &Turn{Deg: -8, X: 540, Y: 900, Shapes: []Shape{{Box: Box{X: 540, Y: 900, Radius: 250, Fill: "#000000", Opacity: "0.22"}, Circle: true, Stroke: "#FFFFFF", StrokeOpacity: "1", StrokeWidth: 5}}, Arcs: []ArcText{{Text: text, ID: "arc0", Path: "M 334 900 A 206 206 0 0 1 746 900"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<radialGradient id="radial"`, `<ellipse cx="540.000" cy="960.000" rx="680.000" ry="400.000" fill="url(#radial)"/>`, `<g transform="rotate(-8.000 540.000 900.000)">`, `<circle cx="540.000" cy="900.000" r="250.000" fill="#000000" fill-opacity="0.22" stroke="#FFFFFF"`, `<path id="arc0" d="M 334 900 A 206 206 0 0 1 746 900"/>`, `<textPath href="#arc0" startOffset="50%">한우</textPath>`} {
		if !strings.Contains(out, want) {
			t.Fatal(want, out)
		}
	}
	plain, err := c.Render("region", RegionView{CopyView: CopyView{Canvas: Canvas{Width: 1080, Height: 1920}, Lines: []Text{text}}, Rules: []Box{{Width: 520, Height: 2, Fill: "#FFFFFF", Opacity: "0.55"}}})
	if err != nil || strings.Contains(plain, "<g") || strings.Contains(plain, "<circle") || strings.Contains(plain, "textPath") || strings.Contains(plain, "radial") {
		t.Fatal(plain, err)
	}
}
