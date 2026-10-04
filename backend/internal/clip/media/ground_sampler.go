package media

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
)

// The ground CDS-44 reads under an unplated text is the footage the viewer sees
// there: the composed output, cut by cut, crossfaded where a transition joins two
// cuts. Both render kinds measure it with this one sampler over the retained
// originals, so a server render and the grounds handed to a browser render reach
// the same scrim, accent and contrast decisions by construction (CLIP-157,
// CLIP-192). Each output frame is rebuilt from the frames of the cut or cuts it
// is made of, through the same chain the composition renders its bare footage
// with, so no composed footage has to exist for a browser render to be sampled.

// footageTimeline is compositionGraph's frame arithmetic: each cut contributes
// its frames, a transition overlaps the previous cut's tail, and a zero
// transition is a join.
type footageTimeline struct {
	starts, frames, overlaps []int
	kinds                    []string
	total                    int
}

func newFootageTimeline(cfg clip.RenderConfig, plan clip.EditPlan) footageTimeline {
	frames, transitions := cutFrames(plan, cfg.FPS), planTransitions(plan)
	t := footageTimeline{starts: make([]int, len(frames)), frames: frames, overlaps: make([]int, len(frames)), kinds: make([]string, len(frames))}
	if len(frames) == 0 {
		return t
	}
	elapsed := frames[0]
	for i := 1; i < len(frames); i++ {
		t.overlaps[i] = transitionFrames(cfg, transitions[i])
		t.kinds[i] = transitionKind(transitions[i])
		t.starts[i] = elapsed - t.overlaps[i]
		elapsed += frames[i] - t.overlaps[i]
	}
	t.total = elapsed
	return t
}

// footageTake is one cut's own frame, counted from the first frame the cut
// contributes.
type footageTake struct{ cut, local int }

// outputFootage is how the composition makes one output frame: one cut's own
// frame, or inside an overlap the xfade of the outgoing and incoming cuts at
// xfade's own progress, which is 1 on the overlap's first frame.
type outputFootage struct {
	outgoing, incoming footageTake
	kind               string
	progress           float64
}

func (o outputFootage) blended() bool { return o.kind != "" }

func (t footageTimeline) at(frame int) (outputFootage, bool) {
	if frame < 0 || frame >= t.total {
		return outputFootage{}, false
	}
	for i := len(t.frames) - 1; i >= 0; i-- {
		if frame < t.starts[i] {
			continue
		}
		if frame >= t.starts[i]+t.frames[i] {
			return outputFootage{}, false
		}
		own := footageTake{cut: i, local: frame - t.starts[i]}
		if i > 0 && frame < t.starts[i]+t.overlaps[i] {
			k := frame - t.starts[i]
			return outputFootage{outgoing: footageTake{cut: i - 1, local: frame - t.starts[i-1]}, incoming: own,
				kind: t.kinds[i], progress: 1 - float64(k)/float64(t.overlaps[i])}, true
		}
		return outputFootage{outgoing: footageTake{cut: -1}, incoming: own}, true
	}
	return outputFootage{}, false
}

// groundRead is one of the three frames CDS-44 reads for one visual, and what
// the cuts under it gave: each take's mean colour over the region, and for a
// fade through black each take's region pixels, which that blend needs one by
// one because it clips.
type groundRead struct {
	visual  int
	footage outputFootage
	region  clip.Region
	means   [2][3]float64
	pixels  [2][]uint8
	filled  [2]bool
}

type groundSampler struct {
	canvas clip.Canvas
	reads  []groundRead
}

// newGroundSampler names the frames CDS-44 reads for every unplated caption,
// info, hook and ending visual: the first, middle and last frame of its window,
// over its own sampled bounds. An output-side seek keeps the first frame at or
// after the instant asked for, which is the frame this names.
func (r *Rendering) newGroundSampler(canvas clip.Canvas, plan clip.EditPlan, visuals []declaredVisual) groundSampler {
	timeline := newFootageTimeline(r.cfg, plan)
	duration := timeline.total * 1000 / r.cfg.FPS
	s := groundSampler{canvas: canvas}
	for i := range visuals {
		switch visuals[i].manifest.Role {
		case "caption", "info", "hook", "ending":
		default:
			continue
		}
		bounds := sampledBounds(visuals[i])
		if bounds.Width <= 0 || bounds.Height <= 0 {
			continue
		}
		window := declaredSampleWindow(visuals[i].manifest.StartMS, visuals[i].manifest.EndMS, duration, r.cfg.FPS)
		for _, at := range sampleOffsets(clip.Cut{EndMS: duration, Focal: clip.Point{X: .5, Y: .5}}, window) {
			frame := min(timeline.total-1, (at*r.cfg.FPS+999)/1000)
			footage, ok := timeline.at(frame)
			if !ok {
				continue
			}
			s.reads = append(s.reads, groundRead{visual: i, footage: footage, region: bounds})
		}
	}
	return s
}

// needs is every frame of one cut the reads take, in order.
func (s groundSampler) needs(cut int) []int {
	seen := map[int]bool{}
	for _, read := range s.reads {
		for _, take := range []footageTake{read.footage.outgoing, read.footage.incoming} {
			if take.cut == cut {
				seen[take.local] = true
			}
		}
	}
	out := make([]int, 0, len(seen))
	for local := range seen {
		out = append(out, local)
	}
	sort.Ints(out)
	return out
}

// take records what one decoded frame of a cut gives every read that uses it.
func (s *groundSampler) take(cut, local int, frame image.Image) {
	for i := range s.reads {
		read := &s.reads[i]
		for side, take := range []footageTake{read.footage.outgoing, read.footage.incoming} {
			if take.cut != cut || take.local != local {
				continue
			}
			if read.footage.kind == "fadeblack" {
				read.pixels[side] = regionPixels(frame, read.region)
			}
			r, g, b := regionMean(frame, read.region)
			read.means[side] = [3]float64{r, g, b}
			read.filled[side] = true
		}
	}
}

// sampleCut decodes the frames one cut contributes to the reads, through the
// chain the composition renders that cut with, while the cut's original is at
// hand. The frames share one decode of the source; the batch is bounded because
// each output carries its own chain.
func (r *Rendering) sampleCut(ctx context.Context, ws clip.MediaWorkspace, s *groundSampler, index int, cut clip.Cut, source clip.MediaSource) error {
	locals := s.needs(index)
	if len(locals) == 0 {
		return nil
	}
	if err := r.media.sourcePath(ws, source.Path); err != nil {
		return err
	}
	for start := 0; start < len(locals); start += r.cfg.SampleBatch {
		batch := locals[start:min(len(locals), start+r.cfg.SampleBatch)]
		if err := r.media.capacity(ws, int64(len(batch))*4*1024*1024); err != nil {
			return err
		}
		args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-ss", seconds(cut.StartMS), "-i", source.Path)
		paths := make([]string, len(batch))
		for i, local := range batch {
			paths[i] = filepath.Join(ws.Path, "ground-"+strconv.Itoa(index)+"-"+strconv.Itoa(local)+".png")
			args = append(args, "-map", "0:V:0", "-vf", groundFrameChain(r.cfg, s.canvas, cut, local), "-frames:v", "1", "-c:v", "png", "-threads", strconv.Itoa(r.media.cfg.EncodeThreads), paths[i])
		}
		_, err := r.media.run(ctx, ws, r.media.cfg.FFmpegPath, args...)
		for i, path := range paths {
			if err != nil {
				_ = os.Remove(path)
				continue
			}
			frame, e := readFrame(path)
			_ = os.Remove(path)
			if e != nil {
				err = e
				continue
			}
			s.take(index, batch[i], frame)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// groundFrameChain is bareFootageGraph cut down to one of the cut's frames.
func groundFrameChain(cfg clip.RenderConfig, canvas clip.Canvas, cut clip.Cut, local int) string {
	return "trim=duration=" + seconds(cut.SourceSpanMS()) + "," + rateChain(cut.Rate(), cfg.FPS) + "," + coverChain(canvas, cut.Focal) +
		",setsar=1,trim=start_frame=" + strconv.Itoa(local) + ":end_frame=" + strconv.Itoa(local+1) + ",setpts=PTS-STARTPTS,format=yuv444p"
}

// grounds is CDS-44's measurement of every sampled visual, in visual order. A
// read whose frames a cut never gave means the sampler was not handed that cut,
// which is an error rather than a ground of zero.
func (s groundSampler) grounds() (map[int]Luminance, error) {
	values, colours := map[int][]float64{}, map[int][][3]float64{}
	order := []int{}
	for _, read := range s.reads {
		if !read.filled[1] || read.footage.blended() && !read.filled[0] {
			return nil, clip.ErrInvalidMedia
		}
		mean := read.mean()
		if _, seen := values[read.visual]; !seen {
			order = append(order, read.visual)
		}
		values[read.visual] = append(values[read.visual], relativeLuminance(mean[0], mean[1], mean[2]))
		colours[read.visual] = append(colours[read.visual], mean)
	}
	out := make(map[int]Luminance, len(order))
	for _, visual := range order {
		out[visual] = summarize(values[visual], colours[visual])
	}
	return out, nil
}

// mean is the read's region mean colour in the composed frame. A dissolve is
// linear in every pixel, so its mean is the dissolve of the two means; a fade
// through black clips, so it is blended pixel by pixel.
func (read groundRead) mean() [3]float64 {
	switch read.footage.kind {
	case "":
		return read.means[1]
	case "fadeblack":
		return fadeBlackMean(read.pixels[0], read.pixels[1], read.footage.progress)
	}
	p := read.footage.progress
	var out [3]float64
	for c := range out {
		out[c] = read.means[0][c]*p + read.means[1][c]*(1-p)
	}
	return out
}

// apply gives every sampled visual its ground and what that ground implies.
func (s groundSampler) apply(canvas clip.Canvas, visuals []declaredVisual) error {
	grounds, err := s.grounds()
	if err != nil {
		return err
	}
	for index, ground := range grounds {
		visuals[index].ground = ground
		applyDeclaredGround(canvas, &visuals[index])
	}
	return nil
}

// SampleGrounds measures, from the retained originals, the ground CDS-44 names
// under every unplated declared text of a portable plan, loading each cut's
// original only while its own frames are read (CLIP-192). It is the server's
// measurement a browser render of the same plan is drawn with.
func (r *Rendering) SampleGrounds(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) ([]clip.SampledGround, error) {
	if err := r.media.validWorkspace(ws, true); err != nil {
		return nil, err
	}
	if load == nil || plan.Portable == nil {
		return nil, clip.ErrInvalid
	}
	layout, err := r.layoutComposition(ctx, ws, plan)
	if err != nil {
		return nil, err
	}
	plan = layout.plan
	if err := clip.ValidateEditPlan(r.cfg, compositionGeometry(plan), sources); err != nil {
		return nil, err
	}
	canvas, err := clip.ClipCanvas(plan.Ratio)
	if err != nil {
		return nil, err
	}
	byID := map[string]clip.RenderSource{}
	for _, source := range sources {
		byID[source.ID] = source
	}
	sampler := r.newGroundSampler(canvas, plan, layout.visuals)
	for _, i := range cutsBySource(plan.Cuts) {
		cut := plan.Cuts[i]
		if len(sampler.needs(i)) == 0 {
			continue
		}
		calls := 0
		err := load(ctx, cut.SourceID, func(source clip.MediaSource) error {
			calls++
			if calls != 1 || !loadedAsRendered(source, cut, byID[cut.SourceID]) {
				return clip.ErrInvalidMedia
			}
			return r.sampleCut(ctx, ws, &sampler, i, cut, source)
		})
		if err != nil {
			return nil, err
		}
		if calls != 1 {
			return nil, clip.ErrInvalidMedia
		}
	}
	grounds, err := sampler.grounds()
	if err != nil {
		return nil, err
	}
	return layout.sampledGrounds(grounds), nil
}

// cutsBySource is the order both cut loops visit a plan in: every cut of one
// original together, the originals in the order the plan first draws them and
// each one's cuts in plan order. A loader holds one original at a time, so a
// plan that alternates its sources would otherwise download each of them again
// for every run of cuts (ARCH-48); a fixed order keeps the grounds and the
// render's progress reproducible.
func cutsBySource(cuts []clip.Cut) []int {
	groups, order := map[string][]int{}, []string{}
	for i, cut := range cuts {
		if _, seen := groups[cut.SourceID]; !seen {
			order = append(order, cut.SourceID)
		}
		groups[cut.SourceID] = append(groups[cut.SourceID], i)
	}
	out := make([]int, 0, len(cuts))
	for _, id := range order {
		out = append(out, groups[id]...)
	}
	return out
}

// loadedAsRendered is the check a render makes of the original a loader hands
// it: the cut's own source, fingerprint and probed facts.
func loadedAsRendered(source clip.MediaSource, cut clip.Cut, expected clip.RenderSource) bool {
	return source.SourceID == cut.SourceID && source.Fingerprint == cut.Fingerprint && source.Info.DurationMS == expected.Info.DurationMS &&
		source.Info.Width == expected.Info.Width && source.Info.Height == expected.Info.Height && source.Info.HasAudio == expected.Info.HasAudio
}

// sampledGrounds names each measured visual by its instance and, for a rapid
// caption laid out as one visual per phrase, by the phrase's place in its
// caption.
func (layout declaredLayout) sampledGrounds(grounds map[int]Luminance) []clip.SampledGround {
	out := []clip.SampledGround{}
	phrases := map[string]int{}
	for i, visual := range layout.visuals {
		id := visual.manifest.InstanceID
		phrase := phrases[id]
		phrases[id]++
		ground, ok := grounds[i]
		if !ok {
			continue
		}
		out = append(out, clip.SampledGround{InstanceID: id, Phrase: phrase, Mean: ground.Mean, Sigma: ground.Sigma,
			R: ground.R, G: ground.G, B: ground.B, Frames: append([]float64(nil), ground.Frames...)})
	}
	return out
}

// applyGrounds draws every visual a measurement names with that ground, exactly
// as a render that sampled it itself would. A measurement naming no visual of
// this layout is ignored: the render it came from was frozen at this layout's
// revision, so it can only be one the layout does not lay out.
func (layout *declaredLayout) applyGrounds(canvas clip.Canvas, grounds []clip.SampledGround) {
	type key struct {
		id     string
		phrase int
	}
	byKey := make(map[key]clip.SampledGround, len(grounds))
	for _, g := range grounds {
		byKey[key{g.InstanceID, g.Phrase}] = g
	}
	phrases := map[string]int{}
	for i := range layout.visuals {
		id := layout.visuals[i].manifest.InstanceID
		phrase := phrases[id]
		phrases[id]++
		g, ok := byKey[key{id, phrase}]
		if !ok || len(g.Frames) == 0 {
			continue
		}
		layout.visuals[i].ground = Luminance{Mean: g.Mean, Sigma: g.Sigma, R: g.R, G: g.G, B: g.B, Frames: append([]float64(nil), g.Frames...)}
		applyDeclaredGround(canvas, &layout.visuals[i])
	}
}

// regionMean is a frame's mean colour over a region, read on every second pixel
// in each direction as CDS-44's measurement always has been.
func regionMean(frame image.Image, region clip.Region) (float64, float64, float64) {
	var sr, sg, sb, n float64
	forRegionSamples(frame, region, func(r, g, b float64) {
		sr, sg, sb, n = sr+r, sg+g, sb+b, n+1
	})
	if n == 0 {
		return 0, 0, 0
	}
	return sr / n, sg / n, sb / n
}

// regionPixels keeps the same samples as 8-bit RGB, for a blend that has to be
// taken pixel by pixel.
func regionPixels(frame image.Image, region clip.Region) []uint8 {
	out := []uint8{}
	forRegionSamples(frame, region, func(r, g, b float64) {
		out = append(out, uint8(r*255+.5), uint8(g*255+.5), uint8(b*255+.5))
	})
	return out
}

func forRegionSamples(frame image.Image, region clip.Region, visit func(r, g, b float64)) {
	bounds := frame.Bounds()
	x0, y0 := max(bounds.Min.X, int(region.X)), max(bounds.Min.Y, int(region.Y))
	x1, y1 := min(bounds.Max.X, int(region.X+region.Width)), min(bounds.Max.Y, int(region.Y+region.Height))
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			r, g, b, _ := frame.At(x, y).RGBA()
			visit(float64(r)/0xffff, float64(g)/0xffff, float64(b)/0xffff)
		}
	}
}

// fadeBlackMean is xfade's fadeblack over the composition's yuv444p footage,
// read back as RGB. Its black is Y=0 with neutral chroma, below video black, so
// the darkest frames of the fade clip to black, which no blend of RGB means
// reproduces.
func fadeBlackMean(outgoing, incoming []uint8, progress float64) [3]float64 {
	const phase = 0.2
	w0 := smoothstep(1-phase, 1, progress) * progress
	w1 := (1 - smoothstep(phase, 1, progress)) * (1 - progress)
	var sum [3]float64
	n := min(len(outgoing), len(incoming)) / 3
	for i := 0; i < n; i++ {
		y0, cb0, cr0 := rgbToYCbCr(outgoing[3*i], outgoing[3*i+1], outgoing[3*i+2])
		y1, cb1, cr1 := rgbToYCbCr(incoming[3*i], incoming[3*i+1], incoming[3*i+2])
		rest := 1 - w0 - w1
		r, g, b := yCbCrToRGB(w0*y0+w1*y1, w0*cb0+w1*cb1+rest*128, w0*cr0+w1*cr1+rest*128)
		sum[0], sum[1], sum[2] = sum[0]+r, sum[1]+g, sum[2]+b
	}
	if n == 0 {
		return sum
	}
	return [3]float64{sum[0] / float64(n), sum[1] / float64(n), sum[2] / float64(n)}
}

func smoothstep(edge0, edge1, x float64) float64 {
	t := min(1, max(0, (x-edge0)/(edge1-edge0)))
	return t * t * (3 - 2*t)
}

// BT.601 limited range, which the footage's untagged yuv444p is read back with.
func rgbToYCbCr(r, g, b uint8) (float64, float64, float64) {
	R, G, B := float64(r), float64(g), float64(b)
	return 16 + 0.256788*R + 0.504129*G + 0.097906*B,
		128 - 0.148223*R - 0.290993*G + 0.439216*B,
		128 + 0.439216*R - 0.367788*G - 0.071427*B
}

func yCbCrToRGB(y, cb, cr float64) (float64, float64, float64) {
	clamp := func(v float64) float64 { return min(255, max(0, v)) / 255 }
	y -= 16
	return clamp(1.164384*y + 1.596027*(cr-128)),
		clamp(1.164384*y - 0.391762*(cb-128) - 0.812968*(cr-128)),
		clamp(1.164384*y + 2.017232*(cb-128))
}
