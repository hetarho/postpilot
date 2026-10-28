import type { HeadingSuggestionInput } from '../model/heading-suggestions'
import { useEffect } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { isTerminal, useJob } from '@/entities/generation-job/@x/template'
import { useHeadingSuggestionsResult, useStartHeadingSuggestions } from './useHeadingSuggestions'
import { appFailureFromConnect } from '@/shared/api'
import { headingSuggestionsMachine } from '../model/heading-suggestions-machine'

export interface HeadingSectionInput {
  id: string
  role: string
  description: string
}
export function useWizardHeadingSuggestions(
  ownerId: string,
  targetKind: HeadingSuggestionInput['targetKind'],
  targetPath: readonly string[],
  sections: readonly HeadingSectionInput[],
  maxChars?: number,
) {
  const startRequest = useStartHeadingSuggestions()
  const actor = useActorRef(headingSuggestionsMachine, {
    input: { ownerId, start: startRequest, maxChars },
  })
  const snapshot = useSelector(actor, (value) => value)
  const signature = JSON.stringify({ targetKind, targetPath, sections })
  const { jobId } = snapshot.context
  const { job } = useJob(jobId)
  const reading = snapshot.matches('reading')
  const result = useHeadingSuggestionsResult(jobId, reading)
  useEffect(() => {
    if (job?.id !== jobId || !isTerminal(job)) return
    actor.send({
      type: 'terminal',
      jobId,
      status: job!.status as 'done' | 'failed' | 'cancelled',
      failure: job!.failure,
    })
  }, [actor, job, jobId])
  useEffect(() => {
    if (!reading || !result.groups) return
    actor.send({ type: 'result', jobId, groups: result.groups })
  }, [actor, jobId, reading, result.groups])
  const current = snapshot.context.signature === signature && snapshot.matches('done')
  return {
    pending: snapshot.matches('starting') || snapshot.matches('running') || reading,
    failure: snapshot.context.failure,
    readFailure: reading && result.error ? appFailureFromConnect(result.error) : undefined,
    retryRead: () => void result.refetch(),
    suggestions: (id: string) =>
      current
        ? (snapshot.context.groups[sections.findIndex((section) => section.id === id)] ?? [])
        : [],
    /** The owner's wording request travels with this one job only (TMPL-73). */
    start: (writeModel: HeadingSuggestionInput['writeModel'], styleRequest = '') =>
      actor.send({
        type: 'start',
        ownerId,
        signature,
        request: {
          writeModel,
          targetKind,
          targetPath,
          sections: sections.map(({ role, description }) => ({ role, description })),
          styleRequest: styleRequest.trim(),
        },
      }),
  }
}
export type WizardHeadingSuggestionsOperation = ReturnType<typeof useWizardHeadingSuggestions>
