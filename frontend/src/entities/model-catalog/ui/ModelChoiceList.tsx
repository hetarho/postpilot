import { useId, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Check } from 'lucide-react'
import { Checkbox, ListGroup, SurfaceButton, listRowStyles, typographyStyles } from '@/shared/ui'
import { levelOf, levelPrefix, type LevelName } from '../model/level'
import { postCreditLabel, stagePostFigure } from '../model/post-credits'
import { refKey, type CatalogModel, type StageName } from '../model/types'

export interface ModelChoiceListProps {
  stage: StageName
  /** The rows, already in the order the caller shows them. */
  models: readonly CatalogModel[]
  /** `radio`: a tap picks the one model (the stage picker, MODEL-96). `checkbox`: several are
   *  checked (a test's contestants, MODEL-98). */
  mode: 'radio' | 'checkbox'
  checked: (model: CatalogModel) => boolean
  /** Why this model cannot be picked, empty when it can. The row stays listed, disabled, saying so. */
  issue: (model: CatalogModel) => string
  onPick: (model: CatalogModel) => void
  /** Holds every row, e.g. while a pick is being saved. */
  disabled?: boolean
  /** `true` sets the rows under their grade's heading, one heading per run of a grade, for an order
   *  that keeps each grade together. `false` lists them under no heading, each naming its grade. */
  grouped?: boolean
  /** A mark beside a row's name, such as 사용 중. */
  mark?: (model: CatalogModel) => ReactNode
  /** The grade headings' level under the caller's own section heading. */
  headingLevel?: 2 | 3 | 4
}

interface GradeRun {
  level: LevelName | undefined
  models: CatalogModel[]
}

/** Consecutive rows of one grade share a heading; the caller's order is never changed. */
function gradeRuns(
  models: readonly CatalogModel[],
  stage: StageName,
  grouped: boolean,
): GradeRun[] {
  if (!grouped) return [{ level: undefined, models: [...models] }]
  const runs: GradeRun[] = []
  for (const model of models) {
    const level = levelOf(model, stage)
    const last = runs.at(-1)
    if (last && last.level === level) last.models.push(model)
    else runs.push({ level, models: [model] })
  }
  return runs
}

/** Guide: the one list a stage's models are chosen from — compact rows under grade headings
 *  (무료, 가성비 … 최고, then 등급 미정), each row the model's name and what one post costs at this
 *  stage (MODEL-96 →MODEL-98 →QUOTA-64). The stage picker and a test's contestant checklist both
 *  draw it; each keeps its own ordering, count and save. */
export function ModelChoiceList({
  stage,
  models,
  mode,
  checked,
  issue,
  onPick,
  disabled = false,
  grouped = true,
  mark,
  headingLevel = 3,
}: ModelChoiceListProps) {
  const { t } = useTranslation('models')
  const baseId = useId()
  const runs = gradeRuns(models, stage, grouped)
  return (
    <div className="flex flex-col gap-4">
      {runs.map((run, runIndex) => (
        <ListGroup
          key={`${run.level ?? 'ungraded'}-${runIndex}`}
          heading={
            grouped ? (run.level ? t(`level.${run.level}`) : t('choice.ungraded')) : undefined
          }
          headingLevel={headingLevel}
        >
          {run.models.map((model, rowIndex) => {
            const key = refKey(model.ref)
            const reason = issue(model)
            const isChecked = checked(model)
            const figure = stagePostFigure(model, stage)
            const cost =
              model.access?.[stage]?.grade === 'free'
                ? t('choice.free')
                : figure
                  ? postCreditLabel(figure)
                  : t('choice.priceUnknown')
            const meta = reason || `${grouped ? '' : levelPrefix(model, stage)}${cost}`
            // By position, not by model key: a key may hold a space, which would split an id list.
            const nameId = `${baseId}-${runIndex}-${rowIndex}-name`
            const metaId = `${baseId}-${runIndex}-${rowIndex}-meta`
            const text = (
              <span className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
                <span className="flex min-w-0 items-center gap-2">
                  <span id={nameId} className="min-w-0 break-words">
                    {model.label}
                  </span>
                  {mark?.(model)}
                </span>
                <span id={metaId} className={typographyStyles({ variant: 'meta' })}>
                  {meta}
                </span>
              </span>
            )
            if (mode === 'checkbox') {
              const locked = !!reason || disabled
              return (
                <li key={key}>
                  <label
                    className={listRowStyles({
                      interactive: !locked,
                      className: clsx(
                        'min-h-12 py-2',
                        locked ? 'cursor-not-allowed opacity-60' : 'cursor-pointer',
                      ),
                    })}
                  >
                    <Checkbox
                      checked={isChecked}
                      disabled={locked && !isChecked}
                      aria-labelledby={nameId}
                      aria-describedby={metaId}
                      onChange={() => !disabled && onPick(model)}
                    />
                    {text}
                  </label>
                </li>
              )
            }
            return (
              <li key={key}>
                <SurfaceButton
                  presentation="row"
                  role="radio"
                  aria-checked={isChecked}
                  aria-labelledby={`${nameId} ${metaId}`}
                  disabled={!!reason || disabled}
                  onClick={() => onPick(model)}
                  className={listRowStyles({
                    interactive: !reason,
                    className: 'min-h-12 rounded-none py-2 disabled:cursor-not-allowed',
                  })}
                >
                  {text}
                  <span
                    aria-hidden="true"
                    className={clsx(
                      'flex size-6 shrink-0 items-center justify-center rounded-full',
                      isChecked ? 'bg-button-cta-bg text-button-cta-fg' : 'bg-progress-step-todo',
                    )}
                  >
                    {isChecked && <Check className="size-4" />}
                  </span>
                </SurfaceButton>
              </li>
            )
          })}
        </ListGroup>
      ))}
    </div>
  )
}
