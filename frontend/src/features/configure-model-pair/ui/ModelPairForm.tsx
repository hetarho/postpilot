import { useState } from 'react'
import {
  filterForStage,
  refKey,
  type ModelRef,
  type StageName,
  useModels,
  useModelSetup,
  useSaveComparisonPair,
  useSaveSelection,
  useStageSelection,
} from '@/entities/model-catalog'
import { Button, FieldLabel, FieldMessage, Select } from '@/shared/ui'

export function ModelPairForm({ stage }: { stage: StageName }) {
  const { models } = useModels()
  const active = useStageSelection(stage)
  const { pairs } = useModelSetup()
  const saveActive = useSaveSelection()
  const savePair = useSaveComparisonPair()
  const suitable = filterForStage(models, stage)
  const pair = pairs.find((item) => item.stage === stage)
  const initialA = pair?.candidateA ? refKey(pair.candidateA.ref) : ''
  const initialB = pair?.candidateB ? refKey(pair.candidateB.ref) : ''

  return (
    <ModelPairFields
      key={`${stage}:${initialA}:${initialB}`}
      stage={stage}
      initialA={initialA}
      initialB={initialB}
      suitable={suitable}
      active={active}
      saveActive={saveActive}
      savePair={savePair}
    />
  )
}

function ModelPairFields({
  stage,
  initialA,
  initialB,
  suitable,
  active,
  saveActive,
  savePair,
}: {
  stage: StageName
  initialA: string
  initialB: string
  suitable: ReturnType<typeof useModels>['models']
  active: ReturnType<typeof useStageSelection>
  saveActive: ReturnType<typeof useSaveSelection>
  savePair: ReturnType<typeof useSaveComparisonPair>
}) {
  const [a, setA] = useState(initialA)
  const [b, setB] = useState(initialB)
  const find = (key: string): ModelRef | undefined =>
    suitable.find((model) => refKey(model.ref) === key)?.ref
  const invalid = !a || !b || a === b || !find(a) || !find(b)
  return (
    <div className="space-y-4">
      <ModelSelect
        label="활성 모델"
        value={active.selected ? refKey(active.selected) : ''}
        models={suitable}
        onChange={(key) => {
          const ref = find(key)
          if (ref) void saveActive.save(stage, ref)
        }}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <ModelSelect label="후보 A" value={a} models={suitable} onChange={setA} />
        <ModelSelect label="후보 B" value={b} models={suitable} onChange={setB} />
      </div>
      {a && a === b && <FieldMessage>서로 다른 모델을 선택해 주세요.</FieldMessage>}
      <Button
        variant="secondary"
        disabled={invalid || savePair.isPending}
        onClick={() => {
          const left = find(a)
          const right = find(b)
          if (left && right) void savePair.save(stage, left, right)
        }}
      >
        {savePair.isPending ? '저장 중…' : 'A/B 조합 저장'}
      </Button>
    </div>
  )
}

function ModelSelect({
  label,
  value,
  models,
  onChange,
}: {
  label: string
  value: string
  models: ReturnType<typeof useModels>['models']
  onChange: (value: string) => void
}) {
  const id = `model-${label}`
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Select
        id={id}
        className="mt-1"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">모델을 선택하세요</option>
        {models.map((model) => (
          <option key={refKey(model.ref)} value={refKey(model.ref)} disabled={model.disabled}>
            {model.label}
            {model.disabled ? ` · ${model.disabledReason}` : ''}
          </option>
        ))}
      </Select>
      {value && <ModelMeta model={models.find((model) => refKey(model.ref) === value)} />}
    </div>
  )
}

function ModelMeta({
  model,
}: {
  model: ReturnType<typeof useModels>['models'][number] | undefined
}) {
  if (!model) return null
  const context = Number(model.contextTokens).toLocaleString()
  return (
    <p className="text-content-tertiary mt-1 text-xs">
      컨텍스트 {context} · 입력 ${model.inputUsdPerMillion || '?'} / 출력 $
      {model.outputUsdPerMillion || '?'} · 1M 토큰 기준 ≈ ·{' '}
      {model.pricingCheckedAt || '가격 미확인'}
    </p>
  )
}
