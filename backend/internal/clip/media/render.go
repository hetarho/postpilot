package media

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
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

// cutGraph composes one cut: its cover-cropped footage, the static furniture
// layer (the disclosure badge and this cut's chips, which never move) and then
// the caption layers. They are separate overlay inputs because
// the badge stays fixed (CDS-31) while captions follow their pace (CDS-4).
func cutGraph(cfg clip.RenderConfig, canvas clip.Canvas, c clip.EditCut, source clip.MediaInfo, frames int, l layers, withAudio bool) string {
	var graph strings.Builder
	fmt.Fprintf(&graph, "[0:V:0]trim=duration=%s,setpts=PTS-STARTPTS,fps=%d,%s,setsar=1,format=yuv420p[base];", seconds(c.EndMS-c.StartMS), cfg.FPS, coverChain(canvas, c.Focal))
	last, input := "[base]", 1
	if l.Fixed != "" {
		fmt.Fprintf(&graph, "%s[%d:v:0]overlay=0:0:format=auto:shortest=0[fixed];", last, input)
		last, input = "[fixed]", input+1
	}
	// Each caption has its own plate and window: sentence copies under CDS-43,
	// or rapid phrases under CDS-59.
	for j, plate := range l.Copies {
		if plate == "" {
			continue
		}
		// Sentence mode uses a 180 ms fade-in that settles 12 px
		// upward, eased, and a 120 ms fade-out that does not move. Both are
		// expressions on the one looped plate image — never a second raster per
		// frame, and never anything else.
		start, end := c.CaptionWindow(j)
		pace := ""
		if j < len(c.Copies) {
			pace = c.Copies[j].Pace
		}
		m := design.CaptionMotion(pace)
		origin := clip.Region{}
		if j < len(l.CopyRegions) {
			origin = l.CopyRegions[j]
		}
		if pace == "rapid" {
			fmt.Fprintf(&graph, "[%d:v:0]format=rgba[plate%d];", input, j)
			fmt.Fprintf(&graph, "%s[plate%d]overlay=x=%.0f:y=%.0f:format=auto:shortest=0", last, j, origin.X, origin.Y)
		} else {
			in, out := float64(m.InMS)/1000, float64(m.OutMS)/1000
			fmt.Fprintf(&graph, "[%d:v:0]format=rgba,loop=loop=%d:size=1:start=0,fade=t=in:st=%s:d=%s:alpha=1,fade=t=out:st=%s:d=%s:alpha=1[plate%d];", input, frames-1, seconds(start), strconv.FormatFloat(in, 'f', 3, 64), seconds(end-m.OutMS), strconv.FormatFloat(out, 'f', 3, 64), j)
			fmt.Fprintf(&graph, "%s[plate%d]overlay=x=0:y='%.0f*pow(1-min(1,max(0,(t-%s)/%s)),3)':format=auto:shortest=0", last, j, m.InDY, seconds(start), strconv.FormatFloat(in, 'f', 3, 64))
		}
		fmt.Fprintf(&graph, ":enable='gte(t,%s)*lt(t,%s)'", seconds(start), seconds(end))
		fmt.Fprintf(&graph, "[copy%d];", j)
		last = fmt.Sprintf("[copy%d]", j)
		input++
	}
	// The card is the last layer over the footage: nothing shows under it
	// (CDS-45). It carries no motion of its own beyond the fade CDS-28 and
	// CDS-29 name — out for the hook, in for the ending, and no movement.
	if l.Card != "" {
		card := l.Window
		fade := design.Transition.FadeMS
		start, end := max(0, card.StartMS), card.EndMS
		if card.Kind == "hook" {
			fmt.Fprintf(&graph, "[%d:v:0]format=rgba,loop=loop=%d:size=1:start=0,fade=t=out:st=%s:d=%s:alpha=1[card];", l.cardInput(), frames-1, seconds(max(0, end-fade)), seconds(fade))
		} else {
			fmt.Fprintf(&graph, "[%d:v:0]format=rgba,loop=loop=%d:size=1:start=0,fade=t=in:st=%s:d=%s:alpha=1[card];", l.cardInput(), frames-1, seconds(start), seconds(fade))
		}
		fmt.Fprintf(&graph, "%s[card]overlay=0:0:format=auto:shortest=0:enable='gte(t,%s)*lt(t,%s)'[carded];", last, seconds(start), seconds(end))
		last = "[carded]"
	}
	graph.WriteString(last)
	fmt.Fprintf(&graph, "trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv420p[v]", frames)
	if withAudio {
		if source.HasAudio {
			// Preserve the seek-relative audio clock before resetting timestamps.
			// An AAC frame can start one packet after video; resetting STARTPTS
			// first pulled speech ~21 ms earlier and closed real timestamp gaps.
			fmt.Fprintf(&graph, ";[0:a:0]aresample=%d:async=1:min_hard_comp=0:first_pts=0,atrim=duration=%s,aformat=sample_fmts=fltp:channel_layouts=stereo,volume=%.6f", cfg.AudioRate, seconds(c.EndMS-c.StartMS), c.OriginalVolume())
			// The original audio dips under the hook card so its title is not
			// fighting the footage (CDS-35). The dip is the card's window, not
			// the cut's, and normalisation happens after it (T108).
			if l.Card != "" && l.Window.Kind == "hook" {
				fmt.Fprintf(&graph, ",volume=volume=%.6fdB:eval=frame:enable='lt(t,%s)'", design.Audio.HookDipDB, seconds(l.Window.EndMS))
			}
		} else {
			fmt.Fprintf(&graph, ";anullsrc=r=%d:cl=stereo", cfg.AudioRate)
		}
		fmt.Fprintf(&graph, ",apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", frames*cfg.AudioRate/cfg.FPS)
	}
	return graph.String()
}

// The frames one transition overlaps. Every admitted value is a whole number of
// frames at 30 fps, so the output timeline stays exactly integral (CDS-36).
func transitionFrames(cfg clip.RenderConfig, ms int) int { return ms * cfg.FPS / 1000 }

// The xfade CDS-36 names for a transition: a plain dissolve for the 200 ms fade
// a scene change earns, and through black for the 300 ms one a manual plan may
// ask for around the cards.
func transitionKind(ms int) string {
	if ms == design.Transition.BlackMS {
		return "fadeblack"
	}
	return "fade"
}
func compositionGraph(cfg clip.RenderConfig, frames, transitions []int, pixelFormat string) string {
	var graph strings.Builder
	for i := range frames {
		fmt.Fprintf(&graph, "[%d:v:0]settb=AVTB,setpts=PTS-STARTPTS[v%d];", i, i)
	}
	v, elapsed := "v0", frames[0]
	for i := 1; i < len(frames); i++ {
		overlap := transitionFrames(cfg, transitions[i])
		// A hard cut is a JOIN, not a zero-length dissolve: concat keeps both
		// cuts' own frames intact where xfade would resample the boundary.
		if overlap == 0 {
			fmt.Fprintf(&graph, "[%s][v%d]concat=n=2:v=1:a=0[vx%d];", v, i, i)
		} else {
			fmt.Fprintf(&graph, "[%s][v%d]xfade=transition=%s:duration=%s:offset=%s[vx%d];", v, i, transitionKind(transitions[i]), seconds(transitions[i]), frameSeconds(elapsed-overlap, cfg.FPS), i)
		}
		v = fmt.Sprintf("vx%d", i)
		elapsed += frames[i] - overlap
	}
	fmt.Fprintf(&graph, "[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,format=%s[v]", v, elapsed, pixelFormat)
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
	a := []string{"-map", "[v]", "-c:v", "libx264", "-preset", preset, "-crf", strconv.Itoa(crf), "-threads", strconv.Itoa(r.media.cfg.Threads), "-r", strconv.Itoa(r.cfg.FPS), "-fps_mode", "cfr", "-pix_fmt", pixelFormat}
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
	return []string{"-hide_banner", "-nostdin", "-v", "error", "-xerror", "-n", "-filter_threads", strconv.Itoa(r.media.cfg.Threads), "-filter_complex_threads", strconv.Itoa(r.media.cfg.Threads)}
}

// layers is what a cut overlays: the fixed badge and chips, one animated plate
// per copy, and a card whose window is CUT-RELATIVE by the time it gets here.
type layers struct {
	Fixed       string
	Copies      []string
	CopyRegions []clip.Region
	Card        string
	Window      cardLayout
}

// inputs is every layer image this cut hands FFmpeg, in the order renderCut adds
// them. An empty entry is a copy that drew nothing and takes no input.
func (l layers) inputs() []string {
	out := []string{}
	for _, layer := range append([]string{l.Fixed}, l.Copies...) {
		if layer != "" {
			out = append(out, layer)
		}
	}
	if l.Card != "" {
		out = append(out, l.Card)
	}
	return out
}

// cardInput is the card image's own ffmpeg input index: the source is 0 and each
// present layer takes the next one.
func (l layers) cardInput() int {
	return len(l.inputs())
}

func (r *Rendering) renderCut(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, cut clip.EditCut, source clip.MediaSource, frames int, l layers, path string, audio bool) error {
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	for _, layer := range l.inputs() {
		// Decode each PNG once. The fixed overlay repeats its last frame; the
		// animated layers loop one cached frame for exactly this cut's duration.
		// Repeated image decoding both buffers full canvases and can strand the
		// input scheduler after an overlay stops consuming its infinite input.
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-framerate", strconv.Itoa(r.cfg.FPS), "-i", layer)
	}
	args = append(args, "-filter_complex", cutGraph(r.cfg, canvas, cut, source.Info, frames, l, audio))
	args = append(args, r.encodeArgs(audio)...)
	args = append(args, "-t", frameSeconds(frames, r.cfg.FPS))
	if err := r.runRender(ctx, ws, path, args); err != nil {
		return err
	}
	return r.media.sourcePath(ws, path)
}
func (r *Rendering) Render(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (result clip.RenderedVideo, err error) {
	if plan.Portable != nil {
		return r.renderComposition(ctx, ws, plan, sources, load)
	}
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
	transitions := planTransitions(plan)
	offsets := cutOffsets(plan)
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
	c, err := r.layout(ctx, ws, canvas, plan)
	if err != nil {
		return result, err
	}
	// A compiled plan that fails here walks CDS-55's ladder — style, anchor,
	// drop — before any source byte is fetched; a person's plan is refused with
	// the check named, and a blocking furniture failure fails at once (CDS-52).
	// Overlap is advisory here and at final verification (CDS-56).
	if err = r.repair(ctx, ws, canvas, &c); err != nil {
		return result, err
	}
	plan = c.plan
	// The layers that need no source pixels: the fixed one (the disclosure badge
	// and this cut's chips, which never move) and the card. The copy's own plate
	// waits for the cut's own callback, because an unplated style does not know
	// what it draws until the footage under it has been sampled (CDS-44).
	fixed, cardPlates := make([]string, len(plan.Cuts)), make([]string, len(plan.Cuts))
	for i := range plan.Cuts {
		if fixed[i], err = r.furniturePlate(ctx, ws, canvas, c.furniture[i], i); err != nil {
			return result, err
		}
		if cardPlates[i], err = r.cardPlate(ctx, ws, canvas, c.cards[i], i); err != nil {
			return result, err
		}
		for _, layer := range []string{fixed[i], cardPlates[i]} {
			if layer != "" {
				paths = append(paths, layer)
			}
		}
	}
	cutPaths := make([]string, len(plan.Cuts))
	for i := range plan.Cuts {
		cut := c.plan.Cuts[i]
		cutPaths[i] = filepath.Join(ws.Path, fmt.Sprintf("render-cut-%04d.mp4", i))
		paths = append(paths, cutPaths[i])
		calls := 0
		err = load(ctx, cut.SourceID, func(source clip.MediaSource) error {
			calls++
			expected := byID[cut.SourceID]
			if calls != 1 || source.SourceID != cut.SourceID || source.Fingerprint != cut.Fingerprint || source.Info.DurationMS != expected.Info.DurationMS || source.Info.Width != expected.Info.Width || source.Info.Height != expected.Info.Height || source.Info.HasAudio != expected.Info.HasAudio {
				return clip.ErrInvalidMedia
			}
			// One plate per copy, in the cut's own order (CDS-43).
			plates := make([]string, len(c.plan.Cuts[i].Copies))
			regions := make([]clip.Region, len(plates))
			for j := range c.plan.Cuts[i].Copies {
				plate, err := r.copyLayer(ctx, ws, canvas, &c, i, j, source)
				if err != nil {
					return err
				}
				plates[j] = plate
				regions[j] = copyCrop(canvas, c.plan.Cuts[i].Copies[j], c.layouts[i][j], c.grounds[i][j])
				if plate != "" {
					paths = append(paths, plate)
				}
			}
			return r.renderCut(ctx, ws, canvas, c.plan.Cuts[i], source, frames[i], layers{Fixed: fixed[i], Copies: plates, CopyRegions: regions, Card: cardPlates[i], Window: c.cards[i].relativeTo(offsets[i])}, cutPaths[i], audio)
		})
		if err != nil {
			return result, err
		}
		if calls != 1 {
			return result, clip.ErrInvalidMedia
		}
	}
	// The manifest is not the one that was verified before the download: every
	// unplated copy now carries the ground it was measured against and a
	// contrast fallback may have changed a style. What the cuts actually show is
	// what has to hold, so the whole manifest is verified again before they
	// become one clip (CDS-52).
	manifest := c.manifest
	if err = clip.VerifyLayout(plan.Ratio, plan.Styles, manifest, plan.HideDisclosure); err != nil {
		return result, err
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
	// CDS-35's two passes. The track is assembled once on the video's own
	// timeline, measured, and then corrected inside the single final encode:
	// one dynamic pass drifts on a file this short and would leave V12's window.
	// A plan with no audio anywhere still gets no audio track at all.
	assembled, measured := "", loudness{}
	if audio {
		assembled = filepath.Join(ws.Path, "compose-audio.wav")
		paths = append(paths, assembled)
		if err = r.assembleAudio(ctx, ws, cutPaths, frames, transitions, assembled); err != nil {
			return result, err
		}
		if measured, err = r.measureLoudness(ctx, ws, assembled); err != nil {
			return result, err
		}
	}
	args, err := r.compositionInputs(ctx, ws, cutPaths, frames, transitions, assembled, &measured, &paths)
	if err != nil {
		return result, err
	}
	args = append(args, r.encodeArgs(audio)...)
	if err = r.runRender(ctx, ws, output, args); err != nil {
		return result, err
	}
	// Correct small loudness drift before the final probe. The already encoded
	// picture is copied; the corrected track is encoded from the same PCM.
	if audio {
		totalFrames := 0
		for i, n := range frames {
			totalFrames += n - transitionFrames(r.cfg, transitions[i])
		}
		if err = r.finishLoudness(ctx, ws, output, assembled, measured, totalFrames); err != nil {
			return result, err
		}
	}
	return r.validateRenderedOutput(ctx, ws, output, plan, audio, manifest)
}

func (r *Rendering) validateRenderedOutput(ctx context.Context, ws clip.MediaWorkspace, output string, plan clip.EditPlan, audio bool, manifest clip.Manifest) (result clip.RenderedVideo, err error) {
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	info, err := r.media.Probe(ctx, ws, output)
	if err != nil {
		return result, err
	}
	if info.Width != canvas.Width || info.Height != canvas.Height || info.Rotation != 0 || info.PixelFormat != "yuv420p" || info.SampleAspectRatio != "1:1" || info.FrameRateNumerator != r.cfg.FPS*info.FrameRateDenominator || info.HasAudio != audio || math.Abs(float64(info.DurationMS-plan.DurationMS)) > 1000/float64(r.cfg.FPS) {
		return result, errors.New("rendered clip failed output validation")
	}
	// V12 reads the DELIVERED file, not the arguments that produced it: the
	// canvas, 30 fps, H.264 High and 48 kHz AAC (CDS-52).
	for _, s := range info.Streams {
		if s.Kind == "video" && (s.Codec != "h264" || s.Profile != h264Profile) || s.Kind == "audio" && s.Codec != "aac" {
			return result, errors.New("rendered clip codec mismatch")
		}
	}
	if audio && info.AudioRate != r.cfg.AudioRate {
		return result, errors.New("rendered clip codec mismatch")
	}
	stat, err := os.Stat(output)
	if err != nil {
		return result, err
	}
	return clip.RenderedVideo{Path: output, Info: info, Bytes: stat.Size(), Manifest: manifest}, nil
}

// Output-timeline offset of each cut: a cut starts where the one before it ends,
// less the transition it leads in with (CDS-36).
func cutOffsets(plan clip.EditPlan) []int {
	offsets, elapsed := make([]int, len(plan.Cuts)), 0
	for i, c := range plan.Cuts {
		offsets[i] = elapsed - c.TransitionMS
		elapsed = offsets[i] + c.EndMS - c.StartMS
	}
	return offsets
}

// planTransitions is each cut's own leading transition, in cut order.
func planTransitions(plan clip.EditPlan) []int {
	out := make([]int, len(plan.Cuts))
	for i, c := range plan.Cuts {
		out[i] = c.TransitionMS
	}
	return out
}

// layout measures and places every copy, the disclosure badge and the chips
// without touching one source pixel, so the manifest and its verification come
// before any download.
func (r *Rendering) layout(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, plan clip.EditPlan) (composed, error) {
	layouts, plates, manifest := make([][]copyLayout, len(plan.Cuts)), make([]furniture, len(plan.Cuts)), clip.Manifest{}
	cards := make([]cardLayout, len(plan.Cuts))
	offsets := cutOffsets(plan)
	answers := map[string]string{}
	for _, a := range plan.Facts {
		answers[a.Label] = a.Text
	}
	phrase, ok := design.Disclosure[plan.Disclosure]
	if !ok {
		// Campaign identity remains required independently of visibility.
		return composed{}, clip.ErrDisclosureRequired
	}
	if plan.HideDisclosure {
		phrase = ""
	}
	// The two cards, on the first and the last cut (CDS-28, CDS-29). A cut can
	// hold only one, which is why a single-cut clip renders the hook card and
	// then the ending card over the same footage.
	accent := answerAccent(plan)
	badged := false
	for i, cut := range plan.Cuts {
		labels := plan.ChipLabels(cut)
		// The badge rides the first cut's furniture plate; every later cut gets
		// one only if it carries chips, and the badge is drawn on all of them so
		// it is present for the whole clip (CDS-5).
		f, err := r.badgeAndChips(ctx, ws, canvas, plan.Ratio, phrase, labels, answers)
		if err != nil {
			return composed{}, err
		}
		plates[i] = f
		start, end := offsets[i], offsets[i]+cut.EndMS-cut.StartMS
		elements := f.Elements(plan.DurationMS, i, start, end)
		if badged && phrase != "" {
			elements = elements[1:] // one badge in the manifest, one disclosure
		}
		badged = true
		manifest = append(manifest, elements...)
		// One layout per copy: a cut of 4 s or more may carry two, one after the
		// other (CDS-43).
		layouts[i] = make([]copyLayout, len(cut.Copies))
		for j, copy := range cut.Copies {
			if strings.TrimSpace(copy.Text) == "" {
				continue
			}
			l, err := r.layoutCopy(ctx, ws, canvas, copy)
			if err != nil {
				return composed{}, err
			}
			layouts[i][j] = l
			copyStart, copyEnd := cut.CaptionWindow(j)
			manifest = append(manifest, l.Elements(i, j, copy, offsets[i]+copyStart, offsets[i]+copyEnd)...)
		}
	}
	// The cards are measured last so the copy they hide is already placed: no
	// copy or chip shows under either card (CDS-45).
	for i, kind := range map[int]string{0: "hook", len(plan.Cuts) - 1: "end"} {
		card := hookCard(plan.Ratio, plan.Hook, plan.Preset, answers["상호"], accent, plan.DurationMS)
		if kind == "end" {
			card = endingCard(plan.Ratio, plan.Preset, plan.CTA, answers, accent, plan.DurationMS)
		}
		if card.empty() {
			continue
		}
		measured, err := r.measureCard(ctx, ws, canvas, plan.Ratio, card)
		if err != nil {
			return composed{}, err
		}
		// One cut can carry both cards only by drawing them on one plate; the
		// hook's window ends long before the ending card's begins.
		if !cards[i].empty() {
			cards[i].Lines = append(cards[i].Lines, measured.Lines...)
			cards[i].Bounds = append(cards[i].Bounds, measured.Bounds...)
		} else {
			cards[i] = measured
		}
		manifest = append(manifest, measured.Elements(i)...)
	}
	grounds := make([][]Luminance, len(plan.Cuts))
	for i, cut := range plan.Cuts {
		grounds[i] = make([]Luminance, len(cut.Copies))
	}
	return composed{plan: plan, layouts: layouts, furniture: plates, cards: cards, manifest: manifest,
		grounds: grounds}, nil
}

// composed is everything one pass of layout produced: the plan it was measured
// from, the layers each cut overlays, and the manifest the verifier reads. The
// brightness sampler updates it per cut as the render walks the cuts, which is
// why it travels as one value (CDS-44).
type composed struct {
	plan      clip.EditPlan
	layouts   [][]copyLayout
	furniture []furniture
	cards     []cardLayout
	manifest  clip.Manifest
	// What the sampler measured under each COPY, empty for one whose style is
	// plated or which was dropped. Parallel to the cut's own copies (CDS-43).
	grounds [][]Luminance
}

// cutElements is one cut's own caption elements, which is the scope a contrast
// decision is made on.
func (c *composed) cutElements(cut, copy int) clip.Manifest {
	out := clip.Manifest{}
	for _, e := range c.manifest {
		if e.Cut == cut && e.Copy == copy && e.Style != "" {
			out = append(out, e)
		}
	}
	return out
}

// resolve rewrites the manifest to what the sampled grounds mean: every text of
// an unplated copy is read against the footage under it, washed by the scrim
// where CDS-32 puts one, and the scrim itself joins the manifest sharing the
// copy's own window. It is idempotent — it drops the scrims it added before
// adding them again — so it can run after every sample and after a fallback
// re-measured the whole plan.
func (c *composed) resolve(canvas clip.Canvas) {
	kept := make(clip.Manifest, 0, len(c.manifest))
	for _, e := range c.manifest {
		if e.Kind != "scrim" {
			kept = append(kept, e)
		}
	}
	offsets := cutOffsets(c.plan)
	for i, cut := range c.plan.Cuts {
		for j, copy := range cut.Copies {
			ground := c.grounds[i][j]
			if !ground.Sampled() {
				continue
			}
			background := ground.Background(canvas, c.layouts[i][j].Style, copy.Anchor, c.layouts[i][j].Region)
			for k := range kept {
				if kept[k].Cut == i && kept[k].Copy == j && kept[k].Kind == "copy" && kept[k].Style != "" {
					kept[k].Background = background
				}
			}
			s, ok := scrimFor(canvas, copy.Anchor)
			if !ok || !ground.Scrim() {
				continue
			}
			start, end := cut.CaptionWindow(j)
			kept = append(kept, design.Element{
				Cut: i, Copy: j, Kind: "scrim", Style: copy.Style, Anchor: copy.Anchor, Pace: copy.Pace,
				Region: design.Region(s.Region), StartMS: offsets[i] + start, EndMS: offsets[i] + end,
				Background: design.Scrim[s.Edge].Hex,
				InMS:       design.CaptionMotion(copy.Pace).InMS, OutMS: design.CaptionMotion(copy.Pace).OutMS, DY: design.CaptionMotion(copy.Pace).InDY,
			})
		}
	}
	c.manifest = kept
}

// copyLayer resolves and rasterizes one cut's copy plate. A plated style asks
// nothing of the footage — ink at α ≥ 0.72 under white text stays above the
// floor even over a white frame (CDS-16) — and is drawn straight away. An
// unplated one samples the three frames CDS-44 names under the copy first: the
// ground decides whether a scrim appears, whether the accent word turns white,
// and what contrast V3 then measures. A pairing still under the floor puts the
// sentence back on a plate when the compiler chose the style, and is refused
// when a person did.
func (r *Rendering) copyLayer(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c *composed, index, copyIndex int, source clip.MediaSource) (string, error) {
	cut := c.plan.Cuts[index]
	copy := cut.Copies[copyIndex]
	if strings.TrimSpace(copy.Text) == "" {
		return "", nil
	}
	if c.layouts[index][copyIndex].Style.Plate != "" || copy.Style == "simple" {
		return r.copyPlate(ctx, ws, canvas, copy, c.layouts[index][copyIndex], index*design.Rapid.MaxPerCut+copyIndex, Luminance{})
	}
	start, end := cut.CaptionWindow(copyIndex)
	ground, err := r.sample(ctx, ws, canvas, source, cut, [2]int{start, end}, c.layouts[index][copyIndex].Region, index)
	if err != nil {
		return "", err
	}
	c.grounds[index][copyIndex] = ground
	c.resolve(canvas)
	if !design.Legible(c.cutElements(index, copyIndex)) {
		if !c.plan.Compiled() {
			return "", clip.LayoutViolation(design.ViolationContrast, index, copyIndex)
		}
		// CDS-44's last clause is rung 1 of the one ladder: 깔끔하게, whose plate
		// needs no ground at all (CDS-16). It is recorded as the contrast fallback
		// it is, so step ② can say the footage, not the words, moved the style.
		if _, err := r.rung(ctx, ws, canvas, c, index, copyIndex, rungStyle, "contrast"); err != nil {
			return "", err
		}
	}
	return r.copyPlate(ctx, ws, canvas, c.plan.Cuts[index].Copies[copyIndex], c.layouts[index][copyIndex], index*design.Rapid.MaxPerCut+copyIndex, c.grounds[index][copyIndex])
}

// The three rungs of CDS-55's repair ladder, in the order they are tried: the
// style falls back to 깔끔하게 (whose plate answers V2, V3 and V5 by itself), the
// anchor falls back to the style's own default, and the copy is dropped so the
// cut shows its footage.
const (
	rungStyle = iota
	rungAnchor
	rungDrop
	rungsExhausted
)

// repair walks the ladder for a COMPILED plan whose manifest fails a check,
// re-laying out and verifying after every rung, until the manifest verifies or
// the ladder is exhausted for a failing caption. It costs at most one full
// ladder per caption and one layout per rung, and no download. A plan a person
// corrected is refused with the check named and never moved, and a failure the
// design system's own furniture caused fails at once (CDS-52, CDS-55).
// Overlap never enters the ladder: delivery leaves it for owner review (CDS-56).
func (r *Rendering) repair(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c *composed) error {
	taken := map[[2]int]int{}
	for {
		err := clip.VerifyLayout(c.plan.Ratio, c.plan.Styles, c.manifest, c.plan.HideDisclosure)
		if err == nil {
			return nil
		}
		var failure *clip.LayoutError
		if !errors.As(err, &failure) || failure.Furniture() || !c.plan.Compiled() {
			return err
		}
		key := [2]int{failure.Cut, failure.Copy}
		if failure.Cut >= len(c.plan.Cuts) || failure.Copy >= len(c.plan.Cuts[failure.Cut].Copies) {
			return err
		}
		next := taken[key]
		for ; next < rungsExhausted; next++ {
			changed, err := r.rung(ctx, ws, canvas, c, failure.Cut, failure.Copy, next, "")
			if err != nil {
				return err
			}
			if changed {
				break
			}
		}
		if next >= rungsExhausted {
			return err
		}
		taken[key] = next + 1
	}
}

// rung applies one step of the ladder to one caption and re-measures the whole
// plan so the manifest describes what is really drawn. It reports false when
// the caption already sits where the rung would put it, so the next rung is
// tried without a layout pass. The record names the step, or the caller's own
// name for it (the post-sample contrast fallback).
func (r *Rendering) rung(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, c *composed, index, copyIndex, step int, record string) (bool, error) {
	plan := c.plan
	plan.Cuts = slices.Clone(plan.Cuts)
	plan.Cuts[index].Copies = slices.Clone(plan.Cuts[index].Copies)
	plan.Decisions = slices.Clone(plan.Decisions)
	copied := plan.Cuts[index].Copies[copyIndex]
	switch step {
	case rungStyle:
		if copied.Style == "clean" {
			return false, nil
		}
		copied.Style = "clean"
		// The anchor is kept when 깔끔하게 may stand there and is otherwise its own
		// (CDS-24 is the law on which anchors a style takes).
		if !slices.Contains(design.StyleAnchors("clean"), copied.Anchor) {
			copied.Anchor = design.Styles["clean"].Anchor
		}
		copied.Align = design.Styles["clean"].Align
		if record == "" {
			record = "style"
		}
	case rungAnchor:
		style, ok := design.Styles[copied.Style]
		if !ok || (copied.Anchor == style.Anchor && copied.Align == style.Align) {
			return false, nil
		}
		copied.Anchor, copied.Align = style.Anchor, style.Align
		if record == "" {
			record = "anchor"
		}
	case rungDrop:
		if strings.TrimSpace(copied.Text) == "" {
			return false, nil
		}
		copied = clip.Copy{}
		if record == "" {
			record = "dropped"
		}
	default:
		return false, nil
	}
	plan.Cuts[index].Copies[copyIndex] = copied
	if index < len(plan.Decisions) {
		plan.Decisions[index].Recorded(record)
	}
	next, err := r.layout(ctx, ws, canvas, plan)
	if err != nil {
		return false, err
	}
	// The grounds already sampled still describe the other copies; this one is
	// plated or gone, so what it was measured against is no longer drawn.
	next.grounds = c.grounds
	next.grounds[index][copyIndex] = Luminance{}
	next.resolve(canvas)
	*c = next
	return true, nil
}

// The project accent, which every card and chip paints with (CLIP-14).
func answerAccent(plan clip.EditPlan) string {
	for _, c := range plan.Cuts {
		for _, copy := range c.Copies {
			if copy.Accent != "" {
				return copy.Accent
			}
		}
	}
	return plan.Accent
}

// Layout is the same pass on its own workspace, for a caller that wants the
// manifest without rendering: it downloads nothing and writes no video.
func (r *Rendering) Layout(ctx context.Context, plan clip.EditPlan, sources []clip.RenderSource) (repaired clip.EditPlan, manifest clip.Manifest, err error) {
	if plan.Portable != nil {
		var elements []clip.CompositionElement
		repaired, elements, err = r.LayoutComposition(ctx, plan, sources)
		for _, element := range elements {
			manifest = append(manifest, element.Parts...)
		}
		return repaired, manifest, err
	}
	if err = clip.ValidateEditPlan(r.cfg, plan, sources); err != nil {
		return plan, nil, err
	}
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	repaired = plan
	err = r.media.WithWorkspace(ctx, "clip-layout", func(ws clip.MediaWorkspace) error {
		c, err := r.layout(ctx, ws, canvas, plan)
		if err != nil {
			return err
		}
		// The same ladder the render walks, so a compiled plan that verifies
		// here is the plan the render will get (CDS-55).
		err = r.repair(ctx, ws, canvas, &c)
		manifest, repaired = c.manifest, c.plan
		return err
	})
	return repaired, manifest, err
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
