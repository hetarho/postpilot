package media

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// Rounding cumulative cut time (not every individual cut independently) keeps
// the final CFR duration within half a frame of the millisecond plan.
func cutFrames(plan clip.EditPlan, fps int) []int {
	frames := make([]int, len(plan.Cuts))
	elapsed, previous := 0, 0
	for i, c := range plan.Cuts {
		elapsed += c.EndMS - c.StartMS
		next := int(math.Round(float64(elapsed*fps) / 1000))
		frames[i] = next - previous
		previous = next
	}
	return frames
}
func frameSeconds(frames, fps int) string {
	return strconv.FormatFloat(float64(frames)/float64(fps), 'f', 9, 64)
}
func cutGraph(cfg clip.RenderConfig, canvas clip.Canvas, c clip.EditCut, source clip.MediaInfo, frames int, plate, withAudio bool) string {
	var graph strings.Builder
	fmt.Fprintf(&graph, "[0:V:0]trim=duration=%s,setpts=PTS-STARTPTS,fps=%d,scale=%d:%d:force_original_aspect_ratio=increase:force_divisible_by=2:reset_sar=1,crop=%d:%d:x='max(0,min(iw-ow,iw*%.6f-ow/2))':y='max(0,min(ih-oh,ih*%.6f-oh/2))',setsar=1,format=yuv420p[base];", seconds(c.EndMS-c.StartMS), cfg.FPS, canvas.Width, canvas.Height, canvas.Width, canvas.Height, c.Focal.X, c.Focal.Y)
	if plate {
		graph.WriteString("[base][1:v:0]overlay=0:0:format=auto:shortest=0")
		if c.Copy.StartMS != 0 || c.Copy.EndMS != 0 {
			fmt.Fprintf(&graph, ":enable='gte(t,%s)*lt(t,%s)'", seconds(c.Copy.StartMS), seconds(c.Copy.EndMS))
		}
		graph.WriteString("[copy];[copy]")
	} else {
		graph.WriteString("[base]")
	}
	fmt.Fprintf(&graph, "trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p[v]", frames)
	if withAudio {
		if source.HasAudio {
			fmt.Fprintf(&graph, ";[0:a:0]atrim=duration=%s,asetpts=PTS-STARTPTS,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo,volume=%.6f", seconds(c.EndMS-c.StartMS), cfg.AudioRate, c.OriginalVolume())
		} else {
			fmt.Fprintf(&graph, ";anullsrc=r=%d:cl=stereo", cfg.AudioRate)
		}
		fmt.Fprintf(&graph, ",apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", frames*cfg.AudioRate/cfg.FPS)
	}
	return graph.String()
}
func compositionGraph(cfg clip.RenderConfig, frames []int, audio bool) string {
	var graph strings.Builder
	for i, n := range frames {
		fmt.Fprintf(&graph, "[%d:v:0]settb=AVTB,setpts=PTS-STARTPTS[v%d];", i, i)
		if audio {
			fmt.Fprintf(&graph, "[%d:a:0]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a%d];", i, n*cfg.AudioRate/cfg.FPS, i)
		}
	}
	v, a := "v0", "a0"
	elapsed := frames[0]
	fadeFrames := cfg.FadeMS * cfg.FPS / 1000
	for i := 1; i < len(frames); i++ {
		fmt.Fprintf(&graph, "[%s][v%d]xfade=transition=fade:duration=%s:offset=%s[vx%d];", v, i, seconds(cfg.FadeMS), frameSeconds(elapsed-fadeFrames, cfg.FPS), i)
		v = fmt.Sprintf("vx%d", i)
		if audio {
			fmt.Fprintf(&graph, "[%s][a%d]acrossfade=ns=%d:c1=tri:c2=tri[ax%d];", a, i, cfg.FadeMS*cfg.AudioRate/1000, i)
			a = fmt.Sprintf("ax%d", i)
		}
		elapsed += frames[i] - fadeFrames
	}
	fmt.Fprintf(&graph, "[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p[v]", v, elapsed)
	if audio {
		fmt.Fprintf(&graph, ";[%s]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", a, elapsed*cfg.AudioRate/cfg.FPS)
	}
	return graph.String()
}
func (r *Rendering) encodeArgs(audio bool) []string {
	a := []string{"-map", "[v]", "-c:v", "libx264", "-preset", "veryfast", "-crf", strconv.Itoa(r.cfg.CRF), "-threads", strconv.Itoa(r.media.cfg.Threads), "-r", strconv.Itoa(r.cfg.FPS), "-fps_mode", "cfr", "-pix_fmt", "yuv420p"}
	if audio {
		a = append(a, "-map", "[a]", "-c:a", "aac", "-b:a", strconv.Itoa(r.cfg.AudioBitrate), "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2")
	} else {
		a = append(a, "-an")
	}
	return append(a, "-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1", "-metadata:s:v:0", "rotate=0", "-movflags", "+faststart", "-f", "mp4")
}
func (r *Rendering) baseArgs() []string {
	return []string{"-hide_banner", "-nostdin", "-v", "error", "-xerror", "-n", "-filter_complex_threads", strconv.Itoa(r.media.cfg.Threads)}
}
func (r *Rendering) renderCut(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, cut clip.EditCut, source clip.MediaSource, frames int, plate, path string, audio bool) error {
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	if plate != "" {
		args = append(args, "-loop", "1", "-framerate", strconv.Itoa(r.cfg.FPS), "-i", plate)
	}
	args = append(args, "-filter_complex", cutGraph(r.cfg, canvas, cut, source.Info, frames, plate != "", audio))
	args = append(args, r.encodeArgs(audio)...)
	args = append(args, "-t", frameSeconds(frames, r.cfg.FPS), path)
	if _, err := r.media.run(ctx, ws, r.media.cfg.FFmpegPath, args...); err != nil {
		return err
	}
	return r.media.sourcePath(ws, path)
}
func (r *Rendering) Render(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (result clip.RenderedVideo, err error) {
	if err = clip.ValidateEditPlan(r.cfg, plan, sources); err != nil {
		return result, err
	}
	if load == nil {
		return result, clip.ErrInvalid
	}
	if err = r.media.validWorkspace(ws, true); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	frames := cutFrames(plan, r.cfg.FPS)
	byID := map[string]clip.RenderSource{}
	audio := false
	for _, source := range sources {
		byID[source.ID] = source
	}
	for _, cut := range plan.Cuts {
		audio = audio || byID[cut.SourceID].Info.HasAudio
	}
	paths := []string{}
	defer func() {
		for _, path := range paths {
			if e := os.Remove(path); e != nil && !os.IsNotExist(e) {
				err = errors.Join(err, e)
			}
		}
	}()
	// Validate/rasterize every caption before asking the consumer for source bytes.
	plates := make([]string, len(plan.Cuts))
	for i, cut := range plan.Cuts {
		plates[i], err = r.copyPlate(ctx, ws, canvas, cut.Copy, i)
		if err != nil {
			return result, err
		}
		if plates[i] != "" {
			paths = append(paths, plates[i])
		}
	}
	cutPaths := make([]string, len(plan.Cuts))
	for i, cut := range plan.Cuts {
		cutPaths[i] = filepath.Join(ws.Path, fmt.Sprintf("render-cut-%04d.mp4", i))
		paths = append(paths, cutPaths[i])
		calls := 0
		err = load(ctx, cut.SourceID, func(source clip.MediaSource) error {
			calls++
			expected := byID[cut.SourceID]
			if calls != 1 || source.SourceID != cut.SourceID || source.Fingerprint != cut.Fingerprint || source.Info.DurationMS != expected.Info.DurationMS || source.Info.Width != expected.Info.Width || source.Info.Height != expected.Info.Height || source.Info.HasAudio != expected.Info.HasAudio {
				return clip.ErrInvalidMedia
			}
			return r.renderCut(ctx, ws, canvas, cut, source, frames[i], plates[i], cutPaths[i], audio)
		})
		if err != nil {
			return result, err
		}
		if calls != 1 {
			return result, clip.ErrInvalidMedia
		}
	}
	output := filepath.Join(ws.Path, "clip-result.mp4")
	if _, e := os.Lstat(output); !os.IsNotExist(e) {
		return result, errors.New("clip result path already exists")
	}
	defer func() {
		if err != nil {
			if e := os.Remove(output); e != nil && !os.IsNotExist(e) {
				err = errors.Join(err, e)
			}
		}
	}()
	args := r.baseArgs()
	for _, path := range cutPaths {
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-i", path)
	}
	args = append(args, "-filter_complex", compositionGraph(r.cfg, frames, audio))
	args = append(args, r.encodeArgs(audio)...)
	args = append(args, output)
	if _, err = r.media.run(ctx, ws, r.media.cfg.FFmpegPath, args...); err != nil {
		return result, err
	}
	info, err := r.media.Probe(ctx, ws, output)
	if err != nil {
		return result, err
	}
	if info.Width != canvas.Width || info.Height != canvas.Height || info.Rotation != 0 || info.PixelFormat != "yuv420p" || info.SampleAspectRatio != "1:1" || info.FrameRateNumerator != r.cfg.FPS*info.FrameRateDenominator || info.HasAudio != audio || math.Abs(float64(info.DurationMS-plan.DurationMS)) > 1000/float64(r.cfg.FPS) {
		return result, errors.New("rendered clip failed output validation")
	}
	for _, s := range info.Streams {
		if s.Kind == "video" && s.Codec != "h264" || s.Kind == "audio" && s.Codec != "aac" {
			return result, errors.New("rendered clip codec mismatch")
		}
	}
	stat, err := os.Stat(output)
	if err != nil {
		return result, err
	}
	return clip.RenderedVideo{Path: output, Info: info, Bytes: stat.Size()}, nil
}
