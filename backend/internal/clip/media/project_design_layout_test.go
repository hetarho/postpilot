package media

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
	"github.com/postpilot/backend/internal/clip/design"
)

// The frozen document keeps the slot text a template declared; WHICH preset
// holds it, and which styles a caption may take, are read from the project
// (CLIP-139, CLIP-142). The body below says intro="b" and the project says "a".
const designedBody = `<clip version="1" intro="b" caption="bold" outro="e">` +
	`<text id="hello" kind="fixed" role="hook" basis="output-start"><row>안녕하세요</row></text>` +
	`<text id="one" kind="fixed" role="caption" basis="whole">첫 문장</text>` +
	`<text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>`

func TestLayoutTakesTheRegionPresetFromTheProjectNotTheFrozenDocument(t *testing.T) {
	for _, chosen := range []string{"a", "b"} {
		plan := declaredPlan(t, designedBody, "vertical")
		plan.IntroPreset, plan.OutroPreset = chosen, "e"
		layout := measuredDeclared(t, plan)
		for _, visual := range layout.visuals {
			if visual.text.Resolved.Element.Role != "hook" {
				continue
			}
			if err := design.VerifyRegion("intro", chosen, "vertical", 0, true, []string{"안녕하세요"}, visual.manifest.Parts); err != nil {
				t.Fatal("the intro did not render in the preset the project chose", chosen, err)
			}
			other := "b"
			if chosen == "b" {
				other = "a"
			}
			if design.VerifyRegion("intro", other, "vertical", 0, true, []string{"안녕하세요"}, visual.manifest.Parts) == nil {
				t.Fatal("both presets accepted the same geometry, so this proves nothing", chosen)
			}
		}
	}
}

func TestCaptionStylesComeFromTheProjectAndAnUnknownOneIsRefused(t *testing.T) {
	// No selection at all resolves to the default style alone (CDS-25).
	plan := declaredPlan(t, designedBody, "vertical")
	captions := 0
	for _, visual := range measuredDeclared(t, plan).visuals {
		if visual.text.Resolved.Element.Role != "caption" {
			continue
		}
		captions++
		if visual.copy.Style != design.DefaultCaptionStyle {
			t.Fatal("an empty selection did not resolve to the default style", visual.copy.Style)
		}
	}
	if captions != 1 {
		t.Fatal("the fixture laid out no caption", captions)
	}
	// A style id the approved set does not carry is an authoring error, named
	// at layout rather than silently dropped (CDS-66).
	unknown := declaredPlan(t, designedBody, "vertical")
	unknown.CaptionStyles = []string{"sparkle"}
	a, r := measured(t)
	err := a.WithWorkspace(t.Context(), "unknown-style", func(ws clip.MediaWorkspace) error {
		_, err := r.layoutComposition(t.Context(), ws, unknown)
		return err
	})
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.Reason != "invalid_design" {
		t.Fatal("an unapproved caption style was accepted", err)
	}
}
