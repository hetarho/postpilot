package clip

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
