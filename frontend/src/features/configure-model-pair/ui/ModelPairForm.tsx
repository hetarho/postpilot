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
import { AppFailureMessage, Button, FieldMessage, Typography } from '@/shared/ui'

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
  const find = (key: string): ModelRef | undefined =>
    suitable.find((model) => refKey(model.ref) === key)?.ref
  const invalid = !a || !b || a === b || !find(a) || !find(b)
  return (
    <div className="space-y-4">
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <ModelSelect
          label={t('candidateA')}
          stage={stage}
          value={a}
          models={suitable}
          onChange={setA}
        />
        <ModelSelect
          label={t('candidateB')}
          stage={stage}
          value={b}
          models={suitable}
          onChange={setB}
        />
      </div>
      {a && a === b && <FieldMessage>{t('differentModels')}</FieldMessage>}
      <div>
        <Button
          variant="secondary"
          className="w-full sm:w-auto"
          disabled={invalid}
          pending={savePair.isPending}
          onClick={() => {
            const left = find(a)
            const right = find(b)
            if (left && right) {
              void savePair.save(stage, left, right).catch(() => {
                // The mutation state carries the structured failure rendered below.
              })
            }
          }}
        >
          {t('savePair')}
        </Button>
        {/* The comparison uses the saved pair, so its save result belongs beside this action. */}
        {savePair.failure && (
          <Typography
            variant="body"
            as="div"
            role="alert"
            className="text-field-error mt-2 break-words"
          >
            <AppFailureMessage failure={savePair.failure} />
          </Typography>
        )}
        {savePair.isSuccess && (
          <Typography variant="body" role="status" className="text-content-secondary mt-2">
            {t('pair.saved')}
          </Typography>
        )}
      </div>
    </div>
  )
}
