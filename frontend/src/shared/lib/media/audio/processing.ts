import type {
  AudioNormalization,
  AudioOperation,
  AudioWorkerResponse,
  EncodedAudioTrack,
  PcmChannels,
} from './processing-types'

/** CPU work and codecs stay off the UI thread; cancellation releases every pending request. */
export function createAudioProcessor(signal: AbortSignal) {
  signal.throwIfAborted()
  const worker = new Worker(new URL('./processing.worker.ts', import.meta.url), { type: 'module' })
  let id = 0,
    closed = false
  const pending = new Map<
    number,
    {
      resolve: (value: unknown) => void
      reject: (error: unknown) => void
      progress?: (completed: number, total: number) => void
    }
  >()
  const close = (
    reason: unknown = new DOMException('Audio processing cancelled', 'AbortError'),
  ) => {
    if (closed) return
    closed = true
    signal.removeEventListener('abort', abort)
    worker.terminate()
    for (const request of pending.values()) request.reject(reason)
    pending.clear()
  }
  const abort = () => close(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  worker.onmessage = (event: MessageEvent<AudioWorkerResponse>) => {
    const message = event.data,
      request = pending.get(message.id)
    if (!request) return
    if (message.kind === 'progress')
      request.progress?.(message.completedFrames, message.totalFrames)
    else {
      pending.delete(message.id)
      if (message.kind === 'error') request.reject(new Error(message.error))
      else request.resolve(message.result)
    }
  }
  worker.onerror = (event) => {
    event.preventDefault()
    close(new Error(event.message || 'AUDIO_WORKER_FAILED'))
  }
  worker.onmessageerror = () => close(new Error('AUDIO_WORKER_FAILED'))
  const request = <T>(
    operation: AudioOperation,
    progress?: (completed: number, total: number) => void,
  ) => {
    signal.throwIfAborted()
    if (closed) throw new Error('AUDIO_WORKER_CLOSED')
    return new Promise<T>((resolve, reject) => {
      const requestId = ++id
      pending.set(requestId, { resolve: (result) => resolve(result as T), reject, progress })
      try {
        worker.postMessage(
          { ...operation, id: requestId },
          operation.channels.map((channel) => channel.buffer),
        )
      } catch (error) {
        pending.delete(requestId)
        reject(error)
      }
    })
  }
  return {
    close,
    stretch: (channels: PcmChannels, sampleRate: number, rate: number, frames: number, gain = 1) =>
      request<PcmChannels>({ kind: 'stretch', channels, sampleRate, rate, frames, gain }),
    normalize: (channels: PcmChannels, target: number, ceiling: number) =>
      request<AudioNormalization>({ kind: 'normalize', channels, target, ceiling }),
    encode: (
      channels: PcmChannels,
      config: AudioEncoderConfig,
      batchFrames: number,
      queueSize: number,
      progress?: (completed: number, total: number) => void,
    ) =>
      request<EncodedAudioTrack>(
        { kind: 'encode', channels, config, batchFrames, queueSize },
        progress,
      ),
  }
}
