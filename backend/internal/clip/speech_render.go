package clip

import (
	"context"
	"math"
)

const SpeechMaxAssetBytes = 8 << 20
const SpeechManifestMaxBytes = 128 << 10

// SpeechPlacement is immutable audio provenance on the output clock, independent of cuts.
type SpeechPlacement struct {
	SegmentID                      string
	StartMS, EndMS, VolumePermille int
	Speech                         SpeechRef
}

func RequestedSpeech(p EditPlan) []SpeechPlacement {
	if p.Narration == nil || !p.Narration.Enabled {
		return nil
	}
	out := make([]SpeechPlacement, 0, len(p.Narration.Segments))
	for _, segment := range p.Narration.Segments {
		if segment.Speech == nil {
			return nil
		}
		out = append(out, SpeechPlacement{segment.ID, segment.StartMS, segment.EndMS, p.Narration.VolumePermille, *segment.Speech})
	}
	return out
}

// The loader lends a bounded worker-local private input for exactly the callback's lifetime.
type RenderSpeechLoader func(context.Context, string, func(string) error) error
type NarrationRenderer interface {
	RenderNarrated(context.Context, MediaWorkspace, EditPlan, []RenderSource, RenderSourceLoader, RenderSpeechLoader) (RenderedVideo, error)
}

// OutputNarrationReadiness includes cumulative CFR rounding, so even the last
// sample of a requested asset fits in the delivered video/audio interval.
func OutputNarrationReadiness(p EditPlan, fps, rate int) error {
	if err := NarrationReadiness(p); err != nil {
		return err
	}
	if p.Narration == nil || !p.Narration.Enabled {
		return nil
	}
	if fps <= 0 || rate <= 0 {
		return ErrInvalid
	}
	elapsed, overlap := 0, 0
	for i, c := range p.Cuts {
		elapsed += c.OutputDurationMS()
		if i > 0 {
			overlap += c.TransitionMS * fps / 1000
		}
	}
	samples := int64(int(math.Round(float64(elapsed*fps)/1000))-overlap) * int64(rate) / int64(fps)
	for _, s := range p.Narration.Segments {
		end := int64(s.StartMS)*int64(rate)/1000 + (s.Speech.Samples*int64(rate)+int64(s.Speech.SampleRate)-1)/int64(s.Speech.SampleRate)
		if end > samples {
			return spokenRefusal(s.ID, "spoken_timing_conflict")
		}
	}
	return nil
}
