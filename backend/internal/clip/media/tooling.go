package media

import "strings"

// What this package hands to the bundled binaries by name.
//
// The image ships a `--disable-everything` ffmpeg (backend/build/media-tools.sh), so a
// name this code emits correctly can still be absent from the binary, and the gap shows
// only at render time: `atempo` shipped missing that way and every rated cut carrying
// source audio was broken in production while the graph it emitted was right. ARCH-39
// makes the build prove the binary carries all of these.
//
// These are compared against what the binaries report, never against the build script. A
// configure flag and a runtime name are different strings — `--enable-muxer=pcm_s16le`
// yields the muxer `s16le` — and a list checked against its own source proves nothing.

// Every filter the render path emits. `TestGoldenFiltersAreDeclared` holds this honest
// against the graph fixtures, so a filter that reaches a golden cannot be missing here.
var requiredFilters = []string{
	"acrossfade", "afade", "aformat", "anull", "anullsrc", "apad", "aresample",
	"asetpts", "atempo", "atrim", "concat", "crop", "fade", "format", "fps",
	"loop", "loudnorm", "overlay", "scale", "setpts", "setsar", "settb", "trim",
	"vfrdet", "volume", "xfade",
}

// Encoders named by `-c:v` / `-c:a`. `copy` is stream copying, not an encoder.
var requiredEncoders = []string{"aac", "libx264", "pcm_s16le", "png"}

// Muxers named by `-f`.
var requiredMuxers = []string{"mp4", "null", "wav"}

// Decoders for what this package writes and reads back: the h264/aac chunks and analysis
// copies it produces, the PNG plates resvg renders for overlay, and the PCM it measures
// loudness on. A source's own codec is not here — the render code never names it, ffmpeg
// selects it from the container.
var requiredDecoders = []string{"aac", "h264", "pcm_s16le", "png"}

func parseListing(out string) map[string]bool {
	names := map[string]bool{}
	body := false
	for _, line := range strings.Split(out, "\n") {
		trimmed := strings.TrimSpace(line)
		if !body {
			// The legend ends at a rule of dashes; rows follow it.
			if trimmed != "" && strings.Trim(trimmed, "-") == "" {
				body = true
			}
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		// A format row can name several at once, e.g. `matroska,webm`.
		for _, name := range strings.Split(fields[1], ",") {
			names[name] = true
		}
	}
	return names
}
