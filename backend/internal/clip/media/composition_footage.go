package media

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

// Native cut nodes contain only footage. Text is applied after every source
// transition, so an output-level element cannot be faded or composited twice.
func bareFootageGraph(cfg clip.RenderConfig, canvas clip.Canvas, cut clip.Cut, frames int) string {
	return fmt.Sprintf("[0:V:0]trim=duration=%s,setpts=PTS-STARTPTS,fps=%d,%s,setsar=1,trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv444p[v]", seconds(cut.EndMS-cut.StartMS), cfg.FPS, coverChain(canvas, cut.Focal), frames)
}

func (r *Rendering) renderBareFootage(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, cut clip.Cut, source clip.MediaSource, frames int, output string) error {
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	args = append(args, "-filter_complex", bareFootageGraph(r.cfg, canvas, cut, frames))
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	args = append(args, "-t", frameSeconds(frames, r.cfg.FPS))
	return r.runRender(ctx, ws, output, args)
}

func bareAudioGraph(cfg clip.RenderConfig, cut clip.Cut, hasAudio bool, frames int) string {
	var graph strings.Builder
	if hasAudio {
		fmt.Fprintf(&graph, "[0:a:0]aresample=%d:async=1:min_hard_comp=0:first_pts=0,atrim=duration=%s,aformat=sample_fmts=fltp:channel_layouts=stereo,volume=%.6f", cfg.AudioRate, seconds(cut.EndMS-cut.StartMS), cut.OriginalVolume())
	} else {
		fmt.Fprintf(&graph, "anullsrc=r=%d:cl=stereo", cfg.AudioRate)
	}
	fmt.Fprintf(&graph, ",apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", frames*cfg.AudioRate/cfg.FPS)
	return graph.String()
}

// Audio is retained as PCM until the final AAC encode. The hook dip belongs to
// the assembled output clock, not the first source cut or a preset window.
func (r *Rendering) renderBareAudio(ctx context.Context, ws clip.MediaWorkspace, cut clip.Cut, source clip.MediaSource, frames int, output string) error {
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	args := r.baseArgs()
	if source.Info.HasAudio {
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
	}
	args = append(args, "-filter_complex", bareAudioGraph(r.cfg, cut, source.Info.HasAudio, frames), "-map", "[a]", "-vn", "-c:a", "pcm_s16le", "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2", "-f", "wav")
	return r.runRender(ctx, ws, output, args)
}

func declaredAudioGraph(cfg clip.RenderConfig, frames, transitions []int, elements []clip.CompositionElement) string {
	graph := compositionAudioGraph(cfg, frames, transitions, 0)
	windows := []string{}
	for _, element := range elements {
		if element.Role == "hook" {
			windows = append(windows, fmt.Sprintf("gte(t,%s)*lt(t,%s)", seconds(element.StartMS), seconds(element.EndMS)))
		}
	}
	if len(windows) > 0 {
		graph = strings.TrimSuffix(graph, "[a]") + fmt.Sprintf("[joined];[joined]volume=volume=%.6fdB:eval=frame:enable='gt(%s,0)'[a]", design.Audio.HookDipDB, strings.Join(windows, "+"))
	}
	return graph
}

func (r *Rendering) assembleDeclaredAudio(ctx context.Context, ws clip.MediaWorkspace, cuts []string, frames, transitions []int, elements []clip.CompositionElement, output string) error {
	args := r.baseArgs()
	for _, cut := range cuts {
		args = r.inputArgs(args, cut)
	}
	args = append(args, "-filter_complex", declaredAudioGraph(r.cfg, frames, transitions, elements), "-map", "[a]", "-vn", "-c:a", "pcm_s16le", "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2", "-f", "wav")
	return r.runRender(ctx, ws, output, args)
}
