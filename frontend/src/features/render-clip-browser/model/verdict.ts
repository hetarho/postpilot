import { clipBrowserEncoderConfig, type BrowserVideoTrack } from '@/entities/clip-preview'
import { type ClipRatio } from '@/entities/clip-project'
import { CLIP_BROWSER_RENDER, CLIP_DESIGN } from '@/entities/clip-design'
import type { EncodedAudioTrack } from '@/shared/lib/media'

export function browserRenderVerdict(
  video: BrowserVideoTrack,
  audio: (EncodedAudioTrack & { speechFingerprint?: string }) | undefined,
  ratio: ClipRatio,
  durationMS: number,
) {
  const expected = clipBrowserEncoderConfig(ratio).video
  const measured = video.outputMeasurements
  const codec = measured?.codec ?? video.decoderConfig.codec
  const measurements = {
    compositionVersion: video.compositionVersion ?? '',
    snapshotFingerprint: video.snapshotFingerprint ?? '',
    backgroundVersion: video.backgroundEvidence?.version ?? '',
    backgroundSnapshotFingerprint: video.backgroundEvidence?.snapshotFingerprint ?? '',
    backgroundDigest: video.backgroundEvidence?.digest ?? '',
    backgroundComplete: !!video.backgroundEvidence,
    backgroundSampleCount: (video.backgroundEvidence?.measurements.length ?? 0) * 3,
    backgroundNotices: video.backgroundEvidence?.notices ?? [],
    width: measured?.width ?? video.config.width,
    height: measured?.height ?? video.config.height,
    frameRateNumerator: video.config.framerate ?? 0,
    frameRateDenominator: 1,
    videoFrames: measured?.videoFrames ?? video.packetCount ?? video.chunks.length,
    videoCodec: codec.startsWith('avc1.') ? 'h264' : codec,
    videoProfile: /^avc1\.64/i.test(codec) ? 'High' : codec,
    hasAudio: measured?.hasAudio ?? Boolean(audio),
    audioCodec:
      (measured?.audioCodec ?? audio?.decoderConfig.codec) === 'mp4a.40.2'
        ? 'aac'
        : (measured?.audioCodec ?? audio?.decoderConfig.codec ?? ''),
    audioRate: measured?.audioRate ?? audio?.decoderConfig.sampleRate ?? 0,
    loudnessLufs: audio?.loudnessLUFS,
    silent: audio?.silent ?? false,
    truePeakDbtp: audio && Number.isFinite(audio.truePeakDBTP) ? audio.truePeakDBTP : undefined,
    speechFingerprint: audio?.speechFingerprint ?? '',
  }
  const fps = measurements.frameRateNumerator
  const duration = measured ? measured.videoDurationUs / 1e6 : measurements.videoFrames / fps
  // Encoders return decode order; B-frames may have earlier presentation times.
  const presentation = [...video.chunks].sort((a, b) => a.timestamp - b.timestamp)
  const passed =
    (!video.compositionVersion ||
      (!!video.backgroundEvidence &&
        video.backgroundEvidence.version === 'clip-browser-background-v1' &&
        video.backgroundEvidence.sourceColorVersion === 'native-source-color-v1' &&
        video.backgroundEvidence.snapshotFingerprint === video.snapshotFingerprint &&
        !!video.localSnapshotFingerprint &&
        video.backgroundEvidence.localSnapshotFingerprint === video.localSnapshotFingerprint &&
        /^[a-f0-9]{64}$/u.test(video.backgroundEvidence.digest) &&
        video.backgroundEvidence.measurements.length <= 800)) &&
    measurements.width === expected.width &&
    measurements.height === expected.height &&
    fps === CLIP_BROWSER_RENDER.frameRate &&
    measurements.videoCodec === 'h264' &&
    measurements.videoProfile === 'High' &&
    measurements.videoFrames > 0 &&
    video.frameCount === measurements.videoFrames &&
    (video.packetCount === undefined ||
      (!!measured && video.presentationValid === true && measured.presentationValid)) &&
    (video.packetCount === undefined ||
      video.speechFingerprint === (audio?.speechFingerprint ?? '')) &&
    measurements.hasAudio === !!audio &&
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
            Math.abs(audio.loudnessLUFS - CLIP_DESIGN.audio.loudnorm.i) <= 1 &&
            Number.isFinite(audio.truePeakDBTP) &&
            audio.truePeakDBTP <= CLIP_DESIGN.audio.loudnorm.tp))))
  return { measurements, passed }
}
export type BrowserRenderVerdict = ReturnType<typeof browserRenderVerdict>
