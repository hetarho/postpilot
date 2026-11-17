import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import {
  filterForStage,
  levelOf,
  orderModelsForStage,
  postCreditLabel,
  refKey,
  stagePostFigure,
  type CatalogModel,
  type ModelRef,
  type StageName,
} from '@/entities/model-catalog'
import { Badge, Checkbox, Typography, cardStyles, typographyStyles } from '@/shared/ui'
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

/** One list of the stage's models in which the owner checks exactly the format's count; each row
 *  carries its grade and credits per post (MODEL-98 →QUOTA-64). Order follows the stage picker
 *  (MODEL-96). A locked or incompatible model stays as a disabled row stating its reason. */
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
  const { t: tModels } = useTranslation('models')
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
      <div className="flex flex-col gap-3">
        {rows.map((model) => {
          const key = refKey(model.ref)
          const reason = issue(model)
          const checked = checkedKeys.has(key)
          const level = levelOf(model, stage)
          const figure = stagePostFigure(model, stage)
          const free = model.access?.[stage]?.grade === 'free'
          const cost = free ? t('free') : figure ? postCreditLabel(figure) : t('priceUnknown')
          const locked = !!reason || !!disabled
          return (
            <label
              key={key}
              className={cardStyles({
                padding: 'sm',
                interactive: !locked,
                className: clsx(
                  'flex w-full items-center gap-3',
                  locked ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
                  checked && 'ring-interaction-focus ring-2',
                ),
              })}
            >
              <Checkbox
                checked={checked}
                disabled={locked && !checked}
                aria-labelledby={`${baseId}-${key}-name`}
                aria-describedby={`${baseId}-${key}-meta`}
                onChange={() => !disabled && toggle(model)}
              />
              <span className="flex min-w-0 flex-1 flex-col gap-1">
                <span className="flex flex-wrap items-center gap-2">
                  {level && <Badge tone="accent">{tModels(`level.${level}`)}</Badge>}
                </span>
                <span
                  id={`${baseId}-${key}-name`}
                  className={typographyStyles({ variant: 'fieldTitle', className: 'break-words' })}
                >
                  {model.label}
                </span>
                <span
                  id={`${baseId}-${key}-meta`}
                  className={typographyStyles({ variant: 'meta' })}
                >
                  {reason || cost}
                </span>
              </span>
            </label>
          )
        })}
      </div>
      <Typography variant="meta" as="p" role="status">
        {limitHit && full ? t('checkLimit', { count }) : ''}
      </Typography>
    </div>
  )
}
