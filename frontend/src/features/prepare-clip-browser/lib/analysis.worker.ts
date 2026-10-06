import type { EncodedAudioTrack } from '@/shared/lib'
import { encodeAnalysisCopy, measureAnalysisOriginal } from './analysis-media'
import type { AnalysisCopySlot, AnalysisSource } from '../model/types'

const controller = new AbortController()
let busy = false
self.onmessage = async (
  event: MessageEvent<
    | { kind: 'cancel' }
    | { kind: 'measure'; id: number; source: AnalysisSource }
    | {
        kind: 'encode'
        id: number
        source: AnalysisSource
        slot: AnalysisCopySlot
        audio?: EncodedAudioTrack
      }
  >,
) => {
  const message = event.data
  if (message.kind === 'cancel') {
    controller.abort()
    if (!busy) {
      self.postMessage({ kind: 'cancelled' })
      self.close()
    }
    return
  }
  if (busy) {
    self.postMessage({ kind: 'error', id: message.id, error: 'CLIP_ANALYSIS_QUEUE_LIMIT' })
    return
  }
  busy = true
  try {
    const result =
      message.kind === 'measure'
        ? await measureAnalysisOriginal(message.source, controller.signal)
        : await encodeAnalysisCopy(
            message.source,
            message.slot,
            controller.signal,
            (fraction) => self.postMessage({ kind: 'progress', id: message.id, fraction }),
            message.audio,
          )
    controller.signal.throwIfAborted()
    self.postMessage(
      { kind: 'result', id: message.id, result },
      'buffer' in result ? { transfer: [result.buffer] } : undefined,
    )
  } catch (error) {
    self.postMessage({
      kind: 'error',
      id: message.id,
      error: error instanceof Error ? error.message : 'CLIP_ANALYSIS_WORKER_FAILED',
    })
  } finally {
    busy = false
    if (controller.signal.aborted) {
      self.postMessage({ kind: 'cancelled' })
      self.close()
    }
  }
}
