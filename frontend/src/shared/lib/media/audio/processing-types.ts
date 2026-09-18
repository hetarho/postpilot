export type PcmChannels = Float32Array<ArrayBuffer>[]
export interface EncodedAudioTrack {
  config: AudioEncoderConfig
  decoderConfig: AudioDecoderConfig
  chunks: {
    type: EncodedAudioChunkType
    timestamp: number
    duration: number
    data: Uint8Array<ArrayBuffer>
  }[]
  sampleFrames: number
  durationUs: number
  primingFrames: number
  loudnessLUFS?: number
  truePeakDBTP: number
  silent: boolean
}
export interface AudioNormalization {
  channels: PcmChannels
  loudnessLUFS?: number
  truePeakDBTP: number
  silent: boolean
  gain: number
}
export type AudioOperation =
  | {
      kind: 'stretch'
      channels: PcmChannels
      sampleRate: number
      rate: number
      frames: number
      gain: number
    }
  | { kind: 'normalize'; channels: PcmChannels; target: number; ceiling: number }
  | {
      kind: 'encode'
      channels: PcmChannels
      config: AudioEncoderConfig
      batchFrames: number
      queueSize: number
    }
export type AudioWorkerRequest = AudioOperation & { id: number }
export type AudioWorkerResponse =
  | { id: number; kind: 'result'; result: PcmChannels | AudioNormalization | EncodedAudioTrack }
  | { id: number; kind: 'error'; error: string }
  | { id: number; kind: 'progress'; completedFrames: number; totalFrames: number }
