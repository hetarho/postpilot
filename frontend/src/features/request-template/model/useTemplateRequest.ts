import { useCallback, useEffect, useRef, useState } from 'react'
import { isTerminal, useJob, type GenerationJob } from '@/entities/generation-job'
import {
  useCancelTemplateRequest,
  useStartTemplateRequest,
  useTemplateRequestResult,
  type TemplateDraftTexts,
  type TemplateRequestInput,
} from '@/entities/template'
import type { AppFailure } from '@/shared/api'

export type TemplateRequestPhase = 'idle' | 'running' | 'done' | 'failed' | 'cancelled'

export interface TemplateRequestState {
  phase: TemplateRequestPhase
  /** True from the press until the job is terminal: the draft is locked meanwhile (TMPL-63). */
  running: boolean
  job: GenerationJob | undefined
  startPending: boolean
  startFailure: AppFailure | undefined
  /** The separated wishes of the last applied answer (TMPL-61); empty after an undo. */
  wishes: string[]
  /** Whether 요청 전으로 되돌리기 has a draft to restore. */
  canUndo: boolean
  send: (input: TemplateRequestInput) => Promise<boolean>
  cancel: () => void
  undo: () => void
}

/** One template request's life on the editor (TMPL-58, TMPL-63): the press, the job it polls,
 *  the answer it applies to the page's draft, and the one-step undo.
 *
 *  The page owns the draft, so the answer and the undo both go through `apply`; this hook owns
 *  only what the request itself has to remember — the job, the snapshot taken at the press, and
 *  the wishes of the answer it applied. */
export function useTemplateRequest(
  apply: (next: TemplateDraftTexts) => void,
): TemplateRequestState {
  const [jobId, setJobId] = useState('')
  const [snapshot, setSnapshot] = useState<TemplateDraftTexts | null>(null)
  // The job whose answer was undone: its wishes and its undo are spent.
  const [undoneJob, setUndoneJob] = useState('')
  // Which job's answer reached the draft, read only inside the effect that applies it.
  const appliedJob = useRef('')
  const start = useStartTemplateRequest()
  const cancelRequest = useCancelTemplateRequest()
  const { job } = useJob(jobId)
  const done = job?.id === jobId && job.status === 'done'
  const { result } = useTemplateRequestResult(jobId, done)

  // The answer replaces the draft at once, once per job (TMPL-63).
  useEffect(() => {
    if (!result || appliedJob.current === jobId) return
    appliedJob.current = jobId
    apply(result.draft)
  }, [apply, jobId, result])

  const terminal = job?.id === jobId && isTerminal(job)
  const running = jobId !== '' && !terminal
  const phase: TemplateRequestPhase =
    jobId === ''
      ? 'idle'
      : running
        ? 'running'
        : job?.status === 'done'
          ? 'done'
          : job?.status === 'cancelled'
            ? 'cancelled'
            : 'failed'

  const send = useCallback(
    async (input: TemplateRequestInput) => {
      start.reset()
      try {
        const id = await start.start(input)
        // The undo restores the draft as it stood at THIS press (TMPL-63).
        setSnapshot(input.draft)
        setJobId(id)
        return true
      } catch {
        // The refusal renders in place; the box keeps its text.
        return false
      }
    },
    [start],
  )

  const cancel = useCallback(() => {
    if (!jobId || !running) return
    // Fire and forget: leaving the screen must not wait on it, and the queue settles a request
    // that keeps running to its confirmed usage only (QUOTA-49).
    void cancelRequest.cancel(jobId).catch(() => {})
  }, [cancelRequest, jobId, running])

  const undo = useCallback(() => {
    if (!snapshot) return
    apply(snapshot)
    setSnapshot(null)
    setUndoneJob(jobId)
  }, [apply, jobId, snapshot])

  // An answer that has arrived is the one the effect applies; it is spent once undone.
  const answered = phase === 'done' && result !== undefined && undoneJob !== jobId

  return {
    phase,
    running,
    job: job?.id === jobId ? job : undefined,
    startPending: start.isPending,
    startFailure: start.failure,
    wishes: answered ? result.wishes : [],
    canUndo: answered && snapshot !== null,
    send,
    cancel,
    undo,
  }
}
