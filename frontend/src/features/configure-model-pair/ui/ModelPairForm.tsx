import { useState } from 'react'
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
import { FieldMessage } from '@/shared/ui'

export function ModelPairForm({ stage }: { stage: StageName }) {
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
    />
  )
}

function ModelPairFields({
  stage,
  initialA,
  initialB,
  suitable,
  savePair,
}: {
  stage: StageName
  initialA: string
  initialB: string
  suitable: ReturnType<typeof useModels>['models']
  savePair: ReturnType<typeof useSaveComparisonPair>
}) {
  const { t } = useTranslation('models')
  const [a, setA] = useState(initialA)
  const [b, setB] = useState(initialB)
  // Which side was changed last, so a refusal sits under the control that caused it instead
  // of under both.
  const [changed, setChanged] = useState<'a' | 'b' | ''>('')
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
      // is what takes it off the screen.
      setChanged('')
      return
    }
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
        />
        <ModelSelect
          label={t('candidateB')}
          stage={stage}
          value={b}
          models={suitable}
          onChange={(key) => commit('b', key)}
          saving={savePair.isPending}
          error={changed === 'b' ? savePair.failure : undefined}
        />
      </div>
      {a && a === b && <FieldMessage>{t('differentModels')}</FieldMessage>}
    </div>
  )
}
