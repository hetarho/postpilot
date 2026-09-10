import i18next from 'i18next'
import type { CatalogModel, StageName } from './types'

/** The operator's user-facing grade for one model AT ONE STAGE (MODEL-57). Four words that
 *  say which of the offered models is the cheap one and which is the good one, so a user
 *  does not have to read prices to choose.
 *
 *  Display and ordering only — it gates nothing, and an ungraded model is exactly as
 *  selectable as a graded one (MODEL-58). */
export type LevelName = 'value' | 'balanced' | 'premium' | 'top'

/** ASCENDING — 가성비 · 밸런스 · 고급 · 최고. Every list that orders by level orders by this
 *  array, so the picker and the operator's own tab cannot disagree about what "higher" is. */
export const LEVELS: readonly LevelName[] = ['value', 'balanced', 'premium', 'top']

export function isLevelName(value: string): value is LevelName {
  return (LEVELS as readonly string[]).includes(value)
}

/** The grade this model carries for this stage, or undefined when it has none. */
export function levelOf(model: CatalogModel, stage: StageName): LevelName | undefined {
  return model.levels[stage]
}

/** Orders a stage's models 가성비 → 최고, ungraded last.
 *
 *  Cheapest first because that is the choice a user with credits to spend should meet by
 *  default; the ungraded tail is the operator's backlog, not a judgement about the models
 *  in it. The sort is STABLE, so inside one grade the server's order survives.
 *
 *  Pure and shared rather than done per component: two features and three fields render
 *  this same list, and a picker that ordered differently from its neighbour would read as
 *  a bug in the catalog. */
export function orderModelsForStage(
  models: readonly CatalogModel[],
  stage: StageName,
): CatalogModel[] {
  return [...models].sort((a, b) => rank(a, stage) - rank(b, stage))
}

function rank(model: CatalogModel, stage: StageName): number {
  const level = model.levels[stage]
  // An absent grade sorts after every present one — LEVELS.length is past the last rank.
  return level === undefined ? LEVELS.length : LEVELS.indexOf(level)
}

/** The option-label prefix for a model at a stage: `"가성비 · "`, or `""` when ungraded.
 *
 *  The grade leads the label rather than trailing it. `ListboxOption.label` is a string and
 *  the CLOSED trigger truncates — at 360px it is ~284px wide, which a `provider/model` path
 *  alone already fills — so a trailing grade is the part the user never sees. Leading it
 *  also means the closed field reads as the answer to "which one is this", which is the
 *  question the grade exists to answer.
 *
 *  One place rather than three: the two pair fields and the stage selector build their
 *  labels separately, and the copy decision belongs to the domain, not to each of them. */
export function levelPrefix(model: CatalogModel, stage: StageName): string {
  const level = model.levels[stage]
  return level ? `${i18next.t(`level.${level}`, { ns: 'models' })} · ` : ''
}
