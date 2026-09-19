import {
  AUTOSAVE_DEBOUNCE_MS,
  AUTOSAVE_RETRY_BASE_MS,
  AUTOSAVE_RETRY_MAX_MS,
} from '@/shared/config'
import type { SaveState } from './save-state'

/** One keyed autosave machine, shared by every feature that saves while the user types.
 *
 *  What it guarantees, and what three hand-rolled copies used to guarantee separately:
 *  ONE request in flight per key · the LATEST draft wins (a change during a flight is sent after
 *  it, never raced against it) · a transient failure retries with a doubling backoff · a refusal
 *  the server will repeat does not retry · the queue outlives the component, so unmounting the
 *  form that owns the text cannot strand a save, and a listener may subscribe before the first
 *  keystroke has created anything.
 *
 *  It is framework-free (ARCH-18): the feature supplies what to send and how two drafts fold
 *  together, and nothing else about the product reaches this file. */
export type AutosaveSend<TDraft, TResult> = (
  draft: TDraft,
  context: { key: string; result: TResult | undefined },
) => Promise<TResult>

export interface AutosaveOptions<TDraft, TResult> {
  /** How long after the last change the first attempt goes out. */
  debounceMs?: number
  retryBaseMs?: number
  retryMaxMs?: number
  /** Whether a failure is worth sending again. Default: everything retries. */
  retry?: (error: unknown) => boolean
  /** A failure that means the server has moved past this draft. The queue stops on it and says
   *  `conflict`; only a `discard` (or a fresh queue) clears it. Default: never. */
  conflict?: (error: unknown) => boolean
  /** Folds a newly queued draft into what the server is still owed — which includes a draft
   *  now in flight, since a request cannot be recalled and a retry carries whatever is pending
   *  when it fires. Default: the newest wins. */
  merge?: (pending: TDraft | undefined, next: TDraft) => TDraft
  /** Whether a draft is already what the server holds (or what a request now out is making it
   *  hold), so nothing needs sending. */
  settled?: (draft: TDraft, sending: TDraft | undefined, key: string) => boolean
  /** What to do with a draft that arrived while a request was out: send it as soon as that
   *  request lands ('now', the default) or let it wait out another debounce first. */
  follow?: 'now' | 'debounce'
  /** What a new edit does to a scheduled RETRY: 'restart' debounces from the edit (the edit is
   *  the new attempt), 'keep' leaves the backoff running so a fast typist cannot turn a failing
   *  save into a request per keystroke. */
  retryOnEdit?: 'restart' | 'keep'
  send: AutosaveSend<TDraft, TResult>
}

interface Waiter<TResult> {
  resolve: (result: TResult | undefined) => void
  reject: (error: unknown) => void
  /** Rejects on the first failure instead of waiting for the retries to settle. */
  failFast: boolean
}

interface Entry<TDraft, TResult> {
  pending: TDraft | undefined
  /** What a request now out is trying to make the server hold; it cannot be recalled. */
  sending: TDraft | undefined
  result: TResult | undefined
  inFlight: boolean
  failed: 'error' | 'refused' | 'conflict' | undefined
  error: unknown
  attempts: number
  saved: boolean
  /** A teardown asked to send while a request was out: go again the moment it lands. */
  urgent: boolean
  timer: ReturnType<typeof setTimeout> | undefined
  waiting: Waiter<TResult>[]
  listeners: Set<() => void>
}

export interface AutosaveQueue<TDraft, TResult> {
  /** Records the newest draft and (re)starts the debounce. */
  queue: (key: string, draft: TDraft) => void
  /** Sends what is queued NOW, cancelling a waiting debounce or backoff, and resolves once the
   *  key runs dry. `failFast` rejects on the first failure rather than waiting out the retries. */
  flush: (key: string, failFast?: boolean) => Promise<TResult | undefined>
  /** What the server is not known to hold yet — for a form mounting again over its own queue. */
  peek: (key: string) => TDraft | undefined
  /** The last result a send returned (a revision, a slug, …). */
  result: (key: string) => TResult | undefined
  state: (key: string) => SaveState | 'conflict'
  /** The failure the queue stopped on, for a caller that renders the server's own refusal. */
  failure: (key: string) => unknown
  /** Subscribing never CREATES a queue: a status line subscribes before anything is typed. */
  subscribe: (key: string, listener: () => void) => () => void
  /** Drops a key's queue and everything it was going to send; subscribers stay, so a queue
   *  created for this key again adopts them. Whoever was waiting on a flush RESOLVES, unless a
   *  `rejectWith` is given — a deleted post's flush must not report that the save landed. */
  discard: (key: string, rejectWith?: unknown) => void
  discardAll: (rejectWith?: unknown) => void
}

export function createAutosaveQueue<TDraft, TResult = void>(
  options: AutosaveOptions<TDraft, TResult>,
): AutosaveQueue<TDraft, TResult> {
  const debounceMs = options.debounceMs ?? AUTOSAVE_DEBOUNCE_MS
  const retryBaseMs = options.retryBaseMs ?? AUTOSAVE_RETRY_BASE_MS
  const retryMaxMs = options.retryMaxMs ?? AUTOSAVE_RETRY_MAX_MS
  const entries = new Map<string, Entry<TDraft, TResult>>()
  /** Listeners that arrived before the first change created the entry. */
  const early = new Map<string, Set<() => void>>()

  function entryFor(key: string): Entry<TDraft, TResult> {
    const existing = entries.get(key)
    if (existing) return existing
    const created: Entry<TDraft, TResult> = {
      pending: undefined,
      sending: undefined,
      result: undefined,
      inFlight: false,
      failed: undefined,
      error: undefined,
      attempts: 0,
      saved: false,
      urgent: false,
      timer: undefined,
      waiting: [],
      listeners: new Set(early.get(key)),
    }
    early.delete(key)
    entries.set(key, created)
    return created
  }

  function publish(entry: Entry<TDraft, TResult>) {
    for (const listener of entry.listeners) listener()
  }

  function stateOf(entry: Entry<TDraft, TResult> | undefined): SaveState | 'conflict' {
    if (!entry) return 'idle'
    if (entry.inFlight) return 'saving'
    if (entry.failed) return entry.failed
    if (entry.pending !== undefined) return 'dirty'
    return entry.saved ? 'saved' : 'idle'
  }

  function settle(entry: Entry<TDraft, TResult>) {
    if (entry.inFlight || entry.pending !== undefined) return
    for (const waiter of entry.waiting.splice(0)) waiter.resolve(entry.result)
  }

  function schedule(key: string, entry: Entry<TDraft, TResult>, delay: number) {
    if (entry.timer) clearTimeout(entry.timer)
    entry.timer = setTimeout(() => {
      entry.timer = undefined
      void run(key, entry)
    }, delay)
  }

  async function run(key: string, entry: Entry<TDraft, TResult>): Promise<void> {
    if (entry.inFlight || entry.pending === undefined || entry.failed === 'conflict') return
    if (entries.get(key) !== entry) return
    const draft = entry.pending
    entry.sending = draft
    entry.inFlight = true
    entry.failed = undefined
    entry.error = undefined
    publish(entry)
    try {
      const result = await options.send(draft, { key, result: entry.result })
      if (entries.get(key) !== entry) return
      entry.result = result
      entry.inFlight = false
      entry.sending = undefined
      entry.attempts = 0
      entry.saved = true
      // Only the draft that was sent is settled; anything typed during the flight goes out next,
      // which is what makes the LATEST draft win rather than the fastest response.
      if (entry.pending === draft) entry.pending = undefined
      publish(entry)
      if (entry.pending !== undefined) {
        const now = entry.urgent || (options.follow ?? 'now') === 'now'
        entry.urgent = false
        if (now) void run(key, entry)
        else schedule(key, entry, debounceMs)
        return
      }
      entry.urgent = false
      settle(entry)
    } catch (error) {
      if (entries.get(key) !== entry) return
      entry.inFlight = false
      entry.sending = undefined
      entry.urgent = false
      entry.error = error
      const conflicted = options.conflict?.(error) ?? false
      const retriable = !conflicted && (options.retry?.(error) ?? true)
      entry.failed = conflicted ? 'conflict' : retriable ? 'error' : 'refused'
      publish(entry)
      const failFast = entry.waiting.filter((one) => one.failFast)
      entry.waiting = entry.waiting.filter((one) => !one.failFast)
      for (const one of failFast) one.reject(error)
      if (!retriable) {
        // Nothing more will happen on its own: whoever waited is told why, rather than hanging
        // on a queue that has stopped.
        for (const one of entry.waiting.splice(0)) one.reject(error)
        return
      }
      entry.attempts += 1
      schedule(key, entry, Math.min(retryBaseMs * 2 ** (entry.attempts - 1), retryMaxMs))
    }
  }

  return {
    queue(key, draft) {
      const entry = entryFor(key)
      if (entry.failed === 'conflict') return
      const next = options.merge ? options.merge(entry.pending, draft) : draft
      if (options.settled?.(next, entry.sending, key)) {
        entry.pending = undefined
        publish(entry)
        settle(entry)
        return
      }
      const retrying = entry.failed === 'error' && entry.timer !== undefined
      entry.pending = next
      if (!(retrying && (options.retryOnEdit ?? 'restart') === 'keep')) {
        entry.failed = undefined
        entry.error = undefined
        if (!entry.inFlight) schedule(key, entry, debounceMs)
      }
      publish(entry)
    },
    flush(key, failFast = false) {
      const entry = entries.get(key)
      if (!entry) return Promise.resolve(undefined)
      if (entry.failed === 'conflict') return Promise.reject(entry.error)
      if (!entry.inFlight && entry.pending === undefined) return Promise.resolve(entry.result)
      const settled = new Promise<TResult | undefined>((resolve, reject) =>
        entry.waiting.push({ resolve, reject, failFast }),
      )
      if (entry.timer) {
        clearTimeout(entry.timer)
        entry.timer = undefined
      }
      if (entry.inFlight) entry.urgent = true
      else void run(key, entry)
      return settled
    },
    peek: (key) => entries.get(key)?.pending,
    result: (key) => entries.get(key)?.result,
    state: (key) => stateOf(entries.get(key)),
    failure: (key) => entries.get(key)?.error,
    subscribe(key, listener) {
      const entry = entries.get(key)
      if (entry) entry.listeners.add(listener)
      else early.set(key, (early.get(key) ?? new Set()).add(listener))
      return () => {
        early.get(key)?.delete(listener)
        entries.get(key)?.listeners.delete(listener)
      }
    },
    discard(key, rejectWith) {
      const entry = entries.get(key)
      if (!entry) return
      if (entry.timer) clearTimeout(entry.timer)
      entries.delete(key)
      publish(entry)
      // Subscribers outlive the queue — the status line is still mounted — so they go back to
      // waiting, and a queue created for this key again adopts them instead of stranding them.
      if (entry.listeners.size) early.set(key, new Set(entry.listeners))
      for (const one of entry.waiting.splice(0))
        if (rejectWith === undefined) one.resolve(entry.result)
        else one.reject(rejectWith)
    },
    discardAll(rejectWith) {
      for (const key of [...entries.keys()]) this.discard(key, rejectWith)
    },
  }
}
