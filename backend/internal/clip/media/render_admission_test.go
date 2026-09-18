package media

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/composition"
)

func TestRenderAdmissionChecksLegacyConversionAndSourceRate(t *testing.T) {
	_, r := measured(t)
	plan := clip.EditPlan{Ratio: "square", DurationMS: 15000, HideDisclosure: true, Hook: "two\nlines", Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "source", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}}}}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "source", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	_, err := r.ValidateRenderPlan(t.Context(), plan, sources)
	var problem *composition.Problem
	if !errors.As(err, &problem) || problem.ElementID != "legacy-hook" || problem.Reason != "copy_limit" {
		t.Fatal(err)
	}
	plan.Hook = ""
	plan.Cuts[0].PlaybackRatePermille = 500
	if _, err := r.ValidateRenderPlan(t.Context(), plan, sources); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("unverified source rate admitted", err)
	}
}
