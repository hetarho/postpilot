import { useMemo, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { publishingAgentsQueryKey } from '@/entities/publishing-agent'
import { publishingClientFor } from '@/shared/api'
import { Button, FieldLabel, Notice, TextField } from '@/shared/ui'

export function PairPublishingAgent({ ownerId }: { ownerId: string }) {
  const [label, setLabel] = useState('내 Mac')
  const transport = useTransport()
  const client = useMemo(() => publishingClientFor(transport), [transport])
  const queryClient = useQueryClient()
  const pairing = useMutation({
    mutationFn: () => client.createAgentPairing({ label: label.trim() }),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: publishingAgentsQueryKey(transport, ownerId) }),
  })

  return (
    <section aria-labelledby="pair-agent-heading">
      <h2 id="pair-agent-heading" className="text-lg font-semibold tracking-tight">
        Mac 연결
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        네이버 로그인은 Mac의 전용 브라우저에만 남습니다. 아래 코드를 Mac 설정 화면에 입력하세요.
      </p>
      <div className="mt-4">
        <FieldLabel htmlFor="publishing-agent-label">연결 이름</FieldLabel>
        <TextField
          id="publishing-agent-label"
          value={label}
          onChange={(event) => setLabel(event.target.value)}
          type="text"
          autoComplete="off"
          autoCapitalize="off"
          autoCorrect="off"
          enterKeyHint="done"
          className="mt-2"
        />
      </div>
      <Button
        variant="cta"
        className="mt-4 w-full sm:w-auto"
        onClick={() => pairing.mutate()}
        pending={pairing.isPending}
        disabled={!label.trim()}
      >
        연결 코드 만들기
      </Button>
      <div role="status" aria-live="polite" className="mt-4 empty:hidden">
        {pairing.data && (
          <Notice tone="success">
            <span>
              연결 코드 <strong className="font-mono text-base">{pairing.data.deviceCode}</strong>
              <br />
              {new Date(pairing.data.expiresAt).toLocaleString('ko-KR')}까지 한 번만 사용할 수
              있어요.
            </span>
          </Notice>
        )}
        {pairing.isError && <Notice tone="danger">연결 코드를 만들지 못했어요.</Notice>}
      </div>
    </section>
  )
}
