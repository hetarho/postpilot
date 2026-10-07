import { useTranslation } from 'react-i18next'
import {
  refKey,
  useSaveSelection,
  useStageSelection,
  useModels,
  useSelections,
  type StageName,
} from '@/entities/model-catalog'
import { ModelSelect } from './ModelSelect'
import { Button, Notice } from '@/shared/ui'

export function ActiveModelForm({ stage }: { stage: StageName }) {
  const { t } = useTranslation('models')
  const active = useStageSelection(stage)
  const save = useSaveSelection()
  const models = useModels()
  const selections = useSelections()
  const failed = models.isError || selections.isError
  const reference = active.selected ?? active.unavailable?.ref
  const name = reference
    ? (active.models.find((model) => refKey(model.ref) === refKey(reference))?.label ??
      refKey(reference))
    : ''
  const stageName = t(`stage.${stage}`)
  const savingReference = save.variables?.ref
    ? { providerId: save.variables.ref.providerId ?? '', modelId: save.variables.ref.modelId ?? '' }
    : undefined
  const savingName = savingReference
    ? (active.models.find((model) => refKey(model.ref) === refKey(savingReference))?.label ??
      refKey(savingReference))
    : name
  const status = failed
    ? ''
    : active.isPending
      ? t('activeState.loading', { stage: stageName })
      : save.isPending
        ? t('activeState.saving', { stage: stageName, name: savingName })
        : active.unavailable
          ? t('activeState.unavailable', {
              stage: stageName,
              name,
              reason: active.unavailable.reason,
            })
          : active.selected
            ? t(save.isSuccess ? 'activeState.saved' : 'activeState.usable', {
                stage: stageName,
                name,
              })
            : t('activeState.empty', { stage: stageName })
  return (
    <div>
      <ModelSelect
        label={t('selectField.label', { stage: t(`selectField.stage.${stage}`), optional: '' })}
        stage={stage}
        value={reference ? refKey(reference) : ''}
        models={active.models}
        onChange={(key) => {
          const model = active.models.find((candidate) => refKey(candidate.ref) === key)
          if (model) void save.save(stage, model.ref)
        }}
        saving={save.isPending}
        error={save.failure}
        savedIssue={active.unavailable?.reason}
        disabled={active.isPending || failed}
        status={status}
      />
      {failed && (
        <Notice tone="danger" role="alert" className="mt-3">
          {t('activeState.failed', { stage: stageName })}
          <Button
            variant="ghost"
            className="mt-3"
            onClick={() => {
              models.refetch()
              selections.refetch()
            }}
          >
            {t('activeState.retry')}
          </Button>
        </Notice>
      )}
    </div>
  )
}
