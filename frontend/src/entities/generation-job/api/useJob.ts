import { useEffect, useMemo, useRef } from 'react'
import { useQuery } from '@connectrpc/connect-query'
import { type QueryKey, useQueryClient } from '@tanstack/react-query'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { GenerationService } from '@/shared/api'
import { isTerminal } from '../model/types'
import { toGenerationJob } from './job-mappers'

/** Polls a durable server job and refreshes its owner once it reaches either terminal state. */
export function useJob(jobId: string, invalidateOnTerminal: readonly QueryKey[] = []) {
  const queryClient = useQueryClient()
  const invalidatedJob = useRef<string | undefined>(undefined)
  const query = useQuery(
    GenerationService.method.getGeneration,
    { id: jobId },
    {
      enabled: jobId !== '',
      refetchInterval: (state) => {
        const found = state.state.data?.job
        return found && isTerminal(found) ? false : POLL_INTERVAL_MS
      },
    },
  )
  const job = useMemo(
    () => (query.data?.job ? toGenerationJob(query.data.job) : undefined),
    [query.data],
  )

  useEffect(() => {
    if (!job || !isTerminal(job) || invalidatedJob.current === job.id) return
    invalidatedJob.current = job.id
    for (const queryKey of invalidateOnTerminal) {
      void queryClient.invalidateQueries({ queryKey })
    }
  }, [invalidateOnTerminal, job, queryClient])

  return {
    job,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
