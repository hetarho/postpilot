import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import { createAutosaveQueue, type SaveState } from '@/shared/lib'
import type { ClipProjectDraft } from '@/entities/clip-project'

/** The clip settings' autosave, one queue per project and OUTSIDE React (CLIP-39).
 *
 *  The machine itself is `shared/lib/autosave` — one request in flight, the latest draft wins, a
 *  doubling backoff, flush waiters. What is the clip's own is here: a refusal the server will
 *  repeat does NOT retry, because a clip whose generation is running is refused with `CLIP_BUSY`
 *  until that job ends and a retry loop would only spend requests saying so.
 *
 *  Module level for the same reason the post's draft queue is (tech/draft-autosave.md), and here
 *  for one more: ① is a step PANEL, so the settings form unmounts the moment the owner looks at
 *  ② or ③. A debounce living in the component would take the last keystrokes with it. */
export type SendClipDraft = (draft: ClipProjectDraft) => Promise<void>

let latest: SendClipDraft | undefined
const queue = createAutosaveQueue<ClipProjectDraft, void>({
  // The newest render's mutation closes over the live transport, so always take the newest.
  send: (draft) => latest!(draft),
  retry: retriableTransportFailure,
})

/** Queues the draft and saves it a beat after the owner stops typing. The caller decides what
 *  counts as a change — the form already compares the normalized draft against its baseline — so
 *  a re-render with untouched fields never reaches here. */
export function queueClipDraft(
  projectId: string,
  draft: ClipProjectDraft,
  send: SendClipDraft,
): void {
  latest = send
  queue.queue(projectId, draft)
}

/** Sends whatever is queued NOW and resolves once the queue is dry. Rejects with the refusal if
 *  the server will not take the draft, so a caller that must not proceed on unsaved settings —
 *  the credit approval — can stop. */
export function flushClipDraft(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast).then(() => undefined)
}

/** The draft this project still owes the server, for a settings form that is mounting again.
 *  It outranks what the server last reported: it is what the previous form was in the middle of
 *  saving when a step change unmounted it, so it is newer by exactly the keystrokes since. */
export function peekPendingClipDraft(projectId: string): ClipProjectDraft | undefined {
  return queue.peek(projectId)
}

export function clipDraftState(projectId: string): SaveState {
  const state = queue.state(projectId)
  // The clip settings have no revision to conflict over; the queue never reports one.
  return state === 'conflict' ? 'error' : state
}

/** A stable snapshot: reading the refusal never creates a queue or a new object. The queue
 *  keeps the error the server sent; normalizing it is this feature's, and the result is cached
 *  per error so a status line subscribed to the queue is not re-rendered by its own read. */
const refusals = new WeakMap<object, AppFailure>()
export function clipDraftFailure(projectId: string): AppFailure | undefined {
  const error = queue.failure(projectId)
  if (!error || typeof error !== 'object' || queue.state(projectId) !== 'refused') return undefined
  const known = refusals.get(error)
  if (known) return known
  const failure = appFailureFromConnect(error)
  refusals.set(error, failure)
  return failure
}

export function subscribeClipDraft(projectId: string, listener: () => void): () => void {
  return queue.subscribe(projectId, listener)
}

/** Drops a project's queue and everything it was going to send. Called when the project is
 *  deleted: a retry left running would keep saving an id the server no longer has. */
export function discardClipDraftQueue(projectId: string): void {
  queue.discard(projectId)
}

export function discardClipDraftQueues(): void {
  queue.discardAll()
}
