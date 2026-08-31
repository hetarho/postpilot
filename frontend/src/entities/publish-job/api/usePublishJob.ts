import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { appFailureFromConnect, publishingClientFor } from '@/shared/api'
import { PUBLISH_JOB_POLL_MS } from '@/shared/config'
import { TERMINAL_PUBLISH_STATUSES, toPublishJob } from '../model/types'

export const publishJobQueryKey = (ownerId: string, postSlug: string) =>
  ['publish-job', ownerId, postSlug] as const

export function usePublishJob(ownerId: string, postSlug: string) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const query = useQuery({
    queryKey: publishJobQueryKey(ownerId, postSlug),
    queryFn: async () => {
      try {
        const response = await client.getPublishJob({ postSlug })
        return response.job ? toPublishJob(response.job) : null
      } catch (cause) {
        if (appFailureFromConnect(cause).reason === 'PUBLISH_NOT_FOUND') return null
        throw cause
      }
    },
    enabled: Boolean(ownerId && postSlug),
    refetchInterval: (state) => {
      const status = state.state.data?.status
      return status && !TERMINAL_PUBLISH_STATUSES.has(status) ? PUBLISH_JOB_POLL_MS : false
    },
  })
  return {
    job: query.data ?? undefined,
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
