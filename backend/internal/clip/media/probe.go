package media

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

type probeDocument struct {
	Streams []struct {
		Index             int    `json:"index"`
		Codec             string `json:"codec_name"`
		Kind              string `json:"codec_type"`
		Width             int    `json:"width"`
		Height            int    `json:"height"`
		FrameRate         string `json:"avg_frame_rate"`
		SampleAspectRatio string `json:"sample_aspect_ratio"`
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

func (a *Adapter) run(ctx context.Context, ws clip.MediaWorkspace, binary string, args ...string) ([]byte, error) {
	if err := a.validWorkspace(ws, true); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, a.cfg.OperationTimeout)
	defer cancel()
	return a.runner.Run(ctx, Command{Binary: binary, Dir: ws.Path, Args: args})
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
func (a *Adapter) Probe(ctx context.Context, ws clip.MediaWorkspace, path string) (clip.MediaInfo, error) {
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
	info := clip.MediaInfo{}
	found := false
	for _, s := range doc.Streams {
		info.Streams = append(info.Streams, clip.MediaStream{Index: s.Index, Kind: s.Kind, Codec: s.Codec})
		if s.Kind == "audio" {
			info.HasAudio = true
		}
		if s.Kind != "video" || s.Disposition.Attached != 0 || found {
			continue
		}
		found = true
		if s.Width <= 0 || s.Height <= 0 || s.Width > a.cfg.MaxDimension || s.Height > a.cfg.MaxDimension {
			return clip.MediaInfo{}, clip.ErrInvalidMedia
		}
		info.Width, info.Height = s.Width, s.Height
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
	// A container's duration is only a claim. Decode every selected stream with
	// xerror, then use the actual output clock. A strict upper bound also prevents
	// an adversarial header from turning a 30-minute admission into a day of work.
	data, err = a.run(ctx, ws, a.cfg.FFmpegPath, "-hide_banner", "-nostdin", "-v", "error", "-xerror", "-nostats", "-stats_period", "3600", "-progress", "pipe:1", "-threads", strconv.Itoa(a.cfg.Threads), "-protocol_whitelist", "file,pipe", "-i", path, "-map", "0:V:0", "-map", "0:a?", "-t", seconds(a.cfg.Sources.MaxDurationMS+a.cfg.DurationToleranceMS), "-fps_mode", "passthrough", "-f", "null", "-")
	if err != nil {
		return clip.MediaInfo{}, errors.Join(clip.ErrInvalidMedia, err)
	}
	frames, micros := int64(0), int64(0)
	complete := false
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		switch key {
		case "frame":
			frames = max(frames, n)
		case "out_time_us":
			micros = max(micros, n)
		case "progress":
			complete = value == "end"
		}
	}
	if !complete || frames <= 0 || micros <= 0 || micros > int64(a.cfg.Sources.MaxDurationMS)*1000 {
		return clip.MediaInfo{}, clip.ErrInvalidMedia
	}
	info.DurationMS = int(math.Round(float64(micros) / 1000))
	return info, nil
}
func seconds(ms int) string { return strconv.FormatFloat(float64(ms)/1000, 'f', 3, 64) }
