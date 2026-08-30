import type { ReactNode } from 'react'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { voiceValidationQueryKey } from '@/entities/voice'
import { VoiceValidationService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { Badge, Button, Notice } from '@/shared/ui'

export function VoiceValidationPage() {
  const { voiceId, id } = useParams({ from: '/authenticated/voices/$voiceId/validations/$id' })
  const { user } = useSession()
  const transport = useTransport()
  const key = voiceValidationQueryKey(transport, user?.id ?? '', voiceId, id)
  const query = useQuery({
    queryKey: key,
    queryFn: () =>
      createClient(VoiceValidationService, transport).getVoiceProfileValidation({
        validationId: id,
      }),
    refetchInterval: (state) =>
      ['queued', 'running'].includes(state.state.data?.validation?.status ?? '')
        ? POLL_INTERVAL_MS
        : false,
  })
  const jobState = useJob(query.data?.validation?.jobId ?? '', [key])
  const retry = useMutation(VoiceValidationService.method.retryVoiceProfileValidation)
  if (query.isPending) return <Placeholder>검증 결과를 불러오는 중…</Placeholder>
  const validation = query.data?.validation
  if (!validation || query.isError)
    return (
      <Placeholder>
        검증 결과를 불러오지 못했어요.{' '}
        <Button variant="ghost" onClick={() => void query.refetch()}>
          다시 시도
        </Button>
      </Placeholder>
    )
  if (validation.voiceId !== voiceId) {
    return (
      <Placeholder>
        <p role="alert">이 검증은 다른 말투의 기록이에요.</p>
      </Placeholder>
    )
  }
  const canRetry = validation.status === 'partial' || validation.status === 'failed' || jobState.job?.status === 'failed'
  return <main className="mx-auto w-full max-w-4xl px-4 py-6 sm:px-6"><Link to="/voices/$voiceId/validations" params={{ voiceId }} className="text-link-fg inline-flex min-h-11 items-center text-sm">← 프로필 검증</Link><div className="mt-4 flex flex-wrap items-baseline justify-between gap-2"><h1 className="text-2xl font-semibold">프로필 검증 v{validation.profileVersion.toString()}</h1><Badge tone={validation.status === 'done' ? 'success' : validation.status === 'failed' ? 'danger' : 'neutral'}>{validation.status}</Badge></div>{canRetry && <Button variant="secondary" className="mt-4" pending={retry.isPending} onClick={() => void retry.mutateAsync({ validationId: id }).then(() => query.refetch())}>실패한 항목 다시 실행</Button>}{validation.judgeEnabled && validation.totalCount > 0 && <Notice tone="info" className="mt-4">5개 항목 일치율: {Math.round(validation.yCount / validation.totalCount * 100)}% ({validation.yCount}/{validation.totalCount})</Notice>}<div className="mt-6 space-y-8">{validation.items.map((item, index) => <article key={item.id}><h2 className="font-semibold">검증 글 {index + 1}</h2>{item.error && <Notice tone="danger" className="mt-2">{item.error}</Notice>}<div className="mt-3 grid gap-4 md:grid-cols-2"><section className="bg-surface-raised rounded-lg p-4"><h3 className="text-sm font-medium">원문</h3><p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">{item.original}</p></section><section className="bg-surface-raised rounded-lg p-4"><h3 className="text-sm font-medium">중립 주제 요약</h3><p className="text-content-secondary mt-2 text-sm">{item.neutralSummary || '생성 중…'}</p><h3 className="mt-4 text-sm font-medium">프로필로 다시 쓴 글</h3><p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">{item.regenerated || '생성 중…'}</p>{item.scores.length > 0 && <div className="mt-3 flex flex-wrap gap-2">{item.scores.map((score) => <Badge key={score.dimension} tone={score.matched ? 'success' : 'warning'}>{score.dimension} {score.matched ? 'Y' : 'N'}</Badge>)}</div>}</section></div></article>)}</div></main>
}
function Placeholder({ children }: { children: ReactNode }) { return <main className="text-content-tertiary mx-auto w-full max-w-2xl px-4 py-10 text-sm sm:px-6">{children}</main> }
