import { Code, ConnectError } from '@connectrpc/connect'
import {
  AUTOSAVE_DEBOUNCE_MS,
  AUTOSAVE_RETRY_BASE_MS,
  AUTOSAVE_RETRY_MAX_MS,
} from '@/shared/config'
import type { SaveState } from '@/shared/lib'
import type { ClipProjectDraft } from '@/entities/clip-project'

/** The clip settings' autosave queue, one per project and OUTSIDE React (CLIP-39).
 *
 *  Module level for the same reason the post's draft queue is (tech/draft-autosave.md), and here
 *  for one more: ① is a step PANEL, so the settings form unmounts the moment the owner looks at
 *  ② or ③. A debounce living in the component would take the last keystrokes with it, and a save
 *  already in flight would lose the answer it was waiting for.
 *
 *  What it guarantees: one request in flight at a time · the LATEST draft wins (a change during a
 *  flight is sent after it, never raced against it) · a transient failure retries with a doubling
 *  backoff · a refusal the server will repeat does NOT retry, because a clip whose generation is
 *  running is refused with `CLIP_BUSY` until that job ends and a retry loop would only spend
 *  requests saying so. */
export type SendClipDraft = (draft: ClipProjectDraft) => Promise<void>

interface Queue {
  /** What is not on the server yet, `undefined` once everything queued has been accepted. */
  pending: ClipProjectDraft | undefined
  send: SendClipDraft
  timer: ReturnType<typeof setTimeout> | undefined
  inFlight: boolean
  /** Set when the last attempt failed. Cleared by the next accepted save. */
  failed: boolean
  /** Doubles per consecutive transient failure, reset on success. */
  backoff: number
  /** Whether this queue has ever had a draft accepted, which is what lets the status line say
   *  저장했어요 rather than falling straight back to the project's own state. */
  saved: boolean
  /** Resolvers handed out by `flushClipDraft`, settled when the queue next runs dry. */
  waiting: { resolve: () => void; reject: (error: unknown) => void }[]
  listeners: Set<() => void>
}

const queues = new Map<string, Queue>()
/** Listeners that arrived before the first keystroke created the queue — the page's status line
 *  subscribes at mount. Adopted by `queueFor`. */
const pendingListeners = new Map<string, Set<() => void>>()

function queueFor(projectId: string, send: SendClipDraft): Queue {
  const existing = queues.get(projectId)
  if (existing) {
    // The newest render's mutation closes over the live transport, so always take the newest.
    existing.send = send
    return existing
  }
  const created: Queue = {
    pending: undefined,
    send,
    timer: undefined,
    inFlight: false,
    failed: false,
    backoff: AUTOSAVE_RETRY_BASE_MS,
    saved: false,
    waiting: [],
    listeners: new Set(pendingListeners.get(projectId)),
  }
  pendingListeners.delete(projectId)
  queues.set(projectId, created)
  return created
}

function stateOf(queue: Queue | undefined): SaveState {
  if (!queue) return 'idle'
  if (queue.failed) return 'error'
  if (queue.inFlight) return 'saving'
  if (queue.pending) return 'dirty'
  return queue.saved ? 'saved' : 'idle'
}

function publish(queue: Queue) {
  for (const listener of queue.listeners) listener()
}

/** A refusal the server will give again for the same draft. Retrying it spends requests to be
 *  told the same thing, so the queue stops and waits for the owner's next edit. */
function terminal(error: unknown): boolean {
  const code = ConnectError.from(error).code
  return (
    code !== Code.Unavailable &&
    code !== Code.DeadlineExceeded &&
    code !== Code.Unknown &&
    code !== Code.Internal &&
    code !== Code.ResourceExhausted &&
    code !== Code.Aborted
  )
}

async function run(projectId: string, queue: Queue) {
  if (queue.inFlight || !queue.pending) return
  const draft = queue.pending
  queue.inFlight = true
  queue.failed = false
  publish(queue)
  try {
    await queue.send(draft)
  } catch (error) {
    queue.inFlight = false
    queue.failed = true
    publish(queue)
    if (terminal(error)) {
      // Hand the refusal to whoever is waiting on a flush — an approval must not price a
      // generation on settings the server never took (CLIP-39).
      const waiting = queue.waiting.splice(0)
      for (const one of waiting) one.reject(error)
      return
    }
    const delay = queue.backoff
    queue.backoff = Math.min(queue.backoff * 2, AUTOSAVE_RETRY_MAX_MS)
    queue.timer = setTimeout(() => {
      queue.timer = undefined
      void run(projectId, queue)
    }, delay)
    return
  }
  queue.inFlight = false
  queue.backoff = AUTOSAVE_RETRY_BASE_MS
  queue.saved = true
  // Only the draft we just sent is settled. Anything typed during the flight is still pending and
  // goes out next, which is what makes the LATEST draft win rather than the fastest response.
  if (queue.pending === draft) queue.pending = undefined
  publish(queue)
  if (queue.pending) {
    void run(projectId, queue)
    return
  }
  const waiting = queue.waiting.splice(0)
  for (const one of waiting) one.resolve()
}

/** Queues the draft and saves it a beat after the owner stops typing. The caller decides what
 *  counts as a change — the form already compares the normalized draft against its baseline — so
 *  a re-render with untouched fields never reaches here. */
export function queueClipDraft(
  projectId: string,
  draft: ClipProjectDraft,
  send: SendClipDraft,
): void {
  const queue = queueFor(projectId, send)
  queue.pending = draft
  queue.failed = false
  if (queue.timer) clearTimeout(queue.timer)
  queue.timer = setTimeout(() => {
    queue.timer = undefined
    void run(projectId, queue)
  }, AUTOSAVE_DEBOUNCE_MS)
  publish(queue)
}

/** Sends whatever is queued NOW and resolves once the queue is dry. Rejects with the refusal if
 *  the server will not take the draft, so a caller that must not proceed on unsaved settings —
 *  the credit approval — can stop. */
export function flushClipDraft(projectId: string): Promise<void> {
  const queue = queues.get(projectId)
  if (!queue || (!queue.pending && !queue.inFlight)) return Promise.resolve()
  if (queue.timer) {
    clearTimeout(queue.timer)
    queue.timer = undefined
  }
  const settled = new Promise<void>((resolve, reject) => queue.waiting.push({ resolve, reject }))
  void run(projectId, queue)
  return settled
}

/** The draft this project still owes the server, for a settings form that is mounting again.
 *  It outranks what the server last reported: it is what the previous form was in the middle of
 *  saving when a step change unmounted it, so it is newer by exactly the keystrokes since. */
export function peekPendingClipDraft(projectId: string): ClipProjectDraft | undefined {
  return queues.get(projectId)?.pending
}

export function clipDraftState(projectId: string): SaveState {
  return stateOf(queues.get(projectId))
}

export function subscribeClipDraft(projectId: string, listener: () => void): () => void {
  // Subscribing must not CREATE a queue: the page's status line subscribes before anything has
  // been typed, and an empty queue is indistinguishable from none (both `idle`). The cleanup
  // clears both homes, because a queue may have been created — and adopted the listener — in
  // between, and a listener left on a live queue leaks a render per keystroke.
  const queue = queues.get(projectId)
  if (queue) queue.listeners.add(listener)
  else pendingListeners.set(projectId, (pendingListeners.get(projectId) ?? new Set()).add(listener))
  return () => {
    pendingListeners.get(projectId)?.delete(listener)
    queues.get(projectId)?.listeners.delete(listener)
  }
}

/** Drops a project's queue and everything it was going to send. Called when the project is
 *  deleted: a retry left running would keep saving an id the server no longer has. */
export function discardClipDraftQueue(projectId: string): void {
  const queue = queues.get(projectId)
  if (!queue) return
  if (queue.timer) clearTimeout(queue.timer)
  const waiting = queue.waiting.splice(0)
  queues.delete(projectId)
  publish(queue)
  // The subscribers outlive the queue — the status line is still mounted — so they go back to
  // waiting, and a queue created for this id again adopts them instead of stranding them.
  if (queue.listeners.size) pendingListeners.set(projectId, new Set(queue.listeners))
  for (const one of waiting) one.resolve()
}

export function discardClipDraftQueues(): void {
  for (const id of [...queues.keys()]) discardClipDraftQueue(id)
}
