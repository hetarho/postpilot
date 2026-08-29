import { useState, type ReactNode } from 'react'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { voiceComparisonQueryKey, voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice-profile'
import { VoiceValidationService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { ActionBar, Button, SegmentedControl } from '@/shared/ui'
import { TextCandidateComparison } from '@/widgets/candidate-comparison'

export function VoiceRuleComparisonPage() {
  const { id } = useParams({ from: '/authenticated/voice/rules/$id/compare' })
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const transport = useTransport()
  const queryClient = useQueryClient()
  const key = voiceComparisonQueryKey(transport, ownerId, id)
  const query = useQuery({ queryKey: key, queryFn: () => createClient(VoiceValidationService, transport).getVoiceRuleComparison({ comparisonId: id }), refetchInterval: (state) => ['queued', 'running'].includes(state.state.data?.comparison?.status ?? '') ? POLL_INTERVAL_MS : false })
  const jobState = useJob(query.data?.comparison?.jobId ?? '', [key])
  const decide = useMutation(VoiceValidationService.method.decideVoiceRuleComparison)
  const retry = useMutation(VoiceValidationService.method.retryVoiceRuleComparison)
  const [active, setActive] = useState('')
  if (query.isPending) return <Placeholder>비교 결과를 불러오는 중…</Placeholder>
  const comparison = query.data?.comparison
  if (!comparison || query.isError) return <Placeholder>비교 결과를 불러오지 못했어요. <Button variant="ghost" onClick={() => void query.refetch()}>다시 시도</Button></Placeholder>
  const candidates = comparison.candidates.map((candidate, index) => ({ id: candidate.id, label: candidate.side || (index === 0 ? 'A' : 'B'), text: candidate.output, status: candidate.status, error: candidate.error }))
  const activeId = candidates.some((candidate) => candidate.id === active) ? active : (candidates[0]?.id ?? '')
  const ready = comparison.status === 'review' && candidates.every((candidate) => candidate.status === 'succeeded')
  const choose = async (candidateId: string) => {
    const side = comparison.candidates.find((candidate) => candidate.id === candidateId)?.side
    if (!side) return
    await decide.mutateAsync({ comparisonId: id, chosenSide: side })
    await queryClient.invalidateQueries({ queryKey: key })
    await queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) })
    await queryClient.invalidateQueries({ queryKey: voiceVersionsQueryKey(transport, ownerId) })
  }
  const canRetry = comparison.status === 'partial' || comparison.status === 'failed' || jobState.job?.status === 'failed'
  return <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6"><Link to="/voice" className="text-link-fg inline-flex min-h-11 items-center text-sm">← 말투</Link><h1 className="mt-4 text-2xl font-semibold">규칙 블라인드 비교</h1><p className="text-content-secondary mt-2 text-sm">두 결과는 같은 입력으로 만들었고 선택 전에는 규칙을 적용한 쪽을 숨깁니다.</p>{canRetry && <Button variant="secondary" className="mt-4" pending={retry.isPending} onClick={() => void retry.mutateAsync({ comparisonId: id }).then(() => query.refetch())}>실패한 후보 다시 만들기</Button>}<div className="mt-6"><TextCandidateComparison candidates={candidates} activeCandidateId={activeId} /></div>{activeId && <ActionBar ariaLabel="후보 전환과 선택"><div className="grid gap-3"><SegmentedControl value={activeId} options={candidates.map((candidate) => ({ value: candidate.id, label: candidate.label }))} onChange={setActive} ariaLabel="선택할 후보" />{comparison.chosenSide ? <p className="text-sm">선택을 반영했어요.</p> : <Button variant="cta" disabled={!ready} pending={decide.isPending} onClick={() => void choose(activeId)}>이 글이 더 나아요</Button>}</div></ActionBar>}</main>
}
function Placeholder({ children }: { children: ReactNode }) { return <main className="text-content-tertiary mx-auto w-full max-w-2xl px-4 py-10 text-sm sm:px-6">{children}</main> }
