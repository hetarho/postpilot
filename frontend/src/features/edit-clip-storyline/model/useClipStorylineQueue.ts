import { useCallback, useSyncExternalStore } from 'react'
import type { AppFailure } from '@/shared/api'
import type { ClipStorylineParagraph } from '@/entities/clip-project'
import { clipStorylineFailure, peekClipStoryline, subscribeClipStoryline } from './storyline-queue'

/** What the storyline space reads from its autosave queue, which outlives it: the paragraphs the
 *  project still owes the server, and the failure the queue is retrying or stopped on. */
export function useClipStorylineQueue(projectId: string): {
  owed: ClipStorylineParagraph[] | undefined
  failure: AppFailure | undefined
} {
  const subscribe = useCallback(
    (listener: () => void) => subscribeClipStoryline(projectId, listener),
    [projectId],
  )
  const owed = useSyncExternalStore(
    subscribe,
    () => peekClipStoryline(projectId),
    () => undefined,
  )
  const failure = useSyncExternalStore(
    subscribe,
    () => clipStorylineFailure(projectId),
    () => undefined,
  )
  return { owed, failure }
}
