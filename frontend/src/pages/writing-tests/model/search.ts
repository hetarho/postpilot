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
  const source = text('source') ?? text('sourcePost') ?? text('sourcePostSlug')
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
export const writingTestHistorySearchSchema = (raw: Record<string, unknown>) => ({
  voiceId: typeof raw.voiceId === 'string' ? raw.voiceId : undefined,
})
