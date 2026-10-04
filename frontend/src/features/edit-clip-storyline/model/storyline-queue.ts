import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import { createAutosaveQueue, type SaveState } from '@/shared/lib'
import { serialClipWrite, type ClipStorylineParagraph } from '@/entities/clip-project'

/** The clip storyline's autosave, one queue per project and outside React (CLIP-178, CLIP-39),
 *  on the shared machine: one request in flight, the latest paragraphs win, a transient failure
 *  retries, a refusal waits for the owner's next edit.
 *
 *  Module level because ② is a step panel: the space unmounts the moment the owner looks at ①
 *  or ③, and a debounce living in it would take the last words with it. Each send runs in the
 *  project's write lane, so it neither crosses a settings, slot or correction save in flight nor
 *  is crossed by one. */
export type SendClipStoryline = (paragraphs: ClipStorylineParagraph[]) => Promise<void>

// A beat after the last edit, so one save carries a word, not every keystroke.
const SAVE_DELAY_MS = 600

/** One sender per project, replaced on every queue call so it closes over the newest render's
 *  mutation and transport, and never sends one clip's storyline through another's. */
const senders = new Map<string, SendClipStoryline>()
const queue = createAutosaveQueue<ClipStorylineParagraph[], void>({
  debounceMs: SAVE_DELAY_MS,
  send: (paragraphs, { key }) => serialClipWrite(key, () => senders.get(key)!(paragraphs)),
  retry: retriableTransportFailure,
})

export function queueClipStoryline(
  projectId: string,
  paragraphs: ClipStorylineParagraph[],
  send: SendClipStoryline,
): void {
  senders.set(projectId, send)
  queue.queue(projectId, paragraphs)
}

/** Sends what is owed NOW and resolves once the queue is dry, or rejects with the failure — so a
 *  build or a request from the storyline never runs on the server's older one. */
export function flushClipStoryline(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast).then(() => undefined)
}

/** The paragraphs this project still owes the server: queued, in flight or failed. While there
 *  are any, they are the owner's text and outrank what the server last reported. */
export function peekClipStoryline(projectId: string): ClipStorylineParagraph[] | undefined {
  return queue.peek(projectId)
}

export function clipStorylineState(projectId: string): SaveState {
  const state = queue.state(projectId)
  // The storyline has no revision to conflict over; the queue never reports one.
  return state === 'conflict' ? 'error' : state
}

/** A stable snapshot of the failure the queue stopped or is retrying on, cached per error so a
 *  subscriber is not re-rendered by its own read. */
const failures = new WeakMap<object, AppFailure>()
export function clipStorylineFailure(projectId: string): AppFailure | undefined {
  const error = queue.failure(projectId)
  const state = queue.state(projectId)
  if (!error || typeof error !== 'object' || (state !== 'refused' && state !== 'error'))
    return undefined
  const known = failures.get(error)
  if (known) return known
  const failure = appFailureFromConnect(error)
  failures.set(error, failure)
  return failure
}

export function subscribeClipStoryline(projectId: string, listener: () => void): () => void {
  return queue.subscribe(projectId, listener)
}

export function discardClipStorylineQueue(projectId: string): void {
  queue.discard(projectId)
  senders.delete(projectId)
}

export function discardClipStorylineQueues(): void {
  queue.discardAll()
  senders.clear()
}
