package design

// PopExposureVersion changes both renderer/cache compatibility identities. The
// existing four-word entrance remains exact; longer fixed text shares its
// stagger envelope so every word exists in the native rapid representative.
const PopExposureVersion = "pop-exposure-v2"

const popOnset = 0.05
const popStagger = 0.13
const popRise = 0.30
const popEnvelopeWords = 4

func popWordProgress(progress float64, index, words int) float64 {
	stagger := popStagger
	if words > popEnvelopeWords {
		stagger *= float64(popEnvelopeWords-1) / float64(words-1)
	}
	return clamp01((progress - (popOnset + float64(index)*stagger)) / popRise)
}

// CaptionTransformPaint is the existing painters' extra decoration, separate
// from their face/role paint. Exported into the hashed browser ink catalog.
func CaptionTransformPaint() map[string]any {
	return map[string]any{
		"version": PopExposureVersion,
		"pop":     map[string]any{"onset": popOnset, "stagger": popStagger, "rise": popRise, "envelopeWords": popEnvelopeWords},
		"white":   "#FFFFFF", "black": "#000000", "stackInk": "#10130A", "stickerEdge": "#FF3B6B", "serifRule": "#E9E3D6",
	}
}
