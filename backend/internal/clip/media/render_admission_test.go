package media

import (
	"errors"
	"testing"

	"github.com/postpilot/backend/internal/clip"
)

func TestRenderAdmissionChecksTheSourceRate(t *testing.T) {
	_, r := measured(t)
	plan := clip.EditPlan{Ratio: "square", DurationMS: 15000, HideDisclosure: true, Cuts: []clip.Cut{{ID: "cut", SourceID: "source", Fingerprint: "source", EndMS: 15000, Focal: clip.Point{X: .5, Y: .5}, PlaybackRatePermille: 500}}}
	sources := []clip.RenderSource{{ID: "source", Fingerprint: "source", Info: clip.MediaInfo{DurationMS: 15000, Width: 1920, Height: 1080}}}
	if _, err := r.ValidateRenderPlan(t.Context(), plan, sources); !errors.Is(err, clip.ErrInvalid) {
		t.Fatal("unverified source rate admitted", err)
	}
}
