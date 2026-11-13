import { useEffect, useRef } from 'react'
import type { GenerationJob } from '@/entities/generation-job'
import type { ClipStep } from './steps'

/** The step a finished run carries the owner to (CLIP-234): ② after analysis, ③ after a
 *  storyline, ④ after a plan; any other run leaves the step where it is. */
export function stepAfterJob(kind: string): ClipStep | undefined {
  if (kind === 'analyze_clip') return 'observe'
  if (kind === 'storyline_clip') return 'storyline'
  if (kind === 'generate_clip') return 'refine'
  return undefined
}

const TERMINAL = ['done', 'failed', 'cancelled'] as const
type TerminalStatus = (typeof TERMINAL)[number]

/** An upload attempt is opened when a run starts and must be closed by the run's OUTCOME, not by
 *  the panel that started it: the owner may be on another step, or have reloaded the page, when
 *  the job ends. The attempt is what holds the picked originals, so a missed close leaks them. */
export function useUploadAttemptLifecycle(
  job: GenerationJob | undefined,
  upload: { finishAttempt: (jobId: string, status: TerminalStatus) => void },
) {
  useEffect(() => {
    if (job && (TERMINAL as readonly string[]).includes(job.status))
      upload.finishAttempt(job.id, job.status as TerminalStatus)
  }, [job, upload])
}

/** A correction that switched original sound hands the server's answer back to the upload
 *  session, so the strip shows the batch the plan was saved against rather than the one the
 *  picker last built. */
export function useSoundBatchHandoff<T>(
  soundBatch: T | undefined,
  accept: (batch: T) => void,
): void {
  useEffect(() => {
    if (soundBatch) accept(soundBatch)
  }, [soundBatch, accept])
}

/** The run view takes the whole screen, so it takes the focus with it — a screen reader is
 *  otherwise left on a control that is no longer rendered. */
export function useRunFocus(focused: boolean) {
  const root = useRef<HTMLElement>(null)
  useEffect(() => {
    if (focused) root.current?.focus()
  }, [focused])
  return root
}

/** A save queue outlives the form that filled it: once the project is finalized nothing may be
 *  saved against it, so the queue is dropped rather than left retrying a refused write. */
export function useDiscardQueueWhenFinalized(
  projectId: string,
  finalized: unknown,
  discard: (projectId: string) => void,
) {
  useEffect(() => {
    if (finalized) discard(projectId)
  }, [finalized, projectId, discard])
}
