package media

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func TestBundledFontFamilyNames(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	for _, family := range []string{"Paperlogy 8 ExtraBold", "8Paperlogy", "Paperlogy -8", "Wanted Sans Variable", "", "Paperlogy, serif"} {
		if resolvesFontFamily(r.fonts["paperlogy"], family) {
			t.Fatalf("Paperlogy file accepted %q", family)
		}
	}
	// NanumMyeongjo ExtraBold declares the unparsable NanumMyeongjoExtraBold as
	// its legacy family and NanumMyeongjo as its typographic one, which is the
	// name both weights answer to.
	if resolvesFontFamily(r.fonts["nanummyeongjo-800"], "NanumMyeongjoExtraBold") {
		t.Fatal("a family name SVG cannot parse was accepted")
	}
	for key, family := range map[string]string{
		"wantedsans": "Wanted Sans Variable", "paperlogy": "Paperlogy", "jua": "Jua",
		"nanummyeongjo": "NanumMyeongjo", "nanummyeongjo-800": "NanumMyeongjo",
	} {
		if !resolvesFontFamily(r.fonts[key], family) {
			t.Fatalf("%s did not resolve %q", key, family)
		}
	}
}

func TestRendererRejectsUnresolvableFontFamily(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	original := design.Faces["paperlogy"]
	t.Cleanup(func() { design.Faces["paperlogy"] = original })
	for _, family := range []string{"Paperlogy 8 ExtraBold", "8Paperlogy", "Missing Face"} {
		design.Faces["paperlogy"] = family
		got, err := NewRenderer(r.media, r.cfg)
		if got != nil || !errors.Is(err, ErrFontFamily) || !strings.Contains(err.Error(), "faces.paperlogy") {
			t.Fatalf("%q: renderer=%v err=%v", family, got, err)
		}
	}
	design.Faces["paperlogy"] = "Paperlogy"
	if _, err := NewRenderer(r.media, r.cfg); err != nil {
		t.Fatal(err)
	}
	design.Faces["unbundled"] = "Paperlogy"
	t.Cleanup(func() { delete(design.Faces, "unbundled") })
	if _, err := NewRenderer(r.media, r.cfg); !errors.Is(err, ErrFontFamily) {
		t.Fatalf("unbundled configured face accepted: %v", err)
	}
}
