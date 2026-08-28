// The draft save pipeline, per post, outside React.
//
// It lives here rather than inside the hook because the text belongs to the user, not to
// the component that happens to be mounted: leaving the editor with a save still failing
// must not end the retries, and the editor the mint navigation mounts must not start a
// second, competing chain of saves for the same post. One queue per post gives both, plus
// the invariant everything else rests on — for a given post there is at most one save in
// flight, and what it sends is always the newest text queued.
import {
  AUTOSAVE_DEBOUNCE_MS,
  AUTOSAVE_RETRY_BASE_MS,
  AUTOSAVE_RETRY_MAX_MS,
} from '@/shared/config'

/** What the editor tells the user about the last save.
 *
 *  `error` is not terminal: a retry is already scheduled. It exists so nobody is told
 *  "저장됨" while the server holds something older. */
export type SaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

export interface Draft {
  title: string
  memo: string
}

/** Performs one save and resolves with the post's slug.
 *
 *  Injected so this module knows nothing of React or of the transport. It must REJECT
 *  when the server did not confirm the post: a resolve is taken as proof the text landed,
 *  and for a draft with no slug yet, as proof the post now exists. */
export type SendDraft = (slug: string, draft: Draft) => Promise<string>

export interface DraftQueueHandle {
  state: () => SaveState
  /** Records the newest text and (re)starts the debounce. Called on every keystroke. */
  queue: (draft: Draft) => void
  /** Sends what is queued right now, cancelling a waiting debounce or backoff — for a
   *  page or an editor that is going away. */
  saveNow: () => void
  /** Detaches the editor. A queue with work left keeps going without it. */
  release: () => void
}

interface Queue {
  key: string
  /** Empty until the first save mints one. */
  slug: string
  /** What the server is known to hold. */
  saved: Draft
  /** The text a request currently out is trying to make the server hold. A request cannot
   *  be recalled, so this — not `saved` — is what the editor's text has to be compared
   *  against while one is in flight. */
  sending: Draft | undefined
  /** The newest text the server is not known to hold. */
  pending: Draft | undefined
  /** True once a save has succeeded, so an untouched editor stays silent. */
  everSaved: boolean
  failed: boolean
  /** Set when the session this queue belongs to ended. A request already out still lands,
   *  but nothing this queue does afterwards may touch the registry. */
  discarded: boolean
  inFlight: boolean
  /** Set when a teardown asked to save while a request was already out: the leftover text
   *  then goes out the moment that request lands, instead of waiting for a debounce
   *  nobody will be around for. */
  urgent: boolean
  attempts: number
  debounceTimer: number | undefined
  retryTimer: number | undefined
  send: SendDraft
  listener: ((state: SaveState) => void) | undefined
  onMinted: ((slug: string) => void) | undefined
}

const queues = new Map<string, Queue>()

let newDraftSequence = 0

/** A draft with no slug yet gets a key of its own rather than a shared one: two "새 글"
 *  editors in the same tab are two different drafts, and one must not be able to clear or
 *  claim the other's unfinished save. Every real slug is `YYYYMMDD-…`
 *  (spec/policy/posts.md), so this prefix cannot collide with one. */
function newDraftKey(): string {
  newDraftSequence += 1
  return `new:${newDraftSequence}`
}

/** Exponential backoff, capped. The cap keeps a tab left open on a dead network from
 *  becoming a request loop while still recovering once the network returns. */
function retryDelay(attempt: number): number {
  return Math.min(AUTOSAVE_RETRY_BASE_MS * 2 ** (attempt - 1), AUTOSAVE_RETRY_MAX_MS)
}

function sameDraft(a: Draft, b: Draft): boolean {
  return a.title === b.title && a.memo === b.memo
}

function stateOf(queue: Queue): SaveState {
  if (queue.inFlight) return 'saving'
  if (queue.failed) return 'error'
  if (queue.pending) return 'dirty'
  return queue.everSaved ? 'saved' : 'idle'
}

function publish(queue: Queue): void {
  queue.listener?.(stateOf(queue))
}

function clearTimers(queue: Queue): void {
  window.clearTimeout(queue.debounceTimer)
  window.clearTimeout(queue.retryTimer)
  queue.debounceTimer = undefined
  queue.retryTimer = undefined
}

function scheduleDebounce(queue: Queue): void {
  window.clearTimeout(queue.debounceTimer)
  queue.debounceTimer = window.setTimeout(() => {
    queue.debounceTimer = undefined
    void run(queue)
  }, AUTOSAVE_DEBOUNCE_MS)
}

/** Drops a queue that has nothing left to do, so a long session does not accumulate one
 *  object per post it visited. */
function collect(queue: Queue): void {
  const busy =
    queue.inFlight ||
    queue.pending !== undefined ||
    queue.debounceTimer !== undefined ||
    queue.retryTimer !== undefined
  if (!busy && !queue.listener) queues.delete(queue.key)
}

function rekey(queue: Queue, slug: string): void {
  queues.delete(queue.key)
  queue.key = slug
  queue.slug = slug
  queues.set(slug, queue)
}

async function run(queue: Queue): Promise<void> {
  if (queue.inFlight || !queue.pending) return

  const sent = queue.pending
  queue.inFlight = true
  queue.sending = sent
  publish(queue)

  try {
    const slug = await queue.send(queue.slug, sent)
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined
    queue.attempts = 0
    queue.failed = false
    queue.saved = sent
    queue.everSaved = true
    // Only the text that actually went out is settled; anything typed during the round
    // trip is still pending.
    if (queue.pending && sameDraft(queue.pending, sent)) queue.pending = undefined
    if (!queue.slug && slug) {
      rekey(queue, slug)
      queue.onMinted?.(slug)
    }
    publish(queue)

    if (queue.pending) {
      // Immediately only for a teardown that could not wait: doing it unconditionally
      // would turn continuous typing into one save per round trip.
      if (queue.urgent) void run(queue)
      else scheduleDebounce(queue)
    }
    queue.urgent = false
  } catch {
    // Swallowed rather than rethrown: every caller is a timer or a teardown handler with
    // nobody to catch it. The retry is what the user is actually promised.
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined

    if (queue.pending && sameDraft(queue.pending, queue.saved)) {
      // Typed back to what the server holds while this attempt was out — there is nothing
      // left to retry, and "다시 시도 중" would stand there forever.
      queue.pending = undefined
      queue.failed = false
      queue.attempts = 0
      publish(queue)
      collect(queue)
      return
    }

    queue.attempts += 1
    queue.failed = true
    queue.urgent = false
    publish(queue)
    // The retry sends whatever is pending WHEN IT FIRES, so typing during an outage
    // neither resets the delay nor adds requests of its own.
    queue.retryTimer = window.setTimeout(() => {
      queue.retryTimer = undefined
      void run(queue)
    }, retryDelay(queue.attempts))
  }
  collect(queue)
}

export function attachDraftQueue(options: {
  /** The post being written to, or undefined for a draft with no slug yet. */
  slug: string | undefined
  /** What the server holds, as this editor was told. Used only when there is no queue
   *  yet: an existing one has watched every save and knows better. */
  saved: Draft
  send: SendDraft
  onState: (state: SaveState) => void
  onMinted: (slug: string) => void
}): DraftQueueHandle {
  const key = options.slug ?? newDraftKey()
  let queue = queues.get(key)

  if (!queue) {
    queue = {
      key,
      slug: options.slug ?? '',
      saved: options.saved,
      sending: undefined,
      pending: undefined,
      everSaved: false,
      failed: false,
      discarded: false,
      inFlight: false,
      urgent: false,
      attempts: 0,
      debounceTimer: undefined,
      retryTimer: undefined,
      send: options.send,
      listener: undefined,
      onMinted: undefined,
    }
    queues.set(key, queue)
  }

  // A queue that outlived its editor is adopted, not replaced. Taking the new editor's
  // callbacks also takes its live transport: the previous editor's is bound to a mutation
  // observer that is gone.
  const attached = queue
  attached.send = options.send
  attached.listener = options.onState
  attached.onMinted = options.onMinted

  return {
    state: () => stateOf(attached),

    queue: (draft) => {
      // Against the request in flight when there is one: a save already sent cannot be
      // recalled, so comparing with the older `saved` would call an undo "already saved"
      // and never send it.
      if (sameDraft(draft, attached.sending ?? attached.saved)) {
        // Typed back to what the server holds. Leaving "저장 대기 중" or "다시 시도 중" on
        // screen with nothing to send would be a standing lie.
        if (attached.pending === undefined && !attached.failed) return
        attached.pending = undefined
        attached.failed = false
        attached.attempts = 0
        clearTimers(attached)
        publish(attached)
        return
      }

      attached.pending = { ...draft }
      publish(attached)
      // Not during a backoff: that timer already covers sending the newest text, and
      // restarting the debounce on every keystroke would defeat the backoff entirely.
      if (attached.retryTimer === undefined) scheduleDebounce(attached)
    },

    saveNow: () => {
      clearTimers(attached)
      if (attached.inFlight) attached.urgent = true
      else void run(attached)
    },

    release: () => {
      attached.listener = undefined
      attached.onMinted = undefined
      collect(attached)
    },
  }
}

/** The newest text a queue for `slug` is still trying to save, if any.
 *
 *  The editor mounted after a mint (or after coming back to a post whose save is still
 *  failing) seeds itself from this, so characters typed during a save round trip are not
 *  replaced by the older text the response carries. Read-only and idempotent, so it is
 *  safe to call from a component body. */
export function peekPendingDraft(slug: string): Draft | undefined {
  return queues.get(slug)?.pending
}

/** Drops every queue, cancelling whatever they were still going to send.
 *
 *  A queue outlives its editor on purpose, but it must never outlive its session: a retry
 *  that fires after someone else has signed in on the same device would send the previous
 *  account's draft under the new account's cookie, and the server would file it there.
 *  Called by the app layer on logout and on a session that died mid-use. */
export function discardDraftQueues(): void {
  for (const queue of queues.values()) {
    queue.discarded = true
    clearTimers(queue)
  }
  queues.clear()
}
