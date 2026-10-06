import type {
  AudioNormalization,
  AudioOperation,
  AudioWorkerResponse,
  EncodedAudioTrack,
  PcmChannels,
  AudioProcessorLimits,
} from './processing-types'

/** CPU work and codecs stay off the UI thread; cancellation releases every pending request. */
export function createAudioProcessor(signal: AbortSignal, limits: AudioProcessorLimits) {
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
      timer: ReturnType<typeof setTimeout>
    }
  >()
  const close = (
    reason: unknown = new DOMException('Audio processing cancelled', 'AbortError'),
  ) => {
    if (closed) return
    closed = true
    signal.removeEventListener('abort', abort)
    for (const request of pending.values()) {
      clearTimeout(request.timer)
      request.reject(reason)
    }
    pending.clear()
    cleanupTimer = setTimeout(terminate, limits.cleanupTimeoutMs)
    try {
      worker.postMessage({ kind: 'cancel' })
    } catch {
      terminate()
    }
  }
  let cleanupTimer: ReturnType<typeof setTimeout> | undefined
  const terminate = () => {
    if (cleanupTimer) clearTimeout(cleanupTimer)
    worker.terminate()
  }
  const abort = () => close(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  worker.onmessage = (event: MessageEvent<AudioWorkerResponse>) => {
    const message = event.data,
      request = 'id' in message ? pending.get(message.id) : undefined
    if (message.kind === 'cancelled') {
      terminate()
      return
    }
    if (closed) return
    if (!request) return
    if (message.kind === 'progress')
      request.progress?.(message.completedFrames, message.totalFrames)
    else {
      pending.delete(message.id)
      clearTimeout(request.timer)
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
    if (pending.size) throw new Error('AUDIO_WORKER_QUEUE_LIMIT')
    const inputBytes = operation.channels.reduce((total, channel) => total + channel.byteLength, 0)
    const outputBytes =
      operation.kind === 'stretch'
        ? operation.frames * operation.channels.length * Float32Array.BYTES_PER_ELEMENT
        : 0
    if (
      !Number.isSafeInteger(inputBytes + outputBytes) ||
      inputBytes + outputBytes > limits.maxPcmBytes
    )
      throw new Error('AUDIO_PCM_MEMORY_LIMIT')
    return new Promise<T>((resolve, reject) => {
      const requestId = ++id
      const timer = setTimeout(
        () => close(new Error('AUDIO_WORKER_TIMEOUT')),
        limits.operationTimeoutMs,
      )
      pending.set(requestId, { resolve: (result) => resolve(result as T), reject, progress, timer })
      try {
        worker.postMessage(
          { ...operation, id: requestId, limits },
          operation.channels.map((channel) => channel.buffer),
        )
      } catch (error) {
        pending.delete(requestId)
        clearTimeout(timer)
        reject(error)
      }
    })
  }
  return {
    close,
    stretch: (channels: PcmChannels, sampleRate: number, rate: number, frames: number, gain = 1) =>
      request<PcmChannels>({ kind: 'stretch', channels, sampleRate, rate, frames, gain }),
    normalize: (channels: PcmChannels, target: number, ceiling: number, rangeCeiling: number) =>
      request<AudioNormalization>({ kind: 'normalize', channels, target, ceiling, rangeCeiling }),
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
