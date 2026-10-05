package clip

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
)

// Length-framed UTF-8 fields avoid JSON escaping/nullable alignment differences
// between executors. Every immutable audio/placement/gain field is included.
func SpeechFingerprint(speech []SpeechPlacement) string {
	if len(speech) == 0 {
		return ""
	}
	h := sha256.New()
	fmt.Fprintf(h, "speech-render-v1|%d|", len(speech))
	field := func(value string) { fmt.Fprintf(h, "%d:", len(value)); io.WriteString(h, value) }
	integer := func(value int) { field(strconv.Itoa(value)) }
	for _, p := range speech {
		r := p.Speech
		field(p.SegmentID)
		integer(p.StartMS)
		integer(p.EndMS)
		integer(p.VolumePermille)
		for _, value := range []string{r.AssetID, r.VoiceID, r.BindingDigest, r.InputHash, r.SettingsHash, r.AudioHash, r.ProfileID} {
			field(value)
		}
		field(strconv.FormatInt(r.ProfileRevision, 10))
		field(strconv.FormatInt(r.Samples, 10))
		integer(r.SampleRate)
		integer(r.Channels)
		integer(len(r.Timing))
		for _, t := range r.Timing {
			field(t.Text)
			integer(t.StartMS)
			integer(t.EndMS)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
