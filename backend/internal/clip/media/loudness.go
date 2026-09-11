package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// loudness is what one measurement pass reported about a track. FFmpeg prints
// every value as a STRING, including "-inf" for a silent input, which is why
// this is parsed rather than unmarshalled straight into numbers.
type loudness struct {
	I, TP, LRA, Threshold, Offset float64
	// A track of digital silence measures −inf LUFS. There is no gain that
	// brings silence to −16, so it is carried as a fact rather than a number.
	Silent bool
}

type loudnessReport struct {
	I         string `json:"input_i"`
	TP        string `json:"input_tp"`
	LRA       string `json:"input_lra"`
	Threshold string `json:"input_thresh"`
	Offset    string `json:"target_offset"`
}

var errLoudnessUnreadable = errors.New("clip loudness measurement unreadable")

// loudnormFilter is the ONE spelling of CDS-35's target. A measurement pass
// passes no measured values; the applying pass passes every one of them, which
// is what makes the second pass linear instead of dynamic.
func loudnormFilter(measured *loudness) string {
	l := design.Audio.Loudnorm
	filter := fmt.Sprintf("loudnorm=I=%s:TP=%s:LRA=%s", number(l.I), number(l.TP), number(l.LRA))
	if measured == nil {
		return filter + ":print_format=json"
	}
	// Silence is delivered as silence: there is no gain that makes it −16 LUFS,
	// and a normaliser handed one would only amplify the encoder's own noise.
	if measured.Silent {
		return "anull"
	}
	return filter + fmt.Sprintf(":measured_I=%s:measured_TP=%s:measured_LRA=%s:measured_thresh=%s:offset=%s:linear=true:print_format=summary",
		number(measured.I), number(measured.TP), number(measured.LRA), number(measured.Threshold), number(measured.Offset))
}
func number(value float64) string { return strconv.FormatFloat(value, 'f', -1, 64) }

// parseLoudness reads the report FFmpeg prints at the END of its log. The log
// carries brackets of its own, so the LAST balanced object is the one to read.
func parseLoudness(log []byte) (loudness, error) {
	text := string(log)
	open, closing := strings.LastIndex(text, "{"), strings.LastIndex(text, "}")
	if open < 0 || closing < open {
		return loudness{}, errLoudnessUnreadable
	}
	var report loudnessReport
	if json.Unmarshal([]byte(text[open:closing+1]), &report) != nil {
		return loudness{}, errLoudnessUnreadable
	}
	out := loudness{}
	for _, field := range []struct {
		raw   string
		value *float64
	}{{report.I, &out.I}, {report.TP, &out.TP}, {report.LRA, &out.LRA}, {report.Threshold, &out.Threshold}, {report.Offset, &out.Offset}} {
		n, err := strconv.ParseFloat(strings.TrimSpace(field.raw), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 1) {
			return loudness{}, errLoudnessUnreadable
		}
		// −inf is what a measurable but SILENT track reports, in any of these
		// fields. It is not a reading to normalise against.
		if math.IsInf(n, -1) {
			return loudness{Silent: true}, nil
		}
		*field.value = n
	}
	return out, nil
}

// measureLoudness is the first of CDS-35's two passes: it decodes the assembled
// track and reports what it actually measures, so the second pass can correct it
// linearly. A single dynamic pass drifts on a short file and would leave the
// clip off target, which V12 then refuses.
func (r *Rendering) measureLoudness(ctx context.Context, ws clip.MediaWorkspace, path string) (loudness, error) {
	if err := r.media.sourcePath(ws, path); err != nil {
		return loudness{}, err
	}
	log, err := r.media.runLog(ctx, ws, r.media.cfg.FFmpegPath,
		"-hide_banner", "-nostdin", "-nostats", "-v", "info", "-xerror", "-protocol_whitelist", "file,pipe",
		"-threads", strconv.Itoa(r.media.cfg.Threads), "-i", path,
		"-map", "0:a:0", "-af", loudnormFilter(nil), "-f", "null", "-")
	if err != nil {
		return loudness{}, err
	}
	return parseLoudness(log)
}

// V12's loudness half: the delivered file is measured again and has to sit
// within 1 LU of the target. A miss fails the render exactly as a codec or a
// format mismatch does — an off-target clip is not shippable.
func (r *Rendering) verifyLoudness(ctx context.Context, ws clip.MediaWorkspace, path string) error {
	measured, err := r.measureLoudness(ctx, ws, path)
	if err != nil {
		return err
	}
	if measured.Silent {
		return nil
	}
	if math.Abs(measured.I-design.Audio.Loudnorm.I) > loudnessToleranceLU {
		return fmt.Errorf("rendered clip loudness %.2f LUFS is not within %.0f LU of %.1f", measured.I, loudnessToleranceLU, design.Audio.Loudnorm.I)
	}
	return nil
}

// CDS-35 states the target and V12 the tolerance around it; nothing else in the
// design system needs the number, so it lives beside the check that reads it.
const loudnessToleranceLU float64 = 1
