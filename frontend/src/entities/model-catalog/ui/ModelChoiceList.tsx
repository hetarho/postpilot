import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ChoiceRow, ListGroup } from '@/shared/ui'
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
  /** The row whose pick is being saved, which shows it in place of its mark. */
  pending?: (model: CatalogModel) => boolean
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
  pending,
  headingLevel = 3,
}: ModelChoiceListProps) {
  const { t } = useTranslation('models')
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
          {run.models.map((model) => {
            const reason = issue(model)
            const figure = stagePostFigure(model, stage)
            const cost =
              model.access?.[stage]?.grade === 'free'
                ? t('choice.free')
                : figure
                  ? postCreditLabel(figure)
                  : t('choice.priceUnknown')
            return (
              <li key={refKey(model.ref)}>
                <ChoiceRow
                  mode={mode}
                  checked={checked(model)}
                  name={model.label}
                  mark={mark?.(model)}
                  meta={reason || `${grouped ? '' : levelPrefix(model, stage)}${cost}`}
                  disabled={!!reason || disabled}
                  pending={pending?.(model)}
                  // The list's hold stops every pick, a checked checkbox row's too: that row stays
                  // enabled only so it still reads as on.
                  onPick={() => !disabled && onPick(model)}
                />
              </li>
            )
          })}
        </ListGroup>
      ))}
    </div>
  )
}
