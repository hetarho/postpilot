import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import {
  appFailureFromConnect,
  publishingClientFor,
  type AppFailure,
  type PublishVisibility,
} from '@/shared/api'
import { toPublishJob } from '../model/types'
import { publishJobQueryKey } from './usePublishJob'
import { retryablePublishJobsQueryKey } from './useRetryablePublishJobs'

/** A publish is a durable server job ([I1], [I5]): the screens start, cancel and retry it, and the
 *  job entity owns the calls and the cache entries they write (ARCH-14, ARCH-17). */

export class PublishStartError extends Error {
  constructor(readonly failure: AppFailure) {
    super('publish start failed')
    this.name = 'PublishStartError'
  }
}

export function usePublishPost(ownerId: string, postSlug: string) {
  const client = usePublishingClient()
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

/** Cancelling a job that outlived its post's screen: the retained list is the only reader. */
export function useCancelRetainedPublishJob(ownerId: string, jobId: string) {
  const client = usePublishingClient()
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => client.cancelPublish({ jobId }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: retryablePublishJobsQueryKey(ownerId) }),
  })
  return { ...mutation, failure: failureOf(mutation.error) }
}

export function useRetryPublishJob(ownerId: string, jobId: string) {
  const client = usePublishingClient()
  const queryClient = useQueryClient()
  const mutation = useMutation({
    mutationFn: () => client.retryPublish({ jobId }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: retryablePublishJobsQueryKey(ownerId) }),
  })
  return { ...mutation, failure: failureOf(mutation.error) }
}

function usePublishingClient() {
  const transport = useTransport()
  return useMemo(() => publishingClientFor(transport), [transport])
}

function failureOf(error: Error | null) {
  return error ? appFailureFromConnect(error) : undefined
}
