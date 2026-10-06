import { decodeOriginalAudioRange } from '@/shared/lib'
import { CLIP_AUDIO_PROCESSING } from '@/entities/clip-preview'

const live = new Map<AudioData, number>()
let peakFrames = 0,
  peakBytes = 0
const nativeClose = AudioData.prototype.close
AudioData.prototype.close = function () {
  live.delete(this)
  return nativeClose.call(this)
}
const NativeDecoder = AudioDecoder
self.AudioDecoder = class extends NativeDecoder {
  constructor(options: AudioDecoderInit) {
    super({
      ...options,
      output: (data) => {
        live.set(data, data.numberOfFrames * data.numberOfChannels * 4)
        peakFrames = Math.max(peakFrames, live.size)
        peakBytes = Math.max(
          peakBytes,
          [...live.values()].reduce((sum, bytes) => sum + bytes, 0),
        )
        options.output(data)
      },
    })
  }
}
self.onmessage = async (
  event: MessageEvent<{
    url: string
    startUs: number
    endUs: number
    cancel?: boolean
    memoryBytes?: number
  }>,
) => {
  const request = event.data,
    controller = new AbortController()
  if (request.cancel)
    setTimeout(() => controller.abort(new DOMException('Fixture cancelled', 'AbortError')), 10)
  try {
    const result = await decodeOriginalAudioRange(
      { kind: 'url', url: request.url },
      { startUs: request.startUs, endUs: request.endUs, targetSampleRate: 48000 },
      {
        ...CLIP_AUDIO_PROCESSING,
        maxPcmBytes: request.memoryBytes ?? CLIP_AUDIO_PROCESSING.maxPcmBytes,
      },
      controller.signal,
    )
    await new Promise((resolve) => setTimeout(resolve, 50))
    self.postMessage(
      {
        result,
        resources: {
          peakFrames,
          peakBytes,
          liveFrames: live.size,
          liveBytes: [...live.values()].reduce((sum, bytes) => sum + bytes, 0),
        },
      },
      { transfer: result?.channels.map((channel) => channel.buffer) ?? [] },
    )
  } catch (error) {
    await new Promise((resolve) => setTimeout(resolve, 50))
    self.postMessage({
      error: error instanceof Error ? error.message : String(error),
      name: error instanceof Error ? error.name : '',
      resources: { peakFrames, peakBytes, liveFrames: live.size },
    })
  }
}
