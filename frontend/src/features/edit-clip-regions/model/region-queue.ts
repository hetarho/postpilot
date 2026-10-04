import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import { createAutosaveQueue, type SaveState } from '@/shared/lib'
import { CLIP_TIMELINE } from '@/entities/clip-design'
import type { ClipRegionsEdits } from './region-draft'

/** The region slots' autosave, one queue per project and outside React (CLIP-188), on the
 *  shared machine: one request in flight, the latest edit wins, a transient failure retries.
 *
 *  The queued edits only say that something is owed. What a send carries is read when it goes
 *  out, from the editor's own edits at that moment: a slot saved a moment ago and since changed
 *  in correction is no longer the owner's to send (CLIP-188). */
export type SendClipRegions = () => Promise<void>

export const regionConflict = (error: unknown) =>
  appFailureFromConnect(error).reason === 'CLIP_PLAN_CONFLICT'

/** One sender per project, replaced on every queue call so it reads the newest editor's edits.
 *  Per project, because a project's owed slots must never go out through, and be marked saved
 *  by, the editor of another. */
const senders = new Map<string, SendClipRegions>()
const queue = createAutosaveQueue<ClipRegionsEdits, void>({
  // The same beat as the correction's, so a slot and a correction row typed in turn save alike.
  debounceMs: CLIP_TIMELINE.autosaveMs,
  send: (_, { key }) => senders.get(key)!(),
  // A stale revision is answered with CLIP_PLAN_CONFLICT on `Aborted`, which a transport retry
  // would send forever: the send takes the winning revision once itself, and past that the
  // conflict waits for the owner's next edit.
  retry: (error) => retriableTransportFailure(error) && !regionConflict(error),
})

export function queueClipRegions(
  projectId: string,
  edits: ClipRegionsEdits,
  send: SendClipRegions,
) {
  senders.set(projectId, send)
  queue.queue(projectId, edits)
}

/** Sends what is owed NOW and resolves once the queue is dry, or rejects with the failure. */
export function flushClipRegions(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast).then(() => undefined)
}

/** What a previous mount of the editor still owed the server when it went away. */
export function peekClipRegions(projectId: string): ClipRegionsEdits | undefined {
  return queue.peek(projectId)
}

export function clipRegionsState(projectId: string): SaveState {
  const state = queue.state(projectId)
  return state === 'conflict' ? 'refused' : state
}

const refusals = new WeakMap<object, AppFailure>()
export function clipRegionsFailure(projectId: string): AppFailure | undefined {
  const error = queue.failure(projectId)
  const state = queue.state(projectId)
  if (!error || typeof error !== 'object' || (state !== 'refused' && state !== 'error'))
    return undefined
  const known = refusals.get(error)
  if (known) return known
  const failure = appFailureFromConnect(error)
  refusals.set(error, failure)
  return failure
}

export function subscribeClipRegions(projectId: string, listener: () => void): () => void {
  return queue.subscribe(projectId, listener)
}

export function discardClipRegionQueue(projectId: string): void {
  queue.discard(projectId)
  senders.delete(projectId)
}

export function discardClipRegionQueues(): void {
  queue.discardAll()
  senders.clear()
}
