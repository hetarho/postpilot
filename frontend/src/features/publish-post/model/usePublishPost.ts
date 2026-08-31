import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishJobQueryKey, toPublishJob } from '@/entities/publish-job'
import {
  appFailureFromConnect,
  publishingClientFor,
  type AppFailure,
  type PublishVisibility,
} from '@/shared/api'

export class PublishStartError extends Error {
  constructor(readonly failure: AppFailure) {
    super('publish start failed')
    this.name = 'PublishStartError'
  }
}

export function usePublishPost(ownerId: string, postSlug: string) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const key = publishJobQueryKey(ownerId, postSlug)
  const start = useMutation({
    mutationFn: async (input: {
      expectedContentRevision: bigint
      agentId: string
      categoryId: string
      visibility: PublishVisibility
    }) => {
      try {
        const response = await client.startPublish({ postSlug, ...input })
        if (!response.job) throw new PublishStartError(appFailureFromConnect(undefined))
        return toPublishJob(response.job)
      } catch (cause) {
        if (cause instanceof PublishStartError) throw cause
        throw new PublishStartError(appFailureFromConnect(cause))
      }
    },
    onSuccess: (job) => queryClient.setQueryData(key, job),
  })
  const cancel = useMutation({
    mutationFn: async (jobId: string) => {
      const response = await client.cancelPublish({ jobId })
      if (!response.job) throw new Error('CancelPublish returned no job')
      return toPublishJob(response.job)
    },
    onSuccess: (job) => queryClient.setQueryData(key, job),
  })
  return {
    start,
    cancel,
    startFailure: start.error
      ? start.error instanceof PublishStartError
        ? start.error.failure
        : appFailureFromConnect(start.error)
      : undefined,
    cancelFailure: cancel.error ? appFailureFromConnect(cancel.error) : undefined,
  }
}
