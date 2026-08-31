import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { retryablePublishJobsQueryKey } from '@/entities/publish-job'
import { publishingClientFor } from '@/shared/api'
import { Button, Dialog, Notice } from '@/shared/ui'

interface CancelRetainedPublishJobButtonProps {
  ownerId: string
  jobId: string
  postSlug: string
}

export function CancelRetainedPublishJobButton({
  ownerId,
  jobId,
  postSlug,
}: CancelRetainedPublishJobButtonProps) {
  const [confirming, setConfirming] = useState(false)
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const cancel = useMutation({
    mutationFn: () => client.cancelPublish({ jobId }),
    onSuccess: async () => {
      setConfirming(false)
      await queryClient.invalidateQueries({
        queryKey: retryablePublishJobsQueryKey(ownerId),
      })
    },
  })

  return (
    <>
      <Button variant="danger" onClick={() => setConfirming(true)}>
        복구 작업 취소
      </Button>
      {cancel.isError && <Notice tone="danger">복구 작업을 취소하지 못했어요.</Notice>}
      <Dialog
        open={confirming}
        title="고정해 둔 발행 작업을 취소할까요?"
        confirmLabel="작업 취소"
        onClose={() => setConfirming(false)}
        onConfirm={() => cancel.mutate()}
        pending={cancel.isPending}
      >
        {postSlug}의 고정된 글과 임시 사진을 삭제합니다. 이 작업은 다시 시도할 수 없습니다.
      </Dialog>
    </>
  )
}
