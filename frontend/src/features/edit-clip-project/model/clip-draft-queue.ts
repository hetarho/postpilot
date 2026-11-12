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
 *  Module level for the same reason the post's draft queue is (POST-6), and here
 *  for one more: ① is a step PANEL, so the settings form unmounts the moment the owner looks at
 *  ② or ③. A debounce living in the component would take the last keystrokes with it. */
export type SendClipDraft = (draft: ClipProjectDraft) => Promise<void>

/** One sender per project, replaced on every queue call so it closes over the newest render's
 *  mutation and transport. Per project, because a save still debouncing or backing off for one
 *  clip must never go out through a form that has since been editing another. */
const senders = new Map<string, SendClipDraft>()
const queue = createAutosaveQueue<ClipProjectDraft, void>({
  send: (draft, { key }) => senders.get(key)!(draft),
  retry: retriableTransportFailure,
})

/** An unminted `/clips/new` queues under its own key `new:<n>`, as a post with no slug does
 *  (POST-7): the first send creates the project and every later send updates it, so edits made
 *  while the create is in flight go out as the next update and never mint a second project. The
 *  minted id is remembered per key, and the id remembers the key, so the form the address change
 *  mounts reads and flushes what the unminted one still owes (CLIP-130). */
let unminted = 0
const mintedIds = new Map<string, string>()
const mintedKeys = new Map<string, string>()

export function newClipDraftKey(): string {
  unminted += 1
  return `new:${unminted}`
}

export function mintedClipId(key: string): string | undefined {
  return mintedIds.get(key)
}

export function recordMintedClip(key: string, id: string): void {
  mintedIds.set(key, id)
  mintedKeys.set(id, key)
}

/** Sends what is queued for an unminted clip NOW — the given draft when nothing is, even an empty
 *  one — and resolves with the minted id, so a picked source can attach before anything is typed
 *  (POST-7). Rejects when the create is refused or fails; nothing navigates then. */
export async function mintClipDraft(
  key: string,
  draft: ClipProjectDraft,
  send: SendClipDraft,
): Promise<string> {
  const known = mintedIds.get(key)
  if (known) return known
  if (queue.peek(key) === undefined) queueClipDraft(key, draft, send)
  else senders.set(key, send)
  await queue.flush(key, true)
  const id = mintedIds.get(key)
  if (!id) throw new Error('The clip was not created')
  return id
}

/** Queues the draft and saves it a beat after the owner stops typing. The caller decides what
 *  counts as a change — the form already compares the normalized draft against its baseline — so
 *  a re-render with untouched fields never reaches here. */
export function queueClipDraft(
  projectId: string,
  draft: ClipProjectDraft,
  send: SendClipDraft,
): void {
  senders.set(projectId, send)
  queue.queue(projectId, draft)
}

/** Sends whatever is queued NOW and resolves once the queue is dry. Rejects with the refusal if
 *  the server will not take the draft, so a caller that must not proceed on unsaved settings —
 *  the credit approval — can stop. */
export function flushClipDraft(projectId: string, failFast = false): Promise<void> {
  const unmintedKey = mintedKeys.get(projectId)
  return Promise.all([
    unmintedKey ? queue.flush(unmintedKey, failFast) : undefined,
    queue.flush(projectId, failFast),
  ]).then(() => undefined)
}

/** The draft this project still owes the server, for a settings form that is mounting again.
 *  It outranks what the server last reported: it is what the previous form was in the middle of
 *  saving when a step change unmounted it, so it is newer by exactly the keystrokes since. */
export function peekPendingClipDraft(projectId: string): ClipProjectDraft | undefined {
  const unmintedKey = mintedKeys.get(projectId)
  return queue.peek(projectId) ?? (unmintedKey ? queue.peek(unmintedKey) : undefined)
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
  const unmintedKey = mintedKeys.get(projectId)
  if (unmintedKey) {
    queue.discard(unmintedKey)
    senders.delete(unmintedKey)
    mintedIds.delete(unmintedKey)
    mintedKeys.delete(projectId)
  }
  queue.discard(projectId)
  senders.delete(projectId)
}

export function discardClipDraftQueues(): void {
  queue.discardAll()
  senders.clear()
  mintedIds.clear()
  mintedKeys.clear()
}
