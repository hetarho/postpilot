package clip

import (
	"context"
	"math"
)

// SampledGround is what CDS-44 measured under one unplated declared text: the
// relative luminance of its three frames, their mean and deviation, and the mean
// colour it is read against. A render draws the scrim, the accent colour and the
// contrast notice from it, so a browser render that is handed the server's
// measurement draws what a server render of the same plan draws (CLIP-192).
//
// InstanceID and Phrase name the visual: a rapid caption is laid out as one
// visual per phrase under the caption's id, numbered in the order they play.
type SampledGround struct {
	InstanceID  string
	Phrase      int
	Mean, Sigma float64
	R, G, B     float64
	Frames      []float64
}

// GroundSampler measures, from the retained originals, the grounds a portable
// plan's unplated texts are read against (CDS-44, CLIP-192).
type GroundSampler interface {
	SampleGrounds(context.Context, MediaWorkspace, EditPlan, []RenderSource, RenderSourceLoader) ([]SampledGround, error)
}

// ValidateSampledGrounds bounds a sample stage's measurement before it is kept:
// at most one ground per laid-out visual, each naming its visual and holding
// relative luminances and colours inside 0..1 over at most three frames.
func ValidateSampledGrounds(grounds []SampledGround, limit int) error {
	if len(grounds) > limit {
		return ErrInvalid
	}
	type key struct {
		id     string
		phrase int
	}
	seen := map[key]bool{}
	unit := func(values ...float64) bool {
		for _, v := range values {
			if math.IsNaN(v) || v < 0 || v > 1 {
				return false
			}
		}
		return true
	}
	for _, g := range grounds {
		k := key{g.InstanceID, g.Phrase}
		if !ValidMediaLabel(g.InstanceID) || g.Phrase < 0 || seen[k] || len(g.Frames) == 0 || len(g.Frames) > 3 || !unit(g.Mean, g.Sigma, g.R, g.G, g.B) || !unit(g.Frames...) {
			return ErrInvalid
		}
		seen[k] = true
	}
	return nil
}
