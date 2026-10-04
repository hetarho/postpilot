package media

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// rateChain is the ONE way a fixed rate becomes pixels: the decoded timestamps
// are scaled and the result is converted to the output cadence by ordinary
// frame-rate conversion. There is no interpolation, synthesis, reverse or
// freeze anywhere in it, and 1x emits exactly the graph it always did so an
// existing result re-renders byte-for-byte (CLIP-28, CLIP-99, CLIP-101).
func rateChain(ratePermille, fps int) string {
	if ratePermille == clip.RateUnitPermille {
		return fmt.Sprintf("setpts=PTS-STARTPTS,fps=%d", fps)
	}
	return fmt.Sprintf("setpts=(PTS-STARTPTS)*%d/%d,fps=%d", clip.RateUnitPermille, ratePermille, fps)
}

// audioRateChain is the same transform for sound, pitch-preserved. Every
// CLIP-98 rate lies inside atempo's single-stage 0.5-2.0 range, so one stage is
// always enough and no chain of stages can compound rounding.
func audioRateChain(ratePermille int) string {
	if ratePermille == clip.RateUnitPermille {
		return ""
	}
	return fmt.Sprintf(",atempo=%s", strconv.FormatFloat(float64(ratePermille)/float64(clip.RateUnitPermille), 'f', 6, 64))
}

// coverChain is the one definition of how a source becomes canvas pixels: scaled
// to cover and cropped around the cut's focal point. The sampler extracts its
// frames through the same chain, so the luminance it measures is the luminance
// the viewer sees, not the raw source's (CDS-44).
func coverChain(canvas clip.Canvas, focal clip.Point) string {
	return fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=increase:force_divisible_by=2:reset_sar=1,crop=%d:%d:x='max(0,min(iw-ow,iw*%.6f-ow/2))':y='max(0,min(ih-oh,ih*%.6f-oh/2))'",
		canvas.Width, canvas.Height, canvas.Width, canvas.Height, focal.X, focal.Y)
}

func frameSeconds(frames, fps int) string {
	return strconv.FormatFloat(float64(frames)/float64(fps), 'f', 9, 64)
}

// compositionGraph joins a timeline's entries into one picture: each enters on
// its own clock, a hard cut CONCATENATES and a transition is the xfade its kind
// names, from the frame the incoming entry starts on (CDS-36).
func compositionGraph(t cutTimeline, pixelFormat string) string {
	var graph strings.Builder
	for i := range t.frames {
		fmt.Fprintf(&graph, "[%d:v:0]settb=AVTB,setpts=PTS-STARTPTS[v%d];", i, i)
	}
	v := "v0"
	for i := 1; i < len(t.frames); i++ {
		// A hard cut is a JOIN, not a zero-length dissolve: concat keeps both
		// cuts' own frames intact where xfade would resample the boundary.
		if t.overlaps[i] == 0 {
			fmt.Fprintf(&graph, "[%s][v%d]concat=n=2:v=1:a=0[vx%d];", v, i, i)
		} else {
			fmt.Fprintf(&graph, "[%s][v%d]xfade=transition=%s:duration=%s:offset=%s[vx%d];", v, i, t.kinds[i], seconds(t.transitions[i]), frameSeconds(t.starts[i], t.fps), i)
		}
		v = fmt.Sprintf("vx%d", i)
	}
	fmt.Fprintf(&graph, "[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,format=%s[v]", v, t.total, pixelFormat)
	return graph.String()
}
func (r *Rendering) encodeArgs(audio bool) []string {
	return r.encodeProfile(audio, r.cfg.CRF, "yuv420p")
}

// V12 asks the delivered clip for H.264 High, so the final encode NAMES it
// rather than trusting libx264's default. The lossless 4:4:4 tree nodes cannot
// carry it and are never delivered.
const h264Profile = "High"

func (r *Rendering) encodeProfile(audio bool, crf int, pixelFormat string) []string {
	preset := "veryfast"
	if crf == 0 {
		// Intermediate nodes are lossless. Spend disk bytes instead of CPU on
		// compressing pixels that will be decoded and removed at the next merge.
		// Delivered video still uses the configured CRF and veryfast preset.
		preset = "ultrafast"
	}
	a := []string{"-map", "[v]", "-c:v", "libx264", "-preset", preset, "-crf", strconv.Itoa(crf), "-threads", strconv.Itoa(r.media.cfg.EncodeThreads), "-r", strconv.Itoa(r.cfg.FPS), "-fps_mode", "cfr", "-pix_fmt", pixelFormat}
	if pixelFormat == "yuv420p" {
		a = append(a, "-profile:v", strings.ToLower(h264Profile))
	}
	if audio {
		a = append(a, "-map", "[a]", "-c:a", "aac", "-b:a", strconv.Itoa(r.cfg.AudioBitrate), "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2")
	} else {
		a = append(a, "-an")
	}
	return append(a, "-sn", "-dn", "-map_metadata", "-1", "-map_chapters", "-1", "-metadata:s:v:0", "rotate=0", "-movflags", "+faststart", "-f", "mp4")
}
func (r *Rendering) baseArgs() []string {
	return []string{"-hide_banner", "-nostdin", "-v", "error", "-xerror", "-n", "-filter_threads", strconv.Itoa(r.media.cfg.EncodeThreads), "-filter_complex_threads", strconv.Itoa(r.media.cfg.EncodeThreads)}
}

// preparePlan is shared by admission/layout and execution so rate refusals
// cannot first appear after a render has started.
func (r *Rendering) preparePlan(plan clip.EditPlan, sources []clip.RenderSource) (clip.EditPlan, error) {
	// Every rate is rechecked against the ORIGINAL's own verified cadence before
	// a single FFmpeg process starts. An unsuitable one is IDENTIFIED, never
	// simulated and never quietly replaced by 1x (CLIP-99, CDS-68).
	if err := clip.RefuseUnrenderableRates(plan, sources); err != nil {
		return plan, err
	}
	return plan, nil
}

func (r *Rendering) Render(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (result clip.RenderedVideo, err error) {
	plan, err = r.preparePlan(plan, sources)
	if err != nil {
		return result, err
	}
	// The composition is the one renderer. A plan written before it is refused
	// here, before any source byte or FFmpeg run, rather than drawn another way.
	if plan.Portable == nil {
		return result, clip.ErrCompositionUnavailable
	}
	return r.renderComposition(ctx, ws, plan, sources, load)
}

var errOutputValidation = errors.New("rendered clip failed output validation")

// A rejected delivery names the property that missed and carries what was
// measured against what was required. One combined verdict cannot say whether
// the clip came back the wrong size, the wrong length or the wrong codec, and
// that is the whole question once a finished encode is refused (CDS-52).
func outputRejection(check string, values map[string]int) error {
	return clip.WithAttemptDiagnostic(errOutputValidation, clip.AttemptDiagnostic{Check: check, Phase: "render", Values: values})
}

func (r *Rendering) validateRenderedOutput(ctx context.Context, ws clip.MediaWorkspace, output string, plan clip.EditPlan, audio bool, manifest clip.Manifest) (result clip.RenderedVideo, err error) {
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	info, err := r.media.Probe(ctx, ws, output)
	if err != nil {
		return result, err
	}
	if info.Width != canvas.Width || info.Height != canvas.Height {
		return result, outputRejection("render_output_canvas", map[string]int{"width": info.Width, "height": info.Height, "expected_width": canvas.Width, "expected_height": canvas.Height})
	}
	if info.Rotation != 0 {
		return result, outputRejection("render_output_rotation", map[string]int{"rotation": info.Rotation})
	}
	if info.PixelFormat != "yuv420p" {
		return result, outputRejection("render_output_pixel_format", nil)
	}
	if info.SampleAspectRatio != "1:1" {
		return result, outputRejection("render_output_aspect", nil)
	}
	if info.FrameRateNumerator != r.cfg.FPS*info.FrameRateDenominator {
		return result, outputRejection("render_output_frame_rate", map[string]int{"frame_rate_numerator": info.FrameRateNumerator, "frame_rate_denominator": info.FrameRateDenominator, "expected_fps": r.cfg.FPS})
	}
	if info.HasAudio != audio {
		return result, outputRejection("render_output_audio", nil)
	}
	// The delivered length is the decoded video track and nothing else: an AAC
	// track declares its own codec padding, so a container or audio reading is
	// always at least as long as the clip actually plays (CDS-52 V12). The rate
	// check above has already pinned the output to cfg.FPS, which is what makes
	// the frame count an exact clock here. One frame either side is where a CFR
	// timeline can land; the other two readings ride along because a diagnosis
	// is exactly the comparison between them.
	delivered := float64(info.DecodedFrames) * 1000 / float64(r.cfg.FPS)
	if math.Abs(delivered-float64(plan.DurationMS)) > 1000/float64(r.cfg.FPS) {
		return result, outputRejection("render_output_duration", map[string]int{"duration_ms": int(math.Round(delivered)), "expected_duration_ms": plan.DurationMS, "video_frames": info.DecodedFrames, "decoded_duration_ms": info.DecodedDurationMS, "container_duration_ms": info.ContainerDurationMS})
	}
	// V12 reads the DELIVERED file, not the arguments that produced it: the
	// canvas, 30 fps, H.264 High and 48 kHz AAC (CDS-52).
	for _, s := range info.Streams {
		if s.Kind == "video" && (s.Codec != "h264" || s.Profile != h264Profile) || s.Kind == "audio" && s.Codec != "aac" {
			return result, outputRejection("render_output_codec", map[string]int{"stream_index": s.Index})
		}
	}
	if audio && info.AudioRate != r.cfg.AudioRate {
		return result, outputRejection("render_output_audio_rate", map[string]int{"audio_rate": info.AudioRate, "expected_audio_rate": r.cfg.AudioRate})
	}
	stat, err := os.Stat(output)
	if err != nil {
		return result, err
	}
	return clip.RenderedVideo{Path: output, Info: info, Bytes: stat.Size(), Manifest: manifest}, nil
}

// Layout is the composition's layout pass, for a caller that wants the manifest
// without rendering: it downloads nothing and writes no video.
func (r *Rendering) Layout(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (repaired clip.EditPlan, manifest clip.Manifest, err error) {
	if plan.Portable == nil {
		return plan, nil, clip.ErrCompositionUnavailable
	}
	var elements []clip.CompositionElement
	repaired, elements, err = r.LayoutComposition(ctx, plan, sources)
	for _, element := range elements {
		manifest = append(manifest, element.Parts...)
	}
	return repaired, manifest, err
}

// ValidateRenderPlan includes the legacy conversion execution will perform.
// Layout itself stays a read of the supplied representation, used by previews
// and diagnostics that must preserve their input's words and structure.
func (r *Rendering) ValidateRenderPlan(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (clip.EditPlan, error) {
	plan, err := r.preparePlan(plan, sources)
	if err != nil {
		return plan, err
	}
	plan, _, err = r.Layout(ctx, plan, sources)
	return plan, err
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
