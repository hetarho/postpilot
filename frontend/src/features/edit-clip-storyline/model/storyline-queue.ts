import { retriableTransportFailure, type AppFailure } from '@/shared/api'
import type { SaveState } from '@/shared/lib'
import { createProjectSaveQueue, type ClipStorylineParagraph } from '@/entities/clip-project'

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

// The storyline has no revision to conflict over; the queue never reports one.
const queue = createProjectSaveQueue<ClipStorylineParagraph[], SendClipStoryline>({
  conflictAs: 'error',
  debounceMs: SAVE_DELAY_MS,
  lane: true,
  run: (send, paragraphs) => send(paragraphs),
  retry: retriableTransportFailure,
})

export function queueClipStoryline(
  projectId: string,
  paragraphs: ClipStorylineParagraph[],
  send: SendClipStoryline,
): void {
  queue.queue(projectId, paragraphs, send)
}

/** Sends what is owed NOW and resolves once the queue is dry, or rejects with the failure — so a
 *  build or a request from the storyline never runs on the server's older one. */
export function flushClipStoryline(projectId: string, failFast = false): Promise<void> {
  return queue.flush(projectId, failFast)
}

/** The paragraphs this project still owes the server: queued, in flight or failed. While there
 *  are any, they are the owner's text and outrank what the server last reported. */
export function peekClipStoryline(projectId: string): ClipStorylineParagraph[] | undefined {
  return queue.peek(projectId)
}

export function clipStorylineState(projectId: string): SaveState {
  return queue.state(projectId)
}

/** A stable snapshot of the failure the queue stopped or is retrying on. */
export function clipStorylineFailure(projectId: string): AppFailure | undefined {
  return queue.failure(projectId)
}

export function subscribeClipStoryline(projectId: string, listener: () => void): () => void {
  return queue.subscribe(projectId, listener)
}

export function discardClipStorylineQueue(projectId: string): void {
  queue.discard(projectId)
}

export function discardClipStorylineQueues(): void {
  queue.discardAll()
}
