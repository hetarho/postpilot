package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/postpilot/backend/internal/clip"
)

func removeIntermediate(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (r *Rendering) renderComposition(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, sources []clip.RenderSource, load clip.RenderSourceLoader) (result clip.RenderedVideo, err error) {
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
	audio := false
	for _, cut := range plan.Cuts {
		audio = audio || byID[cut.SourceID].Info.HasAudio
	}
	cuts, wavs := []string{}, []string{}
	for i, cut := range plan.Cuts {
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
				return r.renderBareAudio(ctx, ws, cut, source, frames[i], wav)
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
	raw := filepath.Join(ws.Path, "composition-footage.mp4")
	cleanup = append(cleanup, raw)
	mergeStart := len(cleanup)
	args, err := r.compositionInputsFormat(ctx, ws, cuts, frames, transitions, "", nil, &cleanup, "yuv444p")
	if err != nil {
		return result, err
	}
	args = append(args, r.encodeProfile(false, 0, "yuv444p")...)
	if err = r.runRender(ctx, ws, raw, args); err != nil {
		return result, err
	}
	for _, path := range append(cuts, cleanup[mergeStart:]...) {
		if err = removeIntermediate(path); err != nil {
			return result, err
		}
	}
	// Sample the final, transitioned footage under the authored output window.
	// The loader's original can already be released at this point.
	composedSource := clip.MediaSource{Path: raw, Info: clip.MediaInfo{Width: canvas.Width, Height: canvas.Height, DurationMS: totalFrames * 1000 / r.cfg.FPS}}
	plates := make([]string, len(layout.visuals))
	for i := range layout.visuals {
		plates[i], err = r.declaredPlate(ctx, ws, canvas, &layout.visuals[i], composedSource, i)
		if err != nil {
			return result, err
		}
		cleanup = append(cleanup, plates[i])
	}
	elements := layout.elements()
	if err = clip.VerifyCompositionManifest(plan, elements, r.cfg.Composition); err != nil {
		return result, err
	}
	assembled, measured := "", loudness{}
	if audio {
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
	pieces, pieceFrames, err := r.overlayComposition(ctx, ws, raw, layout.visuals, plates, totalFrames, &cleanup)
	if err != nil {
		return result, err
	}
	if err = removeIntermediate(raw); err != nil {
		return result, err
	}
	args, err = r.compositionInputs(ctx, ws, pieces, pieceFrames, make([]int, len(pieces)), assembled, &measured, &cleanup)
	if err != nil {
		return result, err
	}
	args = append(args, r.encodeArgs(audio)...)
	if err = r.runRender(ctx, ws, output, args); err != nil {
		return result, err
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
	result, err = r.validateRenderedOutput(ctx, ws, output, plan, audio, manifest)
	if err != nil {
		return result, err
	}
	result.Elements, result.Plan = elements, &plan
	return result, nil
}
