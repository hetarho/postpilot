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
	"github.com/postpilot/backend/internal/clip/design"
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

// cutGraph composes one cut: its cover-cropped footage, the static furniture
// layer (the disclosure badge and this cut's chips, which never move) and then
// the animated copy layer. They are SEPARATE overlay inputs precisely because
// the badge may not move (CDS-31) while the copy must (CDS-4).
func cutGraph(cfg clip.RenderConfig, canvas clip.Canvas, c clip.EditCut, source clip.MediaInfo, frames int, furniture, copy bool, withAudio bool) string {
	var graph strings.Builder
	fmt.Fprintf(&graph, "[0:V:0]trim=duration=%s,setpts=PTS-STARTPTS,fps=%d,scale=%d:%d:force_original_aspect_ratio=increase:force_divisible_by=2:reset_sar=1,crop=%d:%d:x='max(0,min(iw-ow,iw*%.6f-ow/2))':y='max(0,min(ih-oh,ih*%.6f-oh/2))',setsar=1,format=yuv420p[base];", seconds(c.EndMS-c.StartMS), cfg.FPS, canvas.Width, canvas.Height, canvas.Width, canvas.Height, c.Focal.X, c.Focal.Y)
	last, input := "[base]", 1
	if furniture {
		fmt.Fprintf(&graph, "%s[%d:v:0]overlay=0:0:format=auto:shortest=0[fixed];", last, input)
		last, input = "[fixed]", input+1
	}
	if copy {
		// CDS-4 allows exactly two motions: a 180 ms fade-in that settles 12 px
		// upward, eased, and a 120 ms fade-out that does not move. Both are
		// expressions on the one looped plate image — never a second raster per
		// frame, and never anything else.
		start, end := c.CaptionWindow()
		m := design.Motion
		in, out := float64(m.InMS)/1000, float64(m.OutMS)/1000
		fmt.Fprintf(&graph, "[%d:v:0]format=rgba,fade=t=in:st=%s:d=%s:alpha=1,fade=t=out:st=%s:d=%s:alpha=1[plate];", input, seconds(start), strconv.FormatFloat(in, 'f', 3, 64), seconds(end-m.OutMS), strconv.FormatFloat(out, 'f', 3, 64))
		fmt.Fprintf(&graph, "%s[plate]overlay=x=0:y='%.0f*pow(1-min(1,max(0,(t-%s)/%s)),3)':format=auto:shortest=0", last, m.InDY, seconds(start), strconv.FormatFloat(in, 'f', 3, 64))
		fmt.Fprintf(&graph, ":enable='gte(t,%s)*lt(t,%s)'", seconds(start), seconds(end))
		graph.WriteString("[copy];[copy]")
	} else {
		graph.WriteString(last)
	}
	fmt.Fprintf(&graph, "trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p[v]", frames)
	if withAudio {
		if source.HasAudio {
			// Preserve the seek-relative audio clock before resetting timestamps.
			// An AAC frame can start one packet after video; resetting STARTPTS
			// first pulled speech ~21 ms earlier and closed real timestamp gaps.
			fmt.Fprintf(&graph, ";[0:a:0]aresample=%d:async=1:min_hard_comp=0:first_pts=0,atrim=duration=%s,aformat=sample_fmts=fltp:channel_layouts=stereo,volume=%.6f", cfg.AudioRate, seconds(c.EndMS-c.StartMS), c.OriginalVolume())
		} else {
			fmt.Fprintf(&graph, ";anullsrc=r=%d:cl=stereo", cfg.AudioRate)
		}
		fmt.Fprintf(&graph, ",apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", frames*cfg.AudioRate/cfg.FPS)
	}
	return graph.String()
}
func compositionGraph(cfg clip.RenderConfig, frames []int, audio bool) string {
	return compositionVideoGraph(cfg, frames, audio, "yuv420p")
}
func compositionVideoGraph(cfg clip.RenderConfig, frames []int, audio bool, pixelFormat string) string {
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
	fmt.Fprintf(&graph, "[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,format=%s[v]", v, elapsed, pixelFormat)
	if audio {
		fmt.Fprintf(&graph, ";[%s]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", a, elapsed*cfg.AudioRate/cfg.FPS)
	}
	return graph.String()
}
func (r *Rendering) encodeArgs(audio bool) []string {
	return r.encodeProfile(audio, r.cfg.CRF, "yuv420p")
}
func (r *Rendering) encodeProfile(audio bool, crf int, pixelFormat string) []string {
	a := []string{"-map", "[v]", "-c:v", "libx264", "-preset", "veryfast", "-crf", strconv.Itoa(crf), "-threads", strconv.Itoa(r.media.cfg.Threads), "-r", strconv.Itoa(r.cfg.FPS), "-fps_mode", "cfr", "-pix_fmt", pixelFormat}
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
func (r *Rendering) renderCut(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, cut clip.EditCut, source clip.MediaSource, frames int, fixed, plate, path string, audio bool) error {
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	for _, layer := range []string{fixed, plate} {
		if layer != "" {
			args = append(args, "-loop", "1", "-framerate", strconv.Itoa(r.cfg.FPS), "-i", layer)
		}
	}
	args = append(args, "-filter_complex", cutGraph(r.cfg, canvas, cut, source.Info, frames, fixed != "", plate != "", audio))
	args = append(args, r.encodeArgs(audio)...)
	args = append(args, "-t", frameSeconds(frames, r.cfg.FPS))
	if err := r.runRender(ctx, ws, path, args); err != nil {
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
	// Lay out and verify every caption BEFORE asking the consumer for source
	// bytes: a plan that breaks the design system then costs no download and no
	// FFmpeg run (CDS-52).
	layouts, furnitures, manifest, err := r.layout(ctx, ws, canvas, plan)
	if err != nil {
		return result, err
	}
	if err = clip.VerifyLayout(plan.Ratio, manifest); err != nil {
		return result, err
	}
	// Two overlay images per cut: the fixed layer (the disclosure badge and this
	// cut's chips, which never move) and the copy's own plate, which is the only
	// thing that fades and settles.
	fixed, plates := make([]string, len(plan.Cuts)), make([]string, len(plan.Cuts))
	for i, cut := range plan.Cuts {
		if fixed[i], err = r.furniturePlate(ctx, ws, canvas, furnitures[i], i); err != nil {
			return result, err
		}
		if plates[i], err = r.copyPlate(ctx, ws, canvas, cut.Copy, layouts[i], i); err != nil {
			return result, err
		}
		for _, layer := range []string{fixed[i], plates[i]} {
			if layer != "" {
				paths = append(paths, layer)
			}
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
			return r.renderCut(ctx, ws, canvas, cut, source, frames[i], fixed[i], plates[i], cutPaths[i], audio)
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
	args, err := r.compositionInputs(ctx, ws, cutPaths, frames, audio, &paths)
	if err != nil {
		return result, err
	}
	args = append(args, r.encodeArgs(audio)...)
	if err = r.runRender(ctx, ws, output, args); err != nil {
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
	return clip.RenderedVideo{Path: output, Info: info, Bytes: stat.Size(), Manifest: manifest}, nil
}

// Output-timeline offset of each cut: the cuts overlap by one fade each.
func cutOffsets(plan clip.EditPlan, fadeMS int) []int {
	offsets, elapsed := make([]int, len(plan.Cuts)), 0
	for i, c := range plan.Cuts {
		offsets[i] = elapsed
		elapsed += c.EndMS - c.StartMS - fadeMS
	}
	return offsets
}

// layout measures and places every copy, the disclosure badge and the chips
// without touching one source pixel, so the manifest and its verification come
// before any download.
func (r *Rendering) layout(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, plan clip.EditPlan) ([]copyLayout, []furniture, clip.Manifest, error) {
	layouts, plates, manifest := make([]copyLayout, len(plan.Cuts)), make([]furniture, len(plan.Cuts)), clip.Manifest{}
	offsets := cutOffsets(plan, r.cfg.FadeMS)
	answers := map[string]string{}
	for _, a := range plan.Facts {
		answers[a.Label] = a.Text
	}
	phrase, ok := design.Disclosure[plan.Disclosure]
	if !ok {
		// A plan reaching the renderer without a campaign type is refused here
		// rather than rendered without its badge (CDS-5).
		return nil, nil, nil, clip.ErrDisclosureRequired
	}
	badged := false
	for i, cut := range plan.Cuts {
		labels := plan.ChipLabels(cut)
		// The badge rides the first cut's furniture plate; every later cut gets
		// one only if it carries chips, and the badge is drawn on all of them so
		// it is present for the whole clip (CDS-5).
		f, err := r.badgeAndChips(ctx, ws, canvas, plan.Ratio, phrase, labels, answers)
		if err != nil {
			return nil, nil, nil, err
		}
		plates[i] = f
		start, end := offsets[i], offsets[i]+cut.EndMS-cut.StartMS
		elements := f.Elements(plan.DurationMS, i, start, end)
		if badged {
			elements = elements[1:] // one badge in the manifest, one disclosure
		}
		badged = true
		manifest = append(manifest, elements...)
		if strings.TrimSpace(cut.Copy.Text) == "" {
			continue
		}
		l, err := r.layoutCopy(ctx, ws, canvas, cut.Copy)
		if err != nil {
			return nil, nil, nil, err
		}
		layouts[i] = l
		copyStart, copyEnd := cut.CaptionWindow()
		manifest = append(manifest, l.Elements(i, cut.Copy, offsets[i]+copyStart, offsets[i]+copyEnd)...)
	}
	return layouts, plates, manifest, nil
}

// Layout is the same pass on its own workspace, for a caller that wants the
// manifest without rendering: it downloads nothing and writes no video.
func (r *Rendering) Layout(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (manifest clip.Manifest, err error) {
	if err = clip.ValidateEditPlan(r.cfg, plan, sources); err != nil {
		return nil, err
	}
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	err = r.media.WithWorkspace(ctx, "clip-layout", func(ws clip.MediaWorkspace) error {
		_, _, m, err := r.layout(ctx, ws, canvas, plan)
		if err != nil {
			return err
		}
		manifest = m
		return clip.VerifyLayout(plan.Ratio, m)
	})
	return manifest, err
}

func (r *Rendering) runRender(ctx context.Context, ws clip.MediaWorkspace, output string, args []string) error {
	// One file is written at a time. Reserve/check its entire file bound before
	// starting the encoder and leave muxer/packet headroom inside that bound.
	// -fs is a resource stop only: reaching it fails, never a successful render.
	ceiling := r.media.cfg.Sources.MaxFileBytes
	if ceiling <= diskHeadroom {
		return clip.ErrWorkspaceLimit
	}
	if err := r.media.capacity(ws, ceiling); err != nil {
		return err
	}
	limit := ceiling - diskHeadroom
	args = append(args, "-fs", strconv.FormatInt(limit, 10), output)
	_, err := r.media.runBounded(ctx, ws, r.media.cfg.FFmpegPath, output, limit, clip.ErrWorkspaceLimit, args...)
	if err != nil {
		return err
	}
	info, err := os.Stat(output)
	if err != nil {
		return err
	}
	if info.Size() >= limit {
		return clip.ErrWorkspaceLimit
	}
	return nil
}
