import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey } from '@/entities/publishing-agent'
import { publishingClientFor } from '@/shared/api'
import { Button, Dialog, Notice } from '@/shared/ui'

export function RevokePublishingAgent({
  ownerId,
  agentId,
  label,
}: {
  ownerId: string
  agentId: string
  label: string
}) {
  const [confirming, setConfirming] = useState(false)
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const revoke = useMutation({
    mutationFn: () => client.revokePublishingAgent({ agentId }),
    onSuccess: async () => {
      setConfirming(false)
      await queryClient.invalidateQueries({
        queryKey: publishingAgentsQueryKey(ownerId),
      })
    },
  })
  return (
    <>
      <Button variant="danger" onClick={() => setConfirming(true)}>
        연결 해제
      </Button>
      {revoke.isError && <Notice tone="danger">연결을 해제하지 못했어요.</Notice>}
      <Dialog
        open={confirming}
        title="Mac 연결을 해제할까요?"
        confirmLabel="연결 해제"
        onClose={() => setConfirming(false)}
        onConfirm={() => revoke.mutate()}
        pending={revoke.isPending}
      >
        {label}의 발행 토큰이 즉시 무효화됩니다. 네이버 로그인과 브라우저 프로필은 Mac에서 직접
        지우기 전까지 남아 있습니다.
      </Dialog>
    </>
  )
}
