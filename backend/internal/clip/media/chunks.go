package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
)

// All outputs remain workspace-owned until their individual analysis finishes.
// The callback collects only verified paths and metadata, never encoded bytes.
func (a *Adapter) PrepareAnalysisChunks(ctx context.Context, ws clip.MediaWorkspace, source clip.MediaSource, consume func(clip.AnalysisChunk) error) error {
	if err := a.sourcePath(ws, source.Path); err != nil {
		return err
	}
	if consume == nil || source.SourceID == "" || source.Fingerprint == "" || source.Info.DurationMS <= 0 || source.Info.DurationMS > a.cfg.Sources.MaxDurationMS || source.Info.Width <= 0 || source.Info.Height <= 0 {
		return clip.ErrInvalidMedia
	}
	id := sha256.Sum256([]byte(source.SourceID))
	for index, offset := 0, 0; offset < source.Info.DurationMS; index, offset = index+1, offset+a.cfg.ChunkDurationMS {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := clip.AnalysisChunk{Path: filepath.Join(ws.Path, fmt.Sprintf("proxy-%x-%04d.mp4", id, index)), SourceID: source.SourceID, Fingerprint: source.Fingerprint, Index: index, OffsetMS: offset, DurationMS: min(a.cfg.ChunkDurationMS, source.Info.DurationMS-offset)}
		if err := a.prepareChunk(ctx, ws, source, chunk, consume); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) prepareChunk(ctx context.Context, ws clip.MediaWorkspace, source clip.MediaSource, chunk clip.AnalysisChunk, consume func(clip.AnalysisChunk) error) (err error) {
	if _, e := os.Lstat(chunk.Path); !os.IsNotExist(e) {
		return errors.New("clip proxy path already exists")
	}
	retained := false
	defer func() {
		if !retained {
			if remove := os.Remove(chunk.Path); remove != nil && !os.IsNotExist(remove) {
				err = errors.Join(err, remove)
			}
		}
	}()
	if err = a.capacity(ws, a.cfg.AnalysisMaxBytes); err != nil {
		return err
	}
	for attempt := 0; attempt < 2; attempt++ {
		maxRate, buffer := a.cfg.VideoMaxRate, a.cfg.VideoBufferSize
		if attempt == 1 {
			maxRate, buffer = a.cfg.RetryMaxRate, a.cfg.RetryBufferSize
			if err = os.Remove(chunk.Path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		args := a.chunkArgs(source, chunk, maxRate, buffer)
		_, err = a.runBounded(ctx, ws, a.cfg.FFmpegPath, chunk.Path, a.cfg.AnalysisMaxBytes, clip.ErrAnalysisTooLarge, args...)
		if errors.Is(err, clip.ErrAnalysisTooLarge) && ctx.Err() == nil && !errors.Is(err, clip.ErrWorkspaceLimit) {
			continue
		}
		if err != nil {
			return err
		}
		file, e := os.Lstat(chunk.Path)
		if e != nil || !file.Mode().IsRegular() || file.Size() <= 0 {
			return clip.ErrInvalidMedia
		}
		chunk.Bytes = file.Size()
		if chunk.Bytes >= a.cfg.AnalysisMaxBytes {
			// -fs may stop successfully with a playable but incomplete MP4. Never
			// interpret that exit code as full coverage; retry the original interval.
			err = clip.ErrAnalysisTooLarge
			continue
		}
		chunk.Info, err = a.Probe(ctx, ws, chunk.Path)
		if err != nil {
			return err
		}
		if !a.validChunk(source, chunk) {
			return fmt.Errorf("%w: analysis interval %d+%d ms, measured %+v", clip.ErrInvalidMedia, chunk.OffsetMS, chunk.DurationMS, chunk.Info)
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		if err = consume(chunk); err != nil {
			return err
		}
		retained = true
		return nil
	}
	return clip.ErrAnalysisTooLarge
}

func (a *Adapter) chunkArgs(source clip.MediaSource, chunk clip.AnalysisChunk, maxRate, buffer int) []string {
	// Display dimensions already include source rotation/SAR. A fixed even-sized
	// target avoids upscaling and accidental square-pixel aspect distortion.
	scale := math.Min(1, float64(a.cfg.LongEdge)/float64(max(source.Info.Width, source.Info.Height)))
	w, h := max(2, int(float64(source.Info.Width)*scale)/2*2), max(2, int(float64(source.Info.Height)*scale)/2*2)
	filter := fmt.Sprintf("scale=%d:%d,setsar=1,fps=%d,format=yuv420p", w, h, a.cfg.FPS)
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-xerror", "-n", "-filter_threads", "1", "-filter_complex_threads", "1", "-threads", "1", "-protocol_whitelist", "file,pipe", "-ss", seconds(chunk.OffsetMS), "-i", source.Path, "-t", seconds(chunk.DurationMS), "-map", "0:V:0"}
	if source.Info.HasAudio {
		// Trim exact samples before AAC packetization. Output -t alone can leave
		// an extra partial AAC packet in the MP4 (60.011 s for a 60 s interval).
		// first_pts fills a delayed audio start without moving speech earlier.
		audio := fmt.Sprintf("aresample=%d:async=1:min_hard_comp=0:first_pts=0,apad,atrim=end_sample=%d,asetpts=PTS-STARTPTS", a.cfg.AudioRate, int64(chunk.DurationMS)*int64(a.cfg.AudioRate)/1000)
		args = append(args, "-map", "0:a:0", "-af", audio, "-c:a", "aac", "-b:a", strconv.Itoa(a.cfg.AudioBitrate), "-ar", strconv.Itoa(a.cfg.AudioRate), "-ac", "1")
	} else {
		args = append(args, "-an")
	}
	return append(args, "-sn", "-dn", "-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", strconv.Itoa(a.cfg.CRF), "-maxrate", strconv.Itoa(maxRate), "-bufsize", strconv.Itoa(buffer), "-threads", "1", "-fps_mode", "cfr", "-map_metadata", "-1", "-map_chapters", "-1", "-metadata:s:v:0", "rotate=0", "-movflags", "+faststart", "-fs", strconv.FormatInt(a.cfg.AnalysisMaxBytes, 10), "-f", "mp4", chunk.Path)
}

func (a *Adapter) validChunk(source clip.MediaSource, chunk clip.AnalysisChunk) bool {
	p := chunk.Info
	frame := (1000 + a.cfg.FPS - 1) / a.cfg.FPS
	// Decode every stream, then cross-check presentation duration. AAC decoder
	// padding may extend the decode clock by one audio packet, not the MP4's
	// playable timeline. Neither the video nor the container may exceed 60 s.
	if p.Width <= 0 || p.Height <= 0 || max(p.Width, p.Height) > a.cfg.LongEdge || p.Width > source.Info.Width || p.Height > source.Info.Height || p.Rotation != 0 || p.PixelFormat != "yuv420p" || p.SampleAspectRatio != "1:1" || p.FrameRateNumerator != a.cfg.FPS*p.FrameRateDenominator || p.FrameRateDenominator <= 0 || p.HasAudio != source.Info.HasAudio || p.ContainerDurationMS <= 0 || p.ContainerDurationMS > a.cfg.ChunkDurationMS || p.VideoDurationMS <= 0 || p.VideoDurationMS > a.cfg.ChunkDurationMS || abs(p.VideoDurationMS-chunk.DurationMS) > frame || abs(p.ContainerDurationMS-chunk.DurationMS) > frame || abs(p.DurationMS-p.ContainerDurationMS) > frame {
		return false
	}
	if p.DecodedDurationMS > 0 && abs(p.DecodedDurationMS-p.ContainerDurationMS) > frame {
		return false
	}
	if p.HasAudio && (p.AudioChannels != 1 || p.AudioRate != a.cfg.AudioRate || abs(p.AudioDurationMS-chunk.DurationMS) > 22) {
		return false
	}
	for _, stream := range p.Streams {
		if (stream.Kind == "video" && stream.Codec != "h264") || (stream.Kind == "audio" && stream.Codec != "aac") || (stream.Kind != "video" && stream.Kind != "audio") {
			return false
		}
	}
	return true
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
