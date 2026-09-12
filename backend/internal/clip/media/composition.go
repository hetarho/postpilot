package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

type videoBranch struct {
	path string
	// frames is what the branch is long, and transition what joins it to the
	// branch BEFORE it — the leading transition of its first cut, which a merge
	// carries up the tree unchanged.
	frames     int
	transition int
	temporary  bool
}

func (r *Rendering) inputArgs(args []string, path string) []string {
	return append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-i", path)
}

// Decode at most two full-resolution video inputs per process. A linear xfade
// graph opens every future video decoder at once and exceeds the shared-service
// memory budget on long plans. Internal tree nodes are lossless H.264 4:4:4:
// there is no extra lossy encode or repeated chroma downsampling per tree level.
// These nodes carry no audio at all: the track is assembled and measured once
// beside them and joins only at the final encode, which stays a single AAC pass.
func (r *Rendering) compositionInputs(ctx context.Context, ws clip.MediaWorkspace, cuts []string, frames, transitions []int, audio string, measured *loudness, cleanup *[]string) ([]string, error) {
	return r.compositionInputsFormat(ctx, ws, cuts, frames, transitions, audio, measured, cleanup, "yuv420p")
}

func (r *Rendering) compositionInputsFormat(ctx context.Context, ws clip.MediaWorkspace, cuts []string, frames, transitions []int, audio string, measured *loudness, cleanup *[]string, pixelFormat string) ([]string, error) {
	branches := make([]videoBranch, len(cuts))
	for i, path := range cuts {
		branches[i] = videoBranch{path: path, frames: frames[i], transition: transitions[i]}
	}
	for level := 0; len(branches) > 2; level++ {
		next := make([]videoBranch, 0, (len(branches)+1)/2)
		for i := 0; i < len(branches); i += 2 {
			if i+1 == len(branches) {
				next = append(next, branches[i])
				continue
			}
			a, b := branches[i], branches[i+1]
			path := filepath.Join(ws.Path, fmt.Sprintf("compose-%02d-%04d.mp4", level, i/2))
			*cleanup = append(*cleanup, path)
			args := r.inputArgs(r.inputArgs(r.baseArgs(), a.path), b.path)
			args = append(args, "-filter_complex", compositionGraph(r.cfg, []int{a.frames, b.frames}, []int{0, b.transition}, "yuv444p"))
			args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
			if err := r.runRender(ctx, ws, path, args); err != nil {
				return nil, err
			}
			for _, input := range []videoBranch{a, b} {
				if input.temporary {
					if err := os.Remove(input.path); err != nil {
						return nil, err
					}
				}
			}
			next = append(next, videoBranch{path: path, frames: a.frames + b.frames - transitionFrames(r.cfg, b.transition), transition: a.transition, temporary: true})
		}
		branches = next
	}
	args := r.baseArgs()
	videoFrames, videoTransitions := make([]int, len(branches)), make([]int, len(branches))
	for i, branch := range branches {
		args = r.inputArgs(args, branch.path)
		videoFrames[i], videoTransitions[i] = branch.frames, branch.transition
	}
	graph := compositionGraph(r.cfg, videoFrames, videoTransitions, pixelFormat)
	// The audio is already one assembled, measured track by now: the final pass
	// only normalises it, so the clip is still produced by ONE encode (CDS-35).
	if audio != "" {
		args = r.inputArgs(args, audio)
		graph += fmt.Sprintf(";[%d:a:0]%s,aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo[a]", len(branches), loudnormFilter(measured), r.cfg.AudioRate)
	}
	return append(args, "-filter_complex", graph), nil
}

// assembleAudio joins every cut's audio on the SAME timeline the video takes and
// writes it uncompressed, so the measurement pass and the final encode read one
// identical track rather than measuring one thing and encoding another.
func (r *Rendering) assembleAudio(ctx context.Context, ws clip.MediaWorkspace, cuts []string, frames, transitions []int, path string) error {
	args := r.baseArgs()
	for _, cut := range cuts {
		args = r.inputArgs(args, cut)
	}
	args = append(args, "-filter_complex", compositionAudioGraph(r.cfg, frames, transitions, 0))
	args = append(args, "-map", "[a]", "-vn", "-c:a", "pcm_s16le", "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2", "-f", "wav")
	return r.runRender(ctx, ws, path, args)
}

// compositionAudioGraph cross-fades every boundary CDS-35 names, and takes
// exactly as much of the timeline as the video does at each one.
//
// Where a transition overlaps the picture, the audio cross-fades across that
// same overlap and the two stay locked. A HARD cut has no overlap to spend, so
// its 60 ms cross-fade is a matched pair of fades either side of the seam: that
// removes the click without moving a single later cut. An `acrossfade` there
// would consume 60 ms of the track that the picture does not, and every cut
// after the first boundary would run early by more than a frame.
func compositionAudioGraph(cfg clip.RenderConfig, frames, transitions []int, inputOffset int) string {
	var graph strings.Builder
	seam := seconds(design.Audio.CrossfadeMS)
	for i, n := range frames {
		fmt.Fprintf(&graph, "[%d:a:0]atrim=end_sample=%d,asetpts=PTS-STARTPTS", i+inputOffset, n*cfg.AudioRate/cfg.FPS)
		if i > 0 && transitions[i] == 0 {
			fmt.Fprintf(&graph, ",afade=t=in:st=0:d=%s", seam)
		}
		if i+1 < len(frames) && transitions[i+1] == 0 {
			fmt.Fprintf(&graph, ",afade=t=out:st=%s:d=%s", seconds(max(0, n*1000/cfg.FPS-design.Audio.CrossfadeMS)), seam)
		}
		fmt.Fprintf(&graph, "[a%d];", i)
	}
	a, elapsed := "a0", frames[0]
	for i := 1; i < len(frames); i++ {
		if transitions[i] == 0 {
			fmt.Fprintf(&graph, "[%s][a%d]concat=n=2:v=0:a=1[ax%d];", a, i, i)
		} else {
			fmt.Fprintf(&graph, "[%s][a%d]acrossfade=ns=%d:c1=tri:c2=tri[ax%d];", a, i, transitions[i]*cfg.AudioRate/1000, i)
		}
		a = fmt.Sprintf("ax%d", i)
		elapsed += frames[i] - transitionFrames(cfg, transitions[i])
	}
	fmt.Fprintf(&graph, "[%s]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", a, elapsed*cfg.AudioRate/cfg.FPS)
	return graph.String()
}
