import { useTranslation } from 'react-i18next'
import {
  refKey,
  useSaveSelection,
  useStageSelection,
  type StageName,
} from '@/entities/model-catalog'
import { ModelSelect } from './ModelSelect'

export function ActiveModelForm({ stage }: { stage: StageName }) {
  const { t } = useTranslation('models')
  const active = useStageSelection(stage)
  const save = useSaveSelection()
  return (
    <ModelSelect
      label={t('selectField.label', { stage: t(`selectField.stage.${stage}`), optional: '' })}
      stage={stage}
      value={active.selected ? refKey(active.selected) : ''}
      models={active.models}
      onChange={(key) => {
        const model = active.models.find((candidate) => refKey(candidate.ref) === key)
        if (model) void save.save(stage, model.ref)
      }}
      saving={save.isPending}
      error={save.failure}
    />
  )
}
