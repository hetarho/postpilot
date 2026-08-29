import { useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import type { VoiceProfile, VoiceValue, VoiceVersion } from '@/entities/voice-profile'
import { voiceProfileQueryKey, voiceVersionsQueryKey } from '@/entities/voice-profile'
import { VoiceLayer, VoiceService } from '@/shared/api'
import { Badge, Button, Dialog, FieldLabel, TextField } from '@/shared/ui'

export function StructuredProfileEditor({ ownerId, profile, versions }: { ownerId: string; profile: VoiceProfile; versions: VoiceVersion[] }) {
  const transport = useTransport()
  const queryClient = useQueryClient()
  const override = useMutation(VoiceService.method.updateVoiceOverride)
  const restore = useMutation(VoiceService.method.restoreVoiceProfile)
  const [restoreVersion, setRestoreVersion] = useState<bigint>()
  const refresh = () => {
    void queryClient.invalidateQueries({ queryKey: voiceProfileQueryKey(transport, ownerId) })
    void queryClient.invalidateQueries({ queryKey: voiceVersionsQueryKey(transport, ownerId) })
  }
  const fields = [
    { label: '어휘 성격', layer: VoiceLayer.LEXICAL, field: 'description', value: profile.structured.lexical.description },
    { label: '주 종결어미', layer: VoiceLayer.ENDINGS, field: 'base_register', value: profile.structured.endings.baseRegister },
    { label: '접속 방식', layer: VoiceLayer.SYNTAX, field: 'connective_style', value: profile.structured.syntax.connectiveStyle },
    { label: '도입 방식', layer: VoiceLayer.STRUCTURE, field: 'intro_pattern', value: profile.structured.structure.introPattern },
    { label: '마무리 방식', layer: VoiceLayer.STRUCTURE, field: 'closing_pattern', value: profile.structured.structure.closingPattern },
  ]
  return (
    <section aria-labelledby="profile-heading" className="mt-10">
      <div className="flex flex-wrap items-baseline justify-between gap-2">
        <h2 id="profile-heading" className="text-lg font-semibold tracking-tight">현재 말투 프로필</h2>
        <span className="text-content-tertiary text-sm">v{profile.structured.version.toString()} · 완성 글 {profile.finalizedSourceCount}편</span>
      </div>
      {profile.structured.empty ? (
        <p className="text-content-secondary mt-3 text-sm leading-relaxed">아직 배운 말투가 없어요. 첫 글도 그대로 생성할 수 있고, 완성한 글을 직접 확정하면 그때부터 한 편씩 배웁니다.</p>
      ) : (
        <div className="mt-5 space-y-6">
          <div className="grid gap-4 sm:grid-cols-2">
            {fields.map((item) => <EditableValue key={`${item.layer}-${item.field}`} {...item} pending={override.isPending} onSave={(value) => override.mutateAsync({ layer: item.layer, field: item.field, value }).then(refresh)} onClear={() => override.mutateAsync({ layer: item.layer, field: item.field }).then(refresh)} />)}
          </div>
          <section>
            <h3 className="font-medium">종결어미 분포</h3>
            <div className="mt-2 flex flex-wrap gap-2">{profile.structured.endings.distribution.map((item) => <Badge key={item.ending}>{item.ending} {Math.round(item.ratio * 100)}%</Badge>)}</div>
          </section>
          <section>
            <h3 className="font-medium">문장과 구조</h3>
            <dl className="text-content-secondary mt-2 grid gap-2 text-sm sm:grid-cols-2">
              <div><dt className="text-content-tertiary">평균 문장 길이</dt><dd>{profile.structured.syntax.averageSentenceChars.toFixed(1)}자</dd></div>
              <div><dt className="text-content-tertiary">문단당 문장</dt><dd>{profile.structured.structure.paragraphSentencesMin}–{profile.structured.structure.paragraphSentencesMax}개</dd></div>
            </dl>
          </section>
          <section>
            <h3 className="font-medium">여섯 성향 (-3~3)</h3>
            <dl className="text-content-secondary mt-2 grid grid-cols-2 gap-2 text-sm sm:grid-cols-3">{Object.entries(profile.structured.axes).map(([key, value]) => <div key={key}><dt className="text-content-tertiary">{axisLabel(key)}</dt><dd>{value}</dd></div>)}</dl>
          </section>
          {(profile.structured.lexical.bannedWords.length > 0 || profile.structured.lexical.bannedPatterns.length > 0 || profile.structured.endings.bannedEndings.length > 0) && <section><h3 className="font-medium">피할 표현</h3><ul className="text-content-secondary mt-2 list-disc pl-5 text-sm">{profile.structured.lexical.bannedWords.map((v) => <li key={v.value}>{v.value}{v.reason ? ` — ${v.reason}` : ''}</li>)}{profile.structured.lexical.bannedPatterns.map((v) => <li key={v.value}>{v.value}{v.reason ? ` — ${v.reason}` : ''}</li>)}{profile.structured.endings.bannedEndings.map((v) => <li key={v}>{v}</li>)}</ul></section>}
        </div>
      )}
      {versions.length > 0 && <section className="mt-8"><h3 className="font-medium">버전 기록</h3><ul className="divide-divider mt-2 divide-y">{versions.map((version) => <li key={version.version.toString()} className="flex min-h-14 items-center justify-between gap-3 py-2"><span className="text-sm">v{version.version.toString()} · {originLabel(version.origin)}{version.restoredFromVersion > 0n ? ` (v${version.restoredFromVersion.toString()}에서 복원)` : ''}</span><Button variant="ghost" disabled={version.version === profile.structured.version} onClick={() => setRestoreVersion(version.version)}>복원</Button></li>)}</ul></section>}
      <Dialog open={restoreVersion !== undefined} title="이 버전으로 복원할까요?" confirmLabel="새 버전으로 복원" pending={restore.isPending} onClose={() => setRestoreVersion(undefined)} onConfirm={() => restoreVersion !== undefined && void restore.mutateAsync({ version: restoreVersion }).then(() => { setRestoreVersion(undefined); refresh() })}>기존 기록은 지우지 않고, 선택한 스냅샷을 새 현재 버전으로 만듭니다.</Dialog>
    </section>
  )
}

function EditableValue({ label, value, pending, onSave, onClear }: { label: string; value: VoiceValue; pending: boolean; onSave: (value: string) => Promise<unknown>; onClear: () => Promise<unknown> }) {
  const [draft, setDraft] = useState(value.unknown ? '' : value.value)
  return <div><div className="flex items-center gap-2"><FieldLabel>{label}</FieldLabel><Badge tone={value.source === 'manual' ? 'info' : 'neutral'}>{value.unknown ? '알 수 없음' : sourceLabel(value.source)}</Badge></div><TextField aria-label={`${label} 직접 설정`} value={draft} placeholder="알 수 없음" onChange={(event) => setDraft(event.target.value)} className="mt-1" /><div className="mt-2 flex gap-2"><Button variant="secondary" disabled={!draft.trim()} pending={pending} onClick={() => void onSave(draft.trim())}>적용</Button>{value.source === 'manual' && <Button variant="ghost" disabled={pending} onClick={() => void onClear()}>직접 설정 해제</Button>}</div></div>
}
const sourceLabel = (source: VoiceValue['source']) => ({ measured: '측정', analyzed: '분석', manual: '직접 설정', unknown: '알 수 없음' })[source]
const originLabel = (origin: string) => ({ analysis: '분석', manual: '직접 수정', restore: '복원', rule: '규칙 반영', confirmation: '충돌 해결' })[origin] ?? origin
const axisLabel = (key: string) => ({ involvement: '관여도', narrativity: '서사성', persuasionOvertness: '설득 노출', abstractness: '추상성', addresseeFocus: '독자 지향', humor: '유머' } as Record<string, string>)[key] ?? key
