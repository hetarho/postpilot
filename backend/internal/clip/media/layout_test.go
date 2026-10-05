package media

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var markupTag = regexp.MustCompile(`<[^>]*>`)

// measured is a renderer whose face reports one box per queried text: 40 px a
// character at the 100 px measuring size, close enough to the real face that a
// layout's arithmetic can be checked without resvg.
func measured(t *testing.T) (*Adapter, *Rendering) {
	t.Helper()
	a := newAdapter(t, &fakeRunner{run: func(_ context.Context, c Command) ([]byte, error) { return fakeResvg(c) }})
	return a, testRenderer(t, a)
}

// fakeResvg is measured's stand-in for resvg.
func fakeResvg(c Command) ([]byte, error) {
	// A rasterisation, rather than a measurement: stand in for resvg by
	// writing the SVG it was given to the PNG it was asked for, so the
	// frames a sequence style produces exist and differ exactly where its
	// drawing does.
	if !slices.Contains(c.Args, "--query-all") {
		svg, err := os.ReadFile(c.Args[len(c.Args)-2])
		if err != nil {
			return nil, err
		}
		return nil, os.WriteFile(c.Args[len(c.Args)-1], svg, 0600)
	}
	data, err := os.ReadFile(c.Args[len(c.Args)-1])
	if err != nil {
		return nil, err
	}
	out := ""
	for i, part := range strings.Split(string(data), `id="m`)[1:] {
		text := part[strings.Index(part, ">")+1 : strings.Index(part, "</text>")]
		// A substituted run is a tspan inside the text (CDS-84): count its
		// characters, not its markup.
		text = markupTag.ReplaceAllString(text, "")
		out += fmt.Sprintf("m%d,1,320,%d,100\n", i, 40*len([]rune(text)))
	}
	return []byte(out), nil
}

// RENDER.md carries the two verify points and the ladder between them.
func TestRenderNotesDocumentTheRepairLadder(t *testing.T) {
	notes, err := os.ReadFile("../../../build/RENDER.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"repair ladder", "furniture slot", "AFTER the cuts are rendered", "never walks the ladder"} {
		if !strings.Contains(string(notes), phrase) {
			t.Fatalf("RENDER.md does not say %q", phrase)
		}
	}
}
