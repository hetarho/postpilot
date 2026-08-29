import { create } from '@bufbuild/protobuf'
import { useState } from 'react'
import { useNavigate } from '@tanstack/react-router'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { useStageSelection } from '@/entities/model-catalog'
import type { VoiceProfile } from '@/entities/voice-profile'
import { voiceConfirmationsQueryKey, voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice-profile'
import { ModelRefSchema, VoiceLearningService, VoiceRuleStatus, VoiceValidationService } from '@/shared/api'
import { Badge, Button, Dialog, Notice } from '@/shared/ui'

export function VoiceRulesManager({ ownerId, profile, confirmations }: { ownerId: string; profile: VoiceProfile; confirmations: Array<{ id: string; ruleId: string; existingStatement: string; proposedStatement: string; status: string }> }) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const write = useStageSelection('write')
  const statusMutation = useMutation(VoiceLearningService.method.setVoiceRuleStatus)
  const resolveMutation = useMutation(VoiceLearningService.method.resolveRuleConfirmation)
  const compareMutation = useMutation(VoiceValidationService.method.startVoiceRuleComparison)
  const [compareRule, setCompareRule] = useState<string>()
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) })
    void queryClient.invalidateQueries({ queryKey: voiceVersionsQueryKey(transport, ownerId) })
    void queryClient.invalidateQueries({ queryKey: voiceConfirmationsQueryKey(transport, ownerId) })
  }
  const changeStatus = (ruleId: string, status: VoiceRuleStatus) => void statusMutation.mutateAsync({ ruleId, status }).then(refresh)
  const startComparison = async () => {
    const source = profile.structured.sources[0]
    if (!compareRule || !source || !write.selected) return
    const response = await compareMutation.mutateAsync({ ruleId: compareRule, sourceId: source.id, writeModel: create(ModelRefSchema, write.selected) })
    setCompareRule(undefined)
    if (response.comparisonId) void navigate({ to: '/voice/rules/$id/compare', params: { id: response.comparisonId } })
  }
  return <section aria-labelledby="rules-heading" className="mt-10"><h2 id="rules-heading" className="text-lg font-semibold tracking-tight">대조 규칙</h2><p className="text-content-secondary mt-2 text-sm">후보 규칙은 생성에 쓰이지 않습니다. 서로 다른 글에서 근거가 3번 모인 활성 규칙만 적용됩니다.</p>{profile.structured.rules.length === 0 ? <p className="text-content-tertiary mt-4 text-sm">아직 편집 패턴에서 찾은 규칙이 없어요.</p> : <ul className="divide-divider mt-4 divide-y">{profile.structured.rules.map((rule) => <li key={rule.id} className="py-4"><div className="flex flex-wrap items-start justify-between gap-2"><p className="min-w-0 text-sm leading-relaxed">{rule.statement}</p><Badge tone={rule.status === 'active' ? 'success' : rule.status === 'candidate' ? 'warning' : 'neutral'}>{statusLabel(rule.status)} · 근거 {rule.evidenceCount}</Badge></div><p className="text-content-tertiary mt-1 text-xs">{rule.layer}</p><div className="mt-3 flex flex-wrap gap-2">{rule.status !== 'active' && <Button variant="secondary" onClick={() => changeStatus(rule.id, VoiceRuleStatus.ACTIVE)}>활성화</Button>}{rule.status !== 'retired' && <Button variant="ghost" onClick={() => changeStatus(rule.id, VoiceRuleStatus.RETIRED)}>사용 중지</Button>}{rule.status === 'candidate' && <Button variant="ghost" disabled={!profile.structured.sources[0] || !write.selected} onClick={() => setCompareRule(rule.id)}>블라인드 비교</Button>}</div></li>)}</ul>}
    {confirmations.filter((item) => item.status === 'pending').length > 0 && <section className="mt-8"><h3 className="font-medium">확인이 필요한 충돌</h3>{confirmations.filter((item) => item.status === 'pending').map((item) => <Notice key={item.id} tone="warning" className="mt-3"><p>현재: {item.existingStatement}</p><p className="mt-1">새 근거: {item.proposedStatement}</p><div className="mt-3 flex gap-2"><Button variant="secondary" onClick={() => void resolveMutation.mutateAsync({ confirmationId: item.id, replace: false }).then(refresh)}>현재 규칙 유지</Button><Button variant="cta" onClick={() => void resolveMutation.mutateAsync({ confirmationId: item.id, replace: true }).then(refresh)}>새 규칙으로 교체</Button></div></Notice>)}</section>}
    <Dialog open={compareRule !== undefined} title="이 규칙만 비교할까요?" confirmLabel="비교 작업 시작" pending={compareMutation.isPending} onClose={() => setCompareRule(undefined)} onConfirm={() => void startComparison()}>같은 입력과 같은 작성 모델로 두 글을 만들고, 선택한 규칙의 포함 여부만 다르게 합니다. 판정 전에는 어느 쪽에 규칙이 들어갔는지 숨깁니다.</Dialog>
  </section>
}
const statusLabel = (status: string) => ({ candidate: '후보', active: '활성', retired: '중지', rejected: '거절' } as Record<string, string>)[status] ?? status
