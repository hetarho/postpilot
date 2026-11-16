import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import { createAutosaveQueue, type SaveState } from '@/shared/lib'
import { serialClipWrite } from '../lib/write-lane'

export interface ProjectSaveQueueOptions<TDraft, TSender> {
  /** How the queue's `conflict` state is reported: the storyline has no revision to conflict
   *  over (`'error'`), the other drafts show a refusal (`'refused'`). */
  conflictAs: 'error' | 'refused'
  /** Omitted, the shared autosave beat. */
  debounceMs?: number
  /** Runs each send in the project's write lane, so it neither crosses nor is crossed by
   *  another save of the same project. */
  lane?: boolean
  /** What one send does with the project's newest sender and the queued draft. */
  run: (sender: TSender, draft: TDraft) => Promise<void>
  retry?: (error: unknown) => boolean
  /** A failure that means the server has moved past the draft; the queue stops and reports
   *  `conflictAs`. */
  conflict?: (error: unknown) => boolean
}

/** The autosave queue every per-project clip draft shares, outside React: one sender per
 *  project (replaced on every queue call so it closes over the newest render and never sends one
 *  clip's draft through another's), the shared machine underneath, and snapshots stable enough
 *  for `useSyncExternalStore`. Each feature wraps it in its own named functions. */
export function createProjectSaveQueue<TDraft, TSender>(
  options: ProjectSaveQueueOptions<TDraft, TSender>,
) {
  const senders = new Map<string, TSender>()
  const queue = createAutosaveQueue<TDraft, void>({
    debounceMs: options.debounceMs,
    send: (draft, { key }) => {
      const go = () => options.run(senders.get(key)!, draft)
      return options.lane ? serialClipWrite(key, go) : go()
    },
    retry: options.retry,
    conflict: options.conflict,
  })
  /** Cached per error, so a subscriber is not re-rendered by its own read. */
  const failures = new WeakMap<object, AppFailure>()

  return {
    queue(projectId: string, draft: TDraft, sender: TSender): void {
      senders.set(projectId, sender)
      queue.queue(projectId, draft)
    },
    /** Resolves once the queue is dry, or rejects with the failure. */
    flush(projectId: string, failFast = false): Promise<void> {
      return queue.flush(projectId, failFast).then(() => undefined)
    },
    peek: (projectId: string): TDraft | undefined => queue.peek(projectId),
    state(projectId: string): SaveState {
      const state = queue.state(projectId)
      return state === 'conflict' ? options.conflictAs : state
    },
    failure(projectId: string): AppFailure | undefined {
      const error = queue.failure(projectId)
      const state = queue.state(projectId)
      if (!error || typeof error !== 'object' || (state !== 'refused' && state !== 'error'))
        return undefined
      const known = failures.get(error)
      if (known) return known
      const failure = appFailureFromConnect(error)
      failures.set(error, failure)
      return failure
    },
    subscribe: (projectId: string, listener: () => void): (() => void) =>
      queue.subscribe(projectId, listener),
    discard(projectId: string): void {
      queue.discard(projectId)
      senders.delete(projectId)
    },
    discardAll(): void {
      queue.discardAll()
      senders.clear()
    },
  }
}
