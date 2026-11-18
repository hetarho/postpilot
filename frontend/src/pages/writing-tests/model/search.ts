import { safeInternalPath } from '@/shared/lib/navigation'
import type { TestCount, TestFactor, TestStage } from '@/entities/writing-test'

export interface WritingTestSearch {
  factor: TestFactor
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
  const factor = raw.factor ?? 'model',
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
    factor: factor as TestFactor,
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
/** Whether the address itself names the factor (새 비교's items, MODEL-44): read it from the
 *  address as written, since the validated search always carries a defaulted factor. */
export function writingTestFactorNamed(raw: Record<string, unknown>): boolean {
  return typeof raw.factor === 'string' && raw.factor !== ''
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
