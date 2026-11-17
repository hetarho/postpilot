import { useId, useState } from 'react'
import {
  ModelChoiceList,
  filterForStage,
  orderModelsForStage,
  refKey,
  type CatalogModel,
  type ModelRef,
  type StageName,
} from '@/entities/model-catalog'
import { Typography } from '@/shared/ui'
import { useWritingTestTranslation } from '../model/useWritingTestTranslation'

export interface ModelContestantListProps {
  stage: Extract<StageName, 'write' | 'observe'>
  /** How many models the format asks for: A/B 2, then 4, 8 and 16. */
  count: number
  models: readonly CatalogModel[]
  /** The checked refs, in the order they were checked. */
  selected: readonly ModelRef[]
  onChange: (next: ModelRef[]) => void
  /** The caller's reason a model cannot be a contestant, empty when it can (MODEL-98). */
  issue: (model: CatalogModel) => string
  disabled?: boolean
  /** Lets a refused start focus the list. */
  id?: string
}

/** One list of the stage's models in which the owner checks exactly the format's count: the stage
 *  picker's own compact rows under grade headings, each with its credits per post (MODEL-98
 *  →MODEL-96 →QUOTA-64), in the picker's order. A locked or incompatible model stays as a disabled
 *  row stating its reason. */
export function ModelContestantList({
  stage,
  count,
  models,
  selected,
  onChange,
  issue,
  disabled,
  id,
}: ModelContestantListProps) {
  const { t } = useWritingTestTranslation()
  const [limitHit, setLimitHit] = useState(false)
  const baseId = useId()
  const rows = orderModelsForStage(filterForStage(models, stage), stage)
  const checkedKeys = new Set(selected.map(refKey))
  const full = selected.length >= count

  const toggle = (model: CatalogModel) => {
    const key = refKey(model.ref)
    if (checkedKeys.has(key)) {
      setLimitHit(false)
      onChange(selected.filter((ref) => refKey(ref) !== key))
    } else if (full) {
      setLimitHit(true)
    } else {
      setLimitHit(false)
      onChange([...selected, model.ref])
    }
  }

  return (
    <div
      id={id}
      tabIndex={-1}
      role="group"
      aria-labelledby={`${baseId}-count`}
      className="flex flex-col gap-3 outline-none"
    >
      <Typography variant="fieldTitle" as="p" id={`${baseId}-count`} role="status">
        {t('checkedCount', { checked: selected.length, count })}
      </Typography>
      <ModelChoiceList
        stage={stage}
        models={rows}
        mode="checkbox"
        checked={(model) => checkedKeys.has(refKey(model.ref))}
        issue={issue}
        onPick={toggle}
        disabled={disabled}
      />
      <Typography variant="meta" as="p" role="status">
        {limitHit && full ? t('checkLimit', { count }) : ''}
      </Typography>
    </div>
  )
}
