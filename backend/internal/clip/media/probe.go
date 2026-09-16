package media

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

type probeDocument struct {
	Streams []struct {
		Index             int    `json:"index"`
		Codec             string `json:"codec_name"`
		Profile           string `json:"profile"`
		Kind              string `json:"codec_type"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		FrameRate         string `json:"avg_frame_rate"`
		SampleAspectRatio string `json:"sample_aspect_ratio"`
		PixelFormat       string `json:"pix_fmt"`
		Duration          string `json:"duration"`
		Channels          int    `json:"channels"`
		SampleRate        string `json:"sample_rate"`
		Disposition       struct {
			Attached int `json:"attached_pic"`
		} `json:"disposition"`
		SideData []struct {
			Rotation *float64 `json:"rotation"`
		} `json:"side_data_list"`
	} `json:"streams"`
	Format struct {
		Name     string `json:"format_name"`
		Duration string `json:"duration"`
	} `json:"format"`
}

func ratio(value string, separator string) (int, int) {
	parts := strings.Split(value, separator)
	if len(parts) != 2 {
		return 0, 0
	}
	n, _ := strconv.Atoi(parts[0])
	d, _ := strconv.Atoi(parts[1])
	if n <= 0 || d <= 0 {
		return 0, 0
	}
	return n, d
}

// ProbeContainer is the cheap first read: what the container and its stream
// headers claim, with no decode at all. It decides how many analysis copies a
// source needs; the decode that produces those copies then confirms or refuses
// the claim, so an original is never decoded twice (CLIP-33, CLIP-124).
func (a *Adapter) ProbeContainer(ctx context.Context, ws clip.MediaWorkspace, path string) (clip.MediaInfo, error) {
	info, err := a.probeContainer(ctx, ws, path)
	if err != nil {
		return clip.MediaInfo{}, err
	}
	// A claim only, and the shorter of the two a header can carry: a container
	// that outruns its video track never adds a copy the footage cannot fill.
	info.DurationMS = info.ContainerDurationMS
	if info.VideoDurationMS > 0 && info.VideoDurationMS < info.DurationMS {
		info.DurationMS = info.VideoDurationMS
	}
	if info.DurationMS <= 0 || info.DurationMS > a.cfg.Sources.MaxDurationMS {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	return info, nil
}

func (a *Adapter) probeContainer(ctx context.Context, ws clip.MediaWorkspace, path string) (clip.MediaInfo, error) {
	if err := a.sourcePath(ws, path); err != nil {
		return clip.MediaInfo{}, err
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	if _, ok := a.cfg.Sources.Containers[ext]; !ok {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	data, err := a.run(ctx, ws, a.cfg.FFprobePath, "-v", "error", "-protocol_whitelist", "file,pipe", "-show_streams", "-show_format", "-of", "json", path)
	if err != nil {
		return clip.MediaInfo{}, errors.Join(clip.ErrInvalidMedia, err)
	}
	var doc probeDocument
	if json.Unmarshal(data, &doc) != nil || len(doc.Streams) == 0 || len(doc.Streams) > a.cfg.MaxStreams {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	container := "mov"
	if ext == "webm" {
		container = "webm"
	}
	if !slices.Contains(strings.Split(doc.Format.Name, ","), container) {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	info := clip.MediaInfo{ContainerDurationMS: durationMS(doc.Format.Duration)}
	found := false
	for _, s := range doc.Streams {
		info.Streams = append(info.Streams, clip.MediaStream{Index: s.Index, Kind: s.Kind, Codec: s.Codec, Profile: s.Profile})
		if s.Kind == "audio" {
			if !info.HasAudio {
				info.AudioDurationMS = durationMS(s.Duration)
				info.AudioChannels = s.Channels
				info.AudioRate, _ = strconv.Atoi(s.SampleRate)
			}
			info.HasAudio = true
		}
		if s.Kind != "video" || s.Disposition.Attached != 0 || found {
			continue
		}
		found = true
		info.VideoDurationMS = durationMS(s.Duration)
		if s.Width <= 0 || s.Height <= 0 || s.Width > a.cfg.MaxDimension || s.Height > a.cfg.MaxDimension {
			return clip.MediaInfo{}, clip.ErrInvalidMedia
		}
		info.Width, info.Height = s.Width, s.Height
		info.PixelFormat, info.SampleAspectRatio = s.PixelFormat, s.SampleAspectRatio
		if n, d := ratio(s.SampleAspectRatio, ":"); n > 0 {
			info.Width = int(math.Round(float64(s.Width) * float64(n) / float64(d)))
		}
		for _, side := range s.SideData {
			if side.Rotation == nil {
				continue
			}
			rotation := *side.Rotation
			if math.IsNaN(rotation) || math.IsInf(rotation, 0) || rotation != math.Trunc(rotation) {
				return clip.MediaInfo{}, clip.ErrInvalidMedia
			}
			info.Rotation = (int(math.Mod(rotation, 360)) + 360) % 360
		}
		if info.Rotation%90 != 0 {
			return clip.MediaInfo{}, clip.ErrInvalidMedia
		}
		if info.Rotation == 90 || info.Rotation == 270 {
			info.Width, info.Height = info.Height, info.Width
		}
		info.FrameRateNumerator, info.FrameRateDenominator = ratio(s.FrameRate, "/")
	}
	if !found || info.Width <= 0 || info.Height <= 0 || info.Width > a.cfg.MaxDimension || info.Height > a.cfg.MaxDimension || info.FrameRateNumerator == 0 {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	return info, nil
}

// A container's duration is only a claim. Decode every selected stream with
// xerror, then use the actual output clock. A strict upper bound also prevents
// an adversarial header from turning a 30-minute admission into a day of work.
func (a *Adapter) Probe(ctx context.Context, ws clip.MediaWorkspace, path string) (clip.MediaInfo, error) {
	info, err := a.probeContainer(ctx, ws, path)
	if err != nil {
		return clip.MediaInfo{}, err
	}
	data, err := a.runLog(ctx, ws, a.cfg.FFmpegPath, "-hide_banner", "-nostdin", "-v", "info", "-xerror", "-nostats", "-stats_period", "3600", "-progress", "pipe:2", "-threads", strconv.Itoa(a.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-i", path, "-map", "0:V:0", "-map", "0:a?", "-vf", "vfrdet", "-t", seconds(a.cfg.Sources.MaxDurationMS+a.cfg.DurationToleranceMS), "-fps_mode", "passthrough", "-f", "null", "-")
	if err != nil {
		return clip.MediaInfo{}, errors.Join(clip.ErrInvalidMedia, err)
	}
	measured, ok := decodeMeasurement(data)
	if !ok {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	return a.measuredInfo(info, measured)
}

// decodeMeasurement reads what -progress reports about a decode: the frames and
// the output clock of its FIRST output, and whether every measured graph saw a
// constant cadence. A run that did not reach progress=end measured nothing.
type decoded struct {
	Frames, Micros int64
	Constant       bool
}

func decodeMeasurement(log []byte) (decoded, bool) {
	var out decoded
	complete := false
	for _, line := range strings.Split(string(log), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		switch key {
		case "frame":
			out.Frames = max(out.Frames, n)
		case "out_time_us":
			out.Micros = max(out.Micros, n)
		case "progress":
			complete = value == "end"
		}
	}
	out.Constant = constantCadenceReport(log)
	return out, complete
}

// measuredInfo settles a source's length against an actual decode — one whole
// pass, or the analysis copies' own passes summed — by today's rules.
func (a *Adapter) measuredInfo(info clip.MediaInfo, measured decoded) (clip.MediaInfo, error) {
	if measured.Frames <= 0 || measured.Micros <= 0 || measured.Micros > int64(a.cfg.Sources.MaxDurationMS)*1000 {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	info.DurationMS = int(math.Round(float64(measured.Micros) / 1000))
	info.DecodedDurationMS = info.DurationMS
	info.DecodedFrames = int(measured.Frames)
	info.CadenceVerified = measured.Constant
	// MP4 edit lists exclude AAC encoder padding from the playable timeline.
	// Cross-check every declared selected-stream endpoint against a full decode
	// before using it; otherwise a 60 s source could produce a spurious 11 ms
	// second chunk. Never let a short container declaration hide a longer stream.
	if info.VideoDurationMS > 0 && info.ContainerDurationMS >= info.VideoDurationMS && info.ContainerDurationMS >= info.AudioDurationMS && info.ContainerDurationMS <= a.cfg.Sources.MaxDurationMS && abs(info.ContainerDurationMS-info.DecodedDurationMS) <= 22 {
		info.DurationMS = info.ContainerDurationMS
	}
	// A recording cut mid-frame leaves a trailing audio packet the video never
	// reaches, so both the container and the decode clock outrun the last
	// picture. Every later stage treats this length as footage that can be
	// analysed and cut, and neither the analysis proxy nor a rendered cut pads
	// video, so the usable length is never more than the video track itself.
	if info.VideoDurationMS > 0 && info.VideoDurationMS < info.DurationMS {
		info.DurationMS = info.VideoDurationMS
	}
	return info, nil
}
func durationMS(raw string) int {
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 || n > 86400 {
		return 0
	}
	return int(math.Round(n * 1000))
}
func seconds(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }

// vfrdet observes every decoded video timestamp in the same admission pass.
// Require its final integer counts, not its rounded VFR fraction; a missing,
// truncated or malformed report remains unknown rather than implying CFR.
var cadenceReport = regexp.MustCompile(`\[Parsed_vfrdet_[^\]]+\] VFR:[^ ]+ \(([0-9]+)/([0-9]+)\)`)

func constantCadenceReport(log []byte) bool {
	measured := 0
	for _, match := range cadenceReport.FindAllSubmatch(log, -1) {
		varying, e1 := strconv.ParseUint(string(match[1]), 10, 64)
		constant, e2 := strconv.ParseUint(string(match[2]), 10, 64)
		if e1 != nil || e2 != nil || varying != 0 {
			return false
		}
		// FFmpeg also closes an unused graph during initial format negotiation.
		// Its (0/0) report is not evidence; require one actual decoded graph.
		if constant > 0 {
			measured++
		}
	}
	return measured == 1
}
