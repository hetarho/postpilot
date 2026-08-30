import { useMemo } from 'react'
import { Code, ConnectError } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishJobQueryKey, toPublishJob } from '@/entities/publish-job'
import { publishingClientFor, type PublishVisibility } from '@/shared/api'

export class PublishStartError extends Error {}

function startMessage(cause: unknown): string {
  switch (ConnectError.from(cause).code) {
    case Code.Aborted:
      return '다른 화면에서 글이 바뀌었어요. 새로고침한 뒤 다시 확인해 주세요.'
    case Code.AlreadyExists:
      return '이미 진행 중이거나 발행을 마친 글이에요.'
    case Code.FailedPrecondition:
      return '글 확정, Mac 연결, 카테고리 상태를 확인해 주세요.'
    case Code.PermissionDenied:
      return '이 Mac 연결로는 발행할 수 없어요.'
    default:
      return '발행 요청을 저장하지 못했어요. 다시 시도해 주세요.'
  }
}

export function usePublishPost(ownerId: string, postSlug: string) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const key = publishJobQueryKey(transport, ownerId, postSlug)
  const start = useMutation({
    mutationFn: async (input: {
      expectedContentRevision: bigint
      agentId: string
      categoryId: string
      visibility: PublishVisibility
    }) => {
      try {
        const response = await client.startPublish({ postSlug, ...input })
        if (!response.job) throw new Error('StartPublish returned no job')
        return toPublishJob(response.job)
      } catch (cause) {
        if (cause instanceof Error && cause.message === 'StartPublish returned no job') throw cause
        throw new PublishStartError(startMessage(cause))
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
  return { start, cancel }
}
