// The draft save pipeline, per post, outside React.
//
// This one is NOT `shared/lib/autosave` (T266), and deliberately: the clip settings' queue and
// the block editor's are a debounce around one payload, while this queue is a serial channel for
// four things at once — the text, and the voice, 템플릿 and target-language ASSIGNMENTS, each with
// its own waiters, its own "what the server holds" baseline and its own taken-back-on-refusal
// rule — and it re-keys itself mid-flight when the first save mints the slug. Folding that into
// the shared machine would move post rules into `shared/lib` rather than share a machine. The
// three assignments share one shape, so they are one record each, walked by one channel list:
// a new channel is a row of `RULES`, not a dozen hand-kept sites.
// What it does share: `SaveState`, the `AUTOSAVE_*` timing and the state vocabulary.
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
import type { SaveState } from '@/shared/lib'

export type { SaveState }

export interface Draft {
  title: string
  memo: string
  /** The answers to the selected template's data fields, in the order ① renders them. They ride
   *  the same debounce as the memo because they are the same input: what the next run is given
   *  (POST-62). Every entry is an UPSERT of that label, so sending the whole current set is
   *  always safe — it cannot disturb an answer this editor is not showing. */
  answers: TemplateAnswerDraft[]
}

export interface TemplateAnswerDraft {
  label: string
  text: string
  enabled: boolean
}

/** The assignments a draft is written WITH, keyed by the request's own member names. */
export interface Assignments {
  voiceId: string
  templateId: string
  targetLanguage: ContentLanguage
}

export type AssignmentChannel = keyof Assignments

/** One save's request. An undefined assignment leaves the post's value alone. */
export interface DraftRequest extends Partial<Assignments> {
  /** Empty for a draft whose first save mints the post. */
  slug: string
  draft: Draft
  /** Present on every create — a post cannot exist without a voice (spec/legacy/policy/posts.md)
   *  — and on an existing post only while the assignment differs from what the server holds, so
   *  an ordinary title save can never carry a stale voice over a newer one. */
  voiceId?: string
  /** The voice's mechanism with one more state, because a post may have none: '' clears it and
   *  an id assigns it. On a create it is sent only when one was chosen — 없음 is the default and
   *  the server never picks. */
  templateId?: string
  targetLanguage?: ContentLanguage
}

/** Performs one save and resolves with the post's slug.
 *
 *  Injected so this module knows nothing of React or of the transport. It must REJECT
 *  when the server did not confirm the post: a resolve is taken as proof the text landed,
 *  and for a draft with no slug yet, as proof the post now exists. */
export type SendDraft = (request: DraftRequest) => Promise<string>

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
  /** Records one assignment: the voice, the 템플릿 ('' for 없음) or the target language. For a
   *  draft with no post yet that is all it does — the create carries it. For an existing post
   *  it is sent at once, on its own request, so a delayed title save cannot revert a newer
   *  choice, and the promise reports that one save's outcome. A refused assignment is taken
   *  back, so the retries that follow carry text only instead of failing forever on the same
   *  answer. */
  assign: <K extends AssignmentChannel>(channel: K, value: Assignments[K]) => Promise<void>
}

interface MintWaiter {
  resolve: (slug: string) => void
  reject: (error: Error) => void
}

interface FlushWaiter {
  resolve: () => void
  reject: (error: Error) => void
}

interface AssignmentWaiter<T> extends FlushWaiter {
  value: T
}

/** One assignment: what the editor wants, what the server is known to hold, and the callers
 *  waiting for the wanted value to land. */
interface Assignment<T> {
  wanted: T
  saved: T | undefined
  waiters: AssignmentWaiter<T>[]
}

type AssignmentRecords = { [K in AssignmentChannel]: Assignment<Assignments[K]> }

/** What differs between the channels. `onCreate` is what a create carries; `unsaved` is what the
 *  server holds before the post exists. */
const RULES: {
  [K in AssignmentChannel]: {
    noun: string
    onCreate: (wanted: Assignments[K]) => Assignments[K] | undefined
    unsaved: Assignments[K] | undefined
  }
} = {
  // Every create carries its voice; the server holds none until the post exists.
  voiceId: { noun: 'voice', onCreate: (wanted) => wanted, unsaved: '' },
  // On a create, 없음 sends nothing rather than an empty string: the create has no assignment to
  // clear, and omitting it keeps the request identical to what it was before templates existed.
  // On an existing post a dirty '' IS sent, because there it means "clear". '' is also what a
  // draft with no post holds, so the rule needs no extra state.
  templateId: { noun: 'template', onCreate: (wanted) => wanted || undefined, unsaved: '' },
  targetLanguage: { noun: 'target language', onCreate: (wanted) => wanted, unsaved: undefined },
}

const CHANNELS = [
  'voiceId',
  'templateId',
  'targetLanguage',
] as const satisfies readonly AssignmentChannel[]

// Every channel is in the list: a key of `Assignments` missing from CHANNELS fails to type here.
const everyChannelListed: [Exclude<AssignmentChannel, (typeof CHANNELS)[number]>] extends [never]
  ? true
  : never = true
void everyChannelListed

/** Runs `fn` for each channel with its record, typed together — the one place the correlation
 *  between a channel and its record's value type is asserted. */
function each(
  queue: Queue,
  fn: <K extends AssignmentChannel>(channel: K, assignment: Assignment<Assignments[K]>) => void,
): void {
  for (const channel of CHANNELS) fn(channel, queue.assignments[channel] as never)
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
  /** The voice, 템플릿 and target language: each wanted value, its server baseline and its
   *  waiters. */
  assignments: AssignmentRecords
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
  /** Whether a refused save is worth retrying. Taken from the attaching editor like `send`. */
  retry: (cause: unknown) => boolean
  listener: ((state: SaveState) => void) | undefined
  onMinted: ((slug: string) => void) | undefined
  /** Told when a refused save took the text back, so the attached editor can drop it too. */
  onTakenBack: (() => void) | undefined
  /** Callers of `mint` waiting for the first save to land. On the queue, not the handle,
   *  so they survive the editor swap the mint itself causes. */
  mintWaiters: MintWaiter[]
  flushWaiters: FlushWaiter[]
}

const queues = new Map<string, Queue>()

let newDraftSequence = 0

/** A draft with no slug yet gets a key of its own rather than a shared one: two "새 글"
 *  editors in the same tab are two different drafts, and one must not be able to clear or
 *  claim the other's unfinished save. Every real slug is `YYYYMMDD-…`
 *  (spec/legacy/policy/posts.md), so this prefix cannot collide with one. */
function newDraftKey(): string {
  newDraftSequence += 1
  return `new:${newDraftSequence}`
}

/** Exponential backoff, capped. The cap keeps a tab left open on a dead network from
 *  becoming a request loop while still recovering once the network returns. */
function retryDelay(attempt: number): number {
  return Math.min(AUTOSAVE_RETRY_BASE_MS * 2 ** (attempt - 1), AUTOSAVE_RETRY_MAX_MS)
}

/** DIRECTIONAL: `held` is what the editor holds and `saved` what the server does. Both call
 *  sites pass them in that order, and the answers half needs it — see `answersSettled`. */
function sameDraft(held: Draft, saved: Draft): boolean {
  return (
    held.title === saved.title &&
    held.memo === saved.memo &&
    answersSettled(held.answers, saved.answers)
  )
}

/** Whether the answers the editor holds are already what the server holds.
 *
 *  It is a per-label check over the fields ON SCREEN rather than a whole-set comparison,
 *  because the patch is upsert-only: a draft carries the selected template's fields and nothing
 *  else, so a post holding answers under another template — or under none, with the picker on
 *  없음 — must not read as dirty and fire a save nobody asked for.
 *
 *  A field with no saved row is settled while it holds the DEFAULT the screen renders for it
 *  (empty and switched on). Otherwise every post with a template and no answers yet would be
 *  dirty the moment its editor mounted. */
function answersSettled(held: TemplateAnswerDraft[], saved: TemplateAnswerDraft[]): boolean {
  return held.every((answer) => {
    const stored = saved.find((candidate) => candidate.label === answer.label)
    if (stored) return stored.text === answer.text && stored.enabled === answer.enabled
    return answer.text === '' && answer.enabled
  })
}

/** The baseline after a save: what the server held, with what this save carried laid over it.
 *  Replacing it outright would drop the labels this save did not carry, and the editor would
 *  then re-send them the next time its template made them visible again. */
export function mergeAnswers(
  saved: readonly TemplateAnswerDraft[],
  sent: readonly TemplateAnswerDraft[],
): TemplateAnswerDraft[] {
  const merged = saved.map((answer) => ({ ...answer }))
  for (const answer of sent) {
    const at = merged.findIndex((candidate) => candidate.label === answer.label)
    if (at === -1) merged.push({ ...answer })
    else merged[at] = { ...answer }
  }
  return merged
}

/** True while someone is waiting for this draft to become a post. Then "typed back to
 *  what the server holds" is not a reason to stand down — the server holds nothing yet,
 *  and the empty draft itself has to be created. */
function wantsPost(queue: Queue): boolean {
  return !queue.slug && queue.mintWaiters.length > 0
}

/** True while the editor's assignment differs from what the server holds — one that has not
 *  landed. Like `wantsPost`, it means unchanged text is not a reason to stand down: the save
 *  still has something to carry. */
function dirty(queue: Queue, channel: AssignmentChannel): boolean {
  const assignment = queue.assignments[channel]
  return Boolean(queue.slug) && assignment.wanted !== assignment.saved
}

function anyDirty(queue: Queue): boolean {
  return CHANNELS.some((channel) => dirty(queue, channel))
}

/** What a request carries for one assignment — see `DraftRequest`: the create's rule before the
 *  post exists, and afterwards the value only while it is dirty. */
function toSend<K extends AssignmentChannel>(queue: Queue, channel: K): Assignments[K] | undefined {
  const assignment = queue.assignments[channel]
  if (!queue.slug) return RULES[channel].onCreate(assignment.wanted)
  return dirty(queue, channel) ? assignment.wanted : undefined
}

/** The assignments this save carries, and only those. */
function carried(queue: Queue): Partial<Assignments> {
  const out: Partial<Assignments> = {}
  each(queue, (channel) => {
    const value = toSend(queue, channel)
    if (value !== undefined) out[channel] = value
  })
  return out
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

/** Answers the assignments that have landed. One still differing from what the server holds
 *  keeps waiting for the save that carries it — unless the editor has since chosen a third
 *  value, in which case it will never land and says so. */
function settleAssignments(queue: Queue): void {
  each(queue, (channel, assignment) => {
    const waiting: typeof assignment.waiters = []
    for (const waiter of assignment.waiters) {
      if (waiter.value === assignment.saved) waiter.resolve()
      else if (waiter.value !== assignment.wanted)
        waiter.reject(new Error(`${RULES[channel].noun} assignment superseded`))
      else waiting.push(waiter)
    }
    assignment.waiters = waiting
  })
}

/** Takes one assignment back to what the server holds and rejects its waiters with why. */
function takeBack<K extends AssignmentChannel>(queue: Queue, channel: K, cause: unknown): void {
  const assignment = queue.assignments[channel]
  assignment.wanted = assignment.saved ?? assignment.wanted
  const error =
    cause instanceof Error ? cause : new Error(`${RULES[channel].noun} assignment failed`)
  const waiters = assignment.waiters
  assignment.waiters = []
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
  const sentAssignments = carried(queue)
  queue.inFlight = true
  queue.sending = sent
  publish(queue)

  try {
    const slug = await queue.send({ slug: queue.slug, draft: sent, ...sentAssignments })
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined
    queue.attempts = 0
    queue.failed = false
    queue.saved = { ...sent, answers: mergeAnswers(queue.saved.answers, sent.answers) }
    each(queue, (channel, assignment) => {
      const value = sentAssignments[channel]
      if (value !== undefined) assignment.saved = value
    })
    queue.everSaved = true
    const minted = !queue.slug && Boolean(slug)
    if (minted) rekey(queue, slug)
    // Only what actually went out is settled; anything typed during the round trip is
    // still pending — and so is a voice chosen during it.
    if (queue.pending && sameDraft(queue.pending, sent) && !anyDirty(queue)) {
      queue.pending = undefined
    } else if (anyDirty(queue)) {
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
    settleAssignments(queue)
  } catch (cause) {
    // Swallowed rather than rethrown: every caller is a timer or a teardown handler with
    // nobody to catch it. The retry is what the user is actually promised.
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined

    if (queue.slug) {
      // A refused assignment is taken back rather than retried. Unlike a text save, the refusal
      // is usually an answer — a busy post, a voice or a template deleted meanwhile, a foreign
      // id — and retrying it with every save would keep the title from ever landing again.
      for (const channel of CHANNELS)
        if (sentAssignments[channel] !== undefined) takeBack(queue, channel, cause)
    }

    if (queue.slug && !queue.retry(cause)) {
      // An answer the server will repeat, such as a post published in another tab (POST-86): the
      // rule a refused reassignment follows, applied to the text too. Nothing is retried, the
      // text and every assignment still waiting are taken back, and the status line is not left
      // saying 다시 시도 중 over a save that will never land.
      queue.pending = undefined
      queue.failed = false
      queue.attempts = 0
      queue.urgent = false
      for (const channel of CHANNELS) takeBack(queue, channel, cause)
      clearTimers(queue)
      publish(queue)
      // The text on screen is taken back with the queue's, so the editor never shows — or queues
      // again — what the server refused.
      queue.onTakenBack?.()
      rejectFlushes(queue, cause)
      collect(queue)
      return
    }

    if (
      queue.pending &&
      sameDraft(queue.pending, queue.saved) &&
      !wantsPost(queue) &&
      !anyDirty(queue)
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

const alwaysRetry = () => true

/** An editor's attachment. Every assignment's baseline is required: a channel the caller forgets
 *  does not compile, rather than starting from nothing. */
export interface DraftQueueOptions extends Assignments {
  /** The post being written to, or undefined for a draft with no slug yet. */
  slug: string | undefined
  /** What the server holds, as this editor was told. Used only when there is no queue
   *  yet: an existing one has watched every save and knows better. The assignments are the
   *  same: for a draft with no post yet, the values it will be created with. */
  saved: Draft
  send: SendDraft
  /** Whether a refused save of an existing post is retried. Default: always, since most
   *  refusals are an outage the next attempt outlasts. False takes the text back instead. */
  retry?: (cause: unknown) => boolean
  onState: (state: SaveState) => void
  onMinted: (slug: string) => void
  /** Called once when a save refused as an answer (`retry` false) took the text back. */
  onTakenBack?: () => void
}

function initialAssignment<K extends AssignmentChannel>(
  options: DraftQueueOptions,
  channel: K,
): Assignment<Assignments[K]> {
  return {
    wanted: options[channel],
    saved: options.slug ? options[channel] : RULES[channel].unsaved,
    waiters: [],
  }
}

/** One record per channel, built from the list. */
function initialAssignments(options: DraftQueueOptions): AssignmentRecords {
  return Object.fromEntries(
    CHANNELS.map((channel) => [channel, initialAssignment(options, channel)]),
  ) as AssignmentRecords
}

export function attachDraftQueue(options: DraftQueueOptions): DraftQueueHandle {
  const key = options.slug ?? newDraftKey()
  let queue = queues.get(key)

  if (!queue) {
    queue = {
      key,
      slug: options.slug ?? '',
      saved: options.saved,
      sending: undefined,
      pending: undefined,
      assignments: initialAssignments(options),
      everSaved: false,
      failed: false,
      discarded: false,
      inFlight: false,
      urgent: false,
      attempts: 0,
      debounceTimer: undefined,
      retryTimer: undefined,
      send: options.send,
      retry: alwaysRetry,
      listener: undefined,
      onMinted: undefined,
      onTakenBack: undefined,
      mintWaiters: [],
      flushWaiters: [],
    }
    queues.set(key, queue)
  }

  // A queue that outlived its editor is adopted, not replaced. Taking the new editor's
  // callbacks also takes its live transport: the previous editor's is bound to a mutation
  // observer that is gone.
  const attached = queue
  attached.send = options.send
  attached.retry = options.retry ?? alwaysRetry
  attached.listener = options.onState
  attached.onMinted = options.onMinted
  attached.onTakenBack = options.onTakenBack

  return {
    state: () => stateOf(attached),

    queue: (draft) => {
      // Against the request in flight when there is one: a save already sent cannot be
      // recalled, so comparing with the older `saved` would call an undo "already saved"
      // and never send it.
      if (
        sameDraft(draft, attached.sending ?? attached.saved) &&
        !wantsPost(attached) &&
        !anyDirty(attached)
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

      // The answers are copied too: the queue must not share backing storage with an array
      // the caller mutates next.
      attached.pending = { ...draft, answers: draft.answers.map((answer) => ({ ...answer })) }
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
      attached.onTakenBack = undefined
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

    assign: (channel, value) => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      const assignment = attached.assignments[channel]
      assignment.wanted = value
      // Before the post exists the choice rides along with the create — including a create
      // already in flight, which `run` follows up the moment it lands.
      if (!attached.slug || value === assignment.saved) return Promise.resolve()
      // Nothing typed since the last save: the assignment still needs a request to ride on, so
      // the newest known text is re-sent with it. This is what makes a delayed title save unable
      // to revert a newer choice — the choice goes out on its own request.
      attached.pending ??= { ...(attached.sending ?? attached.saved) }
      publish(attached)
      return new Promise<void>((resolve, reject) => {
        assignment.waiters.push({ value, resolve, reject })
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
 *  A queue outlives its editor on template, but it must never outlive its session: a retry
 *  that fires after someone else has signed in on the same device would send the previous
 *  account's draft under the new account's cookie, and the server would file it there.
 *  Called by the app layer on logout and on a session that died mid-use. */
export function discardDraftQueues(): void {
  for (const queue of queues.values()) discardQueue(queue, 'session ended')
  queues.clear()
}

/** Drops one post's queue, cancelling whatever it was still going to save.
 *
 *  The one exception to "a queue outlives its editor, never its session": an intentional
 *  delete also ends a queue, for that slug alone. Without it a failing save would keep
 *  retrying against a slug the server no longer knows, and the user would be told their
 *  save failed for a post they destroyed on template. Other slugs are untouched. */
export function discardDraftQueue(slug: string): void {
  const queue = queues.get(slug)
  if (!queue) return
  discardQueue(queue, 'post deleted')
  queues.delete(slug)
}

/** The reason is carried into every rejection so a stray rejected promise says which of
 *  the two discards cancelled it. */
function discardQueue(queue: Queue, reason: string): void {
  queue.discarded = true
  clearTimers(queue)
  for (const waiter of queue.mintWaiters) waiter.reject(new Error(reason))
  queue.mintWaiters = []
  for (const waiter of queue.flushWaiters) waiter.reject(new Error(reason))
  queue.flushWaiters = []
  each(queue, (_channel, assignment) => {
    for (const waiter of assignment.waiters) waiter.reject(new Error(reason))
    assignment.waiters = []
  })
}
