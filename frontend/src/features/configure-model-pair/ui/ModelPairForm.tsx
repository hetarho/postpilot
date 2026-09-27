import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ModelSelect } from './ModelSelect'
import {
  filterForStage,
  refKey,
  type ModelRef,
  type StageName,
  useModels,
  useModelSetup,
  useSaveComparisonPair,
} from '@/entities/model-catalog'

/** The pair the two fields show, which may differ from the stored one while a change is
 *  incomplete, names one model twice, or is still being written. */
export interface ShownPair {
  stage: StageName
  a: string
  b: string
}

export function ModelPairForm({
  stage,
  onShownChange,
}: {
  stage: StageName
  /** Told what the fields show, so a start gate reads the pair the screen shows (MODEL-65). */
  onShownChange?: (shown: ShownPair) => void
}) {
  const { models } = useModels()
  const { pairs } = useModelSetup()
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
      savePair={savePair}
      onShownChange={onShownChange}
    />
  )
}

function ModelPairFields({
  stage,
  initialA,
  initialB,
  suitable,
  savePair,
  onShownChange,
}: {
  stage: StageName
  initialA: string
  initialB: string
  suitable: ReturnType<typeof useModels>['models']
  savePair: ReturnType<typeof useSaveComparisonPair>
  onShownChange?: (shown: ShownPair) => void
}) {
  const { t } = useTranslation('models')
  const [a, setA] = useState(initialA)
  const [b, setB] = useState(initialB)
  // Which side was changed last, so a refusal sits under the control that caused it instead
  // of under both.
  const [changed, setChanged] = useState<'a' | 'b' | ''>('')
  // Why the last change wrote nothing, on the field that made it (MODEL-65).
  const [refused, setRefused] = useState<{ side: 'a' | 'b'; reason: 'incomplete' | 'same' }>()
  useEffect(() => {
    onShownChange?.({ stage, a, b })
  }, [onShownChange, stage, a, b])
  const notice = refused
    ? t(refused.reason === 'same' ? 'differentModels' : 'pairIncomplete')
    : undefined
  const find = (key: string): ModelRef | undefined =>
    suitable.find((model) => refKey(model.ref) === key)?.ref

  /** The pair is written as it is chosen (MODEL-65). A change that leaves it incomplete, or
   *  that names the model the other side already holds, writes nothing: the server would
   *  refuse the second one anyway (MODEL-25), and refusing it here keeps the stored pair as
   *  it was instead of clearing it. */
  const commit = (side: 'a' | 'b', nextKey: string) => {
    const keys = side === 'a' ? [nextKey, b] : [a, nextKey]
    if (side === 'a') setA(nextKey)
    else setB(nextKey)
    const [left, right] = [find(keys[0]), find(keys[1])]
    if (!left || !right || keys[0] === keys[1]) {
      // Nothing was written, so an earlier refusal has nothing to point at any more. The
      // mutation's own failure outlives its mutation; forgetting which field it belonged to
      // is what takes it off the screen. What this change did not do is said instead.
      setChanged('')
      setRefused({ side, reason: left && right ? 'same' : 'incomplete' })
      return
    }
    setRefused(undefined)
    setChanged(side)
    void savePair.save(stage, left, right).catch(() => {
      // The mutation state carries the structured failure, rendered on the changed field.
    })
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <ModelSelect
          label={t('candidateA')}
          stage={stage}
          value={a}
          models={suitable}
          onChange={(key) => commit('a', key)}
          // Both sides are one row, so neither may move while that row is being written
          // (MODEL-23).
          saving={savePair.isPending}
          error={changed === 'a' ? savePair.failure : undefined}
          notice={refused?.side === 'a' ? notice : undefined}
        />
        <ModelSelect
          label={t('candidateB')}
          stage={stage}
          value={b}
          models={suitable}
          onChange={(key) => commit('b', key)}
          saving={savePair.isPending}
          error={changed === 'b' ? savePair.failure : undefined}
          notice={refused?.side === 'b' ? notice : undefined}
        />
      </div>
    </div>
  )
}
