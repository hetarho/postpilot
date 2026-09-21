import type { StageName } from '@/entities/model-catalog'

export const searchSchema = (search: Record<string, unknown>): { stage?: StageName } => ({
  stage:
    search.stage === 'observe' || search.stage === 'analyze' || search.stage === 'write'
      ? search.stage
      : undefined,
})
