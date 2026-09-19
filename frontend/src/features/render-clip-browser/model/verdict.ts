import { clipBrowserEncoderConfig, type BrowserVideoTrack } from '@/entities/clip-preview'
import { type ClipRatio } from '@/entities/clip-project'
import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design'
import type { EncodedAudioTrack } from '@/shared/lib/media'

export function browserRenderVerdict(
  video: BrowserVideoTrack,
  audio: EncodedAudioTrack | undefined,
  ratio: ClipRatio,
  durationMS: number,
) {
  const expected = clipBrowserEncoderConfig(ratio).video
  const codec = video.decoderConfig.codec
  const measurements = {
    width: video.config.width,
    height: video.config.height,
    frameRateNumerator: video.config.framerate ?? 0,
    frameRateDenominator: 1,
    videoFrames: video.chunks.length,
    videoCodec: codec.startsWith('avc1.') ? 'h264' : codec,
    videoProfile: /^avc1\.64/i.test(codec) ? 'High' : codec,
    hasAudio: Boolean(audio),
    audioCodec:
      audio?.decoderConfig.codec === 'mp4a.40.2' ? 'aac' : (audio?.decoderConfig.codec ?? ''),
    audioRate: audio?.decoderConfig.sampleRate ?? 0,
    loudnessLufs: audio?.loudnessLUFS,
    silent: audio?.silent ?? false,
  }
  const fps = measurements.frameRateNumerator
  const duration = measurements.videoFrames / fps
  // Encoders return decode order; B-frames may have earlier presentation times.
  const presentation = [...video.chunks].sort((a, b) => a.timestamp - b.timestamp)
  const passed =
    measurements.width === expected.width &&
    measurements.height === expected.height &&
    fps === CLIP_BROWSER_RENDER.frameRate &&
    measurements.videoCodec === 'h264' &&
    measurements.videoProfile === 'High' &&
    measurements.videoFrames > 0 &&
    video.frameCount === measurements.videoFrames &&
    presentation.every(
      (c, i) =>
        Math.abs(c.timestamp - (i * 1e6) / fps) <= 1 && Math.abs(c.duration - 1e6 / fps) <= 1,
    ) &&
    Math.abs(duration * 1000 - durationMS) <= 1000 / fps &&
    (!audio ||
      (measurements.audioCodec === 'aac' &&
        measurements.audioRate === CLIP_BROWSER_RENDER.audioSampleRate &&
        Math.abs(audio.sampleFrames / audio.config.sampleRate - duration) <=
          1 / audio.config.sampleRate &&
        (audio.silent ||
          (audio.loudnessLUFS !== undefined &&
            Number.isFinite(audio.loudnessLUFS) &&
            Math.abs(audio.loudnessLUFS - CLIP_DESIGN.audio.loudnorm.i) <= 1))))
  return { measurements, passed }
}
export type BrowserRenderVerdict = ReturnType<typeof browserRenderVerdict>
