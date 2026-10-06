export {
  formatDuration,
  readVideoMetadata,
  VideoUnreadableError,
  type VideoMetadata,
  probeEncoderSupport,
  type EncoderSupport,
} from './video'
export { createAudioProcessor } from './audio/processing'
export { createAudioRangeReader } from './audio/range-reader'
export { canonicalSelectedAudio } from './audio/selected-pcm'
export { audioGuardWindow, decodeOriginalAudioRange } from './audio/range-audio'
export type {
  AudioRangeLimits,
  AudioSourceRange,
  OriginalAudioMetadata,
  SelectedAudioRange,
} from './audio/range-audio'
export { integratedLoudness48k, normalizeLoudness48k, truePeak48k } from './audio/loudness'
export { mp4HasAudio, mp4AudioDecodedBytes } from './video/mp4-audio'
export type { AudioNormalization, EncodedAudioTrack, PcmChannels } from './audio/processing-types'
export { muxMp4 } from './mux-mp4'
export { MediaPhaseRecorder } from './phase-metrics'
export type { MediaPhaseMeasurement, MediaPhaseSnapshot } from './phase-metrics'
export { createFiniteMediaSource, MediaRangeError } from './video/range-source'
export type {
  BrowserMediaSourceAccess,
  MediaRangeLimits,
  MediaRangeMeasurements,
  MediaRangePorts,
} from './video/range-source'
export { VideoFrameBudget } from './video/frame-budget'
export { openOriginalVideo, OriginalVideoCursor, nativeOutputFrame } from './video/range-video'
export type {
  OriginalVideoInput,
  OriginalVideoMetadata,
  OriginalVideoPorts,
  VideoRangeSample,
  DecodedVideoResource,
  VideoDrawRect,
} from './video/range-video'
