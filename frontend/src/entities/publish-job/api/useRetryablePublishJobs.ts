import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { publishingClientFor } from '@/shared/api'
import { toPublishJob } from '../model/types'

export const retryablePublishJobsQueryKey = (ownerId: string) =>
  ['retryable-publish-jobs', ownerId] as const

export function useRetryablePublishJobs(ownerId: string) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const query = useQuery({
    queryKey: retryablePublishJobsQueryKey(ownerId),
    queryFn: async () => {
      const response = await client.listRetryablePublishJobs({})
      return response.jobs.map(toPublishJob)
    },
    enabled: Boolean(ownerId),
  })
  return {
    jobs: query.data ?? [],
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
