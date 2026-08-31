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
import type { ContentLanguage } from '@/shared/api'

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
 *  and for a draft with no slug yet, as proof the post now exists.
 *
 *  `voiceId` is the assignment to send WITH this save, or undefined to leave the post's
 *  voice alone. It is present on every create — a post cannot exist without a voice
 *  (spec/policy/posts.md) — and on an existing post only while the assignment differs from
 *  what the server holds, so an ordinary title save can never carry a stale voice over a
 *  newer one.
 *
 *  `purposeId` is the same mechanism with one more state, because a post may have none:
 *  undefined leaves the assignment alone, '' clears it, and an id assigns it. On a create it
 *  is sent only when one was chosen — 없음 is the default and the server never picks. */
export type SendDraft = (
  slug: string,
  draft: Draft,
  voiceId: string | undefined,
  purposeId: string | undefined,
  targetLanguage: ContentLanguage | undefined,
) => Promise<string>

export interface DraftQueueHandle {
  state: () => SaveState
  /** Records the newest text and (re)starts the debounce. Called on every keystroke. */
  queue: (draft: Draft) => void
  /** Sends what is queued right now, cancelling a waiting debounce or backoff — for a
   *  page or an editor that is going away. */
  saveNow: () => void
  /** Saves everything currently queued and resolves only when the server is known to
   *  hold it. Used before an action that consumes the saved draft, such as generation. */
  flush: () => Promise<void>
  /** Detaches the editor. A queue with work left keeps going without it. */
  release: () => void
  /** Resolves with the post's slug, creating the post first when it has none yet — for
   *  the first photo picked in a new draft, which needs a post to attach to before any
   *  autosave has fired. Resolves once the create lands, however many retries that
   *  takes; rejects only if the session ends first. */
  mint: () => Promise<string>
  /** Records the 용도 this draft is written for, '' for 없음. Same shape as `assignVoice`:
   *  before the post exists the choice rides along with the create, and afterwards it is sent
   *  at once so a delayed title save cannot revert a newer selection. */
  assignPurpose: (purposeId: string) => Promise<void>
  /** Records the voice this draft is written in. For a draft with no post yet that is all
   *  it does — the create carries it. For an existing post it is a reassignment: sent at
   *  once, and the promise reports that one save's outcome. A refused reassignment is taken
   *  back, so the retries that follow carry text only instead of failing forever on the
   *  same answer. */
  assignVoice: (voiceId: string) => Promise<void>
  /** Records the language requested for the next full write. It is required on create and,
   *  for an existing post, settles through this same serial queue as title and memo. */
  assignTargetLanguage: (language: ContentLanguage) => Promise<void>
}

interface MintWaiter {
  resolve: (slug: string) => void
  reject: (error: Error) => void
}

interface FlushWaiter {
  resolve: () => void
  reject: (error: Error) => void
}

interface VoiceWaiter extends FlushWaiter {
  voiceId: string
}

interface PurposeWaiter extends FlushWaiter {
  purposeId: string
}

interface TargetLanguageWaiter extends FlushWaiter {
  targetLanguage: ContentLanguage
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
  /** The voice the editor wants the post written in. */
  voiceId: string
  /** The voice the server is known to hold; empty until the post exists. */
  savedVoiceId: string
  /** The 용도 the editor wants, '' for 없음. */
  purposeId: string
  /** The 용도 the server is known to hold. Empty means 없음 — which is also the value a post
   *  starts at, so unlike the voice there is no "not known yet" state to distinguish. */
  savedPurposeId: string
  /** The target the editor wants and the target the server is known to hold. */
  targetLanguage: ContentLanguage
  savedTargetLanguage: ContentLanguage | undefined
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
  /** Callers of `mint` waiting for the first save to land. On the queue, not the handle,
   *  so they survive the editor swap the mint itself causes. */
  mintWaiters: MintWaiter[]
  flushWaiters: FlushWaiter[]
  /** Callers of `assignVoice` waiting for their reassignment to land. */
  voiceWaiters: VoiceWaiter[]
  purposeWaiters: PurposeWaiter[]
  targetLanguageWaiters: TargetLanguageWaiter[]
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

/** True while someone is waiting for this draft to become a post. Then "typed back to
 *  what the server holds" is not a reason to stand down — the server holds nothing yet,
 *  and the empty draft itself has to be created. */
function wantsPost(queue: Queue): boolean {
  return !queue.slug && queue.mintWaiters.length > 0
}

/** True while the editor's assignment differs from what the server holds — a reassignment
 *  that has not landed. Like `wantsPost`, it means unchanged text is not a reason to stand
 *  down: the save still has something to carry. */
function voiceDirty(queue: Queue): boolean {
  return Boolean(queue.slug) && queue.voiceId !== queue.savedVoiceId
}

/** What a request carries for the assignment — see `SendDraft`. */
function voiceToSend(queue: Queue): string | undefined {
  if (!queue.slug) return queue.voiceId
  return voiceDirty(queue) ? queue.voiceId : undefined
}

/** True while the editor's 용도 differs from what the server holds. */
function purposeDirty(queue: Queue): boolean {
  return Boolean(queue.slug) && queue.purposeId !== queue.savedPurposeId
}

/** What a request carries for the 용도 — see `SendDraft`.
 *
 *  On a create, 없음 sends nothing rather than an empty string: the create has no assignment
 *  to clear, and omitting it keeps the request identical to what it was before purposes
 *  existed. On an existing post a dirty '' IS sent, because there it means "clear". */
function purposeToSend(queue: Queue): string | undefined {
  if (!queue.slug) return queue.purposeId || undefined
  return purposeDirty(queue) ? queue.purposeId : undefined
}

function targetLanguageDirty(queue: Queue): boolean {
  return Boolean(queue.slug) && queue.targetLanguage !== queue.savedTargetLanguage
}

function targetLanguageToSend(queue: Queue): ContentLanguage | undefined {
  if (!queue.slug) return queue.targetLanguage
  return targetLanguageDirty(queue) ? queue.targetLanguage : undefined
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

function settleFlushes(queue: Queue): void {
  if (queue.inFlight || queue.pending) return
  const waiters = queue.flushWaiters
  queue.flushWaiters = []
  for (const waiter of waiters) waiter.resolve()
}

function rejectFlushes(queue: Queue, cause: unknown): void {
  const error = cause instanceof Error ? cause : new Error('draft save failed')
  const waiters = queue.flushWaiters
  queue.flushWaiters = []
  for (const waiter of waiters) waiter.reject(error)
}

/** Answers the reassignments that have landed. One still differing from what the server
 *  holds keeps waiting for the save that carries it — unless the editor has since chosen a
 *  third voice, in which case it will never land and says so. */
function settlePurposeWaiters(queue: Queue): void {
  const waiting: PurposeWaiter[] = []
  for (const waiter of queue.purposeWaiters) {
    if (waiter.purposeId === queue.savedPurposeId) waiter.resolve()
    else if (waiter.purposeId !== queue.purposeId)
      waiter.reject(new Error('purpose assignment superseded'))
    else waiting.push(waiter)
  }
  queue.purposeWaiters = waiting
}

function rejectPurposeWaiters(queue: Queue, cause: unknown): void {
  const error = cause instanceof Error ? cause : new Error('purpose assignment failed')
  const waiters = queue.purposeWaiters
  queue.purposeWaiters = []
  for (const waiter of waiters) waiter.reject(error)
}

function settleTargetLanguageWaiters(queue: Queue): void {
  const waiting: TargetLanguageWaiter[] = []
  for (const waiter of queue.targetLanguageWaiters) {
    if (waiter.targetLanguage === queue.savedTargetLanguage) waiter.resolve()
    else if (waiter.targetLanguage !== queue.targetLanguage)
      waiter.reject(new Error('target language assignment superseded'))
    else waiting.push(waiter)
  }
  queue.targetLanguageWaiters = waiting
}

function rejectTargetLanguageWaiters(queue: Queue, cause: unknown): void {
  const error = cause instanceof Error ? cause : new Error('target language assignment failed')
  const waiters = queue.targetLanguageWaiters
  queue.targetLanguageWaiters = []
  for (const waiter of waiters) waiter.reject(error)
}

function settleVoiceWaiters(queue: Queue): void {
  const waiting: VoiceWaiter[] = []
  for (const waiter of queue.voiceWaiters) {
    if (waiter.voiceId === queue.savedVoiceId) waiter.resolve()
    else if (waiter.voiceId !== queue.voiceId)
      waiter.reject(new Error('voice assignment superseded'))
    else waiting.push(waiter)
  }
  queue.voiceWaiters = waiting
}

function rejectVoiceWaiters(queue: Queue, cause: unknown): void {
  const error = cause instanceof Error ? cause : new Error('voice assignment failed')
  const waiters = queue.voiceWaiters
  queue.voiceWaiters = []
  for (const waiter of waiters) waiter.reject(error)
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

/** Sends what is queued right now, cancelling a waiting debounce or backoff. */
function sendNow(queue: Queue): void {
  clearTimers(queue)
  if (queue.inFlight) queue.urgent = true
  else void run(queue)
}

async function run(queue: Queue): Promise<void> {
  if (queue.inFlight || !queue.pending) return

  const sent = queue.pending
  const sentVoice = voiceToSend(queue)
  const sentPurpose = purposeToSend(queue)
  const sentTargetLanguage = targetLanguageToSend(queue)
  queue.inFlight = true
  queue.sending = sent
  publish(queue)

  try {
    const slug = await queue.send(queue.slug, sent, sentVoice, sentPurpose, sentTargetLanguage)
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined
    queue.attempts = 0
    queue.failed = false
    queue.saved = sent
    if (sentVoice !== undefined) queue.savedVoiceId = sentVoice
    if (sentPurpose !== undefined) queue.savedPurposeId = sentPurpose
    if (sentTargetLanguage !== undefined) queue.savedTargetLanguage = sentTargetLanguage
    queue.everSaved = true
    const minted = !queue.slug && Boolean(slug)
    if (minted) rekey(queue, slug)
    // Only what actually went out is settled; anything typed during the round trip is
    // still pending — and so is a voice chosen during it.
    if (
      queue.pending &&
      sameDraft(queue.pending, sent) &&
      !voiceDirty(queue) &&
      !purposeDirty(queue) &&
      !targetLanguageDirty(queue)
    ) {
      queue.pending = undefined
    } else if (voiceDirty(queue) || purposeDirty(queue) || targetLanguageDirty(queue)) {
      // An assignment does not wait for a debounce: it is an action, not a keystroke.
      queue.pending ??= { ...sent }
      queue.urgent = true
    }
    if (minted) {
      queue.onMinted?.(slug)
      const waiters = queue.mintWaiters
      queue.mintWaiters = []
      for (const waiter of waiters) waiter.resolve(slug)
    }
    publish(queue)

    if (queue.pending) {
      // Immediately only for a teardown that could not wait: doing it unconditionally
      // would turn continuous typing into one save per round trip.
      if (queue.urgent) void run(queue)
      else scheduleDebounce(queue)
    }
    queue.urgent = false
    settleFlushes(queue)
    settleVoiceWaiters(queue)
    settlePurposeWaiters(queue)
    settleTargetLanguageWaiters(queue)
  } catch (cause) {
    // Swallowed rather than rethrown: every caller is a timer or a teardown handler with
    // nobody to catch it. The retry is what the user is actually promised.
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined

    if (sentVoice !== undefined && queue.slug) {
      // A refused reassignment is taken back rather than retried. Unlike a text save, the
      // refusal is usually an answer — a busy post, a voice deleted meanwhile — and retrying
      // it with every save would keep the title from ever landing again.
      queue.voiceId = queue.savedVoiceId
      rejectVoiceWaiters(queue, cause)
    }

    if (sentPurpose !== undefined && queue.slug) {
      // Taken back for the same reason a refused reassignment is: the refusal is an answer
      // (a purpose deleted meanwhile, a foreign id), and retrying it with every save would
      // keep the title from ever landing again.
      queue.purposeId = queue.savedPurposeId
      rejectPurposeWaiters(queue, cause)
    }

    if (sentTargetLanguage !== undefined && queue.slug) {
      queue.targetLanguage = queue.savedTargetLanguage ?? queue.targetLanguage
      rejectTargetLanguageWaiters(queue, cause)
    }

    if (
      queue.pending &&
      sameDraft(queue.pending, queue.saved) &&
      !wantsPost(queue) &&
      !voiceDirty(queue) &&
      !purposeDirty(queue) &&
      !targetLanguageDirty(queue)
    ) {
      // Typed back to what the server holds while this attempt was out — there is nothing
      // left to retry, and "다시 시도 중" would stand there forever.
      queue.pending = undefined
      queue.failed = false
      queue.attempts = 0
      publish(queue)
      settleFlushes(queue)
      collect(queue)
      return
    }

    queue.attempts += 1
    queue.failed = true
    queue.urgent = false
    publish(queue)
    // A flush is an explicit prerequisite for another action. Report this failed
    // attempt to that action while the ordinary autosave retry continues in background.
    rejectFlushes(queue, cause)
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
  /** The voice the post is written in as this editor was told — or, for a draft with no
   *  post yet, the one it will be created in. Same caveat as `saved`. */
  voiceId: string
  /** The 용도 the post is assigned to as this editor was told, '' for 없음. */
  purposeId: string
  /** Concrete on both new and existing editors; a new draft sends it only when it is created. */
  targetLanguage: ContentLanguage
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
      voiceId: options.voiceId,
      savedVoiceId: options.slug ? options.voiceId : '',
      purposeId: options.purposeId,
      // '' either way: an existing post with no purpose and a draft with no post both hold
      // 없음, so the create's "send only when chosen" rule needs no extra state.
      savedPurposeId: options.slug ? options.purposeId : '',
      targetLanguage: options.targetLanguage,
      savedTargetLanguage: options.slug ? options.targetLanguage : undefined,
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
      mintWaiters: [],
      flushWaiters: [],
      voiceWaiters: [],
      purposeWaiters: [],
      targetLanguageWaiters: [],
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
      if (
        sameDraft(draft, attached.sending ?? attached.saved) &&
        !wantsPost(attached) &&
        !voiceDirty(attached) &&
        !purposeDirty(attached) &&
        !targetLanguageDirty(attached)
      ) {
        // Typed back to what the server holds. Leaving "저장 대기 중" or "다시 시도 중" on
        // screen with nothing to send would be a standing lie.
        if (attached.pending === undefined && !attached.failed) return
        attached.pending = undefined
        attached.failed = false
        attached.attempts = 0
        clearTimers(attached)
        publish(attached)
        settleFlushes(attached)
        return
      }

      attached.pending = { ...draft }
      publish(attached)
      // Not during a backoff: that timer already covers sending the newest text, and
      // restarting the debounce on every keystroke would defeat the backoff entirely.
      if (attached.retryTimer === undefined) scheduleDebounce(attached)
    },

    saveNow: () => sendNow(attached),

    flush: () => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      if (!attached.inFlight && !attached.pending) return Promise.resolve()
      return new Promise<void>((resolve, reject) => {
        attached.flushWaiters.push({ resolve, reject })
        sendNow(attached)
      })
    },

    release: () => {
      attached.listener = undefined
      attached.onMinted = undefined
      collect(attached)
    },

    mint: () => {
      if (attached.slug) return Promise.resolve(attached.slug)
      // The editor can outlive its session by a render or two while the redirect runs.
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      return new Promise<string>((resolve, reject) => {
        attached.mintWaiters.push({ resolve, reject })
        // A photo picked before a single keystroke: nothing is pending, but the post has
        // to exist, so the empty draft itself is what gets created.
        if (!attached.pending && !attached.inFlight) attached.pending = { ...attached.saved }
        sendNow(attached)
      })
    },

    assignPurpose: (purposeId) => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      attached.purposeId = purposeId
      // Before the post exists the choice rides along with the create — including a create
      // already in flight, which `run` follows up the moment it lands.
      if (!attached.slug || purposeId === attached.savedPurposeId) return Promise.resolve()
      // Nothing typed since the last save: the assignment still needs a request to ride on,
      // so the newest known text is re-sent with it. This is what makes a delayed title save
      // unable to revert a newer selection — the selection goes out on its own request.
      attached.pending ??= { ...(attached.sending ?? attached.saved) }
      publish(attached)
      return new Promise<void>((resolve, reject) => {
        attached.purposeWaiters.push({ purposeId, resolve, reject })
        sendNow(attached)
      })
    },

    assignVoice: (voiceId) => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      attached.voiceId = voiceId
      // Before the post exists the choice simply rides along with the create — including a
      // create already in flight, which `run` follows up the moment it lands.
      if (!attached.slug || voiceId === attached.savedVoiceId) return Promise.resolve()
      // Nothing typed since the last save: the reassignment still needs a request to ride
      // on, so the newest known text is re-sent with it.
      attached.pending ??= { ...(attached.sending ?? attached.saved) }
      publish(attached)
      return new Promise<void>((resolve, reject) => {
        attached.voiceWaiters.push({ voiceId, resolve, reject })
        sendNow(attached)
      })
    },

    assignTargetLanguage: (targetLanguage) => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      attached.targetLanguage = targetLanguage
      // Selecting a target on /posts/new is local state only. If a create is already in
      // flight, its response is followed by an immediate update carrying this newer choice.
      if (!attached.slug || targetLanguage === attached.savedTargetLanguage) {
        return Promise.resolve()
      }
      attached.pending ??= { ...(attached.sending ?? attached.saved) }
      publish(attached)
      return new Promise<void>((resolve, reject) => {
        attached.targetLanguageWaiters.push({ targetLanguage, resolve, reject })
        sendNow(attached)
      })
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
    for (const waiter of queue.mintWaiters) waiter.reject(new Error('session ended'))
    queue.mintWaiters = []
    for (const waiter of queue.flushWaiters) waiter.reject(new Error('session ended'))
    queue.flushWaiters = []
    for (const waiter of queue.voiceWaiters) waiter.reject(new Error('session ended'))
    queue.voiceWaiters = []
    for (const waiter of queue.purposeWaiters) waiter.reject(new Error('session ended'))
    queue.purposeWaiters = []
    for (const waiter of queue.targetLanguageWaiters) waiter.reject(new Error('session ended'))
    queue.targetLanguageWaiters = []
  }
  queues.clear()
}
