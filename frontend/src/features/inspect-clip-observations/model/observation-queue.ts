import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import { createAutosaveQueue } from '@/shared/lib'
import { serialClipWrite, type ClipObservationEdit } from '@/entities/clip-project'

/** The owner's corrections to observed scenes, keyed by observation id `<sourceId>/<index>`. */
export type ClipObservationEdits = Readonly<Record<string, ClipObservationEdit>>
export type SendClipObservationEdits = (edits: readonly ClipObservationEdit[]) => Promise<void>

/** ②'s observation corrections autosave on a pause like the other clip drafts (CLIP-39,
 *  CLIP-235), one queue per project and outside React so a step change cannot drop the last
 *  words. Every queued value carries every correction still owed, so the latest one wins and a
 *  resend is idempotent; each send runs in the project's write lane. */
const senders = new Map<string, SendClipObservationEdits>()
const queue = createAutosaveQueue<ClipObservationEdits, void>({
  send: (edits, { key }) => serialClipWrite(key, () => senders.get(key)!(Object.values(edits))),
  retry: retriableTransportFailure,
})

/** Adds one field's correction to what the project owes and queues the whole set. */
export function queueClipObservationEdit(
  projectId: string,
  edit: ClipObservationEdit,
  send: SendClipObservationEdits,
): void {
  senders.set(projectId, send)
  const owed = queue.peek(projectId) ?? {}
  queue.queue(projectId, {
    ...owed,
    [edit.observationId]: { ...owed[edit.observationId], ...edit },
  })
}

/** Sends what is owed NOW and resolves once the queue is dry, or rejects with the failure — so
 *  no committing action runs on observations the server has not taken (CLIP-39). */
export function flushClipObservationEdits(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast).then(() => undefined)
}

/** The corrections this project still owes the server: queued, in flight or failed. */
export function peekClipObservationEdits(projectId: string): ClipObservationEdits | undefined {
  return queue.peek(projectId)
}

export function clipObservationSaveState(projectId: string) {
  const state = queue.state(projectId)
  return state === 'conflict' ? 'refused' : state
}

const failures = new WeakMap<object, AppFailure>()
export function clipObservationEditFailure(projectId: string): AppFailure | undefined {
  const error = queue.failure(projectId)
  const state = queue.state(projectId)
  if (!error || typeof error !== 'object' || (state !== 'refused' && state !== 'error'))
    return undefined
  const known = failures.get(error)
  if (known) return known
  const failure = appFailureFromConnect(error)
  failures.set(error, failure)
  return failure
}

export function subscribeClipObservationEdits(projectId: string, listener: () => void) {
  return queue.subscribe(projectId, listener)
}

export function discardClipObservationQueue(projectId: string): void {
  queue.discard(projectId)
  senders.delete(projectId)
}

export function discardClipObservationQueues(): void {
  queue.discardAll()
  senders.clear()
}
