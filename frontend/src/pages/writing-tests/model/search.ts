import { safeInternalPath } from '@/shared/lib/navigation'
import type { TestCount, TestFactor, TestStage } from '@/entities/writing-test'

export interface WritingTestSearch {
  /** Present only when the address names the factor (새 비교's items): the defaulted factor stays
   *  out of the validated search, so the router's address normalization never writes one an
   *  entry did not name (MODEL-44). Read the effective factor with `writingTestFactor`. */
  factor?: TestFactor
  stage: TestStage
  count: TestCount
  source?: string
  voiceId?: string
  templateId?: string
  guidelineId?: string
  entry?: string
  draft?: string
}
export function writingTestSearchSchema(raw: Record<string, unknown>): WritingTestSearch {
  const named = raw.factor !== undefined && raw.factor !== ''
  const factor = named ? raw.factor : 'model',
    stage = raw.stage ?? 'write'
  if (
    typeof factor !== 'string' ||
    typeof stage !== 'string' ||
    (raw.count !== undefined && typeof raw.count !== 'number' && typeof raw.count !== 'string')
  )
    throw new Error('Unsupported writing test selection')
  const count = raw.count === undefined ? 2 : Number(raw.count)
  if (
    !['model', 'voice', 'template', 'guideline'].includes(factor) ||
    !['observe', 'write'].includes(stage) ||
    (factor !== 'model' && stage !== 'write') ||
    ![2, 4, 8, 16].includes(count)
  )
    throw new Error('Unsupported writing test selection')
  const text = (key: string) =>
    typeof raw[key] === 'string' && raw[key] !== '' ? (raw[key] as string) : undefined
  const source = text('source') ?? text('sourcePost') ?? text('sourcePostSlug') ?? text('postSlug')
  const entry = text('entry')
  return {
    ...(named ? { factor: factor as TestFactor } : {}),
    stage: stage as TestStage,
    count: count as TestCount,
    source,
    voiceId: text('voiceId'),
    templateId: text('templateId'),
    guidelineId: text('guidelineId'),
    entry: entry && safeInternalPath(entry) ? entry : undefined,
    draft: text('draft'),
  }
}
/** Whether the entry's address named the factor (새 비교's items, MODEL-44). */
export function writingTestFactorNamed(search: Pick<WritingTestSearch, 'factor'>): boolean {
  return search.factor !== undefined
}
/** The factor a setup starts on: the named one, else the model comparison. */
export function writingTestFactor(search: Pick<WritingTestSearch, 'factor'>): TestFactor {
  return search.factor ?? 'model'
}
export interface WritingTestHistorySearch {
  stage?: TestStage | 'voice'
  source?: string
  voiceId?: string
  entry?: string
}
export function writingTestHistorySearchSchema(
  raw: Record<string, unknown>,
): WritingTestHistorySearch {
  const text = (key: string) =>
    typeof raw[key] === 'string' && (raw[key] as string).trim() !== ''
      ? (raw[key] as string)
      : undefined
  return {
    stage:
      raw.stage === 'observe' || raw.stage === 'write' || raw.stage === 'voice'
        ? raw.stage
        : undefined,
    source: text('source') ?? text('sourcePost') ?? text('sourcePostSlug') ?? text('postSlug'),
    voiceId: text('voiceId'),
    ...(text('entry') && safeInternalPath(text('entry')!) ? { entry: text('entry') } : {}),
  }
}
export interface PhotoTestSearch {
  count: TestCount
  entry?: string
  /** One setup's key: a reload keeps it and resumes, a new entry has none and starts over. */
  draft?: string
}
export function photoTestSearchSchema(raw: Record<string, unknown>): PhotoTestSearch {
  const count = Number(raw.count ?? 2)
  const text = (key: string) =>
    typeof raw[key] === 'string' && raw[key] !== '' ? (raw[key] as string) : undefined
  const entry = text('entry')
  return {
    count: ([2, 4, 8, 16].includes(count) ? count : 2) as TestCount,
    ...(entry && safeInternalPath(entry) ? { entry } : {}),
    ...(text('draft') ? { draft: text('draft') } : {}),
  }
}
