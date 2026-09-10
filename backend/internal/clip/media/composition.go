package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

type videoBranch struct {
	path      string
	frames    int
	temporary bool
}

func (r *Rendering) inputArgs(args []string, path string) []string {
	return append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-protocol_whitelist", "file,pipe", "-i", path)
}

// Decode at most two full-resolution video inputs per process. A linear xfade
// graph opens every future video decoder at once and exceeds the shared-service
// memory budget on long plans. Internal tree nodes are lossless H.264 4:4:4:
// there is no extra lossy encode or repeated chroma downsampling per tree level.
// Original cut audio is untouched by these nodes and joins only at the final
// encode, retaining the existing single AAC composition pass.
func (r *Rendering) compositionInputs(ctx context.Context, ws clip.MediaWorkspace, cuts []string, frames []int, audio bool, cleanup *[]string) ([]string, error) {
	branches := make([]videoBranch, len(cuts))
	for i, path := range cuts {
		branches[i] = videoBranch{path: path, frames: frames[i]}
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
			args = append(args, "-filter_complex", compositionVideoGraph(r.cfg, []int{a.frames, b.frames}, false, "yuv444p"))
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
			next = append(next, videoBranch{path: path, frames: a.frames + b.frames - r.cfg.FadeMS*r.cfg.FPS/1000, temporary: true})
		}
		branches = next
	}
	args := r.baseArgs()
	videoFrames := make([]int, len(branches))
	for i, branch := range branches {
		args = r.inputArgs(args, branch.path)
		videoFrames[i] = branch.frames
	}
	graph := compositionGraph(r.cfg, videoFrames, false)
	if audio {
		for _, path := range cuts {
			args = r.inputArgs(args, path)
		}
		graph += ";" + compositionAudioGraph(r.cfg, frames, len(branches))
	}
	return append(args, "-filter_complex", graph), nil
}

func compositionAudioGraph(cfg clip.RenderConfig, frames []int, inputOffset int) string {
	var graph strings.Builder
	for i, n := range frames {
		fmt.Fprintf(&graph, "[%d:a:0]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a%d];", i+inputOffset, n*cfg.AudioRate/cfg.FPS, i)
	}
	a, elapsed := "a0", frames[0]
	for i := 1; i < len(frames); i++ {
		fmt.Fprintf(&graph, "[%s][a%d]acrossfade=ns=%d:c1=tri:c2=tri[ax%d];", a, i, cfg.FadeMS*cfg.AudioRate/1000, i)
		a = fmt.Sprintf("ax%d", i)
		elapsed += frames[i] - cfg.FadeMS*cfg.FPS/1000
	}
	fmt.Fprintf(&graph, "[%s]atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", a, elapsed*cfg.AudioRate/cfg.FPS)
	return graph.String()
}
