import { useMemo } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { retryablePublishJobsQueryKey } from '@/entities/publish-job'
import { publishingClientFor } from '@/shared/api'
import { Button } from '@/shared/ui'

interface RetryPublishJobButtonProps {
  ownerId: string
  jobId: string
}

export function RetryPublishJobButton({ ownerId, jobId }: RetryPublishJobButtonProps) {
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const retry = useMutation({
    mutationFn: async () => client.retryPublish({ jobId }),
    onSuccess: () =>
      queryClient.invalidateQueries({
        queryKey: retryablePublishJobsQueryKey(transport, ownerId),
      }),
  })

  return (
    <div>
      <Button variant="secondary" disabled={retry.isPending} onClick={() => retry.mutate()}>
        {retry.isPending ? '다시 요청하는 중…' : '로그인 복구 후 다시 시도'}
      </Button>
      {retry.isError && (
        <p className="text-notice-danger-fg mt-2 text-sm" role="alert">
          같은 발행 작업을 다시 시작하지 못했어요. Mac 연결과 카테고리를 확인해 주세요.
        </p>
      )}
    </div>
  )
}
