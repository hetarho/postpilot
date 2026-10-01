import { appFailureFromConnect, type PostContent } from '@/shared/api'
import { createAutosaveQueue } from '@/shared/lib'
import { ContentRevisionConflictError, copyPostContent } from '@/entities/post'

/** The block editor's autosave, one queue per post slug and OUTSIDE React.
 *
 *  The machine is `shared/lib/autosave`; what belongs to the content is here — every save
 *  carries the revision it was edited from, and a revision the server has moved past is a
 *  CONFLICT: the queue stops rather than retrying, because the same body will be refused again
 *  and the editor has to be told to reload. */
export type ContentSaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error' | 'conflict'
export interface ContentSnapshot {
  content: PostContent
}
export type SendContent = (snapshot: ContentSnapshot, expectedRevision: bigint) => Promise<bigint>

export interface ContentQueueHandle {
  state: () => ContentSaveState
  queue: (snapshot: ContentSnapshot) => void
  saveNow: () => void
  flush: () => Promise<bigint>
  release: () => void
}

interface Attachment {
  revision: bigint
  machineBaselineRevision: bigint
  saved: ContentSnapshot
  send: SendContent
  onState?: (state: ContentSaveState) => void
  /** A machine write arrived while this editor still owed a save. Keep the old write protected
   * by its original revision and refuse this new editor until the owner reloads. */
  baselineConflict?: boolean
}
const attached = new Map<string, Attachment>()

function copy(snapshot: ContentSnapshot): ContentSnapshot {
  return { content: copyPostContent(snapshot.content) }
}
function same(a: ContentSnapshot, b: ContentSnapshot): boolean {
  return JSON.stringify(a.content) === JSON.stringify(b.content)
}

const queue = createAutosaveQueue<ContentSnapshot, bigint>({
  // A draft that arrived mid-flight waits out another debounce: the editor reports on every
  // keystroke, and the revision each save carries is the one the last answer returned.
  follow: 'debounce',
  conflict: (error) => error instanceof ContentRevisionConflictError,
  // A post published in another tab refuses every content save (POST-86). Retrying would only
  // repeat the refusal; the post's refetch unmounts the editor instead.
  retry: (error) => appFailureFromConnect(error).reason !== 'POST_PUBLISHED_LOCKED',
  // A keystroke that lands back on what the server holds — or on what the request now out is
  // making it hold — is not a change, and a failing save keeps its own backoff.
  retryOnEdit: 'keep',
  settled: (snapshot, sending, slug) => {
    const baseline = sending ?? attached.get(slug)?.saved
    return baseline !== undefined && same(snapshot, baseline)
  },
  send: async (snapshot, { key }) => {
    const post = attached.get(key)
    if (!post) throw new Error('content queue detached')
    // The attachment holds the revision, not the queue: it is what the editor mounted against
    // and what each answered save moves forward.
    const revision = await post.send(copy(snapshot), post.revision)
    post.revision = revision
    post.saved = copy(snapshot)
    return revision
  },
})

function stateOf(slug: string): ContentSaveState {
  const state = queue.state(slug)
  return state === 'refused' ? 'error' : state === 'idle' ? 'saved' : state
}

function notify(slug: string) {
  attached.get(slug)?.onState?.(stateOf(slug))
}

/** Always fail-fast: a caller waiting on the content — 확정, 발행 — must be told the save was
 *  refused now rather than after the backoff has run its course. The queue keeps retrying in the
 *  background either way. */
function flushQueue(slug: string): Promise<bigint> {
  const post = attached.get(slug)
  if (!post) return Promise.reject(new Error('session ended'))
  if (post.baselineConflict) return Promise.reject(new ContentRevisionConflictError())
  return queue.flush(slug, true).then((revision) => revision ?? post.revision)
}

/** Flush this post's content queue without a mounted editor.
 *
 *  The queue outlives the component: it is keyed by slug and kept while it still has work, which
 *  is what lets an unmounted editor's last save finish. 확정 lives on a different step from the
 *  block editor now, so it has to be able to wait for that save — and take its revision — without
 *  the editor on screen. Undefined means there is no queue at all, so there is nothing to wait
 *  for. */
export function flushContentQueue(slug: string): Promise<bigint> | undefined {
  return attached.has(slug) ? flushQueue(slug) : undefined
}

export function attachContentQueue(options: {
  slug: string
  revision: bigint
  machineBaselineRevision: bigint
  saved: ContentSnapshot
  send: SendContent
  onState: (state: ContentSaveState) => void
}): ContentQueueHandle {
  const existing = attached.get(options.slug)
  // A machine write replaces the content this editor opened against. React unmounts the old
  // BlockEditor and mounts the new one in the same commit; release() has only scheduled its
  // attachment cleanup, so this mount can still see the old attachment. Drop that queue and
  // start from the server's new content/revision. A normal content autosave advances revision
  // without moving the machine baseline, so it keeps its in-flight queue instead.
  const replaced =
    existing !== undefined && existing.machineBaselineRevision !== options.machineBaselineRevision
  // A queued, in-flight or failed save still belongs to the owner's previous content. A new
  // machine result must not make that edit disappear, nor may the old pending snapshot be sent
  // against the new revision. Leave the old queue guarded by its original revision and show
  // the same explicit conflict as a save rejected by the server.
  if (
    existing?.baselineConflict ||
    (replaced && !['idle', 'saved'].includes(queue.state(options.slug)))
  ) {
    if (existing) existing.baselineConflict = true
    return {
      state: () => 'conflict',
      queue: () => undefined,
      saveNow: () => undefined,
      flush: () => Promise.reject(new ContentRevisionConflictError()),
      release: () => undefined,
    }
  }
  if (!existing || replaced)
    queue.discard(
      options.slug,
      replaced ? new Error('content replaced by a new AI result') : undefined,
    )
  const post: Attachment =
    existing && !replaced
      ? existing
      : {
          revision: options.revision,
          machineBaselineRevision: options.machineBaselineRevision,
          saved: copy(options.saved),
          send: options.send,
        }
  post.send = options.send
  post.onState = options.onState
  attached.set(options.slug, post)
  const unsubscribe = queue.subscribe(options.slug, () => notify(options.slug))
  return {
    state: () => stateOf(options.slug),
    queue: (snapshot) => {
      if (stateOf(options.slug) === 'conflict') return
      queue.queue(options.slug, copy(snapshot))
    },
    saveNow: () => {
      void queue.flush(options.slug).catch(() => undefined)
    },
    flush: () => flushQueue(options.slug),
    release: () => {
      post.onState = undefined
      unsubscribe()
      // The queue outlives the editor while it still has work; once it is dry the attachment is
      // dropped, so the next editor for this slug starts from the server's content rather than
      // from a baseline this one left behind.
      void queue.flush(options.slug).then(
        () => {
          if (attached.get(options.slug) === post && !post.onState && !post.baselineConflict)
            attached.delete(options.slug)
        },
        () => undefined,
      )
    },
  }
}

export function discardContentQueues(): void {
  for (const slug of [...attached.keys()]) end(slug, 'session ended')
}

/** Drops one post's content queue. The delete path's counterpart to discardDraftQueue:
 *  the same slug's two queues end together, and no other slug is affected. */
export function discardContentQueue(slug: string): void {
  end(slug, 'post deleted')
}

/** A flush waiting on a queue that is being dropped is REJECTED with why: resolving would tell
 *  a caller the content landed when the post it belonged to is gone. */
function end(slug: string, reason: string): void {
  attached.delete(slug)
  queue.discard(slug, new Error(reason))
}
