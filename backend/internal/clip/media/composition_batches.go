package media

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
	"github.com/postpilot/backend/internal/clip/design"
)

type overlayWindow struct {
	StartFrame, EndFrame int
	Layers               []int
}
type declaredMotion struct {
	InMS, OutMS int
	DY          float64
}

func elementMotion(element clip.CompositionElement, pace string) declaredMotion {
	switch element.Role {
	case "caption":
		m := design.CaptionMotion(pace)
		return declaredMotion{m.InMS, m.OutMS, m.InDY}
	case "hook":
		if element.EndMS-element.StartMS >= design.Transition.FadeMS {
			return declaredMotion{OutMS: design.Transition.FadeMS}
		}
	case "ending":
		if element.EndMS-element.StartMS >= design.Transition.FadeMS {
			return declaredMotion{InMS: design.Transition.FadeMS}
		}
	}
	return declaredMotion{}
}

// Partition by cue density, not source cuts. A partition never bisects an alpha
// fade or entrance motion, allowing each short window to use its own local clock
// without replaying or resetting a persistent element's animation.
func compositionWindows(visuals []declaredVisual, totalFrames, fps, batchSize int) []overlayWindow {
	starts := []int{}
	for _, v := range visuals {
		n := (v.manifest.StartMS*fps + 999) / 1000
		if n > 0 && n < totalFrames {
			starts = append(starts, n)
		}
	}
	slices.Sort(starts)
	boundaries := []int{0}
	for i := batchSize - 1; i < len(starts); i += batchSize {
		boundary := starts[i]
		if boundary <= boundaries[len(boundaries)-1] {
			continue
		}
		for changed := true; changed; {
			changed = false
			for _, visual := range visuals {
				e := visual.manifest
				m := elementMotion(e, visual.text.Pace)
				for _, phase := range [][2]int{{e.StartMS, e.StartMS + m.InMS}, {e.EndMS - m.OutMS, e.EndMS}} {
					if phase[0]*fps < boundary*1000 && boundary*1000 < phase[1]*fps {
						boundary = (phase[1]*fps + 999) / 1000
						changed = true
					}
				}
			}
		}
		if boundary < totalFrames {
			boundaries = append(boundaries, boundary)
		}
	}
	boundaries = append(boundaries, totalFrames)
	result := make([]overlayWindow, 0, len(boundaries)-1)
	for i := 1; i < len(boundaries); i++ {
		window := overlayWindow{StartFrame: boundaries[i-1], EndFrame: boundaries[i]}
		for index, visual := range visuals {
			start, end := (visual.manifest.StartMS*fps+999)/1000, (visual.manifest.EndMS*fps+999)/1000
			if start < window.EndFrame && end > window.StartFrame {
				window.Layers = append(window.Layers, index)
			}
		}
		slices.SortStableFunc(window.Layers, func(a, b int) int {
			return clip.CompositionLayer(visuals[a].manifest.Role) - clip.CompositionLayer(visuals[b].manifest.Role)
		})
		result = append(result, window)
	}
	return result
}

// Only integer bounds and code-owned motion enter this graph. PNGs are decoded
// once and repeated for this finite window, with at most OverlayBatchSize inputs.
func declaredOverlayGraph(cfg clip.RenderConfig, window overlayWindow, visuals []declaredVisual, firstPass bool) string {
	var graph strings.Builder
	frames := window.EndFrame - window.StartFrame
	startFrame := 0
	if firstPass {
		// The caller seeks to the preceding whole second. Keeping the final
		// sub-second trim in frames avoids fractional -ss rounding at 30 FPS.
		startFrame = window.StartFrame % cfg.FPS
	}
	fmt.Fprintf(&graph, "[0:V:0]trim=start_frame=%d:end_frame=%d,setpts=PTS-STARTPTS,format=yuv444p[base];", startFrame, startFrame+frames)
	last := "base"
	offset := float64(window.StartFrame) / float64(cfg.FPS)
	secondsFloat := func(n float64) string { return strconv.FormatFloat(n, 'f', 9, 64) }
	for i, index := range window.Layers {
		v := visuals[index]
		e := v.manifest
		m := elementMotion(e, v.text.Pace)
		start, end := float64(e.StartMS)/1000-offset, float64(e.EndMS)/1000-offset
		fmt.Fprintf(&graph, "[%d:v:0]format=rgba,loop=loop=%d:size=1:start=0", i+1, frames-1)
		if m.InMS > 0 && start >= 0 {
			fmt.Fprintf(&graph, ",fade=t=in:st=%s:d=%s:alpha=1", secondsFloat(start), seconds(m.InMS))
		}
		if m.OutMS > 0 && end-float64(m.OutMS)/1000 >= 0 {
			fmt.Fprintf(&graph, ",fade=t=out:st=%s:d=%s:alpha=1", secondsFloat(end-float64(m.OutMS)/1000), seconds(m.OutMS))
		}
		fmt.Fprintf(&graph, "[layer%d];[%s][layer%d]overlay=x=0:y=", i, last, i)
		if m.DY > 0 && m.InMS > 0 && start >= 0 {
			fmt.Fprintf(&graph, "'%.0f*pow(1-min(1,max(0,(t-%s)/%s)),3)'", m.DY, secondsFloat(start), seconds(m.InMS))
		} else {
			graph.WriteString("0")
		}
		fmt.Fprintf(&graph, ":format=auto:shortest=0:enable='gte(t,%s)*lt(t,%s)'[overlay%d];", secondsFloat(start), secondsFloat(end), i)
		last = fmt.Sprintf("overlay%d", i)
	}
	fmt.Fprintf(&graph, "[%s]trim=end_frame=%d,setpts=PTS-STARTPTS,format=yuv444p[v]", last, frames)
	return graph.String()
}
