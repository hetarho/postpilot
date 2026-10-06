import type { ClipAnalysisOriginalMeasurement } from '@/entities/clip-project'
import {
  canonicalSelectedAudio,
  createAudioRangeReader,
  createAudioProcessor,
  type EncodedAudioTrack,
} from '@/shared/lib'
import {
  ANALYSIS_AUDIO_ENCODER_LIMITS,
  ANALYSIS_AUDIO_LIMITS,
  ANALYSIS_PREPARATION_LIMITS as limits,
} from '../config/limits'
import { analysisMono } from '../model/mono'
import type {
  AnalysisCopyArtifact,
  AnalysisCopySlot,
  AnalysisEncoder,
  AnalysisSource,
} from '../model/types'

export function createAnalysisEncoder(
  signal: AbortSignal,
  progress: (fraction: number) => void = () => {},
): AnalysisEncoder {
  signal.throwIfAborted()
  if (typeof Worker === 'undefined') throw new Error('CLIP_ANALYSIS_ENCODER_UNSUPPORTED')
  const worker = new Worker(new URL('./analysis.worker.ts', import.meta.url), { type: 'module' })
  let preparingAudio = false
  let id = 0,
    closed = false
  let pending:
    | {
        id: number
        resolve: (value: unknown) => void
        reject: (error: unknown) => void
        timer: ReturnType<typeof setTimeout>
      }
    | undefined
  let audioReader: ReturnType<typeof createAudioRangeReader> | undefined
  let audioProcessor: ReturnType<typeof createAudioProcessor> | undefined
  let cleanup: ReturnType<typeof setTimeout> | undefined
  const terminate = () => {
    if (cleanup) clearTimeout(cleanup)
    worker.terminate()
  }
  function close(reason: unknown = new DOMException('Preparation cancelled', 'AbortError')) {
    if (closed) return
    closed = true
    signal.removeEventListener('abort', abort)
    audioReader?.close()
    audioProcessor?.close()
    if (pending) {
      clearTimeout(pending.timer)
      pending.reject(reason)
      pending = undefined
    }
    cleanup = setTimeout(terminate, limits.cleanupTimeoutMs)
    try {
      worker.postMessage({ kind: 'cancel' })
    } catch {
      terminate()
    }
  }
  const abort = () => close(signal.reason)
  signal.addEventListener('abort', abort, { once: true })
  worker.onmessage = (event: MessageEvent) => {
    const message = event.data
    if (message.kind === 'cancelled') {
      terminate()
      return
    }
    if (closed || !pending || pending.id !== message.id) return
    if (message.kind === 'progress') {
      progress(message.fraction)
      return
    }
    const request = pending
    clearTimeout(request.timer)
    pending = undefined
    if (message.kind === 'error') request.reject(new Error(message.error))
    else request.resolve(message.result)
  }
  worker.onerror = (event) => {
    event.preventDefault()
    close(new Error(event.message || 'CLIP_ANALYSIS_WORKER_FAILED'))
  }
  worker.onmessageerror = () => close(new Error('CLIP_ANALYSIS_WORKER_FAILED'))
  async function request(
    kind: 'measure' | 'encode',
    source: AnalysisSource,
    slot?: AnalysisCopySlot,
    audio?: EncodedAudioTrack,
  ) {
    signal.throwIfAborted()
    if (closed || pending || (kind === 'measure' && preparingAudio))
      throw new Error('CLIP_ANALYSIS_QUEUE_LIMIT')
    return new Promise<unknown>((resolve, reject) => {
      const requestId = ++id
      const timer = setTimeout(
        () => close(new Error('CLIP_ANALYSIS_TIMEOUT')),
        limits.operationTimeoutMs,
      )
      pending = { id: requestId, resolve, reject, timer }
      try {
        worker.postMessage(
          { kind, id: requestId, source, slot, audio },
          audio ? { transfer: audio.chunks.map((chunk) => chunk.data.buffer) } : undefined,
        )
      } catch (error) {
        close(error)
      }
    })
  }
  return {
    measure: (source) => request('measure', source) as Promise<ClipAnalysisOriginalMeasurement>,
    encode: async (source, slot) => {
      signal.throwIfAborted()
      if (closed || pending || preparingAudio) throw new Error('CLIP_ANALYSIS_QUEUE_LIMIT')
      preparingAudio = true
      try {
        let audio: EncodedAudioTrack | undefined
        if (slot.hasAudio) {
          audioReader ??= createAudioRangeReader(signal, ANALYSIS_AUDIO_LIMITS)
          const range = await audioReader.decode(source.access, {
            startUs: slot.offsetMs * 1000,
            endUs: (slot.offsetMs + slot.durationMs) * 1000,
            targetSampleRate: limits.audioRate,
          })
          if (!range) throw new Error('CLIP_SOURCE_AUDIO_MISSING')
          const frames = (slot.durationMs * limits.audioRate) / 1000
          const pcm = await canonicalSelectedAudio(
            range,
            (slot.offsetMs * limits.audioRate) / 1000,
            frames,
            limits.audioRate,
            signal,
          )
          const mono = analysisMono(pcm)
          pcm.length = 0
          signal.throwIfAborted()
          audioProcessor ??= createAudioProcessor(signal, ANALYSIS_AUDIO_ENCODER_LIMITS)
          audio = await audioProcessor.encode(
            [mono],
            {
              codec: 'mp4a.40.2',
              sampleRate: limits.audioRate,
              numberOfChannels: 1,
              bitrate: limits.audioBitrate,
            },
            4096,
            4,
          )
          signal.throwIfAborted()
        }
        return (await request('encode', source, slot, audio)) as AnalysisCopyArtifact
      } finally {
        preparingAudio = false
      }
    },
    close,
  }
}
