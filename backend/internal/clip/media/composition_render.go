package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/postpilot/backend/internal/clip"
)

func removeIntermediate(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	// A caption's frame sequence is a directory; every other intermediate is a
	// file, so the fallback cannot swallow a path it was not given.
	if err != nil {
		if info, statErr := os.Lstat(path); statErr == nil && info.IsDir() {
			return os.RemoveAll(path)
		}
	}
	return err
}

// releaseLayers deletes what the overlay pass has finished reading. A caption's
// frames are dead the moment the pass that consumed them is encoded, so a long
// sequence never sits in the workspace beside the output it helped make
// (CLIP-33).
func releaseLayers(layers []captionLayer) error {
	var err error
	for _, layer := range layers {
		for _, path := range layer.paths() {
			err = errors.Join(err, removeIntermediate(path))
		}
	}
	return err
}

// removeConsumed deletes every candidate the merge tree did not carry into the
// arguments it returned. A merge cannot do this itself: the cuts it reads are
// the CALLER's, and on the per-cut path they still hold the audio that has to
// outlive the picture. Here the footage is bare and its sound is already a
// separate track, so what the tree stopped reading is dead the moment it
// returns rather than when the whole render ends.
func removeConsumed(candidates, args []string) error {
	alive := map[string]bool{}
	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) {
			alive[args[i+1]] = true
		}
	}
	for _, path := range candidates {
		if alive[path] {
			continue
		}
		if err := removeIntermediate(path); err != nil {
			return err
		}
	}
	return nil
}

func (r *Rendering) renderComposition(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (result clip.RenderedVideo, err error) {
	substage, started := "render_layout", time.Now()
	step := func(next string) {
		clip.ReportMediaStage(ctx, substage, time.Since(started))
		substage, started = next, time.Now()
	}
	defer func() {
		clip.ReportMediaStage(ctx, substage, time.Since(started))
		if err != nil {
			if _, known := clip.DiagnosticFromError(err); !known {
				err = clip.WithAttemptDiagnostic(err, clip.AttemptDiagnostic{Check: substage, Phase: "render"})
			}
		}
	}()
	if err = r.media.validWorkspace(ws, true); err != nil {
		return result, err
	}
	if load == nil {
		return result, clip.ErrInvalid
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	output := filepath.Join(ws.Path, "clip-result.mp4")
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return result, errors.New("clip result path already exists")
	}
	cleanup := []string{}
	defer func() {
		for _, path := range cleanup {
			err = errors.Join(err, removeIntermediate(path))
		}
		if err != nil {
			err = errors.Join(err, removeIntermediate(output))
		}
	}()
	layout, err := r.layoutComposition(ctx, ws, plan)
	if err != nil {
		return result, err
	}
	plan = layout.plan
	if err = clip.ValidateEditPlan(r.cfg, compositionGeometry(plan), sources); err != nil {
		return result, err
	}
	canvas, _ := clip.ClipCanvas(plan.Ratio)
	frames, transitions := cutFrames(plan, r.cfg.FPS), planTransitions(plan)
	totalFrames := 0
	for i, n := range frames {
		totalFrames += n - transitionFrames(r.cfg, transitions[i])
	}
	byID := map[string]clip.RenderSource{}
	for _, source := range sources {
		byID[source.ID] = source
	}
	// The clip has an audio track when at least one selected cut comes from a
	// source the owner ENABLED that actually carries sound; otherwise the MP4
	// gets no audio stream at all rather than a synthetic silent one (CDS-35).
	audio := false
	for _, cut := range plan.Cuts {
		audio = audio || plan.RetainsOriginalAudio(cut) && byID[cut.SourceID].Info.HasAudio
	}
	cuts, wavs := []string{}, []string{}
	step("render_footage")
	for i, cut := range plan.Cuts {
		step("render_footage")
		video := filepath.Join(ws.Path, fmt.Sprintf("bare-%04d.mp4", i))
		cleanup = append(cleanup, video)
		cuts = append(cuts, video)
		wav := filepath.Join(ws.Path, fmt.Sprintf("audio-%04d.wav", i))
		if audio {
			cleanup = append(cleanup, wav)
			wavs = append(wavs, wav)
		}
		calls := 0
		err = load(ctx, cut.SourceID, func(source clip.MediaSource) error {
			calls++
			expected := byID[cut.SourceID]
			if calls != 1 || source.SourceID != cut.SourceID || source.Fingerprint != cut.Fingerprint || source.Info.DurationMS != expected.Info.DurationMS || source.Info.Width != expected.Info.Width || source.Info.Height != expected.Info.Height || source.Info.HasAudio != expected.Info.HasAudio {
				return clip.ErrInvalidMedia
			}
			if err := r.renderBareFootage(ctx, ws, canvas, cut, source, frames[i], video); err != nil {
				return err
			}
			if audio {
				step("render_audio")
				return r.renderBareAudio(ctx, ws, cut, source, frames[i], wav, plan.RetainsOriginalAudio(cut))
			}
			return nil
		})
		if err != nil {
			return result, err
		}
		if calls != 1 {
			return result, clip.ErrInvalidMedia
		}
	}
	step("render_encode")
	raw := filepath.Join(ws.Path, "composition-footage.mp4")
	cleanup = append(cleanup, raw)
	mergeStart := len(cleanup)
	args, err := r.compositionInputsFormat(ctx, ws, cuts, frames, transitions, "", nil, &cleanup, "yuv444p")
	if err != nil {
		return result, err
	}
	// Free what the tree stopped reading BEFORE the root encode, not after it.
	// Waiting until the end kept the leaves, the merges and the growing root
	// alive together: three lossless copies of one clip in a workspace bounded
	// for about two, which is where a long plan ran out of room.
	consumed := append(append([]string{}, cuts...), cleanup[mergeStart:]...)
	if err = removeConsumed(consumed, args); err != nil {
		return result, err
	}
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	if err = r.runRender(ctx, ws, raw, args); err != nil {
		return result, err
	}
	for _, path := range consumed {
		if err = removeIntermediate(path); err != nil {
			return result, err
		}
	}
	// Sample the final, transitioned footage under the authored output window.
	// The loader's original can already be released at this point.
	composedSource := clip.MediaSource{Path: raw, Info: clip.MediaInfo{Width: canvas.Width, Height: canvas.Height, DurationMS: totalFrames * 1000 / r.cfg.FPS}}
	step("render_overlay")
	if err = r.sampleDeclaredGrounds(ctx, ws, canvas, composedSource, layout.visuals); err != nil {
		return result, err
	}
	layers := make([]captionLayer, len(layout.visuals))
	for i := range layout.visuals {
		layers[i], err = r.declaredLayer(ctx, ws, canvas, &layout.visuals[i], composedSource, i)
		if err != nil {
			return result, err
		}
		cleanup = append(cleanup, layers[i].paths()...)
	}
	layout.recordContrastNotices()
	plan = layout.plan
	elements := layout.elements()
	if err = clip.VerifyCompositionManifest(plan, elements, r.cfg.Composition); err != nil {
		return result, err
	}
	assembled, measured := "", loudness{}
	if audio {
		step("render_audio")
		assembled = filepath.Join(ws.Path, "composition-audio.wav")
		cleanup = append(cleanup, assembled)
		if err = r.assembleDeclaredAudio(ctx, ws, wavs, frames, transitions, elements, assembled); err != nil {
			return result, err
		}
		for _, path := range wavs {
			if err = removeIntermediate(path); err != nil {
				return result, err
			}
		}
		measured, err = r.measureLoudness(ctx, ws, assembled)
		if err != nil {
			return result, err
		}
	}
	step("render_overlay")
	windows := compositionWindows(layout.visuals, totalFrames, r.cfg.FPS, r.cfg.OverlayBatchSize)
	// One window whose layers fit one batch needs no intermediate at all: the
	// overlay graph runs straight into the delivery encode.
	if len(windows) == 1 && len(windows[0].Layers) <= r.cfg.OverlayBatchSize {
		step("render_encode")
		args = r.deliveredOverlayArgs(raw, windows[0], layout.visuals, layers, assembled, &measured)
		args = append(args, r.encodeArgs(audio)...)
		if err = r.runRender(ctx, ws, output, args); err != nil {
			return result, err
		}
		if err = errors.Join(removeIntermediate(raw), releaseLayers(layers)); err != nil {
			return result, err
		}
	} else {
		pieces, pieceFrames, err := r.overlayComposition(ctx, ws, raw, windows, layout.visuals, layers, &cleanup)
		if err != nil {
			return result, err
		}
		if err = errors.Join(removeIntermediate(raw), releaseLayers(layers)); err != nil {
			return result, err
		}
		args, err = r.compositionInputs(ctx, ws, pieces, pieceFrames, make([]int, len(pieces)), assembled, &measured, &cleanup)
		if err != nil {
			return result, err
		}
		step("render_encode")
		args = append(args, r.encodeArgs(audio)...)
		if err = r.runRender(ctx, ws, output, args); err != nil {
			return result, err
		}
	}
	if audio {
		if err = r.finishLoudness(ctx, ws, output, assembled, measured, totalFrames); err != nil {
			return result, err
		}
	}
	manifest := clip.Manifest{}
	for _, element := range elements {
		manifest = append(manifest, element.Parts...)
	}
	step("render_validate")
	result, err = r.validateRenderedOutput(ctx, ws, output, plan, audio, manifest)
	if err != nil {
		return result, err
	}
	result.Elements, result.Plan = elements, &plan
	return result, nil
}
