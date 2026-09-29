package voice

import "time"

const (
	SampleMinChars     = 200
	LabelFallbackChars = 20
)

// A photo prompt's photo is stored as a post photo is (VOICE-60, →POST-33 … POST-38).
const (
	PhotoContentType  = "image/jpeg"
	MaxPhotoDimension = 20000
	photoObjectPrefix = "voices/"
)

// PhotoLimits are the post photo's storage limits, handed over by the composition root.
type PhotoLimits struct {
	PutTTL, GetTTL time.Duration
	MaxBytes       int64
}
