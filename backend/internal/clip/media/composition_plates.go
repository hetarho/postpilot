package media

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

func (r *Rendering) declaredPlate(ctx context.Context, ws clip.MediaWorkspace, canvas clip.Canvas, visual *declaredVisual, source clip.MediaSource, index int) (string, error) {
	var body string
	var err error
	switch visual.manifest.Role {
	case "caption":
		if visual.caption.Style.Plate == "" && visual.copy.Style != "simple" {
			cut := clip.Cut{StartMS: 0, EndMS: source.Info.DurationMS, Focal: clip.Point{X: .5, Y: .5}}
			window := declaredSampleWindow(visual.manifest.StartMS, visual.manifest.EndMS, source.Info.DurationMS, r.cfg.FPS)
			visual.ground, err = r.sample(ctx, ws, canvas, source, cut, window, visual.caption.Region, index)
			if err != nil {
				return "", err
			}
			for i := range visual.manifest.Parts {
				part := &visual.manifest.Parts[i]
				if part.Kind == "copy" {
					part.Background = visual.ground.Background(canvas, visual.caption.Style, visual.copy.Anchor, clip.Region(part.Region))
				}
			}
			if visual.ground.Scrim() {
				if scrim, ok := scrimFor(canvas, visual.copy.Anchor); ok {
					visual.manifest.Parts = append(visual.manifest.Parts, design.Element{Kind: "scrim", Region: design.Region(scrim.Region), StartMS: visual.manifest.StartMS, EndMS: visual.manifest.EndMS})
				}
			}
		}
		body, err = r.overlays.Render("copy."+visual.copy.Style, copyView(canvas, visual.copy, visual.caption, visual.ground))
	case "badge":
		body, err = r.overlays.Render("furniture", furnitureView(canvas, visual.furniture))
	case "info":
		body, err = r.overlays.Render("copy.clean", visual.info)
	case "hook", "ending":
		body, err = r.overlays.Render("card."+visual.card.Kind, cardView(canvas, visual.card))
	default:
		return "", elementProblem(visual.text, "invalid_role")
	}
	if err != nil {
		return "", err
	}
	return r.rasterize(ctx, ws, canvas, body, fmt.Sprintf("declared-%04d", index))
}

// Seeking past the final frame timestamp yields no image even though it is
// before the MP4 duration. Keep all three samples on an existing output frame.
func declaredSampleWindow(start, end, duration, fps int) [2]int {
	last := max(0, duration-(1000+fps-1)/fps)
	end = min(end, last+1)
	return [2]int{min(start, end-1), end}
}

// Only this window's small layer batch is opened. Subsequent batches read the
// preceding lossless window, not the original full-length composition.
func (r *Rendering) renderOverlayWindow(ctx context.Context, ws clip.MediaWorkspace, input string, window overlayWindow, visuals []declaredVisual, plates []string, firstPass bool, output string) error {
	args := r.baseArgs()
	if firstPass {
		args = append(args, "-ss", strconv.Itoa(window.StartFrame/r.cfg.FPS))
	}
	args = r.inputArgs(args, input)
	for _, index := range window.Layers {
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.Threads), "-framerate", strconv.Itoa(r.cfg.FPS), "-i", plates[index])
	}
	args = append(args, "-filter_complex", declaredOverlayGraph(r.cfg, window, visuals, firstPass))
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	args = append(args, "-t", frameSeconds(window.EndFrame-window.StartFrame, r.cfg.FPS))
	return r.runRender(ctx, ws, output, args)
}

func (r *Rendering) overlayComposition(ctx context.Context, ws clip.MediaWorkspace, input string, visuals []declaredVisual, plates []string, totalFrames int, cleanup *[]string) ([]string, []int, error) {
	windows := compositionWindows(visuals, totalFrames, r.cfg.FPS, r.cfg.OverlayBatchSize)
	paths, frames := []string{}, []int{}
	for i, window := range windows {
		previous := input
		for offset := 0; offset < len(window.Layers) || offset == 0; offset += r.cfg.OverlayBatchSize {
			batch := window
			batch.Layers = window.Layers[offset:min(len(window.Layers), offset+r.cfg.OverlayBatchSize)]
			path := filepath.Join(ws.Path, fmt.Sprintf("overlay-%04d-%04d.mp4", i, offset/r.cfg.OverlayBatchSize))
			*cleanup = append(*cleanup, path)
			if err := r.renderOverlayWindow(ctx, ws, previous, batch, visuals, plates, offset == 0, path); err != nil {
				return nil, nil, err
			}
			if previous != input {
				if err := removeIntermediate(previous); err != nil {
					return nil, nil, err
				}
			}
			previous = path
		}
		paths = append(paths, previous)
		frames = append(frames, window.EndFrame-window.StartFrame)
	}
	return paths, frames, nil
}
