import { clone } from '@bufbuild/protobuf'
import type { PostContent } from '@/shared/api'
import { PostContentSchema } from '@/shared/api'
import {
  AUTOSAVE_DEBOUNCE_MS,
  AUTOSAVE_RETRY_BASE_MS,
  AUTOSAVE_RETRY_MAX_MS,
} from '@/shared/config'
import { ContentRevisionConflictError } from '@/entities/post'

export type ContentSaveState = 'idle' | 'dirty' | 'saving' | 'saved' | 'error' | 'conflict'
export interface ContentSnapshot {
  content: PostContent
  targetLength: number
}
export type SendContent = (snapshot: ContentSnapshot, expectedRevision: bigint) => Promise<bigint>

interface Waiter {
  resolve: () => void
  reject: (error: Error) => void
}
interface Queue {
  slug: string
  revision: bigint
  saved: ContentSnapshot
  sending?: ContentSnapshot
  pending?: ContentSnapshot
  inFlight: boolean
  attempts: number
  failed: boolean
  conflict: boolean
  discarded: boolean
  urgent: boolean
  debounceTimer?: number
  retryTimer?: number
  send: SendContent
  listener?: (state: ContentSaveState) => void
  waiters: Waiter[]
}

export interface ContentQueueHandle {
  state: () => ContentSaveState
  queue: (snapshot: ContentSnapshot) => void
  saveNow: () => void
  flush: () => Promise<void>
  release: () => void
}

const queues = new Map<string, Queue>()

function copy(snapshot: ContentSnapshot): ContentSnapshot {
  return { content: clone(PostContentSchema, snapshot.content), targetLength: snapshot.targetLength }
}
function same(a: ContentSnapshot, b: ContentSnapshot): boolean {
  return a.targetLength === b.targetLength && JSON.stringify(a.content) === JSON.stringify(b.content)
}
function stateOf(queue: Queue): ContentSaveState {
  if (queue.inFlight) return 'saving'
  if (queue.conflict) return 'conflict'
  if (queue.failed) return 'error'
  if (queue.pending) return 'dirty'
  return 'saved'
}
function publish(queue: Queue) {
  queue.listener?.(stateOf(queue))
}
function clearTimers(queue: Queue) {
  window.clearTimeout(queue.debounceTimer)
  window.clearTimeout(queue.retryTimer)
  queue.debounceTimer = undefined
  queue.retryTimer = undefined
}
function rejectWaiters(queue: Queue, cause: unknown) {
  const error = cause instanceof Error ? cause : new Error('content save failed')
  const waiters = queue.waiters.splice(0)
  for (const waiter of waiters) waiter.reject(error)
}
function settleWaiters(queue: Queue) {
  if (queue.inFlight || queue.pending) return
  const waiters = queue.waiters.splice(0)
  for (const waiter of waiters) waiter.resolve()
}
function collect(queue: Queue) {
  if (!queue.listener && !queue.inFlight && !queue.pending && !queue.retryTimer)
    queues.delete(queue.slug)
}
function retryDelay(attempt: number) {
  return Math.min(AUTOSAVE_RETRY_BASE_MS * 2 ** (attempt - 1), AUTOSAVE_RETRY_MAX_MS)
}
function schedule(queue: Queue) {
  window.clearTimeout(queue.debounceTimer)
  queue.debounceTimer = window.setTimeout(() => {
    queue.debounceTimer = undefined
    void run(queue)
  }, AUTOSAVE_DEBOUNCE_MS)
}
function sendNow(queue: Queue) {
  clearTimers(queue)
  if (queue.inFlight) queue.urgent = true
  else void run(queue)
}
async function run(queue: Queue): Promise<void> {
  if (queue.inFlight || !queue.pending || queue.conflict || queue.discarded) return
  const sent = copy(queue.pending)
  queue.sending = sent
  queue.inFlight = true
  publish(queue)
  try {
    const nextRevision = await queue.send(sent, queue.revision)
    if (queue.discarded) return
    queue.revision = nextRevision
    queue.saved = sent
    queue.inFlight = false
    queue.sending = undefined
    queue.failed = false
    queue.attempts = 0
    if (queue.pending && same(queue.pending, sent)) queue.pending = undefined
    publish(queue)
    if (queue.pending) {
      if (queue.urgent) void run(queue)
      else schedule(queue)
    }
    queue.urgent = false
    settleWaiters(queue)
  } catch (cause) {
    if (queue.discarded) return
    queue.inFlight = false
    queue.sending = undefined
    queue.urgent = false
    if (cause instanceof ContentRevisionConflictError) {
      queue.conflict = true
      queue.failed = false
      clearTimers(queue)
    } else {
      queue.failed = true
      queue.attempts += 1
      queue.retryTimer = window.setTimeout(() => {
        queue.retryTimer = undefined
        void run(queue)
      }, retryDelay(queue.attempts))
    }
    publish(queue)
    rejectWaiters(queue, cause)
  }
  collect(queue)
}

export function attachContentQueue(options: {
  slug: string
  revision: bigint
  saved: ContentSnapshot
  send: SendContent
  onState: (state: ContentSaveState) => void
}): ContentQueueHandle {
  let queue = queues.get(options.slug)
  if (!queue) {
    queue = {
      slug: options.slug,
      revision: options.revision,
      saved: copy(options.saved),
      inFlight: false,
      attempts: 0,
      failed: false,
      conflict: false,
      discarded: false,
      urgent: false,
      send: options.send,
      listener: options.onState,
      waiters: [],
    }
    queues.set(options.slug, queue)
  }
  const attached = queue
  attached.send = options.send
  attached.listener = options.onState
  return {
    state: () => stateOf(attached),
    queue: (snapshot) => {
      if (attached.conflict || attached.discarded) return
      if (same(snapshot, attached.sending ?? attached.saved)) {
        attached.pending = undefined
        publish(attached)
        settleWaiters(attached)
        return
      }
      attached.pending = copy(snapshot)
      publish(attached)
      if (!attached.retryTimer) schedule(attached)
    },
    saveNow: () => sendNow(attached),
    flush: () => {
      if (attached.discarded) return Promise.reject(new Error('session ended'))
      if (attached.conflict) return Promise.reject(new ContentRevisionConflictError())
      if (!attached.inFlight && !attached.pending) return Promise.resolve()
      return new Promise<void>((resolve, reject) => {
        attached.waiters.push({ resolve, reject })
        sendNow(attached)
      })
    },
    release: () => {
      attached.listener = undefined
      sendNow(attached)
      collect(attached)
    },
  }
}

export function discardContentQueues(): void {
  for (const queue of queues.values()) {
    queue.discarded = true
    clearTimers(queue)
    rejectWaiters(queue, new Error('session ended'))
  }
  queues.clear()
}
