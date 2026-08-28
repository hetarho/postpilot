/** The three places a model is chosen ([I3]); the app never fills one in. */
export type StageName = 'observe' | 'write' | 'analyze'

export const STAGES: readonly StageName[] = ['observe', 'write', 'analyze']

export const STAGE_LABELS: Record<StageName, string> = {
  observe: '관찰',
  write: '작성',
  analyze: '문체 분석',
}

/** One model of one provider — what a job records and what a dropdown saves. */
export interface ModelRef {
  providerId: string
  modelId: string
}

/** A registry entry as the catalog exposes it: ids, label and flags. Never a key. */
export interface CatalogModel {
  ref: ModelRef
  label: string
  vision: boolean
  structuredOutput: boolean
  disabled: boolean
  disabledReason: string
}

/** The acting user's saved choice for a stage. `missing`: the model is no longer
 *  registered — the server has already cleared the row; this is shown once. */
export interface StageSelection {
  stage: StageName
  ref: ModelRef
  missing: boolean
}

export function refKey(ref: ModelRef): string {
  return `${ref.providerId}/${ref.modelId}`
}

export function sameRef(a: ModelRef, b: ModelRef): boolean {
  return a.providerId === b.providerId && a.modelId === b.modelId
}

/** The observe stage looks at photos, so it lists vision models only (PRD §6.4); the
 *  other stages list everything. Disabled models stay in the list — greyed, with the
 *  reason — rather than vanishing, so the user learns why a model is unavailable. */
export function filterForStage(models: readonly CatalogModel[], stage: StageName): CatalogModel[] {
  return stage === 'observe' ? models.filter((model) => model.vision) : [...models]
}
