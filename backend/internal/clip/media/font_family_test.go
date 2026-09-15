package media

import (
	"errors"
	"strings"
	"testing"

	"github.com/postpilot/backend/internal/clip/design"
)

func TestBundledFontFamilyNames(t *testing.T) {
	r := testRenderer(t, newAdapter(t, &fakeRunner{}))
	for _, family := range []string{"Paperlogy 8 ExtraBold", "8Paperlogy", "Paperlogy -8", "Pretendard Variable", "", "Paperlogy, serif"} {
		if resolvesFontFamily(r.display, family) {
			t.Fatalf("Paperlogy file accepted %q", family)
		}
	}
	if !resolvesFontFamily(r.display, "Paperlogy") || !resolvesFontFamily(r.font, "Pretendard Variable") {
		t.Fatal("bundled typographic families did not resolve")
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
