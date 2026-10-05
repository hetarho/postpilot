package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/clip"
)

// MP3 info/priming/tail frames can be removed by gapless native decoders. Keep
// the measured frame-duration bound, padding only the codec's missing silence.
const speechCodecPaddingSamples = 3 * 1152

func (r *Rendering) prepareSpeechAudio(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, load clip.RenderSpeechLoader) (map[string]string, []string, error) {
	files := map[string]string{}
	cleanup := []string{}
	if plan.Narration == nil || !plan.Narration.Enabled {
		return files, cleanup, nil
	}
	if load == nil {
		return nil, cleanup, clip.ErrCompositionUnavailable
	}
	for _, segment := range plan.Narration.Segments {
		ref := segment.Speech
		if _, exists := files[ref.AssetID]; exists {
			continue
		}
		output := filepath.Join(ws.Path, fmt.Sprintf("speech-%03d.s16le", len(files)))
		cleanup = append(cleanup, output)
		calls := 0
		err := load(ctx, ref.AssetID, func(path string) error {
			calls++
			if calls != 1 {
				return clip.ErrInvalidMedia
			}
			if err := r.media.sourcePath(ws, path); err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			info, err := file.Stat()
			if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > clip.SpeechMaxAssetBytes {
				file.Close()
				return clip.ErrInvalidMedia
			}
			sum := sha256.New()
			n, err := io.Copy(sum, io.LimitReader(file, clip.SpeechMaxAssetBytes+1))
			err = errors.Join(err, file.Close())
			if err != nil {
				return err
			}
			if n != info.Size() || hex.EncodeToString(sum.Sum(nil)) != ref.AudioHash {
				return clip.ErrInvalidMedia
			}
			ceiling := ref.Samples*int64(ref.Channels)*2 + 1
			if err = r.media.capacity(ws, ceiling); err != nil {
				return err
			}
			args := append(r.baseArgs(), "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-c:a", "mp3", "-i", path, "-map", "0:a:0", "-vn", "-ar", strconv.Itoa(ref.SampleRate), "-ac", strconv.Itoa(ref.Channels), "-c:a", "pcm_s16le", "-f", "s16le", "-fs", strconv.FormatInt(ceiling, 10), output)
			if _, err = r.media.runBounded(ctx, ws, r.media.cfg.FFmpegPath, output, ceiling, clip.ErrInvalidMedia, args...); err != nil {
				return err
			}
			pcm, err := os.Stat(output)
			if err != nil {
				return err
			}
			frames := pcm.Size() / int64(ref.Channels*2)
			if pcm.Size()%int64(ref.Channels*2) != 0 || frames <= 0 || frames > ref.Samples || ref.Samples-frames > speechCodecPaddingSamples {
				return clip.ErrInvalidMedia
			}
			return nil
		})
		if err != nil {
			return nil, cleanup, err
		}
		if calls != 1 {
			return nil, cleanup, clip.ErrInvalidMedia
		}
		files[ref.AssetID] = output
	}
	return files, cleanup, nil
}

func narrationAudioGraph(cfg clip.RenderConfig, plan clip.EditPlan, source bool, inputs map[string]int, totalFrames int) string {
	var graph strings.Builder
	labels := []string{}
	if source {
		volume := 1.0
		if plan.SourceVolumePermille != nil {
			volume = float64(*plan.SourceVolumePermille) / 1000
		}
		fmt.Fprintf(&graph, "[0:a:0]volume=%.6f[source];", volume)
		labels = append(labels, "[source]")
	}
	for i, segment := range plan.Narration.Segments {
		ref := segment.Speech
		// Natural-rate PCM is never tied to a cut's rate, fade, hook dip or trim.
		frames := (ref.Samples*int64(cfg.AudioRate) + int64(ref.SampleRate) - 1) / int64(ref.SampleRate)
		delay := int64(segment.StartMS) * int64(cfg.AudioRate) / 1000
		fmt.Fprintf(&graph, "[%d:a:0]aresample=%d,aformat=sample_fmts=fltp:channel_layouts=stereo,apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS,volume=%.6f,adelay=%dS:all=1[speech%d];", inputs[segment.ID], cfg.AudioRate, frames, float64(plan.Narration.VolumePermille)/1000, delay, i)
		labels = append(labels, fmt.Sprintf("[speech%d]", i))
	}
	fmt.Fprintf(&graph, "%samix=inputs=%d:duration=longest:normalize=0:dropout_transition=0,apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS[a]", strings.Join(labels, ""), len(labels), totalFrames*cfg.AudioRate/cfg.FPS)
	return graph.String()
}

func (r *Rendering) mixNarrationAudio(ctx context.Context, ws clip.MediaWorkspace, plan clip.EditPlan, source string, speech map[string]string, totalFrames int, output string) error {
	args := r.baseArgs()
	index := 0
	if source != "" {
		args = r.inputArgs(args, source)
		index++
	}
	inputs := map[string]int{}
	for _, segment := range plan.Narration.Segments {
		ref := segment.Speech
		path := speech[ref.AssetID]
		if path == "" {
			return clip.ErrInvalidMedia
		}
		args = append(args, "-threads", strconv.Itoa(r.media.cfg.DecodeThreads), "-protocol_whitelist", "file,pipe", "-f", "s16le", "-ar", strconv.Itoa(ref.SampleRate), "-ac", strconv.Itoa(ref.Channels), "-i", path)
		inputs[segment.ID], index = index, index+1
	}
	args = append(args, "-filter_complex", narrationAudioGraph(r.cfg, plan, source != "", inputs, totalFrames), "-map", "[a]", "-vn", "-c:a", "pcm_s16le", "-ar", strconv.Itoa(r.cfg.AudioRate), "-ac", "2", "-f", "wav")
	return r.runRender(ctx, ws, output, args)
}
