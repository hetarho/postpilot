package media

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/postpilot/backend/internal/clip"
)

// Exactly one proxy exists at a time. The synchronous callback observes/uploads
// it before it is deleted; no list of byte-bearing results escapes this method.
func (a *Adapter) PrepareAnalysisChunks(ctx context.Context, ws clip.MediaWorkspace, source clip.MediaSource, consume func(clip.AnalysisChunk) error) error {
	if err := a.sourcePath(ws, source.Path); err != nil {
		return err
	}
	if consume == nil || source.SourceID == "" || source.Fingerprint == "" || source.Info.DurationMS <= 0 || source.Info.DurationMS > a.cfg.Sources.MaxDurationMS {
		return clip.ErrInvalidMedia
	}
	for index, offset := 0, 0; offset < source.Info.DurationMS; index, offset = index+1, offset+a.cfg.ChunkDurationMS {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := clip.AnalysisChunk{Path: filepath.Join(ws.Path, fmt.Sprintf("proxy-%04d.mp4", index)), SourceID: source.SourceID, Fingerprint: source.Fingerprint, Index: index, OffsetMS: offset, DurationMS: min(a.cfg.ChunkDurationMS, source.Info.DurationMS-offset)}
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
	defer func() {
		if remove := os.Remove(chunk.Path); remove != nil && !os.IsNotExist(remove) {
			err = errors.Join(err, remove)
		}
	}()
	filter := fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease:force_divisible_by=2:reset_sar=1,fps=%d,format=yuv420p", a.cfg.LongEdge, a.cfg.LongEdge, a.cfg.FPS)
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-xerror", "-n", "-threads", strconv.Itoa(a.cfg.Threads), "-protocol_whitelist", "file,pipe", "-ss", seconds(chunk.OffsetMS), "-i", source.Path, "-t", seconds(chunk.DurationMS), "-map", "0:V:0"}
	if source.Info.HasAudio {
		args = append(args, "-map", "0:a:0", "-c:a", "aac", "-b:a", strconv.Itoa(a.cfg.AudioBitrate), "-ar", strconv.Itoa(a.cfg.AudioRate), "-ac", "2")
	} else {
		args = append(args, "-an")
	}
	args = append(args, "-sn", "-dn", "-vf", filter, "-c:v", "libx264", "-preset", "veryfast", "-crf", strconv.Itoa(a.cfg.CRF), "-threads", strconv.Itoa(a.cfg.Threads), "-fps_mode", "cfr", "-map_metadata", "-1", "-map_chapters", "-1", "-metadata:s:v:0", "rotate=0", "-movflags", "+faststart", "-f", "mp4", chunk.Path)
	if _, err = a.run(ctx, ws, a.cfg.FFmpegPath, args...); err != nil {
		return err
	}
	if err = a.sourcePath(ws, chunk.Path); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return consume(chunk)
}
