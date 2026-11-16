import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import type { SaveState } from '@/shared/lib'
import { CLIP_TIMELINE } from '@/entities/clip-design'
import { createProjectSaveQueue } from '@/entities/clip-project'
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

/** One sender per project (inside the queue), so a project's owed slots never go out through,
 *  and are never marked saved by, the editor of another. */
const queue = createProjectSaveQueue<ClipRegionsEdits, SendClipRegions>({
  conflictAs: 'refused',
  // The same beat as the correction's, so a slot and a correction row typed in turn save alike.
  debounceMs: CLIP_TIMELINE.autosaveMs,
  run: (send) => send(),
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
  queue.queue(projectId, edits, send)
}

/** Sends what is owed NOW and resolves once the queue is dry, or rejects with the failure. */
export function flushClipRegions(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast)
}

/** What a previous mount of the editor still owed the server when it went away. */
export function peekClipRegions(projectId: string): ClipRegionsEdits | undefined {
  return queue.peek(projectId)
}

export function clipRegionsState(projectId: string): SaveState {
  return queue.state(projectId)
}

export function clipRegionsFailure(projectId: string): AppFailure | undefined {
  return queue.failure(projectId)
}

export function subscribeClipRegions(projectId: string, listener: () => void): () => void {
  return queue.subscribe(projectId, listener)
}

export function discardClipRegionQueue(projectId: string): void {
  queue.discard(projectId)
}

export function discardClipRegionQueues(): void {
  queue.discardAll()
}
