import {
  decodeOriginalAudioRange,
  type AudioRangeLimits,
  type AudioSourceRange,
} from './range-audio'
import type { BrowserMediaSourceAccess } from '../video/range-source'

const controller = new AbortController()
let active: Promise<unknown> | undefined
self.onmessage = (
  event: MessageEvent<
    | {
        kind: 'decode'
        id: number
        access: BrowserMediaSourceAccess
        range: AudioSourceRange
        limits: AudioRangeLimits
      }
    | { kind: 'cancel' }
  >,
) => {
  const message = event.data
  if (message.kind === 'cancel') {
    controller.abort(new DOMException('Audio range cancelled', 'AbortError'))
    void Promise.resolve(active)
      .finally(() => self.postMessage({ kind: 'cancelled' }))
      .catch(() => {})
    return
  }
  if (active) {
    self.postMessage({ kind: 'error', id: message.id, error: 'CLIP_SOURCE_AUDIO_QUEUE_LIMIT' })
    return
  }
  const operation = decodeOriginalAudioRange(
    message.access,
    message.range,
    message.limits,
    controller.signal,
  )
  active = operation
  void operation
    .then((result) => {
      controller.signal.throwIfAborted()
      self.postMessage(
        { kind: 'result', id: message.id, result },
        { transfer: result?.channels.map((channel) => channel.buffer) ?? [] },
      )
    })
    .catch((error: unknown) =>
      self.postMessage({
        kind: 'error',
        id: message.id,
        error: error instanceof Error ? error.message : String(error),
      }),
    )
    .finally(() => {
      active = undefined
    })
}
