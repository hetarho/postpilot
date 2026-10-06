package media

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/postpilot/backend/internal/clip"
	"math"
	"os"

	"sort"
	"strconv"
)

// The compact explicit codec avoids relying on Go field-name conversion.
type analysisPacket struct {
	StreamIndex int    `json:"stream_index"`
	PTS         string `json:"pts_time"`
	DTS         string `json:"dts_time"`
	Duration    string `json:"duration_time"`
	Size        string `json:"size"`
}
type analysisPackets struct {
	Packets []analysisPacket `json:"packets"`
}

// The tested analysis AAC encoders include 2112 priming samples plus less than
// one 1024-sample tail unit (Apple TN2258). This is a raw codec allowance only:
// presented duration and every decoded sample still satisfy the strict copy
// contract. It does not permit extra audible source coverage or hidden tails.
const analysisAACRawPaddingSamples = 2112 + 1024 - 1

// VerifyAnalysisCopy scans through packet EOF before decoding. A clipped -t
// decode is not EOF evidence: short headers/edit lists can hide a longer tail.
// Only the submitted bounded MP4 is read, never its source original.
func (a *Adapter) VerifyAnalysisCopy(ctx context.Context, ws clip.MediaWorkspace, path string, c clip.AnalysisCopy) (clip.AnalysisCopyVerification, error) {
	out := clip.AnalysisCopyVerification{Slot: c.Slot, Bytes: c.Bytes, Digest: c.Digest, Provenance: clip.AnalysisCopyProvenance}
	if e := a.sourcePath(ws, path); e != nil {
		return out, e
	}
	file, e := os.Lstat(path)
	if e != nil || !file.Mode().IsRegular() || file.Size() != c.Bytes || c.Bytes <= 0 || c.Bytes > a.cfg.AnalysisMaxBytes {
		return out, clip.ErrInvalidMedia
	}
	ctx, cancel := context.WithTimeout(ctx, clip.AnalysisVerificationTimeout)
	defer cancel()
	info, e := a.probeContainerWithOptions(ctx, ws, path, []string{"-threads", strconv.Itoa(a.cfg.DecodeThreads), "-err_detect", "explode", "-max_pixels", strconv.Itoa(a.cfg.LongEdge * a.cfg.LongEdge), "-max_alloc", strconv.Itoa(clip.AnalysisVerificationAllocationBytes)})
	if e != nil {
		return out, clip.ErrInvalidMedia
	}
	frame := (1000 + a.cfg.FPS - 1) / a.cfg.FPS
	video, audio := 0, 0
	for _, s := range info.Streams {
		switch s.Kind {
		case "video":
			video++
			if s.Codec != "h264" {
				return out, clip.ErrInvalidMedia
			}
		case "audio":
			audio++
			if s.Codec != "aac" {
				return out, clip.ErrInvalidMedia
			}
		default:
			return out, clip.ErrInvalidMedia
		}
	}
	if video != 1 || audio != boolMedia(c.HasAudio) || info.Width != c.Width || info.Height != c.Height || info.Rotation != 0 || info.PixelFormat != "yuv420p" || info.SampleAspectRatio != "1:1" || info.HasAudio != c.HasAudio || info.FrameRateDenominator <= 0 || info.FrameRateNumerator != a.cfg.FPS*info.FrameRateDenominator || info.ContainerDurationMS <= 0 || info.ContainerDurationMS > 60000 || info.VideoDurationMS <= 0 || info.VideoDurationMS > 60000 || abs(info.ContainerDurationMS-c.DurationMS) > frame || abs(info.VideoDurationMS-c.DurationMS) > frame || c.HasAudio && (info.AudioChannels != 1 || info.AudioRate != a.cfg.AudioRate || abs(info.AudioDurationMS-c.DurationMS) > 22) {
		return out, clip.ErrInvalidMedia
	}
	runner := a.runner
	if _, ok := runner.(ExecRunner); ok {
		runner = ExecRunner{StdoutLimit: clip.AnalysisVerificationLogBytes, StderrLimit: clip.AnalysisVerificationLogBytes, WaitDelay: a.cfg.WaitDelay}
	}
	data, e := runner.Run(ctx, Command{Binary: a.cfg.FFprobePath, Dir: ws.Path, RejectStderr: true, Args: []string{"-v", "error", "-max_alloc", strconv.Itoa(clip.AnalysisVerificationAllocationBytes), "-protocol_whitelist", "file,pipe", "-ignore_editlist", "1", "-show_packets", "-show_entries", "packet=stream_index,pts_time,dts_time,duration_time,size", "-of", "json", path}})
	if e != nil {
		return out, clip.ErrInvalidMedia
	}
	out.VideoPackets, out.AudioPackets, e = validateAnalysisPackets(data, info, c, a.cfg)
	if e != nil {
		return out, fmt.Errorf("%w: packet timeline", e)
	}
	framesData, e := runner.Run(ctx, Command{Binary: a.cfg.FFprobePath, Dir: ws.Path, RejectStderr: true, Args: []string{"-v", "error", "-threads", strconv.Itoa(a.cfg.DecodeThreads), "-err_detect", "explode", "-max_pixels", strconv.Itoa(a.cfg.LongEdge * a.cfg.LongEdge), "-max_alloc", strconv.Itoa(clip.AnalysisVerificationAllocationBytes), "-protocol_whitelist", "file,pipe", "-show_frames", "-show_entries", "frame=media_type,best_effort_timestamp_time,width,height,pix_fmt,sample_aspect_ratio,nb_samples,channels,channel_layout:frame_side_data=", "-of", "json", path}})
	if e != nil {
		return out, clip.ErrInvalidMedia
	}
	frames, samples, e := validateAnalysisFrames(framesData, c, a.cfg)
	if e != nil {
		return out, fmt.Errorf("%w: decoded frame bounds", e)
	}
	info.DurationMS = info.ContainerDurationMS
	info.DecodedFrames = frames
	info.CadenceVerified = frames > 1
	info.DecodedDurationMS = int(math.Round(math.Max(float64(frames)*1000/float64(a.cfg.FPS), float64(samples)*1000/float64(a.cfg.AudioRate))))
	out.Info, out.AudioSamples = info, samples
	if e = clip.ValidateAnalysisCopyVerification(c, out, a.cfg); e != nil {
		return out, fmt.Errorf("%w: measured copy contract", e)
	}
	return out, nil
}
func boolMedia(v bool) int {
	if v {
		return 1
	}
	return 0
}
func finiteTime(value string) (float64, bool) {
	v, e := strconv.ParseFloat(value, 64)
	return v, e == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}
func validateAnalysisPackets(data []byte, info clip.MediaInfo, c clip.AnalysisCopy, cfg clip.MediaConfig) (video, audio int, err error) {
	if len(data) > clip.AnalysisVerificationLogBytes {
		return 0, 0, clip.ErrInvalidMedia
	}
	var doc analysisPackets
	if json.Unmarshal(data, &doc) != nil || len(doc.Packets) == 0 || len(doc.Packets) > clip.AnalysisVerificationPacketMax {
		return 0, 0, clip.ErrInvalidMedia
	}
	kinds := map[int]string{}
	for _, s := range info.Streams {
		kinds[s.Index] = s.Kind
	}
	times := map[string][]float64{}
	var bytes int64
	for _, p := range doc.Packets {
		kind := kinds[p.StreamIndex]
		pts, ok := finiteTime(p.PTS)
		dts, ok2 := finiteTime(p.DTS)
		duration, ok3 := finiteTime(p.Duration)
		size, e := strconv.ParseInt(p.Size, 10, 64)
		if !ok || !ok2 || !ok3 || e != nil || size <= 0 || size > c.Bytes-bytes || duration <= 0 {
			return 0, 0, clip.ErrInvalidMedia
		}
		bytes += size
		guard := 4.0 / float64(cfg.FPS)
		maxDuration := 1.0/float64(cfg.FPS) + 0.000002
		if kind == "audio" {
			guard = float64(analysisAACRawPaddingSamples) / float64(cfg.AudioRate)
			maxDuration = 1024.0/float64(cfg.AudioRate) + 0.000002
		}
		if kind != "audio" && kind != "video" || pts < -guard || dts < -guard || pts+duration > 60+guard || dts > 60+guard || duration > maxDuration {
			return 0, 0, clip.ErrInvalidMedia
		}
		times[kind] = append(times[kind], pts)
		if kind == "video" {
			video++
		} else {
			audio++
		}
	}
	if video == 0 || video > cfg.FPS*60 || c.HasAudio != (audio > 0) {
		return 0, 0, clip.ErrInvalidMedia
	}
	for kind, pts := range times {
		sort.Float64s(pts)
		step := 1.0 / float64(cfg.FPS)
		guard := step
		if kind == "audio" {
			step = 1024.0 / float64(cfg.AudioRate)
			guard = float64(analysisAACRawPaddingSamples) / float64(cfg.AudioRate)
		}
		for i := 1; i < len(pts); i++ {
			if math.Abs(pts[i]-pts[i-1]-step) > 0.000002 {
				return 0, 0, clip.ErrInvalidMedia
			}
		}
		if math.Abs((pts[len(pts)-1]-pts[0]+step)*1000-float64(c.DurationMS)) > guard*1000+1 {
			return 0, 0, clip.ErrInvalidMedia
		}
	}
	return video, audio, nil
}

type analysisDecodedFrame struct {
	Kind          string `json:"media_type"`
	PTS           string `json:"best_effort_timestamp_time"`
	Width, Height int
	PixelFormat   string `json:"pix_fmt"`
	SAR           string `json:"sample_aspect_ratio"`
	Samples       int64  `json:"nb_samples"`
	Channels      int    `json:"channels"`
	Layout        string `json:"channel_layout"`
}

func validateAnalysisFrames(data []byte, c clip.AnalysisCopy, cfg clip.MediaConfig) (frames int, samples int64, err error) {
	if len(data) > clip.AnalysisVerificationLogBytes {
		return 0, 0, clip.ErrInvalidMedia
	}
	var doc struct {
		Frames []analysisDecodedFrame `json:"frames"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Frames) == 0 || len(doc.Frames) > clip.AnalysisVerificationPacketMax {
		return 0, 0, clip.ErrInvalidMedia
	}
	for _, f := range doc.Frames {
		pts, ok := finiteTime(f.PTS)
		if !ok {
			return 0, 0, clip.ErrInvalidMedia
		}
		switch f.Kind {
		case "video":
			if f.Width != c.Width || f.Height != c.Height || f.PixelFormat != "yuv420p" || f.SAR != "1:1" || math.Abs(pts-float64(frames)/float64(cfg.FPS)) > 0.000002 {
				return 0, 0, clip.ErrInvalidMedia
			}
			frames++
			if frames > cfg.FPS*60 {
				return 0, 0, clip.ErrInvalidMedia
			}
		case "audio":
			if !c.HasAudio || f.Channels != 1 || f.Layout != "mono" || f.Samples <= 0 || f.Samples > 1024 || math.Abs(pts-float64(samples)/float64(cfg.AudioRate)) > 0.000002 {
				return 0, 0, clip.ErrInvalidMedia
			}
			samples += f.Samples
			if samples > int64(cfg.AudioRate)*60+1024 {
				return 0, 0, clip.ErrInvalidMedia
			}
		default:
			return 0, 0, clip.ErrInvalidMedia
		}
	}
	if frames == 0 || c.HasAudio != (samples > 0) {
		return 0, 0, clip.ErrInvalidMedia
	}
	return
}
