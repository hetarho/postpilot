// Package overlay loads trusted, file-based SVG presets. It knows neither clip
// policy nor FFmpeg: the caller supplies an already measured view and selects a
// binding. Templates are immutable snapshots and escape dynamic values.
package overlay

import (
	"bytes"
	"embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
)

//go:embed presets
var embedded embed.FS

const (
	maxPresets      = 64
	maxAssetBytes   = 512 << 10
	maxCatalogBytes = 8 << 20
	maxOutputBytes  = 2 << 20
)

var identifier = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type Preset struct {
	ID       string `json:"id"`
	View     string `json:"view"`
	Template string `json:"template"`
}
type index struct {
	Version  int               `json:"version"`
	Bindings map[string]string `json:"bindings"`
}
type entry struct {
	info     Preset
	template *template.Template
}
type Catalog struct {
	bindings map[string]string
	entries  map[string]entry
}

// Builtin uses exactly the same loader as an operator-provided directory.
func Builtin() (*Catalog, error) {
	root, err := fs.Sub(embedded, "presets")
	if err != nil {
		return nil, err
	}
	return Load(root)
}

// Load discovers preset.json/overlay.svg folders and freezes their contents.
// fsys must be a trusted deployment filesystem, not a request-controlled path.
func Load(fsys fs.FS) (*Catalog, error) {
	used := 0
	read := func(name string) ([]byte, error) {
		info, err := fs.Stat(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("overlay asset %s: %w", name, err)
		}
		if !info.Mode().IsRegular() || info.Size() > maxAssetBytes {
			return nil, fmt.Errorf("overlay asset %s exceeds its file contract", name)
		}
		f, err := fsys.Open(name)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, maxAssetBytes+1))
		used += len(data)
		if len(data) > maxAssetBytes || used > maxCatalogBytes {
			return nil, errors.New("overlay catalog exceeds byte limit")
		}
		return data, err
	}
	data, err := read("bindings.json")
	if err != nil {
		return nil, err
	}
	var idx index
	if err := decode(data, &idx); err != nil {
		return nil, fmt.Errorf("overlay bindings: %w", err)
	}
	if idx.Version != 1 || len(idx.Bindings) == 0 || len(idx.Bindings) > maxPresets {
		return nil, errors.New("unsupported overlay bindings contract")
	}
	dirs, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	c := &Catalog{bindings: idx.Bindings, entries: map[string]entry{}}
	for _, dir := range dirs {
		if dir.Type()&fs.ModeSymlink != 0 {
			return nil, errors.New("overlay catalog contains a symlink")
		}
		if !dir.IsDir() {
			continue
		}
		if !identifier.MatchString(dir.Name()) || len(c.entries) >= maxPresets {
			return nil, errors.New("invalid overlay preset directory")
		}
		files, err := fs.ReadDir(fsys, dir.Name())
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if file.Type()&fs.ModeSymlink != 0 || file.IsDir() {
				return nil, fmt.Errorf("overlay preset %s must contain regular files", dir.Name())
			}
		}
		data, err := read(path.Join(dir.Name(), "preset.json"))
		if err != nil {
			return nil, err
		}
		var p Preset
		if err := decode(data, &p); err != nil {
			return nil, fmt.Errorf("overlay preset %s: %w", dir.Name(), err)
		}
		if p.ID != dir.Name() || !slices.Contains([]string{"copy-v1", "furniture-v1", "card-v1"}, p.View) || p.Template != "overlay.svg" {
			return nil, fmt.Errorf("unsupported overlay preset contract: %s", dir.Name())
		}
		data, err = read(path.Join(dir.Name(), p.Template))
		if err != nil {
			return nil, err
		}
		t, err := template.New(p.ID).Option("missingkey=error").Parse(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, fmt.Errorf("overlay template %s: %w", p.ID, err)
		}
		c.entries[p.ID] = entry{p, t}
	}
	for binding, id := range c.bindings {
		p, ok := c.entries[id]
		if !ok {
			return nil, fmt.Errorf("overlay binding %s names missing preset %s", binding, id)
		}
		kind, _, _ := strings.Cut(binding, ".")
		if !identifier.MatchString(strings.ReplaceAll(binding, ".", "-")) || p.info.View != kind+"-v1" {
			return nil, fmt.Errorf("overlay binding %s has incompatible view", binding)
		}
	}
	return c, nil
}

// Presets is a sorted copy; neither it nor the caller's FS can mutate a catalog.
func (c *Catalog) Presets() []Preset {
	result := make([]Preset, 0, len(c.entries))
	for _, e := range c.entries {
		result = append(result, e.info)
	}
	slices.SortFunc(result, func(a, b Preset) int { return strings.Compare(a.ID, b.ID) })
	return result
}

func (c *Catalog) Render(binding string, view any) (string, error) {
	id, ok := c.bindings[binding]
	// Older deployment catalogs predate the additive simple style.
	if !ok && binding == "copy.simple" {
		id, ok = c.bindings["copy.clean"]
	}
	if !ok {
		return "", fmt.Errorf("overlay binding %s is missing", binding)
	}
	return c.render(id, view)
}

// Validate checks a known view against a preset, including an unbound asset.
func (c *Catalog) Validate(id string, view any) error {
	_, err := c.render(id, view)
	return err
}

func (c *Catalog) render(id string, view any) (string, error) {
	e, ok := c.entries[id]
	if !ok {
		return "", fmt.Errorf("overlay preset %s is missing", id)
	}
	var b limitedBuffer
	if err := e.template.Execute(&b, view); err != nil {
		return "", fmt.Errorf("overlay preset %s: %w", id, err)
	}
	if err := validateSVG(b.Bytes()); err != nil {
		return "", fmt.Errorf("overlay preset %s: %w", id, err)
	}
	return b.String(), nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxOutputBytes {
		return 0, errors.New("overlay output exceeds byte limit")
	}
	return b.Buffer.Write(p)
}
func (b *limitedBuffer) WriteString(s string) (int, error) { return b.Write([]byte(s)) }

func decode(data []byte, out any) error {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(out); err != nil {
		return err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return errors.New("trailing overlay JSON")
	}
	return nil
}

// Keep the output a standalone static SVG. Context-aware template escaping is
// responsible for text/attribute data; this checks the asset contract itself.
func validateSVG(data []byte) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	depth, roots := 0, 0
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.New("invalid overlay SVG")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || t.Name.Local != "svg" || t.Name.Space != "http://www.w3.org/2000/svg" {
					return errors.New("overlay must have one SVG root")
				}
			}
			depth++
			switch strings.ToLower(t.Name.Local) {
			case "script", "foreignobject", "animate", "animatetransform", "animatemotion", "set":
				return errors.New("overlay must be static SVG")
			}
			for _, a := range t.Attr {
				name, value := strings.ToLower(a.Name.Local), strings.TrimSpace(a.Value)
				if strings.HasPrefix(name, "on") || ((name == "href" || name == "src") && !strings.HasPrefix(value, "#")) {
					return errors.New("overlay contains an external or active reference")
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive, xml.ProcInst:
			return errors.New("overlay directives are unsupported")
		case xml.CharData:
			if depth == 0 && strings.TrimSpace(string(t)) != "" {
				return errors.New("text outside overlay SVG")
			}
		}
	}
	if roots != 1 || depth != 0 {
		return errors.New("empty overlay SVG")
	}
	return nil
}
